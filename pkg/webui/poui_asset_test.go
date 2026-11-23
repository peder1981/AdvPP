package webui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// GET /poui-asset — serve assets relativos do cwd com a mesma checagem
// de caminho do upload, exigindo sid de sessão viva.

func assetReq(t *testing.T, sid, p string) *httptest.ResponseRecorder {
	t.Helper()
	ts := httptest.NewServer(New("t.prw", nil).Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/poui-asset?sid=" + sid + "&p=" + p)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	rec := httptest.NewRecorder()
	rec.Code = resp.StatusCode
	rec.Body.Write(body)
	return rec
}

func TestAssetSessaoDesconhecida404(t *testing.T) {
	rec := assetReq(t, "nao-existe", "x.png")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestAssetTraversal403(t *testing.T) {
	srv := New("t.prw", nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	srv.mu.Lock()
	srv.sessions["s1"] = &session{sid: "s1"}
	srv.mu.Unlock()
	for _, p := range []string{"../etc/passwd", "..%2F..%2Fetc%2Fpasswd", "a/../../b"} {
		resp, err := http.Get(ts.URL + "/poui-asset?sid=s1&p=" + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
			t.Fatalf("p=%q code=%d", p, resp.StatusCode)
		}
	}
}

func TestAssetOkServeArquivo(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logo.png"), []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New("t.prw", nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	srv.mu.Lock()
	srv.sessions["s1"] = &session{sid: "s1"}
	srv.mu.Unlock()
	resp, err := http.Get(ts.URL + "/poui-asset?sid=s1&p=logo.png")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "PNG" {
		t.Fatalf("code=%d body=%q", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Fatalf("content-type = %q", ct)
	}
}
