package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestBuildStandaloneSmoke builda tests/standalone_console_test.prw com
// `advplc build` e RODA o executável gerado de verdade — mesmo fixture e
// mesma expectativa (sai sozinho, imprime as duas linhas) do smoke test
// de CI que só roda em runner Windows (.github/workflows/test.yml), mas
// aqui de forma portável: pulado automaticamente se não houver display
// disponível pra abrir uma janela Fyne de verdade (a suíte padrão do CI
// em Linux/macOS não instala Xvfb — rodar isso sem display faria
// `go test ./...` travar ali, não só neste pacote).
//
// Existe pra cobrir uma mudança real: stub_template.go ganhou um
// cabeçalho (widget.Label com o título) e um tema custom
// (ui.NewTheme()) — ambos resolvidos depois de app.New(), mas antes
// deste teste isso não tinha nenhuma cobertura automatizada fora do
// smoke test manual em Windows.
func TestBuildStandaloneSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("builda o binário e compila um executável standalone; pulado com -short")
	}
	if runtime.GOOS != "windows" && os.Getenv("DISPLAY") == "" {
		t.Skip("sem display disponível pra abrir uma janela Fyne de verdade (defina DISPLAY, ex. via Xvfb, ou rode no Windows)")
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		t.Fatalf("repoRoot não parece um checkout do AdvPP: %v", err)
	}

	binName := "advplc"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, binName)
	build := exec.Command("go", "build", "-o", binPath, "./cmd/advplc")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build advplc: %v\n%s", err, out)
	}

	outName := "standalone_smoke"
	if runtime.GOOS == "windows" {
		outName += ".exe"
	}
	outPath := filepath.Join(tmpDir, outName)
	// advplc build precisa achar um checkout local do módulo (o stub
	// gerado importa pkg/compiler e pkg/vm) — ADVPP_SRC aponta pra ele
	// explicitamente em vez de depender do cwd, já que build.Dir é
	// repoRoot mas o processo builda de dentro de um diretório temporário.
	buildCmd := exec.Command(binPath, "build", "tests/standalone_console_test.prw", "-o", outPath)
	buildCmd.Dir = repoRoot
	buildCmd.Env = append(os.Environ(), "ADVPP_SRC="+repoRoot)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("advplc build: %v\n%s", err, out)
	}

	// Regressão específica: stub_template.go usa um placeholder
	// (__ADVPP_APP_TITLE__) substituído por BuildStandalone antes de
	// compilar — se a substituição falhar silenciosamente, o binário
	// gerado ainda compila e roda (é só uma string literal), então só um
	// grep no binário pega esse tipo de regressão.
	binData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("lendo o executável gerado: %v", err)
	}
	if strings.Contains(string(binData), "__ADVPP_APP_TITLE__") {
		t.Error("o placeholder de título não foi substituído no binário gerado")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	run := exec.CommandContext(ctx, outPath)
	out, err := run.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("o executável standalone não saiu sozinho em 15s (deveria terminar sem interação, é um programa 100% console)")
	}
	if err != nil {
		t.Fatalf("rodar o executável standalone: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "standalone smoke test: linha 1") {
		t.Errorf("saída não contém a linha 1; saída completa:\n%s", got)
	}
	if !strings.Contains(got, "standalone smoke test: linha 2") {
		t.Errorf("saída não contém a linha 2; saída completa:\n%s", got)
	}
}
