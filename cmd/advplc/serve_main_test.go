package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/vm"
)

// projetoComTela monta um "RPO" de dois fontes: a tela num, uma função
// e um Static de arquivo no outro (como um FwCallApp + REST do Protheus).
func projetoComTela(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	tela := "User Function Tela()\n\tConOut(\"tela:\" + U_Saudacao())\nReturn\n"
	lib := "Static cPrefixo := \"ola\"\nUser Function Saudacao()\nReturn cPrefixo + \" rpo\"\n"
	if err := os.WriteFile(filepath.Join(dir, "tela.prw"), []byte(tela), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lib.prw"), []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestExecutaProgramaComMain(t *testing.T) {
	bc, err := compilaDiretorio(projetoComTela(t), &Options{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	v := vm.NewVM(bc, false)
	v.SetOutputWriter(&out)
	if err := executaPrograma(v, "U_Tela"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "tela:ola rpo") {
		t.Fatalf("saída = %q", out.String())
	}
}

func TestExecutaProgramaSemMainNaoRodaTela(t *testing.T) {
	bc, err := compilaDiretorio(projetoComTela(t), &Options{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	v := vm.NewVM(bc, false)
	v.SetOutputWriter(&out)
	if err := executaPrograma(v, ""); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "tela:") {
		t.Fatalf("sem --main a tela não deveria rodar: %q", out.String())
	}
}

func TestExecutaProgramaMainInexistente(t *testing.T) {
	bc, err := compilaDiretorio(projetoComTela(t), &Options{})
	if err != nil {
		t.Fatal(err)
	}
	v := vm.NewVM(bc, false)
	v.SetOutputWriter(&bytes.Buffer{})
	err = executaPrograma(v, "U_NaoExiste")
	if err == nil || !strings.Contains(err.Error(), "U_NaoExiste") {
		t.Fatalf("erro = %v", err)
	}
}

func TestParseOptionsMain(t *testing.T) {
	if o := parseOptions([]string{"--main", "U_Tela"}); o.main != "U_Tela" {
		t.Fatalf("main = %q", o.main)
	}
}
