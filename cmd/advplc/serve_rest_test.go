package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/db"
	"github.com/advpl/compiler/pkg/tools/shared"
	"github.com/advpl/compiler/pkg/webui"
)

// serveRestFixture monta o mesmo pipeline do `advplc serve` (compila o
// diretório, registra as rotas REST, MountREST no webui.Server) e devolve
// um httptest.Server — sem subir o binário.
func serveRestFixture(t *testing.T, alvo string, opts *Options) *httptest.Server {
	t.Helper()
	bc, err := compilaAlvo(alvo, opts)
	if err != nil {
		t.Fatalf("compilaAlvo: %v", err)
	}
	wsrv := webui.New("teste", func(ui *webui.Provider, console *webui.OutWriter) error {
		return nil
	})
	montaRESTNoServe(bc, opts, wsrv)
	srv := httptest.NewServer(wsrv.Handler())
	t.Cleanup(srv.Close)
	return srv
}

// bancoFixtureREST cria um SQLite temporário com o schema+seed da fixture
// de armazéns (mesma usada pelos testes do adapter) e devolve o caminho —
// o serve resolve o banco por opts.dbPath → attachDatabase.
func bancoFixtureREST(t *testing.T) string {
	t.Helper()
	path := t.TempDir() + "/advpp.db"
	eng, err := db.NewSQLiteEngine(path)
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	sqlBytes, err := os.ReadFile("../../tests/rest_totvs/armazens_fixture.sql")
	if err != nil {
		t.Fatalf("ler fixture sql: %v", err)
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
		if stmt := strings.Join(lines, "\n"); stmt != "" {
			if err := eng.Exec(stmt); err != nil {
				t.Fatalf("fixture sql: %v", err)
			}
		}
	}
	for i := 1; i <= 25; i++ {
		if err := eng.Exec(fmt.Sprintf(
			"INSERT INTO NNR (NNR_FILIAL, NNR_CODIGO, NNR_DESCRI, NNR_ESTOQUE) VALUES ('01', '%05d', 'Armazem %02d', %.2f)",
			i, i, float64(i)*1.5)); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	return path
}

func TestServeMontaRestNoPrefixo(t *testing.T) {
	opts := &Options{dbPath: bancoFixtureREST(t)}
	srv := serveRestFixture(t, "../../tests/rest_totvs", opts)

	resp, err := http.Get(srv.URL + "/rest/api/est/v1/armazens?pageSize=50")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := make([]byte, 1<<20)
	n, _ := resp.Body.Read(body)
	if !strings.Contains(string(body[:n]), `"items"`) || !strings.Contains(string(body[:n]), `"hasNext"`) {
		t.Fatalf("body sem items/hasNext: %s", body[:n])
	}
}

func TestServeRestPrefixoConfigurado(t *testing.T) {
	t.Setenv("ADVPL_REST_PREFIX", "/api-local")
	if p := shared.ResolveRestPrefix(); p != "/api-local" {
		t.Fatalf("ResolveRestPrefix = %q, want /api-local", p)
	}
	opts := &Options{dbPath: bancoFixtureREST(t)}
	srv := serveRestFixture(t, "../../tests/rest_totvs", opts)

	resp, err := http.Get(srv.URL + "/api-local/api/est/v1/armazens?pageSize=50")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status /api-local = %d, want 200", resp.StatusCode)
	}

	resp2, err := http.Get(srv.URL + "/rest/api/est/v1/armazens")
	if err != nil {
		t.Fatalf("GET /rest: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode == http.StatusOK {
		t.Fatalf("rota respondeu 200 em /rest com prefixo /api-local")
	}
}

func TestServeRestRespeitaHost(t *testing.T) {
	opts := &Options{dbPath: bancoFixtureREST(t)}
	srv := serveRestFixture(t, "../../tests/rest_totvs", opts)

	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/est/v1/armazens", nil)
	req.Host = "evil.example"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestServeSemRestInalterado(t *testing.T) {
	srv := serveRestFixture(t, "../../tests/webui_test.prw", &Options{})

	resp, err := http.Get(srv.URL + "/rest/qualquer")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	// Sem rotas REST o handler devolve 404 — nunca o SPA.
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status /rest/x = %d, want 404", resp.StatusCode)
	}

	resp2, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("status / = %d, want 200 (SPA normal)", resp2.StatusCode)
	}
}
