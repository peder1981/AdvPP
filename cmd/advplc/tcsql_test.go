package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestTCSqlFixture roda tests/tcsql_test.prw — exercita TCSqlExec/TCSqlQuery,
// a via de persistência real de SQL direto pra User Function. Ver CHANGELOG.
func TestTCSqlFixture(t *testing.T) {
	if testing.Short() {
		t.Skip("builda o binário; pulado com -short")
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	binName := "advplc"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(t.TempDir(), binName)
	build := exec.Command("go", "build", "-o", binPath, "./cmd/advplc")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	run := exec.Command(binPath, "run", "tests/tcsql_test.prw")
	run.Dir = repoRoot
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run tests/tcsql_test.prw falhou: %v\n%s", err, out)
	}
	want := []string{"linhas=2", "linha1_codigo=A1", "linha1_valor=10.5", "linha2_codigo=A2"}
	got := string(out)
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("saída não contém %q; saída completa:\n%s", w, got)
		}
	}
}

// TestTCSqlErroNaoEncerra roda tests/tcsql_erro_test.prw: erro de SQL volta
// para o programa (TCSqlExec .F., TCSqlQuery {}, TCSqlError) em vez de
// encerrá-lo, e um bloco BEGIN/COMMIT que falha no meio é desfeito.
func TestTCSqlErroNaoEncerra(t *testing.T) {
	if testing.Short() {
		t.Skip("builda o binário; pulado com -short")
	}
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	binName := "advplc"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(t.TempDir(), binName)
	build := exec.Command("go", "build", "-o", binPath, "./cmd/advplc")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	run := exec.Command(binPath, "run", "tests/tcsql_erro_test.prw", "--db-path", filepath.Join(t.TempDir(), "erro.db"))
	run.Dir = repoRoot
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run tests/tcsql_erro_test.prw falhou: %v\n%s", err, out)
	}
	want := []string{
		"exec_com_erro=.F.", "erro_tem_notnull=.T.",
		"exec_ok=.T.", "erro_limpo=.T.",
		"query_com_erro_linhas=0", "query_erro_msg=.T.",
		"bloco_com_erro=.F.", "linhas_depois_do_bloco=1",
		"chegou_ao_fim=.T.",
	}
	got := string(out)
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("saída não contém %q; saída completa:\n%s", w, got)
		}
	}
}
