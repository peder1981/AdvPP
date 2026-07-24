package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestWorkareaFixture roda tests/workarea_test.prw — exercita a API
// clássica de work-area (DbAppend/RecLock/FieldPut/MsUnlock) com
// persistência real. Ver CHANGELOG.
func TestWorkareaFixture(t *testing.T) {
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

	run := exec.Command(binPath, "run", "tests/workarea_test.prw")
	run.Dir = repoRoot
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run tests/workarea_test.prw falhou: %v\n%s", err, out)
	}
	// fieldpos=4: WA_CODIGO é a 4ª coluna física da tabela (R_E_C_N_O_,
	// D_E_L_E_T_, R_E_C_D_E_L_ vêm antes) — FieldPos conta todas as
	// colunas, não só as de negócio.
	want := []string{"qtd=1", "codigo=X1", "valor=42", "fieldpos=4"}
	got := string(out)
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("saída não contém %q; saída completa:\n%s", w, got)
		}
	}
}
