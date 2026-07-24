package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestFgtFixture roda tests/fgt_test.prw — exercita FWGetText com e sem
// 3º argumento bIsPassword. Em execução headless (advplc run), a native
// retorna cDefault independentemente de pw — o teste confirma que não
// crasha com "wrong number of arguments". Motivado pelo GesCon (senha
// mascarada no login/admin, Plano 2).
func TestFgtFixture(t *testing.T) {
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

	run := exec.Command(binPath, "run", "tests/fgt_test.prw")
	run.Dir = repoRoot
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run tests/fgt_test.prw falhou: %v\n%s", err, out)
	}
	want := []string{"texto=[default_text]", "senha=[s3nh4]", "normal=[]"}
	got := string(out)
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("saída não contém %q; saída completa:\n%s", w, got)
		}
	}
}
