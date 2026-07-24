package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestEntryPointPrefersRootFile roda tests/entrypoint_main_test.prw, que tem
// um comentário de cabeçalho antes do #include de uma lib auxiliar
// (entrypoint_lib_test.prw), com a função própria do arquivo declarada
// depois. Sem corpo de nível superior, o compilador precisa escolher qual
// User Function chamar implicitamente — deve ser a do arquivo raiz (EpMain),
// nunca a trazida pelo #include (EpLibHelper, que além de errada crashava a
// VM com SIGSEGV ao comparar o parâmetro nunca passado contra Nil). Ver
// CHANGELOG, pkg/compiler/codegen.go (RootBoundaryLine) e pkg/vm/vm.go
// (newLocals).
func TestEntryPointPrefersRootFile(t *testing.T) {
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

	run := exec.Command(binPath, "run", "tests/entrypoint_main_test.prw")
	run.Dir = repoRoot
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run tests/entrypoint_main_test.prw falhou: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "arquivo raiz chamado - CERTO") {
		t.Errorf("esperava a função do arquivo raiz rodar; saída:\n%s", got)
	}
	if strings.Contains(got, "biblioteca chamada - ERRADO") {
		t.Errorf("função trazida via #include rodou como entry point; saída:\n%s", got)
	}
}
