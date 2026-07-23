package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestParserGapsFixture builda o advplc e roda `check` sobre
// tests/parser_gaps_v1201.prw, que isola os padrões de linguagem achados
// numa varredura de corpus real (811R4 + Protheus 12.1.2510) depois do
// v1.20.0: NamedParam usado fora de lista de argumentos (ex.: ramo de
// IIF), ACTIVATE ... AT, RELEASE OBJECTS (plural), LISTBOX FIELDS ALIAS,
// DEFINE SECTION ... TABLE (singular), METER ... BARCOLOR e
// Default alias->campo. Ver CHANGELOG.md seção [Não lançado] para a causa
// raiz de cada um.
func TestParserGapsFixture(t *testing.T) {
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

	cmd := exec.Command(binPath, "check", "tests/parser_gaps_v1201.prw")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc check tests/parser_gaps_v1201.prw falhou: %v\n%s", err, out)
	}
}
