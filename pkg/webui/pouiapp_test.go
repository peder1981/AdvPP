package webui

import (
	"archive/zip"
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func escreveApp(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "assets", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "data", "appConfig.json"), []byte(`{"api_baseUrl":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func getApp(t *testing.T, a *PouiApp, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://x"+path, nil)
	a.ServeHTTP(rec, req)
	return rec
}

func TestPouiAppPasta(t *testing.T) {
	dir := t.TempDir()
	escreveApp(t, dir)
	a, err := AbrePouiApp(dir, 0)
	if err != nil {
		t.Fatalf("AbrePouiApp: %v", err)
	}
	if rec := getApp(t, a, "/"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") {
		t.Fatalf("GET / = %d %q", rec.Code, rec.Body.String())
	}
	if rec := getApp(t, a, "/assets/data/appConfig.json"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "api_baseUrl") {
		t.Fatalf("GET asset = %d %q", rec.Code, rec.Body.String())
	}
}

func zipApp(t *testing.T, entradas map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for nome, conteudo := range entradas {
		w, err := zw.Create(nome)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(conteudo)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "app.app")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPouiAppZipRaizAninhada(t *testing.T) {
	path := zipApp(t, map[string]string{
		"meuapp/index.html":            "<html>zip</html>",
		"meuapp/assets/appConfig.json": "{}",
	})
	a, err := AbrePouiApp(path, 0)
	if err != nil {
		t.Fatalf("AbrePouiApp: %v", err)
	}
	if rec := getApp(t, a, "/"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "zip") {
		t.Fatalf("GET / = %d %q", rec.Code, rec.Body.String())
	}
}

func TestPouiAppZipRaizDireta(t *testing.T) {
	path := zipApp(t, map[string]string{"index.html": "<html>raiz</html>"})
	a, err := AbrePouiApp(path, 0)
	if err != nil {
		t.Fatalf("AbrePouiApp: %v", err)
	}
	if rec := getApp(t, a, "/"); rec.Code != 200 {
		t.Fatalf("GET / = %d", rec.Code)
	}
}

func TestPouiAppZipSlipRejeitado(t *testing.T) {
	path := zipApp(t, map[string]string{
		"index.html":  "<html>x</html>",
		"../evil.txt": "x",
	})
	if _, err := AbrePouiApp(path, 0); err == nil {
		t.Fatal("esperava erro para entrada ../")
	}
}

func TestPouiAppZipGrande(t *testing.T) {
	path := zipApp(t, map[string]string{"index.html": "<html>x</html>"})
	if _, err := AbrePouiApp(path, 10); err == nil || !strings.Contains(err.Error(), "limite") {
		t.Fatalf("erro = %v, quer menção ao limite", err)
	}
}

func TestPouiAppRotaSpa(t *testing.T) {
	dir := t.TempDir()
	escreveApp(t, dir)
	a, err := AbrePouiApp(dir, 0)
	if err != nil {
		t.Fatalf("AbrePouiApp: %v", err)
	}
	rec := getApp(t, a, "/clientes/123")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<html>app</html>") {
		t.Fatalf("GET /clientes/123 = %d %q", rec.Code, rec.Body.String())
	}
	// Arquivo inexistente COM extensão -> 404, não SPA.
	if rec := getApp(t, a, "/assets/nao-existe.js"); rec.Code != 404 {
		t.Fatalf("GET inexistente.js = %d, want 404", rec.Code)
	}
}

func TestPouiAppIndexCaixa(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Index.html"), []byte("<html>caixa</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := AbrePouiApp(dir, 0)
	if err != nil {
		t.Fatalf("AbrePouiApp: %v", err)
	}
	if rec := getApp(t, a, "/"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "caixa") {
		t.Fatalf("GET / = %d %q", rec.Code, rec.Body.String())
	}
}

func TestPouiAppIndexComShim(t *testing.T) {
	dir := t.TempDir()
	escreveApp(t, dir)
	a, err := AbrePouiApp(dir, 0)
	if err != nil {
		t.Fatalf("AbrePouiApp: %v", err)
	}
	if rec := getApp(t, a, "/"); !strings.Contains(rec.Body.String(), "poui_sid") {
		t.Fatal("index servido sem o shim injetado")
	}
	// Asset não-index não recebe shim.
	if rec := getApp(t, a, "/assets/data/appConfig.json"); strings.Contains(rec.Body.String(), "poui_sid") {
		t.Fatal("asset recebeu shim indevidamente")
	}
}

func TestPouiAppSemIndexErro(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "foo.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AbrePouiApp(dir, 0); err == nil {
		t.Fatal("esperava erro sem index.html")
	}
}

func TestLocalizaPouiAppOrdem(t *testing.T) {
	dir := t.TempDir()
	// nome/ com index.html e nome.app no mesmo dir -> .app vence.
	if err := os.MkdirAll(filepath.Join(dir, "meuapp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meuapp", "index.html"), []byte("d"), 0o644); err != nil {
		t.Fatal(err)
	}
	zipPath := zipApp(t, map[string]string{"index.html": "z"})
	if err := os.Rename(zipPath, filepath.Join(dir, "meuapp.app")); err != nil {
		t.Fatal(err)
	}
	got, err := LocalizaPouiApp("meuapp", []string{dir})
	if err != nil {
		t.Fatalf("LocalizaPouiApp: %v", err)
	}
	if got != filepath.Join(dir, "meuapp.app") {
		t.Fatalf("LocalizaPouiApp = %q, quer .app", got)
	}
	// Sem o .app, acha a pasta.
	if err := os.Remove(filepath.Join(dir, "meuapp.app")); err != nil {
		t.Fatal(err)
	}
	got, err = LocalizaPouiApp("meuapp", []string{dir})
	if err != nil || got != filepath.Join(dir, "meuapp") {
		t.Fatalf("LocalizaPouiApp pasta = %q, %v", got, err)
	}
	// Nome inválido.
	if _, err := LocalizaPouiApp("a/b", []string{dir}); err == nil {
		t.Fatal("nome 'a/b' deveria dar erro")
	}
	// Inexistente.
	if _, err := LocalizaPouiApp("nada", []string{dir}); err == nil {
		t.Fatal("app inexistente deveria dar erro")
	}
}
