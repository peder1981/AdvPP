package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
