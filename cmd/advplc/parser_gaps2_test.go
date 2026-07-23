package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestParserGaps2Fixture roda `advplc check` sobre tests/parser_gaps2_test.prw
// — fixture com um caso mínimo por padrão de linguagem real do corpus
// Protheus (811R4 + 12.1.2510) que já quebrou o parser/compilador numa
// rodada de varredura. Regressão pura: só precisa compilar sem erro.
func TestParserGaps2Fixture(t *testing.T) {
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

	check := exec.Command(binPath, "check", "tests/parser_gaps2_test.prw")
	check.Dir = repoRoot
	if out, err := check.CombinedOutput(); err != nil {
		t.Fatalf("advplc check tests/parser_gaps2_test.prw falhou: %v\n%s", err, out)
	}
}
