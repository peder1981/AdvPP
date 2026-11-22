package webui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pouiServer monta um webui.Server de teste com uma sessão "s1" e um app
// PO-UI aberto nela via Provider.AbrePouiApp.
func pouiServer(t *testing.T) (*Server, *session, *PouiApp) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, err := AbrePouiApp(dir, 0)
	if err != nil {
		t.Fatalf("AbrePouiApp: %v", err)
	}
	srv := New("teste", func(ui *Provider, c *OutWriter) error { return nil })
	s := newSession()
	s.sid = "s1"
	srv.sessions["s1"] = s
	p := &Provider{s: s}
	ctx, err := NovoPouiCtx(app.Nome, "01", "01", "EST", "admin", "pt-br", "/rest", "http://x")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.AbrePouiApp(app, ctx); err != nil {
		t.Fatalf("AbrePouiApp: %v", err)
	}
	return srv, s, app
}

func pouiPost(t *testing.T, url, body string) int {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestPouiCanalIdaEVolta(t *testing.T) {
	srv, s, _ := pouiServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// jsToAdvpl -> PouiNext
	code := pouiPost(t, ts.URL+"/poui-ui?sid=s1", `{"app":"app","type":"preLoad","content":""}`)
	if code != http.StatusNoContent {
		t.Fatalf("POST /poui-ui = %d", code)
	}
	p := &Provider{s: s}
	tipo, _, fechou := p.PouiNext(2 * time.Second)
	if fechou || tipo != "preLoad" {
		t.Fatalf("PouiNext = %q fechou=%v", tipo, fechou)
	}

	// PouiEnvia -> SSE canal=poui recebe advpltojs
	resp, err := http.Get(ts.URL + "/events?s=s1&canal=poui")
	if err != nil {
		t.Fatalf("GET /events canal=poui: %v", err)
	}
	defer resp.Body.Close()
	go p.PouiEnvia("r-1", "ok")
	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("ler SSE: %v", err)
	}
	if !strings.Contains(line, `"advpltojs"`) || !strings.Contains(line, `"r-1"`) {
		t.Fatalf("evento SSE = %q", line)
	}
}

func TestPouiCloseEncerra(t *testing.T) {
	srv, s, _ := pouiServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	p := &Provider{s: s}

	code := pouiPost(t, ts.URL+"/poui-ui?sid=s1", `{"type":"close","content":""}`)
	if code != http.StatusNoContent {
		t.Fatalf("POST /poui-ui = %d", code)
	}
	if _, _, fechou := p.PouiNext(2 * time.Second); !fechou {
		t.Fatal("PouiNext não devolveu fechou após close")
	}
}

func TestPouiAbaFechadaTimeout(t *testing.T) {
	t.Setenv("ADVPP_POUI_DESCONEXAO_SEG", "1")
	srv, s, _ := pouiServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	p := &Provider{s: s}

	// Aba conecta e desconecta.
	resp, err := http.Get(ts.URL + "/events?s=s1&canal=poui")
	if err != nil {
		t.Fatalf("GET /events canal=poui: %v", err)
	}
	resp.Body.Close()

	if _, _, fechou := p.PouiNext(4 * time.Second); !fechou {
		t.Fatal("PouiNext não devolveu fechou após aba sumir")
	}
}

func TestPouiRecargaDentroDoPrazo(t *testing.T) {
	t.Setenv("ADVPP_POUI_DESCONEXAO_SEG", "2")
	srv, s, _ := pouiServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	p := &Provider{s: s}

	// Aba conecta, cai e reconecta dentro do prazo (F5).
	for i := 0; i < 2; i++ {
		resp, err := http.Get(ts.URL + "/events?s=s1&canal=poui")
		if err != nil {
			t.Fatalf("GET /events canal=poui #%d: %v", i, err)
		}
		resp.Body.Close()
		time.Sleep(300 * time.Millisecond)
	}
	// Ainda viva: um jsToAdvpl chega normal.
	pouiPost(t, ts.URL+"/poui-ui?sid=s1", `{"type":"ping","content":""}`)
	if tipo, _, fechou := p.PouiNext(2 * time.Second); fechou || tipo != "ping" {
		t.Fatalf("após recarga: tipo=%q fechou=%v", tipo, fechou)
	}
}

func TestPouiSegundaAba409(t *testing.T) {
	srv, s, _ := pouiServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/events?s=s1&canal=poui")
	if err != nil {
		t.Fatalf("GET /events canal=poui: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("primeira aba = %d", resp.StatusCode)
	}
	resp2, err := http.Get(ts.URL + "/events?s=s1&canal=poui")
	if err != nil {
		t.Fatalf("GET /events canal=poui 2: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("segunda aba = %d, want 409", resp2.StatusCode)
	}
	var m map[string]string
	_ = json.NewDecoder(resp2.Body).Decode(&m)
	if m["message"] != "app já aberto nesta sessão" {
		t.Fatalf("mensagem 409 = %v", m)
	}
	_ = s
}

func TestPouiSidInvalido404(t *testing.T) {
	srv, _, _ := pouiServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	for _, u := range []string{
		"/poui-ctx?sid=nao-existe",
		"/poui/app/?sid=nao-existe",
		"/poui-ctx", // sem sid nem cookie
	} {
		resp, err := http.Get(ts.URL + u)
		if err != nil {
			t.Fatalf("GET %s: %v", u, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET %s = %d, want 404", u, resp.StatusCode)
		}
	}
}

func TestPouiHostEstranho403(t *testing.T) {
	srv, _, _ := pouiServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	for _, u := range []string{"/poui-ctx?sid=s1", "/poui/app/?sid=s1", "/poui-ui?sid=s1"} {
		req, _ := http.NewRequest("GET", ts.URL+u, nil)
		req.Host = "evil.example"
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", u, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("GET %s host estranho = %d, want 403", u, resp.StatusCode)
		}
	}
}

func TestPouiDepoisDeFechar410(t *testing.T) {
	srv, s, app := pouiServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	p := &Provider{s: s}
	p.FechaPouiApp()

	resp, err := http.Get(fmt.Sprintf("%s/poui/%s/?sid=s1", ts.URL, app.Nome))
	if err != nil {
		t.Fatalf("GET /poui: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusGone {
		t.Fatalf("GET /poui após fechar = %d, want 410", resp.StatusCode)
	}
}

func TestPouiCorpoGrande413(t *testing.T) {
	srv, s, _ := pouiServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	grande := strings.Repeat("x", 9<<20)
	code := pouiPost(t, ts.URL+"/poui-ui?sid=s1",
		`{"type":"x","content":"`+grande+`"}`)
	if code != http.StatusRequestEntityTooLarge && code != http.StatusBadRequest {
		t.Fatalf("POST corpo 9MB = %d", code)
	}
	_ = s
}
