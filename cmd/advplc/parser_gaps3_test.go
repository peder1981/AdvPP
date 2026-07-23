package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestParserGaps3Fixture roda `advplc check` e `advplc run` sobre
// tests/parser_gaps3_test.prw — 10 gaps reais achados numa rodada de
// "tente ao menos novamente" analisando os fontes-padrão do corpus
// Protheus (811R4 + 12.1.2510) que sobraram de rodadas anteriores de
// varredura. Ver CHANGELOG [Não lançado].
func TestParserGaps3Fixture(t *testing.T) {
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

	check := exec.Command(binPath, "check", "tests/parser_gaps3_test.prw")
	check.Dir = repoRoot
	if out, err := check.CombinedOutput(); err != nil {
		t.Fatalf("advplc check tests/parser_gaps3_test.prw falhou: %v\n%s", err, out)
	}

	run := exec.Command(binPath, "run", "tests/parser_gaps3_test.prw")
	run.Dir = repoRoot
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run tests/parser_gaps3_test.prw falhou: %v\n%s", err, out)
	}
	want := []string{"ident_macro_for_bound=7", "array_index_compound_assign=10,1"}
	got := string(out)
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("saída não contém %q; saída completa:\n%s", w, got)
		}
	}
}
