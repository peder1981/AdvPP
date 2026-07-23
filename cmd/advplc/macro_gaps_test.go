package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestMacroGapsFixture builda o advplc e roda tests/macro_gaps_test.prw,
// que exercita os quatro gaps corrigidos em 2026-07-23 (BEGIN REPORT QUERY,
// macro-eval em runtime, composição de nome ident&macro, NamedParam fora de
// call args) — ver CHANGELOG [Não lançado].
func TestMacroGapsFixture(t *testing.T) {
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

	cmd := exec.Command(binPath, "run", "tests/macro_gaps_test.prw")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run: %v\n%s", err, out)
	}

	want := []string{"macro_eval=42", "ident_macro=99", "named_param=100", "report_query=OK"}
	got := string(out)
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("saída não contém %q; saída completa:\n%s", w, got)
		}
	}
}
