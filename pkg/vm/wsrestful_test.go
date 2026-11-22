package vm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/advpl/compiler/pkg/rest"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// wsRestfulFonte lê a fixture AdvPL do WSRESTFUL clássico.
func wsRestfulFonte(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../tests/rest_totvs/clientes_classico.prw")
	if err != nil {
		t.Fatalf("ler fixture clientes_classico.prw: %v", err)
	}
	return string(b)
}

// wsRestfulServer compila a fixture e registra as rotas WSRESTFUL num
// rest.Server real, devolvendo o httptest.Server pronto.
func wsRestfulServer(t *testing.T, prefixo string) *httptest.Server {
	t.Helper()
	bc := compilarPrg(t, wsRestfulFonte(t))
	v := NewVM(bc, false)
	s := rest.NewServer("clientes", "1.0")
	if errs := v.RegistrarWSRestful(s, prefixo); len(errs) > 0 {
		t.Fatalf("RegistrarWSRestful: %v", errs)
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func wsDo(t *testing.T, method, url, body string, headers map[string]string) (int, http.Header, string) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, string(b)
}

func wsJSON(t *testing.T, body string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("resposta não é JSON (%q): %v", body, err)
	}
	return m
}

func TestWSRestClassicoGetLista(t *testing.T) {
	srv := wsRestfulServer(t, "/rest")
	code, hdr, body := wsDo(t, "GET", srv.URL+"/rest/api/v1/clientes?page=2&ativo=true&nome=joao", "", nil)
	if code != 200 {
		t.Fatalf("status = %d, quer 200 (body %q)", code, body)
	}
	if ct := hdr.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, quer application/json", ct)
	}
	m := wsJSON(t, body)
	if m["page"] != float64(2) {
		t.Fatalf("page = %v, quer 2", m["page"])
	}
	if m["ativo"] != true {
		t.Fatalf("ativo = %v, quer true", m["ativo"])
	}
	if m["nome"] != "joao" {
		t.Fatalf("nome = %v, quer joao", m["nome"])
	}
}

func TestWSRestClassicoPathParam(t *testing.T) {
	srv := wsRestfulServer(t, "/rest")
	code, _, body := wsDo(t, "GET", srv.URL+"/rest/api/v1/clientes/000123", "", nil)
	if code != 200 {
		t.Fatalf("status = %d (body %q)", code, body)
	}
	m := wsJSON(t, body)
	if m["id"] != "000123" {
		t.Fatalf("id = %v, quer 000123", m["id"])
	}
}

func TestWSRestClassicoPostGetContent(t *testing.T) {
	srv := wsRestfulServer(t, "/rest")
	payload := `{"nome":"ana"}`
	code, hdr, body := wsDo(t, "POST", srv.URL+"/rest/api/v1/clientes", payload, map[string]string{"Content-Type": "application/json"})
	if code != 201 {
		t.Fatalf("status = %d, quer 201 (body %q)", code, body)
	}
	if ct := hdr.Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("Content-Type = %q, quer text/plain", ct)
	}
	if body != payload {
		t.Fatalf("body = %q, quer %q", body, payload)
	}
}

func TestWSRestClassicoSetRestFault(t *testing.T) {
	srv := wsRestfulServer(t, "/rest")
	code, _, body := wsDo(t, "GET", srv.URL+"/rest/api/v1/clientes/falha?page=404", "", nil)
	if code != 404 {
		t.Fatalf("status = %d, quer 404 (body %q)", code, body)
	}
	m := wsJSON(t, body)
	if m["code"] != float64(404) || m["message"] != "nao achei" {
		t.Fatalf("fault = %v", m)
	}
}

func TestWSRestErroRuntimeDevolve500EContinua(t *testing.T) {
	srv := wsRestfulServer(t, "/rest")
	code, _, body := wsDo(t, "GET", srv.URL+"/rest/api/v1/clientes/falha?page=500", "", nil)
	if code != 500 {
		t.Fatalf("status = %d, quer 500 (body %q)", code, body)
	}
	m := wsJSON(t, body)
	if m["code"] != float64(500) {
		t.Fatalf("code = %v, quer 500", m["code"])
	}
	// O servidor continua atendendo depois do erro de runtime.
	code, _, body = wsDo(t, "GET", srv.URL+"/rest/api/v1/clientes/falha?page=1", "", nil)
	if code != 200 {
		t.Fatalf("após erro, status = %d (body %q)", code, body)
	}
	m = wsJSON(t, body)
	if m["page"] != float64(1) {
		t.Fatalf("page = %v, quer 1", m["page"])
	}
}

func TestWSDataNumericaInvalidaViraNil(t *testing.T) {
	srv := wsRestfulServer(t, "/rest")
	code, _, body := wsDo(t, "GET", srv.URL+"/rest/api/v1/clientes?page=abc", "", nil)
	if code != 200 {
		t.Fatalf("status = %d (body %q)", code, body)
	}
	m := wsJSON(t, body)
	// page=abc não converte para INTEGER -> Nil -> método usa default 1.
	if m["page"] != float64(1) {
		t.Fatalf("page = %v, quer 1 (default para Nil)", m["page"])
	}
}

func TestWSRestClassicoQueryString(t *testing.T) {
	srv := wsRestfulServer(t, "/rest")
	// Ordem e repetição preservadas: primeiro par a=1, último a=3.
	code, _, body := wsDo(t, "GET", srv.URL+"/rest/api/v1/qs?a=1&b=2&a=3", "", nil)
	if code != 200 {
		t.Fatalf("status = %d (body %q)", code, body)
	}
	m := wsJSON(t, body)
	if m["n"] != float64(3) {
		t.Fatalf("n = %v, quer 3", m["n"])
	}
	if m["k1"] != "a" || m["v1"] != "1" {
		t.Fatalf("primeiro par = %v=%v, quer a=1", m["k1"], m["v1"])
	}
	if m["k2"] != "a" || m["v2"] != "3" {
		t.Fatalf("último par = %v=%v, quer a=3", m["k2"], m["v2"])
	}
}

func TestWSRestClassicoHeaders(t *testing.T) {
	srv := wsRestfulServer(t, "/rest")
	code, hdr, body := wsDo(t, "GET", srv.URL+"/rest/api/v1/headers", "", map[string]string{"X-Request-Id": "abc-123"})
	if code != 202 {
		t.Fatalf("status = %d, quer 202 (body %q)", code, body)
	}
	if h := hdr.Get("X-Resp"); h != "ok" {
		t.Fatalf("X-Resp = %q, quer ok", h)
	}
	m := wsJSON(t, body)
	if m["rid"] != "abc-123" {
		t.Fatalf("rid = %v, quer abc-123", m["rid"])
	}
}

func TestWSRestClassicoSemPathECuringa(t *testing.T) {
	srv := wsRestfulServer(t, "/rest")
	// Rota default: /rest/clientes (nome do serviço em minúsculas).
	code, _, body := wsDo(t, "GET", srv.URL+"/rest/clientes", "", nil)
	if code != 200 {
		t.Fatalf("status = %d (body %q)", code, body)
	}
	m := wsJSON(t, body)
	if m["n"] != float64(0) {
		t.Fatalf("n = %v, quer 0", m["n"])
	}
	// Wildcard: segmentos restantes viram aURLParms.
	code, _, body = wsDo(t, "GET", srv.URL+"/rest/clientes/a/b", "", nil)
	if code != 200 {
		t.Fatalf("status wildcard = %d (body %q)", code, body)
	}
	m = wsJSON(t, body)
	if m["n"] != float64(2) || m["a"] != "a" || m["b"] != "b" {
		t.Fatalf("wildcard = %v", m)
	}
}

func TestWSRestConcorrenteIsolado(t *testing.T) {
	srv := wsRestfulServer(t, "/rest")
	const n = 16
	var wg sync.WaitGroup
	erros := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			page := 10 + i
			url := srv.URL + "/rest/api/v1/clientes/falha?page=" + strconv.Itoa(page)
			code, _, body := wsDo(t, "GET", url, "", nil)
			if code != 200 {
				erros <- "status " + strconv.Itoa(code)
				return
			}
			m := wsJSON(t, body)
			if m["page"] != float64(page) {
				erros <- "page divergente"
			}
		}(i)
	}
	wg.Wait()
	close(erros)
	for e := range erros {
		t.Error(e)
	}
}

func TestWSRestServerAntigoIntacto(t *testing.T) {
	// Fonte com rota anotada (@Get) + WSRESTFUL: WSRestServer:New deve
	// registrar as duas famílias sem quebrar a anotada.
	src := wsRestfulFonte(t) + `
@Get("/ping")
User Function Ping()
Return {"ok": .T.}
`
	bc := compilarPrg(t, src)
	v := NewVM(bc, false)
	obj := newWSRestServerObject()
	if err := v.callWSRestServerMethod(obj, "NEW", []advplrt.Value{advplrt.NewString("t"), advplrt.NewString("1")}); err != nil {
		t.Fatalf("NEW: %v", err)
	}
	st, ok := obj.Native.(*restState)
	if !ok {
		t.Fatal("obj sem restState")
	}
	var temPing, temWS bool
	for _, r := range st.server.Routes() {
		if r.Method == "GET" && r.Path == "/ping" {
			temPing = true
		}
		if r.Method == "GET" && r.Path == "/api/v1/clientes" {
			temWS = true
		}
	}
	if !temPing {
		t.Error("rota anotada @Get /ping não registrada")
	}
	if !temWS {
		t.Error("rota WSRESTFUL /api/v1/clientes não registrada")
	}
}
