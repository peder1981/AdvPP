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

// TestStackLeakRegression roda tests/stack_regression_test.prw pelo binário
// advplc real e falha se ele não terminar em 30s.
//
// Regressão do FULL-REVIEW C1: um comando-expressão (chamada de função
// isolada, x++, x--) não descartava seu valor, vazando a pilha da VM a
// cada iteração; ao estourar MaxStackSize o programa entrava em loop
// infinito silencioso. Antes da correção, um simples `For 1 To 20000` com
// `nC++` nunca terminava. O timeout do CommandContext transforma esse
// travamento numa falha de teste em vez de um job pendurado.
func TestStackLeakRegression(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	run := exec.CommandContext(ctx, binPath, "run", "tests/stack_regression_test.prw")
	run.Dir = repoRoot
	out, err := run.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("advplc travou (>30s) em tests/stack_regression_test.prw — regressão de vazamento de pilha (C1); saída parcial:\n%s", out)
	}
	if err != nil {
		t.Fatalf("advplc run falhou: %v\n%s", err, out)
	}
	got := string(out)
	for _, w := range []string{"incr nC=20000", "call nC=20000", "aadd len=20000", "decr nC=0", "stack-regression OK"} {
		if !strings.Contains(got, w) {
			t.Errorf("saída não contém %q; saída completa:\n%s", w, got)
		}
	}
}
