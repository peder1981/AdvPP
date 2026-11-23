package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/advpl/compiler/pkg/db"
	"github.com/advpl/compiler/pkg/rest"
	"github.com/advpl/compiler/pkg/webui"
)

// pouiAppFixture cria um app PO-UI mínimo em diretório temporário.
func pouiAppFixture(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "meu-app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"),
		[]byte("<html><body>app</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// servePouiAppFixture monta o mesmo pipeline do servePouiApp sem subir a
// porta: servidor webui + handler raiz com redirect.
func servePouiAppFixture(t *testing.T, appPath, fonte string, opts *Options) *httptest.Server {
	t.Helper()
	maxZip := int64(64 << 20)
	app, err := webui.AbrePouiApp(appPath, maxZip)
	if err != nil {
		t.Fatalf("AbrePouiApp: %v", err)
	}
	srv := webui.New("poui-app:"+app.Nome,
		func(ui *webui.Provider, console *webui.OutWriter) error { return nil })
	if fonte != "" {
		bc, err := compilaAlvo(fonte, opts)
		if err != nil {
			t.Fatalf("compilaAlvo: %v", err)
		}
		montaRESTNoServe(bc, opts, srv)
	} else {
		var p atomic.Pointer[rest.Server]
		empty := rest.NewServer("advpl-serve", "1.0")
		registraFrameworkRoutes(empty, opts)
		p.Store(empty)
		srv.MountREST("/rest", handlerRest(&p))
	}
	inner := srv.Handler()
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			sid, err := srv.NovaSessaoPoui(app, "http://"+r.Host)
			if err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
			http.Redirect(w, r, "/poui/"+app.Nome+"/?sid="+sid+"&app="+app.Nome, http.StatusFound)
			return
		}
		inner.ServeHTTP(w, r)
	})
	ts := httptest.NewServer(root)
	t.Cleanup(ts.Close)
	return ts
}

func TestServePouiAppRedirecionaRaiz(t *testing.T) {
	ts := servePouiAppFixture(t, pouiAppFixture(t), "", &Options{})

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("GET / = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/poui/meu-app/?sid=") || !strings.Contains(loc, "app=meu-app") {
		t.Fatalf("Location = %q", loc)
	}

	// O destino serve o index com o shim injetado.
	resp2, err := http.Get(ts.URL + loc)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != 200 || !strings.Contains(string(body), "twebchannel") {
		t.Fatalf("app = %d, shim? %v", resp2.StatusCode, strings.Contains(string(body), "twebchannel"))
	}
}

func TestServePouiAppSemFonte(t *testing.T) {
	ts := servePouiAppFixture(t, pouiAppFixture(t), "", &Options{})

	// Sessão do app via redirect.
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, _ := client.Get(ts.URL + "/")
	loc := resp.Header.Get("Location")
	resp.Body.Close()
	sid := strings.Split(strings.Split(loc, "sid=")[1], "&")[0]

	// jsToAdvpl aceito e descartado.
	resp2, err := http.Post(ts.URL+"/poui-ui?sid="+sid,
		"application/json", strings.NewReader(`{"app":"meu-app","type":"ping","content":""}`))
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /poui-ui = %d", resp2.StatusCode)
	}

	// Sem fonte: só framework montado — current-user responde.
	resp3, err := http.Get(ts.URL + "/rest/api/framework/current-user")
	if err != nil {
		t.Fatal(err)
	}
	body3, _ := io.ReadAll(resp3.Body)
	resp3.Body.Close()
	if resp3.StatusCode != 200 || !strings.Contains(string(body3), "userName") {
		t.Fatalf("framework sem fonte = %d %s", resp3.StatusCode, body3)
	}
}

func TestServePouiAppComRest(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "t.db")
	eng, err := db.NewSQLiteEngine(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	sqlBytes, err := os.ReadFile("../../tests/rest_totvs/armazens_fixture.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range strings.Split(string(sqlBytes), ";") {
		var lines []string
		for _, ln := range strings.Split(stmt, "\n") {
			ln = strings.TrimSpace(ln)
			if ln == "" || strings.HasPrefix(ln, "--") {
				continue
			}
			lines = append(lines, ln)
		}
		if s := strings.Join(lines, "\n"); s != "" {
			if err := eng.Exec(s); err != nil {
				t.Fatalf("fixture: %v", err)
			}
		}
	}
	eng.Close()

	ts := servePouiAppFixture(t, pouiAppFixture(t), "../../tests/rest_totvs", &Options{dbPath: dbPath})
	resp, err := http.Get(ts.URL + "/rest/api/est/v1/armazens")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "items") {
		t.Fatalf("REST = %d %s", resp.StatusCode, body)
	}
}

func TestExtraiPouiApp(t *testing.T) {
	app, fonte, ok := extraiPouiApp([]string{"--poui-app", "meu-app", "src", "--port", "9"})
	if !ok || app != "meu-app" || fonte != "src" {
		t.Fatalf("app=%q fonte=%q ok=%v", app, fonte, ok)
	}
	app, fonte, ok = extraiPouiApp([]string{"--poui-app=appx", "--watch"})
	if !ok || app != "appx" || fonte != "" {
		t.Fatalf("app=%q fonte=%q ok=%v", app, fonte, ok)
	}
	if _, _, ok := extraiPouiApp([]string{"src.prw", "--port", "9"}); ok {
		t.Fatal("sem --poui-app não deve ativar")
	}
}
