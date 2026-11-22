package vm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/rest"
)

// orestFonte lê a fixture TLPP das rotas anotadas com oRest.
func orestFonte(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../tests/rest_totvs/orest_api.tlpp")
	if err != nil {
		t.Fatalf("ler fixture orest_api.tlpp: %v", err)
	}
	return string(b)
}

// orestServer compila a fixture e registra as rotas anotadas (@Get...) num
// rest.Server real via autoRegisterAnnotatedRoutes.
func orestServer(t *testing.T) *httptest.Server {
	t.Helper()
	bc := compilarPrg(t, orestFonte(t))
	v := NewVM(bc, false)
	s := rest.NewServer("orest", "1.0")
	v.autoRegisterAnnotatedRoutes(s)
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func orestDo(t *testing.T, method, url, body string, headers map[string]string) (int, http.Header, string) {
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

func orestJSON(t *testing.T, body string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("resposta não é JSON (%q): %v", body, err)
	}
	return m
}

func TestORestQueryEPath(t *testing.T) {
	srv := orestServer(t)
	code, _, body := orestDo(t, "GET", srv.URL+"/api/t/v1/item/A1?x=1", "", nil)
	if code != 200 {
		t.Fatalf("status = %d (body %q)", code, body)
	}
	m := orestJSON(t, body)
	if m["code"] != "A1" {
		t.Fatalf("code = %v, quer A1", m["code"])
	}
	if m["x"] != "1" {
		t.Fatalf("x = %v, quer 1", m["x"])
	}
}

func TestORestStatusEHeader(t *testing.T) {
	srv := orestServer(t)
	code, hdr, body := orestDo(t, "GET", srv.URL+"/api/t/v1/status", "", nil)
	if code != 201 {
		t.Fatalf("status = %d, quer 201 (body %q)", code, body)
	}
	if h := hdr.Get("X-Total"); h != "7" {
		t.Fatalf("X-Total = %q, quer 7", h)
	}
	if body != "criado" {
		t.Fatalf("body = %q, quer criado", body)
	}
}

func TestORestJsonObjectSerializado(t *testing.T) {
	srv := orestServer(t)
	code, hdr, body := orestDo(t, "GET", srv.URL+"/api/t/v1/item/B2?x=9", "", nil)
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if ct := hdr.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, quer application/json", ct)
	}
	m := orestJSON(t, body)
	if m["code"] != "B2" || m["x"] != "9" {
		t.Fatalf("corpo = %v", m)
	}
}

func TestAnotacaoEndpointNomeado(t *testing.T) {
	srv := orestServer(t)
	code, _, body := orestDo(t, "GET", srv.URL+"/api/t/v1/n", "", nil)
	if code != 200 {
		t.Fatalf("status = %d (body %q)", code, body)
	}
	m := orestJSON(t, body)
	if m["OK"] != true {
		t.Fatalf("corpo = %v", m)
	}
}

func TestORestSetFault(t *testing.T) {
	srv := orestServer(t)
	code, _, body := orestDo(t, "GET", srv.URL+"/api/t/v1/falha", "", nil)
	if code != 400 {
		t.Fatalf("status = %d, quer 400 (body %q)", code, body)
	}
	m := orestJSON(t, body)
	if m["CODE"] != float64(422) || m["MESSAGE"] != "invalido" {
		t.Fatalf("fault = %v", m)
	}
}

func TestORestGetBodyRequest(t *testing.T) {
	srv := orestServer(t)
	payload := `{"a":1}`
	code, _, body := orestDo(t, "POST", srv.URL+"/api/t/v1/eco", payload, map[string]string{"Content-Type": "application/json"})
	if code != 200 {
		t.Fatalf("status = %d (body %q)", code, body)
	}
	if body != payload {
		t.Fatalf("body = %q, quer %q", body, payload)
	}
}

func TestORestShadowedByLocal(t *testing.T) {
	// Um `Local oRest` na função sombreia o oRest dinâmico injetado — o
	// corpus usa oRest como variável de cliente (FWRest etc.).
	src := `
@Get("/sombra")
User Function SombraORest(oParam)
Local oRest := "sou-local"
Return {"v": oRest}
`
	bc := compilarPrg(t, src)
	v := NewVM(bc, false)
	s := rest.NewServer("t", "1")
	v.autoRegisterAnnotatedRoutes(s)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	code, _, body := orestDo(t, "GET", srv.URL+"/sombra", "", nil)
	if code != 200 {
		t.Fatalf("status = %d (body %q)", code, body)
	}
	m := orestJSON(t, body)
	if m["V"] != "sou-local" {
		t.Fatalf("corpo = %v, quer V=sou-local", m)
	}
}

func TestAnotadoSemORestIntacto(t *testing.T) {
	// Função anotada que NÃO usa oRest continua recebendo o objeto de
	// parâmetros e devolve o valor de retorno como corpo (comportamento
	// anterior à Task 6).
	src := `
@Get("/ping/:id")
User Function PingAntigo(oParam)
Return {"id": oParam:ID, "q": oParam:Q}
`
	bc := compilarPrg(t, src)
	v := NewVM(bc, false)
	s := rest.NewServer("t", "1")
	v.autoRegisterAnnotatedRoutes(s)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	code, _, body := orestDo(t, "GET", srv.URL+"/ping/42?q=oi", "", nil)
	if code != 200 {
		t.Fatalf("status = %d (body %q)", code, body)
	}
	m := orestJSON(t, body)
	if m["ID"] != "42" || m["Q"] != "oi" {
		t.Fatalf("corpo = %v", m)
	}
}
