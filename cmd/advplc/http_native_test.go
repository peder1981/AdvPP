package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHttpNativeFixtures(t *testing.T) {
	if testing.Short() {
		t.Skip("builda o binário; pulado com -short")
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}

	// Start test HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/echo-get", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("GET handler called with %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"method": "GET",
			"query":  r.URL.RawQuery,
			"status": "ok",
		})
	})
	mux.HandleFunc("/echo-post", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("POST handler called with %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"method": "POST",
			"body":   string(body),
			"status": "ok",
		})
	})
	mux.HandleFunc("/echo-put", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"method": "PUT", "status": "ok"})
	})
	mux.HandleFunc("/echo-patch", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"method": "PATCH", "status": "ok"})
	})
	mux.HandleFunc("/echo-delete", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"method": "DELETE", "status": "ok"})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Build advplc binary
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

	// Run test fixture with test server URL
	run := exec.Command(binPath, "run", "tests/http_native_test.prw")
	run.Dir = repoRoot
	run.Env = append(os.Environ(), "ADVPP_TEST_URL="+ts.URL)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run tests/http_native_test.prw falhou: %v\n%s", err, out)
	}

	got := string(out)
	t.Logf("Saida do teste HTTP:\n%s", got)

	checks := []struct {
		name string
		want string
	}{
		{"get_status", "get_status=200"},
		{"get_body_ok", "get_body_ok=1"},
		{"post_status", "post_status=200"},
		{"post_body_ok", "post_body_ok=1"},
		{"put_status", "put_status=200"},
		{"patch_status", "patch_status=200"},
		{"delete_status", "delete_status=200"},
		{"erro_tratado", "erro_tratado=1"},
	}

	for _, c := range checks {
		if !strings.Contains(got, c.want) {
			t.Errorf("saida nao contem %q", c.want)
		}
	}

	if !strings.Contains(got, "--- HttpNativeTest FIM ---") {
		t.Error("teste nao finalizou corretamente")
	}
}

func TestHttpNativeCertError(t *testing.T) {
	if testing.Short() {
		t.Skip("builda o binário; pulado com -short")
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}

	// Test with non-existent cert
	script := `
User function TestCertError()
    Local nStatus := FWHttpPost("https://example.com/api", "{}", "application/json", "/tmp/cert_inexistente.pfx", "senha_errada")
    ConOut("cert_status=" + Str(nStatus))
    ConOut("cert_error_vazio=" + IIf(FWHttpError() == "", "1", "0"))
    IIf(nStatus == 0, ConOut("cert_erro_tratado=1"), ConOut("cert_erro_tratado=0"))
Return
`
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "cert_error_test.prw")
	if err := os.WriteFile(srcPath, []byte(script), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	binName := "advplc"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmpDir, binName)
	build := exec.Command("go", "build", "-o", binPath, "./cmd/advplc")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	run := exec.Command(binPath, "run", srcPath)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("advplc run cert test falhou: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "cert_erro_tratado=1") {
		t.Errorf("cert error nao tratado: %s", got)
	}
}
