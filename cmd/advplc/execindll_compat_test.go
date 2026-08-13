package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestExecInDLLCompat prova, ponta a ponta, que o wrapper de compatibilidade
// TDN (examples/dyncall/execindll_compat.prw) consegue abrir uma DLL/SO
// real via tRunDll, chamar ExecInClientDLL e ler o buffer de saída de
// volta como String — o ciclo completo ExecInDLLOpen/ExecInDLLRun/
// ExecInDLLClose sem SmartClient.
func TestExecInDLLCompat(t *testing.T) {
	if testing.Short() {
		t.Skip("builda binário e DLL; pulado com -short")
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc não encontrado no ambiente, pulando teste de DynCall")
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}

	tmpDir := t.TempDir()

	soExt := ".so"
	if runtime.GOOS == "windows" {
		soExt = ".dll"
	} else if runtime.GOOS == "darwin" {
		soExt = ".dylib"
	}
	soPath := filepath.Join(tmpDir, "execindll"+soExt)
	build := exec.Command("gcc", "-shared", "-fPIC", "-fvisibility=default", "-o", soPath,
		filepath.Join(repoRoot, "pkg", "vm", "testdata", "dyncall", "execindll.c"))
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, out)
	}

	wrapper, err := os.ReadFile(filepath.Join(repoRoot, "examples", "dyncall", "execindll_compat.prw"))
	if err != nil {
		t.Fatalf("ReadFile wrapper: %v", err)
	}

	script := string(wrapper) + `
User Function ExecInDllCompatTest()
    Local hHdl := ExecInDLLOpen(GetEnv("ADVPP_TEST_SO"))
    If hHdl == -1
        ConOut("open_status=falhou")
        Return
    EndIf
    ConOut("open_status=ok")

    Local cRet := ExecInDLLRun(hHdl, 7, "ola")
    ConOut("run_result=" + cRet)

    Local lClosed := ExecInDLLClose(hHdl)
    ConOut("close_status=" + IIf(lClosed, "ok", "falhou"))
Return
`
	srcPath := filepath.Join(tmpDir, "execindll_compat_test.prw")
	if err := os.WriteFile(srcPath, []byte(script), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	binName := "advplc"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmpDir, binName)
	buildBin := exec.Command("go", "build", "-o", binPath, "./cmd/advplc")
	buildBin.Dir = repoRoot
	if out, err := buildBin.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	run := exec.Command(binPath, "run", srcPath)
	run.Env = append(os.Environ(), "ADVPP_TEST_SO="+soPath)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run execindll_compat_test.prw falhou: %v\n%s", err, out)
	}

	got := string(out)
	t.Logf("Saida do teste ExecInDLL compat:\n%s", got)

	for _, want := range []string{"open_status=ok", "run_result=ECHO:7:ola", "close_status=ok"} {
		if !strings.Contains(got, want) {
			t.Errorf("saida nao contem %q\nsaida completa:\n%s", want, got)
		}
	}
}
