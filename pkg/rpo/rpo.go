// Package rpo lê e decompõe a estrutura de container do RPO (Repositório de
// Programas Objeto) do TOTVS Protheus.
//
// O layout abaixo foi confirmado empiricamente (não é spec oficial da
// TOTVS, que não é pública) via análise dinâmica: gdb anexado ao
// appsrvlinux real durante compilações controladas, interceptando write()/
// lseek() no arquivo custom.rpo, cruzado contra RPOs de produção reais de
// tamanhos bem diferentes (21KB a 379MB). Ver docs/rpo-format.md no
// repositório para o histórico completo da investigação.
//
// O que ESTÁ confirmado (mesmo em arquivos de produção não controlados):
//   - cabeçalho: campo de 4 bytes little-endian que é um PONTEIRO — o
//     offset, dentro do próprio arquivo, de um bloco específico. É escrito
//     como placeholder no início da gravação e sobrescrito (via seek de
//     volta à posição 0) como a ÚLTIMA operação antes de fechar o arquivo.
//   - logo após, um bloco fixo de 34 bytes: nome do RPO em ASCII
//     null-padded (16 bytes), 4 bytes zero, sentinela (valor variável),
//     mais bytes finais (geralmente zeros, mas podem conter dados).
//   - um footer fixo de 34 bytes no final do arquivo: magic ASCII de 10
//     bytes ("APNSRM0419" ou uma de mais 4 variantes de versão encontradas
//     literalmente no binário do appserver: 0418/0420/0421/0503), seguido
//     de 24 bytes de conteúdo não decifrado (checksum/hash/assinatura).
//
// Sentinelas observadas em produção:
//   - 0xFFFFFFFF : custom.rpo (magic APNSRM0419)
//   - 0xFFFFFF00 : tttm120.rpo (magic APNSRM0421)
//   - 0x0000FFFF : tlpp.rpo (magic APNSRM0420)
//
// O que NÃO está decifrado: o conteúdo entre o cabeçalho e o footer é
// opaco — contém o P-Code compilado e estruturas internas do appserver
// (mapas "siga*.map" identificados por engenharia reversa dinâmica, mas
// cujo formato de bytes exato não foi decodificado). Recompilar o MESMO
// fonte duas vezes produz bytes completamente diferentes nessa região
// (tamanho incluso), então o miolo é tratado como blob opaco: preservado
// byte a byte em leitura+regravação, nunca interpretado.
package rpo

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	// HeaderPointerSize é o tamanho do campo de offset no início do arquivo.
	HeaderPointerSize = 4
	// NameBlockSize é o tamanho do bloco fixo de nome+sentinela logo após o ponteiro.
	NameBlockSize = 34
	// HeaderSize é o tamanho total da região de cabeçalho confirmada.
	HeaderSize = HeaderPointerSize + NameBlockSize
	// FooterSize é o tamanho do trailer fixo no final do arquivo.
	FooterSize = 34
	// FooterMagicSize é o tamanho do magic ASCII dentro do footer.
	FooterMagicSize = 10

	nameFieldSize  = 16
	sentinelOffset = 4 + nameFieldSize + 4 // dentro do NameBlock: 4 zeros antes da sentinela
	sentinelSize   = 4
)

// KnownSentinels são os valores de sentinela observados em RPOs de produção.
// Cada build do appserver pode usar uma sentinela diferente.
var KnownSentinels = []uint32{
	0xFFFFFFFF, // custom.rpo (APNSRM0419)
	0xFFFFFF00, // tttm120.rpo (APNSRM0421)
	0x0000FFFF, // tlpp.rpo (APNSRM0420)
}

// KnownFooterMagics são os valores de magic de footer confirmados via
// strings literais embutidas no binário libaplinux.so (appserver real,
// build 7.00.210324P) — não é uma lista inventada, é o que existe de fato
// no binário como tabela de versões de formato suportadas.
var KnownFooterMagics = []string{
	"APNSRM0418",
	"APNSRM0419",
	"APNSRM0420",
	"APNSRM0421",
	"APNSRM0503",
}

// File é a decomposição em 4 partes de um RPO, nos limites estruturais
// confirmados. AdminSection e Body são blobs opacos — preservados, não
// interpretados semanticamente. O header completo (selfOffset + nameBlock)
// também é preservado para garantir round-trip byte-a-byte.
type File struct {
	Header       []byte // bytes brutos do cabeçalho (38 bytes): selfOffset + nameBlock
	Name         string // nome do RPO (ex.: "custom"), lido do bloco de nome
	SelfOffset   uint32 // valor do ponteiro de auto-referência no cabeçalho
	Sentinel     uint32 // valor da sentinela encontrado (para diagnóstico)
	AdminSection []byte // do fim do cabeçalho (offset 38) até SelfOffset — opaco
	Body         []byte // de SelfOffset até o início do footer — opaco
	FooterMagic  string // 10 bytes ASCII do footer (ex.: "APNSRM0419")
	FooterTrail  []byte // 24 bytes finais do footer — opaco (checksum/assinatura?)
}

// Parse decompõe um RPO nas regiões estruturalmente confirmadas.
// Não decodifica o conteúdo de AdminSection/Body/FooterTrail — apenas
// localiza os limites reais, verificados empiricamente contra compilações
// controladas e RPOs de produção de tamanhos variados (21KB a 379MB).
// O header completo é preservado para round-trip byte-a-byte.
func Parse(data []byte) (*File, error) {
	if len(data) < HeaderSize+FooterSize {
		return nil, fmt.Errorf("rpo: arquivo pequeno demais (%d bytes, mínimo %d)", len(data), HeaderSize+FooterSize)
	}

	// Preservar header bruto para round-trip perfeito
	header := append([]byte(nil), data[:HeaderSize]...)

	selfOffset := binary.LittleEndian.Uint32(data[0:4])

	nameBlock := data[HeaderPointerSize:HeaderSize]
	sentinel := binary.LittleEndian.Uint32(nameBlock[sentinelOffset-HeaderPointerSize : sentinelOffset-HeaderPointerSize+sentinelSize])
	if !isKnownSentinel(sentinel) {
		return nil, fmt.Errorf("rpo: sentinela do cabeçalho inválida (esperado um de %v, achado 0x%08X) — não parece um RPO Protheus", KnownSentinels, sentinel)
	}
	name := string(bytes.TrimRight(nameBlock[:nameFieldSize], "\x00"))

	footerStart := len(data) - FooterSize
	if int(selfOffset) < HeaderSize || int(selfOffset) >= footerStart {
		return nil, fmt.Errorf("rpo: ponteiro de auto-referência fora dos limites do arquivo (offset=%d, tamanho=%d)", selfOffset, len(data))
	}

	footerMagic := string(data[footerStart : footerStart+FooterMagicSize])
	if !isKnownMagic(footerMagic) {
		return nil, fmt.Errorf("rpo: magic de footer desconhecido (%q) — não é um dos %d formatos confirmados, ou arquivo corrompido/truncado", footerMagic, len(KnownFooterMagics))
	}

	return &File{
		Header:       header,
		Name:         name,
		SelfOffset:   selfOffset,
		Sentinel:     sentinel,
		AdminSection: append([]byte(nil), data[HeaderSize:selfOffset]...),
		Body:         append([]byte(nil), data[selfOffset:footerStart]...),
		FooterMagic:  footerMagic,
		FooterTrail:  append([]byte(nil), data[footerStart+FooterMagicSize:]...),
	}, nil
}

func isKnownSentinel(v uint32) bool {
	for _, s := range KnownSentinels {
		if v == s {
			return true
		}
	}
	return false
}

func isKnownMagic(s string) bool {
	for _, m := range KnownFooterMagics {
		if s == m {
			return true
		}
	}
	return false
}

// Bytes reconstrói o arquivo RPO original byte a byte a partir de seu header
// bruto preservado — round-trip sem perdas, já que AdminSection/Body/FooterTrail
// são preservados como blobs opacos (nunca reinterpretados).
func (f *File) Bytes() []byte {
	buf := make([]byte, 0, len(f.Header)+len(f.AdminSection)+len(f.Body)+FooterSize)
	buf = append(buf, f.Header...)
	buf = append(buf, f.AdminSection...)
	buf = append(buf, f.Body...)
	buf = append(buf, []byte(f.FooterMagic)...)
	buf = append(buf, f.FooterTrail...)
	return buf
}
