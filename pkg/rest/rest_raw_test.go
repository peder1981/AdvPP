package rest

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRawRecebeTudo verifica que uma rota Raw recebe o Request de baixo
// nível completo: método, path, path params (nomes como declarados),
// query string com ordem original preservada, headers e corpo cru.
func TestRawRecebeTudo(t *testing.T) {
	var got *Request
	s := NewServer("t", "1")
	s.AddRoute(Route{
		Method: "GET",
		Path:   "/a/{id}",
		Raw: func(r *Request) *Response {
			got = r
			return &Response{Body: []byte(`{}`)}
		},
	})

	req := httptest.NewRequest("GET", "/a/42?x=1&y=2&x=3", strings.NewReader("corpo cru"))
	req.Header.Set("X-K", "v")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
	}
	if got == nil {
		t.Fatal("Raw não foi chamado")
	}
	if got.Method != "GET" {
		t.Errorf("Method = %q", got.Method)
	}
	if got.Path != "/a/42" {
		t.Errorf("Path = %q", got.Path)
	}
	if got.PathParams["id"] != "42" {
		t.Errorf("PathParams[id] = %q", got.PathParams["id"])
	}
	wantOrder := []string{"x", "y", "x"} // uma entrada por par, repetições preservadas
	if len(got.QueryOrder) != len(wantOrder) {
		t.Fatalf("QueryOrder = %v", got.QueryOrder)
	}
	for i := range wantOrder {
		if got.QueryOrder[i] != wantOrder[i] {
			t.Fatalf("QueryOrder = %v", got.QueryOrder)
		}
	}
	if got.Query.Get("x") != "1" || got.Query.Get("y") != "2" {
		t.Errorf("Query = %v", got.Query)
	}
	wantPairs := []QueryPair{{Key: "x", Value: "1"}, {Key: "y", Value: "2"}, {Key: "x", Value: "3"}}
	if len(got.QueryPairs) != len(wantPairs) {
		t.Fatalf("QueryPairs = %v", got.QueryPairs)
	}
	for i := range wantPairs {
		if got.QueryPairs[i] != wantPairs[i] {
			t.Fatalf("QueryPairs = %v", got.QueryPairs)
		}
	}
	if got.Header.Get("X-K") != "v" {
		t.Errorf("Header[X-K] = %q", got.Header.Get("X-K"))
	}
	if string(got.Body) != "corpo cru" {
		t.Errorf("Body = %q", got.Body)
	}
}

// TestRawStatusEHeaders verifica que o Response de baixo nível controla
// status, headers extras e Content-Type (vazio = application/json).
func TestRawStatusEHeaders(t *testing.T) {
	s := NewServer("t", "1")
	s.AddRoute(Route{
		Method: "POST",
		Path:   "/b",
		Raw: func(r *Request) *Response {
			return &Response{
				Status:      201,
				ContentType: "",
				Header:      map[string]string{"X-T": "1"},
				Body:        []byte(`{}`),
			}
		},
	})

	req := httptest.NewRequest("POST", "/b", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("X-T") != "1" {
		t.Errorf("X-T = %q", rec.Header().Get("X-T"))
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if rec.Body.String() != `{}` {
		t.Errorf("body = %q", rec.Body.String())
	}
}

// TestNormalizaPathDoisPontos converte o estilo :param (Gin/TOTVS) para o
// {param} do ServeMux, preservando maiúsculas no nome.
func TestNormalizaPathDoisPontos(t *testing.T) {
	if got := NormalizaPath("/api/x/:InternalID"); got != "/api/x/{InternalID}" {
		t.Fatalf("NormalizaPath = %q", got)
	}
	if got := NormalizaPath("api/x/:id/b"); got != "/api/x/{id}/b" {
		t.Fatalf("NormalizaPath = %q", got)
	}
	if got := NormalizaPath("/api/x/{id}/b"); got != "/api/x/{id}/b" {
		t.Fatalf("NormalizaPath = %q", got)
	}
	if got := NormalizaPath(""); got != "/" {
		t.Fatalf("NormalizaPath(\"\") = %q", got)
	}
}

// TestAddRouteConflitoNaoEntraEmPanico registra duas rotas que se sobrepõem
// sem especificidade ("/api/x/{a}" e "/api/{b}/y"): AddRouteE devolve erro
// citando as duas, e o Handler segue servindo a primeira sem panicar.
func TestAddRouteConflitoNaoEntraEmPanico(t *testing.T) {
	s := NewServer("t", "1")
	if err := s.AddRouteE(Route{Method: "GET", Path: "/api/x/{a}", Handler: okHandler("primeira")}); err != nil {
		t.Fatalf("primeira rota: %v", err)
	}
	err := s.AddRouteE(Route{Method: "GET", Path: "/api/{b}/y", Handler: okHandler("segunda")})
	if err == nil {
		t.Fatal("esperava erro de conflito")
	}
	if !strings.Contains(err.Error(), "/api/x/{a}") || !strings.Contains(err.Error(), "/api/{b}/y") {
		t.Fatalf("erro não cita as duas rotas: %v", err)
	}

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/x/1", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "primeira") {
		t.Fatalf("rota original não responde: %d %q", rec.Code, rec.Body.String())
	}
}

// TestRotasAntigasIntactas garante que Route.Handler (map de params) segue
// funcionando como antes: params mesclados, resposta JSON 200.
func TestRotasAntigasIntactas(t *testing.T) {
	s := NewServer("t", "1")
	s.AddRoute(Route{
		Method: "GET",
		Path:   "/cli/{id}",
		Handler: func(params map[string]any) (any, error) {
			return map[string]any{"id": params["ID"], "q": params["Q"]}, nil
		},
	})

	req := httptest.NewRequest("GET", "/cli/7?q=abc", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"id":"7"`) || !strings.Contains(body, `"q":"abc"`) {
		t.Fatalf("body = %q", body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
}

// TestRotaComDoisPontosAtende verifica que um path declarado no estilo
// ":param" é normalizado no mux e chega ao Raw com o nome declarado.
func TestRotaComDoisPontosAtende(t *testing.T) {
	var got *Request
	s := NewServer("t", "1")
	if err := s.AddRouteE(Route{
		Method: "GET",
		Path:   "/item/:InternalID",
		Raw: func(r *Request) *Response {
			got = r
			return &Response{Body: []byte(`{}`)}
		},
	}); err != nil {
		t.Fatalf("AddRouteE: %v", err)
	}

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/item/A1", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if got == nil || got.PathParams["InternalID"] != "A1" {
		t.Fatalf("PathParams = %v", got.PathParams)
	}
}

// TestHandlerDoisPontosAtende verifica que Route.Handler (map de params)
// também resolve path params declarados no estilo ":param" — o dispatch
// normaliza o path antes de extrair os nomes.
func TestHandlerDoisPontosAtende(t *testing.T) {
	s := NewServer("t", "1")
	s.AddRoute(Route{
		Method: "GET",
		Path:   "/item/:InternalID",
		Handler: func(params map[string]any) (any, error) {
			return map[string]any{"id": params["INTERNALID"]}, nil
		},
	})

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/item/B2", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"B2"`) {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

// TestRawTemPrecedenciaSobreHandler garante que, quando Route tem Raw e
// Handler preenchidos, só o Raw é chamado.
func TestRawTemPrecedenciaSobreHandler(t *testing.T) {
	rawCalled, handlerCalled := false, false
	s := NewServer("t", "1")
	s.AddRoute(Route{
		Method: "GET",
		Path:   "/p",
		Handler: func(map[string]any) (any, error) {
			handlerCalled = true
			return "handler", nil
		},
		Raw: func(r *Request) *Response {
			rawCalled = true
			return &Response{Body: []byte(`{"raw":true}`)}
		},
	})

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/p", nil))
	if !rawCalled || handlerCalled {
		t.Fatalf("rawCalled=%v handlerCalled=%v", rawCalled, handlerCalled)
	}
	if !strings.Contains(rec.Body.String(), `"raw":true`) {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

// TestCorpoAcimaDoLimiteDevolve400 garante que um corpo maior que
// maxBodyBytes é rejeitado pelo limitBody antes de chegar ao Raw.
func TestCorpoAcimaDoLimiteDevolve400(t *testing.T) {
	rawCalled := false
	s := NewServer("t", "1")
	s.AddRoute(Route{
		Method: "POST",
		Path:   "/up",
		Raw: func(r *Request) *Response {
			rawCalled = true
			return &Response{Body: []byte(`{}`)}
		},
	})

	big := strings.Repeat("a", maxBodyBytes+1)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("POST", "/up", strings.NewReader(big)))

	if rec.Code != 400 {
		t.Fatalf("status = %d, esperava 400", rec.Code)
	}
	if rawCalled {
		t.Fatal("Raw não deveria ter sido chamado com corpo acima do limite")
	}
}

// TestAddRouteESubstituiRota garante que registrar de novo o mesmo
// método+path via AddRouteE substitui o handler, sem erro.
func TestAddRouteESubstituiRota(t *testing.T) {
	s := NewServer("t", "1")
	if err := s.AddRouteE(Route{Method: "GET", Path: "/r", Handler: okHandler("v1")}); err != nil {
		t.Fatalf("1ª AddRouteE: %v", err)
	}
	if err := s.AddRouteE(Route{Method: "GET", Path: "/r", Handler: okHandler("v2")}); err != nil {
		t.Fatalf("2ª AddRouteE: %v", err)
	}

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/r", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "v2") {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "v1") {
		t.Fatalf("handler antigo ainda ativo: %q", rec.Body.String())
	}
}

// TestAddRouteEIgnoraConflitoAntigo garante que um par de rotas
// conflitantes introduzido via AddRoute (que não valida) não impede o
// AddRouteE de registrar uma rota nova e independente.
func TestAddRouteEIgnoraConflitoAntigo(t *testing.T) {
	s := NewServer("t", "1")
	// Conflito introduzido "por baixo" via AddRoute: a 2ª é descartada
	// na montagem do mux, mas fica em s.routes.
	s.AddRoute(Route{Method: "GET", Path: "/api/x/{a}", Handler: okHandler("a")})
	s.AddRoute(Route{Method: "GET", Path: "/api/{b}/y", Handler: okHandler("b")})

	if err := s.AddRouteE(Route{Method: "GET", Path: "/ok", Handler: okHandler("ok")}); err != nil {
		t.Fatalf("AddRouteE de rota independente falhou por conflito antigo: %v", err)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/ok", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "ok") {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

// TestRotasTardiasRemontamMux garante que rotas registradas depois do
// primeiro uso do Handler() passam a responder (remontagem por versão).
func TestRotasTardiasRemontamMux(t *testing.T) {
	s := NewServer("t", "1")
	h := s.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/nova", nil))
	if rec.Code != 404 {
		t.Fatalf("antes do registro: %d", rec.Code)
	}

	s.AddRoute(Route{Method: "GET", Path: "/nova", Handler: okHandler("ok")})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/nova", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "ok") {
		t.Fatalf("depois do registro: %d %q", rec.Code, rec.Body.String())
	}
}

func okHandler(msg string) func(map[string]any) (any, error) {
	return func(map[string]any) (any, error) { return msg, nil }
}
