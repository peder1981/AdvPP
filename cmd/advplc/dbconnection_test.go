package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDbConnectionFixture roda tests/dbconnection_test.prw pelo binário
// advplc real — prova que a classe DbConnection é alcançável a partir de
// fonte AdvPL compilada de verdade (achado #1 da revisão final da branch
// multidb: "DBCONNECTION" faltava em builtinClasses, então
// DbConnection():New(...) compilava como chamada de função desconhecida em
// vez de OP_NEW_INSTANCE; nenhum teste existente até então passava por essa
// via — todos chamavam callDbConnectionMethod diretamente em Go).
//
// Connect() contra 127.0.0.1:1 (porta fechada) devolve .F. rapidamente, sem
// exigir um servidor de banco real disponível no CI.
func TestDbConnectionFixture(t *testing.T) {
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

	run := exec.Command(binPath, "run", "tests/dbconnection_test.prw")
	run.Dir = repoRoot
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run tests/dbconnection_test.prw falhou: %v\n%s", err, out)
	}
	got := string(out)
	want := []string{"connect=false", "temerro=true"}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("saída não contém %q; saída completa:\n%s", w, got)
		}
	}
}
