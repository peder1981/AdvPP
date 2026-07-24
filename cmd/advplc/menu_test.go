package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestMenuFallbackHeadless roda tests/menu_test.prw (advplc run, sem
// UIProvider): FWMenuSelect/FWGetText nunca devem bloquear esperando
// input que não vai vir — devem retornar 0/valor default na hora. Ver
// pkg/vm/natives.go (FWMENUSELECT/FWGETTEXT) e pkg/webui/server.go
// (Provider.Menu/InputText, caminho interativo real via advplc serve).
func TestMenuFallbackHeadless(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	run := exec.CommandContext(ctx, binPath, "run", "tests/menu_test.prw")
	run.Dir = repoRoot
	out, err := run.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("advplc run tests/menu_test.prw travou (deveria retornar 0/default sem UIProvider)")
	}
	if err != nil {
		t.Fatalf("advplc run tests/menu_test.prw falhou: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "escolha=0") {
		t.Errorf("esperava FWMenuSelect retornar 0 sem UIProvider; saída:\n%s", got)
	}
	if !strings.Contains(got, "texto=2026-08") {
		t.Errorf("esperava FWGetText retornar o valor default sem UIProvider; saída:\n%s", got)
	}
}
