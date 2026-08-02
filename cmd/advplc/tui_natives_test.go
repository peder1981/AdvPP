package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestTuiNativesFixture roda tests/tui_natives_test.prw — exercita as novas
// primitivas de TUI (pkg/vm/ui_render.go: UiBox, UiStreamBox/Reset,
// UiMarkdown, UiTermWidth, UiAltScreenEnter/Exit), ConOutRaw, ProcRun e o
// parser real de JsonObject:FromJson (antes um stub que sempre devolvia
// Nil). Passa o próprio binário buildado via env ADVPLC_SELF_PATH pro
// fixture usar como alvo do ProcRun — portátil entre Linux/Windows/macOS
// sem depender de nenhum utilitário externo do SO.
func TestTuiNativesFixture(t *testing.T) {
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

	run := exec.Command(binPath, "run", "tests/tui_natives_test.prw")
	run.Dir = repoRoot
	run.Env = append(run.Environ(), "ADVPLC_SELF_PATH="+binPath)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run tests/tui_natives_test.prw falhou: %v\n%s", err, out)
	}
	got := string(out)

	want := []string{
		"BOX_HAS_TITLE=.T.",
		"BOX_HAS_BODY=.T.",
		"STREAM_OK=.T.",
		"MD_HAS_TEXT=.T.",
		"TERMWIDTH=80",
		"ALTSCREEN_OK=.T.",
		"raw1-raw2",
		"CONIN_EOF_IS_NIL=.T.",
		"JSON_PARSE_OK=.T.",
		"JSON_NOME=AdvPP",
		"JSON_VERSAO=2.18",
		"JSON_OK_BOOL=.T.",
		"JSON_ITEM2=b",
		"JSON_SUB_X=1",
		"JSON_INVALID_OK=.T.",
		"PROCRUN_EXIT=0",
		"PROCRUN_FIRSTLINE=advplc",
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("saída não contém %q; saída completa:\n%s", w, got)
		}
	}

	// UiAltScreenEnter/Exit devem ter escrito as sequências ANSI reais
	// (\x1b[?1049h e \x1b[?1049l) — confere nos bytes crus, não só no log
	// textual acima (o "ALTSCREEN_OK=.T." só prova que não crashou).
	if !strings.Contains(got, "\x1b[?1049h") || !strings.Contains(got, "\x1b[?1049l") {
		t.Error("saída não contém as sequências ANSI de tela alternativa (\\x1b[?1049h / \\x1b[?1049l)")
	}
}
