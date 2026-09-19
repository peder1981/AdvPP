// Package rpo provides parsing and extraction capabilities for TOTVS Protheus
// RPO (Repositório de Programas Objeto) files.
//
// # RPO Format
//
// O RPO usa criptografia em camadas — mecanismo confirmado ao vivo via gdb
// e por DESMONTAGEM REAL de tCryptoEVP::Encrypt (ver docs/rpo-format.md,
// Fases 8 e 15, e docs/rpo-format-sonnet.md). Não é RSA/PBKDF2 nem AES
// fixo como versões antigas desta documentação chegaram a especular:
//   - Body/AdminSection: cada recurso é comprimido com zlib (magic `78 9c`
//     confirmado no buffer de entrada de Encrypt()) e cifrado com uma
//     dentre ~12 CIFRAS LEGADAS DO OPENSSL escolhidas por chamada via uma
//     tabela de despacho indireta dentro de tCryptoEVP (DES, 3DES/DES-EDE,
//     RC4, RC5-32/12/16, CAST5, Blowfish, RC2 — ver cipher_dispatch.go),
//     todas derivando do MESMO par chave+IV de 16+8 bytes EFÊMEROS por
//     sessão de compilação (recompilar o mesmo RPO duas vezes gera chaves
//     diferentes — não há derivação a partir do nome/certificado)
//
// EncryptSegment/DecryptSegment (cipher_dispatch.go) implementam e
// verificam (contra dado real capturado ao vivo, não vetores inventados)
// 11 das ~12 cifras da tabela; IDEA aparece na tabela mas não tem
// implementação verificada ainda (ver idea.go).
//
// A criptografia torna impraticável a leitura offline direta do conteúdo
// SEM uma captura ao vivo prévia (chave/IV/cifra só existem em memória
// durante a compilação) — mas, DADA essa captura, a decodificação real é
// possível e está implementada e testada.
// Esta pacote oferece duas abordagens:
//
//  1. ParseContainer — lê a estrutura de container (cabeçalho, ponteiro, footer)
//     sem interpretar o conteúdo criptografado.
//
//  2. ExtractFromDump — carrega uma lista de funções extraída dinamicamente
//     (via gdb no appserver rodando) e fornece acesso programático.
//
// # Extração dinâmica
//
// Para extrair funções de um RPO real, use o script gdb em
// tools/rpo-live-inspect/extract_rpo.py:
//
//	cd /protheus12/bin/appserver
//	export LD_LIBRARY_PATH=.:$LD_LIBRARY_PATH
//	gdb -q -batch -x /tmp/extract_rpo.py ./appsrvlinux
//
// O script gera:
//   /tmp/rpo_extract.log.funcs  — nomes das funções, um por linha
//   /tmp/rpo_extract.log.json   — JSON com count, functions e all_apos
package rpo

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// normalizeFuncName corrige o prefixo duplicado "U_U_" (artefato conhecido
// do AdvPL: declarar `User Function U_XXX()` já com o prefixo "U_" no fonte
// faz o compilador prependar outro "U_", resultando em "U_U_XXX" no RPO)
// para "U_" apenas. Nomes com caminho de módulo (ex.: "CUSTOM.BLU.U_U_X")
// só têm o último segmento corrigido.
func normalizeFuncName(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[:i+1] + normalizeFuncName(name[i+1:])
	}
	if strings.HasPrefix(name, "U_U_") {
		return "U_" + name[len("U_U_"):]
	}
	return name
}

// ApoInfo contém metadados de um recurso (função, classe, recurso) no RPO.
type ApoInfo struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	FuncName string `json:"func_name,omitempty"` // nome puro da função (sem prefixo de módulo)
}

// ExtractResult é o resultado da extração de funções de um RPO.
type ExtractResult struct {
	Count     int       `json:"count"`
	Functions []string  `json:"functions"`
	Apos      []ApoInfo `json:"all_apos"`
}

// ParseExtractJSON carrega resultados de extração gerados por
// tools/rpo-live-inspect/extract_rpo.py.
func ParseExtractJSON(path string) (*ExtractResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rpo: read extract dump: %w", err)
	}
	var result ExtractResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("rpo: parse extract dump: %w", err)
	}
	for i, f := range result.Functions {
		result.Functions[i] = normalizeFuncName(f)
	}
	for i, a := range result.Apos {
		result.Apos[i].Name = normalizeFuncName(a.Name)
		result.Apos[i].FuncName = normalizeFuncName(a.FuncName)
	}
	return &result, nil
}

// ExtractFromLog carrega a lista de funções de um arquivo de log gerado
// pelo script de extração gdb. Formato: um nome por linha.
func ExtractFromLog(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rpo: read extract log: %w", err)
	}
	var funcs []string
	for _, line := range splitLines(string(data)) {
		line = trimSpace(line)
		if line != "" {
			funcs = append(funcs, normalizeFuncName(line))
		}
	}
	return funcs, nil
}

// ExtractFromRPO tenta carregar resultados de extração de um RPO.
// Se o arquivo .json existir, carrega ele; senão, tenta .funcs.
func ExtractFromRPO(rpoPath string) (*ExtractResult, error) {
	jsonPath := rpoPath + ".extract.json"
	funcsPath := rpoPath + ".extract.funcs"

	if _, err := os.Stat(jsonPath); err == nil {
		return ParseExtractJSON(jsonPath)
	}
	if _, err := os.Stat(funcsPath); err == nil {
		funcs, err := ExtractFromLog(funcsPath)
		if err != nil {
			return nil, err
		}
		return &ExtractResult{
			Count:     len(funcs),
			Functions: funcs,
			Apos:      nil,
		}, nil
	}
	return nil, fmt.Errorf("rpo: nenhum dump de extração encontrado para %s", rpoPath)
}

// FunctionCount retorna o número de funções no resultado.
func (r *ExtractResult) FunctionCount() int {
	if r == nil {
		return 0
	}
	return len(r.Functions)
}

// GetFunction retorna a função no índice especificado.
func (r *ExtractResult) GetFunction(i int) (string, bool) {
	if r == nil || i < 0 || i >= len(r.Functions) {
		return "", false
	}
	return r.Functions[i], true
}

// GetAllApos retorna todos os apoios (funções + classes + recursos).
func (r *ExtractResult) GetAllApos() []ApoInfo {
	if r == nil {
		return nil
	}
	return r.Apos
}

// HasFunction verifica se uma função existe na lista.
func (r *ExtractResult) HasFunction(name string) bool {
	if r == nil {
		return false
	}
	for _, f := range r.Functions {
		if f == name {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	j := len(s)
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}
