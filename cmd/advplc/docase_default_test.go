package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDoCaseDefaultFixture garante que `Default` sozinho na linha dentro
// de `Do Case` executa como ramo otherwise (issue #3), que `Otherwise`
// continua funcionando e que `Default x := ...` continua sendo statement.
func TestDoCaseDefaultFixture(t *testing.T) {
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

	cmd := exec.Command(binPath, "run", "tests/docase_default_test.prw")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run: %v\n%s", err, out)
	}

	want := []string{"BRANCH_DEFAULT", "OTHERWISE_OK", "DEFAULT_STMT=42"}
	got := string(out)
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("saída não contém %q; saída completa:\n%s", w, got)
		}
	}
	// Comparação por linha exata: "BRANCH_D" é prefixo de "BRANCH_DEFAULT",
	// então Contains puro daria falso positivo.
	lines := map[string]bool{}
	for _, ln := range strings.Split(got, "\n") {
		lines[strings.TrimSpace(ln)] = true
	}
	for _, notWant := range []string{"BRANCH_A", "BRANCH_B", "BRANCH_C", "BRANCH_D", "OTHER_A"} {
		if lines[notWant] {
			t.Errorf("saída contém ramo inesperado %q; saída completa:\n%s", notWant, got)
		}
	}
}
