package main

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// isolaMFA aponta o store de segredos MFA para um arquivo temporário —
// sem isso os testes leriam/gravariam ~/.advpp/mfa_secrets.json real.
func isolaMFA(t *testing.T) {
	t.Helper()
	old := mfa
	mfa = &mfaStore{secrets: map[string]*mfaSegredo{}}
	mfa.path = filepath.Join(t.TempDir(), "mfa_secrets.json")
	t.Cleanup(func() { mfa = old })
}

// TestFrameworkOauthToken valida o token local: JWT "header.payload."
// decodificável com userid/exp, e MFA respondendo o contrato
// {hasSecret:false} para a lib-core pular o fluxo.
func TestFrameworkOauthToken(t *testing.T) {
	isolaMFA(t)
	srv := frameworkSrv(t)
	req := httptest.NewRequest("GET", "/api/framework/mfa/hasSecret", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("hasSecret = %d", rec.Code)
	}
	var hs map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &hs); err != nil || hs["hasSecret"] != false {
		t.Fatalf("hasSecret = %d %q", rec.Code, rec.Body.String())
	}
	// POST no token (fluxo password/refresh do lib-core).
	req = httptest.NewRequest("POST", "/api/oauth2/v1/token", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("POST token = %d", rec.Code)
	}
	var tm map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &tm); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	jwt, _ := tm["access_token"].(string)
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("access_token não é JWT: %q", jwt)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("payload não decodifica: %v", err)
	}
	var pj map[string]any
	if err := json.Unmarshal(payload, &pj); err != nil {
		t.Fatalf("payload não é JSON: %v", err)
	}
	if pj["userid"] == nil || pj["exp"] == nil {
		t.Fatalf("payload sem userid/exp: %v", pj)
	}
}

// TestFrameworkMFATotpPercorre o fluxo real de enrollment: TOTPURI gera o
// segredo (otpauth://), tok valida o código RFC 6238 de 6 dígitos e, no
// primeiro acerto, confirma o segredo — hasSecret passa a true.
func TestFrameworkMFATotpPercorre(t *testing.T) {
	isolaMFA(t)
	srv := frameworkSrv(t)

	// 1) hasSecret começa false (sem segredo confirmado).
	code, m := getJSON(t, srv, "/api/framework/mfa/hasSecret")
	if code != 200 || m["hasSecret"] != false {
		t.Fatalf("hasSecret inicial = %d %v", code, m)
	}

	// 2) TOTPURI devolve otpauth:// com o segredo base32.
	code, m = getJSON(t, srv, "/api/framework/mfa/TOTPURI")
	if code != 200 {
		t.Fatalf("TOTPURI = %d", code)
	}
	uri, _ := m["cQrCode"].(string)
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Fatalf("cQrCode = %q", uri)
	}
	q, err := url.ParseQuery(strings.SplitN(uri, "?", 2)[1])
	if err != nil {
		t.Fatalf("parse otpauth: %v", err)
	}
	secretB32 := q.Get("secret")
	if secretB32 == "" {
		t.Fatalf("otpauth sem secret: %q", uri)
	}
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secretB32)
	if err != nil {
		t.Fatalf("secret não é base32: %v", err)
	}

	// 3) Código errado -> tok:0 e hasSecret continua false.
	code, m = getJSON(t, srv, "/api/framework/mfa/tok?token=000000")
	if code != 200 || m["tok"] != float64(0) {
		t.Fatalf("tok errado = %d %v", code, m)
	}

	// 4) Código TOTP válido do período corrente -> tok:1 + confirmação.
	contador := uint64(time.Now().Unix() / 30)
	valido := fmt.Sprintf("%06d", totpCode(raw, contador))
	code, m = getJSON(t, srv, "/api/framework/mfa/tok?token="+valido)
	if code != 200 || m["tok"] != float64(1) {
		t.Fatalf("tok válido = %d %v", code, m)
	}
	code, m = getJSON(t, srv, "/api/framework/mfa/hasSecret")
	if code != 200 || m["hasSecret"] != true {
		t.Fatalf("hasSecret após confirmação = %d %v", code, m)
	}

	// 5) Código seguinte (período +1, dentro da janela ±1) também valida.
	prox := fmt.Sprintf("%06d", totpCode(raw, contador+1))
	if prox == valido {
		prox = fmt.Sprintf("%06d", totpCode(raw, contador+2))
	}
	_, m = getJSON(t, srv, "/api/framework/mfa/tok?token="+prox)
	if m["tok"] != float64(1) {
		t.Fatalf("tok janela +1 = %v", m)
	}

	// 6) O segredo persiste no store isolado.
	sg := mfa.segredoDo(mustUsuario())
	if sg == nil || !sg.Confirmed {
		t.Fatal("segredo não confirmado no store")
	}
}

func mustUsuario() string {
	_, _, _, u := frameworkSessao()
	return u
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
	eng, err := db.NewSQLiteEngine(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// Fecha já: o engine não é usado adiante e no Windows o handle aberto
	// impede o cleanup do TempDir.
	eng.Close()
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
