package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/db"
	"github.com/advpl/compiler/pkg/rest"
)

// frameworkSrv monta um rest.Server com as rotas de framework e um
// SQLite temporário como banco (opts.dbPath).
func frameworkSrv(t *testing.T) *rest.Server {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "t.db")
	eng, err := db.NewSQLiteEngine(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	// Tabela de negócio + SX3 mínimo para o genericList.
	if err := eng.Exec("CREATE TABLE ARMAZENS (A1_COD CHAR(10), A1_NOME CHAR(40), D_E_L_E_T_ CHAR(1))"); err != nil {
		t.Fatal(err)
	}
	if err := eng.Exec("INSERT INTO ARMAZENS VALUES ('01','Central',' '),('02','Filial',' ') ,('03','Apagado','*')"); err != nil {
		t.Fatal(err)
	}
	if err := eng.Exec("CREATE TABLE SX3 (X3_ARQUIVO CHAR(10), X3_CAMPO CHAR(20), X3_TIPO CHAR(1))"); err != nil {
		t.Fatal(err)
	}
	if err := eng.Exec("INSERT INTO SX3 VALUES ('ARMAZENS','A1_COD','C'),('ARMAZENS','A1_NOME','C'),('ARMAZENS','D_E_L_E_T_','C')"); err != nil {
		t.Fatal(err)
	}

	opts := &Options{dbPath: dbPath}
	srv := rest.NewServer("t", "1")
	registraFrameworkRoutes(srv, opts)
	return srv
}

func getJSON(t *testing.T, srv *rest.Server, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	var m map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &m)
	}
	return rec.Code, m
}

func TestFrameworkCompanies(t *testing.T) {
	srv := frameworkSrv(t)
	code, m := getJSON(t, srv, "/api/framework/environment/v1/companies")
	if code != 200 {
		t.Fatalf("code=%d body=%v", code, m)
	}
	items, _ := m["items"].([]any)
	if len(items) == 0 {
		t.Fatal("items vazio")
	}
	it := items[0].(map[string]any)
	if it["Code"] != "01" {
		t.Fatalf("Code = %v", it["Code"])
	}
}

func TestFrameworkBranches(t *testing.T) {
	srv := frameworkSrv(t)
	code, m := getJSON(t, srv, "/api/framework/environment/v1/branches")
	if code != 200 {
		t.Fatalf("code=%d", code)
	}
	items, _ := m["items"].([]any)
	if len(items) == 0 || items[0].(map[string]any)["Code"] != "01" {
		t.Fatalf("items = %v", items)
	}
}

func TestFrameworkCurrentUser(t *testing.T) {
	srv := frameworkSrv(t)
	code, m := getJSON(t, srv, "/api/framework/current-user")
	if code != 200 {
		t.Fatalf("code=%d", code)
	}
	if m["userName"] == nil || m["id"] == nil {
		t.Fatalf("user = %v", m)
	}
}

func TestFrameworkSystemModulesEDefaults(t *testing.T) {
	srv := frameworkSrv(t)
	for _, p := range []string{
		"/api/framework/v1/systemModules",
		"/api/protheus/session/v1/defaults",
		"/api/language/v1/protheus-suported-languages",
		"/api/brand/v1/erp-brand",
		"/api/framework/v1/FwRestTranslate/STR0001",
	} {
		code, _ := getJSON(t, srv, p)
		if code != 200 {
			t.Fatalf("GET %s = %d", p, code)
		}
	}
}

func TestFrameworkGenericListSoSX3(t *testing.T) {
	srv := frameworkSrv(t)

	// Tabela sem SX3 -> 400.
	code, _ := getJSON(t, srv, "/api/framework/v1/genericList?alias=SEM_SX3")
	if code != 400 {
		t.Fatalf("alias sem SX3 = %d, want 400", code)
	}

	// Campo fora do SX3 no filtro -> 400.
	code, _ = getJSON(t, srv, "/api/framework/v1/genericList?alias=ARMAZENS&filter="+url.QueryEscape("A9_X eq '1'"))
	if code != 400 {
		t.Fatalf("campo fora do SX3 = %d, want 400", code)
	}

	// Caminho feliz: paginação + hasNext + deletedFilter.
	code, m := getJSON(t, srv, "/api/framework/v1/genericList?alias=ARMAZENS&page=1&pageSize=1&deletedFilter=true")
	if code != 200 {
		t.Fatalf("genericList = %d %v", code, m)
	}
	items, _ := m["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %v", items)
	}
	if m["hasNext"] != true {
		t.Fatalf("hasNext = %v", m["hasNext"])
	}
	// deletedFilter=true exclui o registro com D_E_L_E_T_='*'.
	for _, it := range items {
		row := it.(map[string]any)
		if row["D_E_L_E_T_"] == "*" {
			t.Fatal("registro apagado vazou")
		}
	}
	if m["remainingRecords"].(float64) != 2 {
		t.Fatalf("remainingRecords = %v", m["remainingRecords"])
	}
}

func TestFrameworkGenericQuery(t *testing.T) {
	srv := frameworkSrv(t)
	code, m := getJSON(t, srv, "/api/framework/v1/genericQuery?tables=ARMAZENS&pageSize=10&order=-A1_COD")
	if code != 200 {
		t.Fatalf("genericQuery = %d %v", code, m)
	}
	items, _ := m["items"].([]any)
	if len(items) != 3 { // sem deletedFilter: inclui o apagado
		t.Fatalf("items = %v", items)
	}
}

func TestFrameworkOauth501(t *testing.T) {
	srv := frameworkSrv(t)
	for _, p := range []string{"/api/oauth2/v1/token", "/api/framework/mfa/hasSecret"} {
		code, m := getJSON(t, srv, p)
		if code != http.StatusNotImplemented {
			t.Fatalf("GET %s = %d, want 501", p, code)
		}
		if !strings.Contains(fmt.Sprint(m["message"]), "não suportado") {
			t.Fatalf("mensagem = %v", m)
		}
	}
}

func TestFrameworkFonteDoUsuarioVence(t *testing.T) {
	// Fonte do usuário declarando a mesma rota substitui a de framework
	// (AddRoute posterior vence method+path).
	srv := frameworkSrv(t)
	srv.AddRoute(rest.Route{
		Method: "GET",
		Path:   "/api/framework/v1/systemModules",
		Handler: func(params map[string]any) (any, error) {
			return map[string]any{"items": []any{map[string]any{"id": "X"}}, "hasNext": false}, nil
		},
	})
	code, m := getJSON(t, srv, "/api/framework/v1/systemModules")
	if code != 200 {
		t.Fatalf("code=%d", code)
	}
	items, _ := m["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != "X" {
		t.Fatalf("resposta do fonte não venceu: %v", m)
	}
}

// Garante que montaRest registra framework antes das rotas do usuário.
func TestMontaRestTemFramework(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "t.db")
	if _, err := db.NewSQLiteEngine(dbPath); err != nil {
		t.Fatal(err)
	}
	fonte := filepath.Join(dir, "vazio.prw")
	if err := os.WriteFile(fonte, []byte("User Function Main()\n\tReturn Nil\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := &Options{dbPath: dbPath}
	bc, err := compilaAlvo(fonte, opts)
	if err != nil {
		t.Fatalf("compilaAlvo: %v", err)
	}
	srv, _ := montaRest(bc, opts)
	if srv == nil {
		t.Fatal("montaRest devolveu nil")
	}
	found := false
	for _, rt := range srv.Routes() {
		if rt.Path == "/api/framework/current-user" {
			found = true
		}
	}
	if !found {
		t.Fatal("rota de framework ausente")
	}
}
