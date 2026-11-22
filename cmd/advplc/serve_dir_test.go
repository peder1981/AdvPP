package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
)

func escreveFonteProjeto(t *testing.T, dir, nome, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, nome), []byte(src), 0o644); err != nil {
		t.Fatalf("write %s: %v", nome, err)
	}
}

// compilaDiretorio carrega todos os .prw/.prx/.tlpp do diretório (sem
// recursão) num único bytecode — o "RPO" do `advplc serve <diretório>`.
func TestCompilaDiretorioDoisFontes(t *testing.T) {
	dir := t.TempDir()
	escreveFonteProjeto(t, dir, "a.prw", `
Static Function JsToAdvpl()
	Return "A"

User Function FA()
	Return JsToAdvpl()
`)
	escreveFonteProjeto(t, dir, "b.tlpp", `
User Function FB()
	Return "B"
`)

	bc, err := compilaDiretorio(dir, &Options{})
	if err != nil {
		t.Fatalf("compilaDiretorio: %v", err)
	}
	if _, ok := bc.Functions["FA"]; !ok {
		t.Fatal("função FA não compilada")
	}
	if _, ok := bc.Functions["FB"]; !ok {
		t.Fatal("função FB não compilada")
	}
	if _, ok := bc.Functions["STATIC:A.PRW:JSTOADVPL"]; !ok {
		t.Fatal("static de a.prw sem nome qualificado STATIC:A.PRW:JSTOADVPL")
	}
	// Corpo principal ignorado: MainOffset aponta para OP_HALT.
	if bc.Code[bc.MainOffset].Op != compiler.OP_HALT {
		t.Fatalf("Code[MainOffset].Op = %v, esperava OP_HALT", bc.Code[bc.MainOffset].Op)
	}
}

// User Function duplicada entre arquivos do diretório é erro citando os
// dois fontes.
func TestCompilaDiretorioUserFunctionDuplicada(t *testing.T) {
	dir := t.TempDir()
	escreveFonteProjeto(t, dir, "a.prw", "User Function UX()\n\tReturn 1\n")
	escreveFonteProjeto(t, dir, "b.prw", "User Function UX()\n\tReturn 2\n")

	_, err := compilaDiretorio(dir, &Options{})
	if err == nil {
		t.Fatal("esperava erro de User Function duplicada")
	}
	if !strings.Contains(err.Error(), "a.prw") || !strings.Contains(err.Error(), "b.prw") {
		t.Fatalf("erro %q não cita os dois arquivos", err)
	}
}

// Só .prw/.prx/.tlpp na raiz do diretório entram; subdiretórios e outras
// extensões são ignorados (não recursivo).
func TestCompilaDiretorioIgnoraSubdiretorioEOutrasExtensoes(t *testing.T) {
	dir := t.TempDir()
	escreveFonteProjeto(t, dir, "ok.prw", "User Function OKF()\n\tReturn 1\n")
	escreveFonteProjeto(t, dir, "notas.txt", "lixo")
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	escreveFonteProjeto(t, sub, "ignorado.prw", "User Function SUBF()\n\tReturn 1\n")

	bc, err := compilaDiretorio(dir, &Options{})
	if err != nil {
		t.Fatalf("compilaDiretorio: %v", err)
	}
	if _, ok := bc.Functions["OKF"]; !ok {
		t.Fatal("OKF não compilada")
	}
	if _, ok := bc.Functions["SUBF"]; ok {
		t.Fatal("arquivo de subdiretório não deveria entrar no projeto")
	}
}

func TestCompilaDiretorioVazioErro(t *testing.T) {
	if _, err := compilaDiretorio(t.TempDir(), &Options{}); err == nil {
		t.Fatal("esperava erro para diretório sem fontes")
	}
}

// #include é resolvido a partir do diretório do projeto: o conteúdo do
// .ch entra no programa que o inclui (a função vira global do projeto).
func TestCompilaDiretorioIncludeResolvidoPeloDiretorio(t *testing.T) {
	dir := t.TempDir()
	escreveFonteProjeto(t, dir, "a.prw", `#include "meu.ch"
User Function FA()
	Return FC()
`)
	escreveFonteProjeto(t, dir, "meu.ch", `User Function FC()
	Return "de dentro"
`)

	bc, deps, err := compilaDiretorioDeps(dir, &Options{})
	if err != nil {
		t.Fatalf("compilaDiretorioDeps: %v", err)
	}
	if _, ok := bc.Functions["FC"]; !ok {
		t.Fatal("função do .ch incluído não foi compilada")
	}
	achou := false
	for _, d := range deps {
		if strings.HasSuffix(d, "meu.ch") {
			achou = true
		}
	}
	if !achou {
		t.Fatalf("dependências %v não contêm meu.ch", deps)
	}
}

// A assinatura do diretório muda ao alterar um fonte .prw e também ao
// alterar um .ch/include resolvido — o --watch precisa recompilar nos
// dois casos.
func TestAssinaturaDiretorioMudaComFonteEInclude(t *testing.T) {
	dir := t.TempDir()
	escreveFonteProjeto(t, dir, "a.prw", `#include "meu.ch"
User Function FA()
	Return 1
`)
	escreveFonteProjeto(t, dir, "meu.ch", "// v1\n")

	_, deps, err := compilaDiretorioDeps(dir, &Options{})
	if err != nil {
		t.Fatalf("compilaDiretorioDeps: %v", err)
	}
	sig1, err := assinaturaFonte(dir, deps)
	if err != nil {
		t.Fatalf("assinaturaFonte: %v", err)
	}

	// Altera o fonte .prw.
	time.Sleep(5 * time.Millisecond)
	escreveFonteProjeto(t, dir, "a.prw", `#include "meu.ch"
User Function FA()
	Return 2
`)
	sig2, err := assinaturaFonte(dir, deps)
	if err != nil {
		t.Fatalf("assinaturaFonte: %v", err)
	}
	if sig2 == sig1 {
		t.Fatal("assinatura não mudou após alterar a.prw")
	}

	// Altera só o .ch (dependência): a assinatura também tem que mudar.
	time.Sleep(5 * time.Millisecond)
	escreveFonteProjeto(t, dir, "meu.ch", "// v2 alterado\n")
	sig3, err := assinaturaFonte(dir, deps)
	if err != nil {
		t.Fatalf("assinaturaFonte: %v", err)
	}
	if sig3 == sig2 {
		t.Fatal("assinatura não mudou após alterar meu.ch")
	}
}

// Arquivo único também vigia seus #include: a assinatura muda quando o
// .ch muda.
func TestAssinaturaArquivoUnicoComInclude(t *testing.T) {
	dir := t.TempDir()
	arq := filepath.Join(dir, "a.prw")
	escreveFonteProjeto(t, dir, "a.prw", `#include "meu.ch"
User Function FA()
	Return 1
`)
	escreveFonteProjeto(t, dir, "meu.ch", "// v1\n")

	_, deps, err := compilaFonteDeps(arq, &Options{})
	if err != nil {
		t.Fatalf("compilaFonteDeps: %v", err)
	}
	sig1, err := assinaturaFonte(arq, deps)
	if err != nil {
		t.Fatalf("assinaturaFonte: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	escreveFonteProjeto(t, dir, "meu.ch", "// v2\n")
	sig2, err := assinaturaFonte(arq, deps)
	if err != nil {
		t.Fatalf("assinaturaFonte: %v", err)
	}
	if sig2 == sig1 {
		t.Fatal("assinatura do arquivo único não mudou após alterar o .ch")
	}
}

// O watch cobre #include que ainda NÃO existia: os caminhos candidatos
// entram na assinatura como "missing" e, quando o .ch é criado depois, a
// assinatura muda — e a recompilação passa a enxergar o conteúdo.
func TestAssinaturaDiretorioIncludeFuturo(t *testing.T) {
	dir := t.TempDir()
	escreveFonteProjeto(t, dir, "a.prw", `#include "depois.ch"
User Function FA()
	Return FC()
`)

	_, deps, err := compilaDiretorioDeps(dir, &Options{})
	if err != nil {
		t.Fatalf("compilaDiretorioDeps: %v", err)
	}
	sig1, err := assinaturaFonte(dir, deps)
	if err != nil {
		t.Fatalf("assinaturaFonte: %v", err)
	}

	// Cria o include depois da primeira compilação.
	time.Sleep(5 * time.Millisecond)
	escreveFonteProjeto(t, dir, "depois.ch", `User Function FC()
	Return "inc"
`)
	sig2, err := assinaturaFonte(dir, deps)
	if err != nil {
		t.Fatalf("assinaturaFonte: %v", err)
	}
	if sig2 == sig1 {
		t.Fatal("assinatura não mudou após criar o .ch referenciado")
	}

	// Recompilação enxerga o conteúdo do include criado.
	bc, _, err := compilaDiretorioDeps(dir, &Options{})
	if err != nil {
		t.Fatalf("compilaDiretorioDeps (2): %v", err)
	}
	if _, ok := bc.Functions["FC"]; !ok {
		t.Fatal("função do .ch criado depois não foi compilada")
	}
}

// `advplc serve <diretório>` usa o mesmo caminho de compilação do projeto;
// `advplc serve <arquivo>` continua compilando um fonte só.
func TestCompilaAlvoDiretorio(t *testing.T) {
	dir := t.TempDir()
	escreveFonteProjeto(t, dir, "a.prw", "User Function FA()\n\tReturn 1\n")

	bc, err := compilaAlvo(dir, &Options{})
	if err != nil {
		t.Fatalf("compilaAlvo(dir): %v", err)
	}
	if _, ok := bc.Functions["FA"]; !ok {
		t.Fatal("FA não compilada via compilaAlvo(dir)")
	}

	arq := filepath.Join(dir, "a.prw")
	bc2, err := compilaAlvo(arq, &Options{})
	if err != nil {
		t.Fatalf("compilaAlvo(file): %v", err)
	}
	if _, ok := bc2.Functions["FA"]; !ok {
		t.Fatal("FA não compilada via compilaAlvo(file)")
	}
}
