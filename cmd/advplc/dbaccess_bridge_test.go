package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDbAccessBridgeFixture roda tests/dbaccess_bridge_test.prw pelo
// advplc real contra o driver DBACCESS (bridge C++ -> dbapi.so TOTVS ->
// dbaccess64 -> Postgres). Só executa com ADVPP_DBAPI_IT=1 e o ambiente
// de pé (advpp-dbapi-bridge em ADVPP_DBAPI_BRIDGE + dbaccess64 na 7890):
//
//	tools/dbapi-bridge/advpp-dbapi-bridge <dbapi.so> 17890 &
//	ADVPP_DBAPI_IT=1 ADVPP_DBAPI_BRIDGE=127.0.0.1:17890 go test ./cmd/advplc -run DbAccessBridge
func TestDbAccessBridgeFixture(t *testing.T) {
	if os.Getenv("ADVPP_DBAPI_IT") != "1" {
		t.Skip("defina ADVPP_DBAPI_IT=1 com bridge+dbaccess no ar")
	}
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

	run := exec.Command(binPath, "run", "tests/dbaccess_bridge_test.prw")
	run.Dir = repoRoot
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run tests/dbaccess_bridge_test.prw falhou: %v\n%s", err, out)
	}
	got := string(out)
	want := []string{
		"connect=true",
		"rows=3",
		"usearea=true",
		"sqlname=SE1990",
		"se1=true",
		"se1emis=",
		"fim=true",
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("saída não contém %q; saída completa:\n%s", w, got)
		}
	}
}
