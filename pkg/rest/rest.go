// Package rest implementa um servidor HTTP REST mínimo, real, sobre
// net/http puro (sem CGO, sem dependências externas — mesmo padrão do
// resto do AdvPP). É o análogo, para WSRESTFUL, do que pkg/mcp é para
// MCPServer: o pacote não sabe nada de AdvPL/VM, só expõe rotas
// (verbo HTTP + path, com suporte a parâmetros `{nome}` via o roteador
// nativo do Go 1.22+) e despacha para um Handler Go — quem faz a ponte
// com a VM (rodar a função AdvPL, converter JSON<->advplrt.Value) é
// pkg/vm/rest_native.go.
package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Limites de endurecimento do servidor HTTP (FULL-REVIEW A2). Sem eles, o
// servidor ficava vulnerável a Slowloris (conexões lentas segurando
// goroutines) e a corpos de requisição arbitrariamente grandes.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 1 << 20  // 1 MiB de cabeçalhos
	maxBodyBytes      = 10 << 20 // 10 MiB de corpo por requisição
)

// Route é um endpoint REST: verbo HTTP + path (aceita `{param}` no estilo
// do ServeMux do Go 1.22+, ex.: "/clientes/{id}") + o handler que resolve
// a chamada.
type Route struct {
	Method string
	Path   string
	// Handler recebe os parâmetros já resolvidos (path params + query
	// string + corpo JSON decodificado, todos mesclados num único mapa —
	// ver precedência em (*Server).ServeHTTP) e devolve o valor Go a ser
	// serializado como corpo JSON da resposta (200) ou um erro (500). Se o
	// valor devolvido for um RawResponse, o corpo vai para a resposta
	// exatamente como está (sem JSON), com o Content-Type e status HTTP
	// que o RawResponse pedir — usado por rotas que servem HTML/texto
	// puro em vez de uma API JSON (ex.: uma página de login).
	Handler func(params map[string]any) (any, error)

	// Raw é o handler de baixo nível: recebe a requisição HTTP crua
	// (método, path, path params, query com ordem original, headers e
	// corpo) e devolve a resposta completa (status, headers, Content-Type
	// e corpo). Quando não-nil, tem precedência sobre Handler — é o que
	// o dispatch do WSRESTFUL clássico (pkg/vm/wsrestful.go) usa.
	Raw func(*Request) *Response
}

// QueryPair é um par chave=valor da query string já decodificado.
type QueryPair struct {
	Key   string
	Value string
}

// Request é a visão de baixo nível de uma requisição HTTP entregue a um
// Route.Raw.
type Request struct {
	Method     string
	Path       string
	PathParams map[string]string // nomes como declarados no path da rota
	Query      url.Values        // pares chave=valor da query string
	// QueryOrder tem uma entrada por par da query, na ordem exata em que
	// aparecem na URL — chaves repetidas aparecem repetidas (ex.:
	// "?x=1&y=2&x=3" -> ["x","y","x"]). Para chave+valor juntos use
	// QueryPairs, que preserva cada par decodificado na mesma ordem.
	QueryOrder []string
	QueryPairs []QueryPair
	Header     http.Header
	Body       []byte
}

// Response é a resposta completa escrita por um Route.Raw.
// Status 0 vira 200; ContentType "" vira "application/json".
type Response struct {
	Status      int
	ContentType string
	Header      map[string]string
	Body        []byte
}

// RawResponse é um corpo de resposta HTTP que não passa pelo
// serializador JSON — o handler assume controle total do Content-Type e
// dos bytes exatos da resposta. Devolvida por um Handler no lugar do
// valor normal quando a rota precisa emitir HTML/texto/binário puro.
type RawResponse struct {
	// Status é o código HTTP; 0 vira 200.
	Status int
	// ContentType vai literal no header Content-Type; vazio vira
	// "text/plain; charset=utf-8".
	ContentType string
	Body        []byte
}

// Server é um servidor REST servindo um conjunto fixo de rotas sobre HTTP.
type Server struct {
	Name    string
	Version string

	mu     sync.Mutex
	routes []Route

	// version incrementa a cada AddRoute/AddRouteE bem-sucedido; o mux
	// cacheado (cachedMux) é remontado na próxima requisição quando a
	// versão muda — rotas registradas depois do primeiro uso de
	// Handler() passam a responder sem precisar recriar o handler.
	version    uint64
	cachedMux  *http.ServeMux
	cachedVer  uint64
	httpServer *http.Server
}

// NewServer cria um servidor REST vazio.
func NewServer(name, version string) *Server {
	return &Server{Name: name, Version: version}
}

// AddRoute registra (ou substitui, se method+path já existirem) uma rota.
// Mantida sem retorno de erro por compatibilidade: conflitos de pattern
// que o ServeMux rejeitaria são ignorados na montagem do mux (a primeira
// rota registrada vence) — para validação explícita use AddRouteE.
func (s *Server) AddRoute(r Route) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addRouteLocked(r)
}

// addRouteLocked aplica a normalização (método em maiúsculas) e a
// semântica de substituição por method+path. s.mu deve estar travado.
func (s *Server) addRouteLocked(r Route) {
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	r.Method = method
	for i, existing := range s.routes {
		if existing.Method == r.Method && existing.Path == r.Path {
			s.routes[i] = r
			s.version++
			return
		}
	}
	s.routes = append(s.routes, r)
	s.version++
}

// AddRouteE registra uma rota como AddRoute, mas antes valida se o pattern
// resultante conflita com alguma rota já registrada (o http.ServeMux do
// Go entra em pânico em conflitos — ex.: "/api/x/{a}" e "/api/{b}/y" no
// mesmo verbo). Em caso de conflito, devolve um erro citando as duas
// rotas e NÃO registra a nova. Rotas idênticas (mesmo método e path já
// registrado) são substituídas, como em AddRoute.
func (s *Server) AddRouteE(r Route) error {
	r.Method = strings.ToUpper(strings.TrimSpace(r.Method))

	s.mu.Lock()
	defer s.mu.Unlock()

	// existing = rotas atuais sem a que seria substituída (mesmo
	// método+path). A validação cobre APENAS a candidata contra elas:
	// conflitos antigos entre rotas já registradas via AddRoute (que não
	// valida) não podem impedir o registro de uma rota nova e sadia.
	existing := make([]Route, 0, len(s.routes))
	for _, e := range s.routes {
		if e.Method == r.Method && e.Path == r.Path {
			continue // será substituída por r
		}
		existing = append(existing, e)
	}

	if err := checkCandidateConflict(existing, r); err != nil {
		return err
	}

	s.addRouteLocked(r)
	return nil
}

// checkCandidateConflict valida a rota candidata contra as já
// registradas: monta um ServeMux de prova registrando as existentes na
// ordem (falhas entre elas são ignoradas, como no buildMux — a primeira
// vence) e tenta registrar a candidata por último. Um pânico do mux
// (conflito de patterns) vira erro citando as rotas. Patterns
// normalizados idênticos são detectados antes, com erro próprio citando
// as duas declarações.
func checkCandidateConflict(existing []Route, cand Route) error {
	candPattern := routePattern(cand)
	mux := http.NewServeMux()
	noop := func(http.ResponseWriter, *http.Request) {}
	for _, e := range existing {
		p := routePattern(e)
		if p == candPattern {
			return fmt.Errorf("rest: rotas conflitantes %s %q e %s %q (mesmo pattern %q)",
				e.Method, e.Path, cand.Method, cand.Path, p)
		}
		tryHandle(mux, p, noop) // conflito antigo: ignora, como no buildMux
	}
	ok, panicMsg := tryHandle(mux, candPattern, noop)
	if !ok {
		return fmt.Errorf("rest: rota %s %q conflita com outra rota registrada: %s",
			cand.Method, cand.Path, panicMsg)
	}
	return nil
}

// tryHandle registra pattern no mux capturando o pânico do ServeMux
// (conflito de patterns). Devolve ok=false e a mensagem do pânico em caso
// de conflito.
func tryHandle(mux *http.ServeMux, pattern string, h http.HandlerFunc) (ok bool, panicMsg string) {
	defer func() {
		if rec := recover(); rec != nil {
			panicMsg = fmt.Sprint(rec)
			ok = false
		}
	}()
	mux.HandleFunc(pattern, h)
	return true, ""
}

// routePattern monta o pattern do ServeMux para a rota: "METHOD /path"
// (ou só "/path" quando Method é vazio), com o path normalizado.
func routePattern(route Route) string {
	path := NormalizaPath(route.Path)
	if route.Method != "" {
		return route.Method + " " + path
	}
	return path
}

// Routes devolve uma cópia ordenada das rotas registradas — usado por
// /_routes (introspecção, análogo ao tools/list do MCP) e por testes.
func (s *Server) Routes() []Route {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Route, len(s.routes))
	copy(out, s.routes)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// buildMux monta o http.ServeMux nativo do Go (padrões "METHOD /path/{p}"
// suportados desde 1.22) a partir das rotas registradas. s.mu deve estar
// travado. Registros que o mux rejeitaria (conflito de patterns vindos de
// AddRoute, que não valida) são pulados — a primeira rota registrada
// vence — para que uma rota ruim nunca tire o servidor do ar.
func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	for _, route := range s.routes {
		route := route
		pattern := routePattern(route)
		handler := func(w http.ResponseWriter, r *http.Request) {
			s.dispatch(w, r, route)
		}
		tryHandle(mux, pattern, handler)
	}
	// Introspecção: lista as rotas registradas, útil para debug manual e
	// para o cliente descobrir a API sem precisar da doc externa.
	tryHandle(mux, "GET /_routes", func(w http.ResponseWriter, r *http.Request) {
		registered := s.Routes()
		list := make([]map[string]string, 0, len(registered))
		for _, route := range registered {
			list = append(list, map[string]string{"method": route.Method, "path": route.Path})
		}
		writeJSON(w, http.StatusOK, list)
	})
	return mux
}

// Handler devolve o http.Handler do servidor (mux + limitBody), pronto
// para ser montado por outro roteador — ex.: o mux do pkg/webui no
// "advplc serve". O mux é remontado sob demanda quando uma rota nova é
// registrada depois do primeiro uso (mutex interno), então o mesmo
// handler pode ser capturado uma única vez.
func (s *Server) Handler() http.Handler {
	return limitBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		if s.cachedMux == nil || s.cachedVer != s.version {
			s.cachedMux = s.buildMux()
			s.cachedVer = s.version
		}
		mux := s.cachedMux
		s.mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
}

// dispatch despacha a requisição para a rota. Route.Raw recebe o Request
// de baixo nível e escreve o Response direto; Route.Handler recebe o mapa
// de parâmetros mesclado (path + query + corpo JSON) como antes.
func (s *Server) dispatch(w http.ResponseWriter, r *http.Request, route Route) {
	if route.Raw != nil {
		s.dispatchRaw(w, r, route)
		return
	}
	params := map[string]any{}

	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			params[strings.ToUpper(k)] = v[0]
		}
	}

	// Path params: Go 1.22+ expõe cada `{nome}` do pattern via r.PathValue.
	// O path é normalizado antes (:param -> {param}) porque o mux registra
	// o pattern normalizado — sem isso, rotas ":param" não populam params.
	for _, name := range pathParamNames(NormalizaPath(route.Path)) {
		params[strings.ToUpper(name)] = r.PathValue(name)
	}

	if r.Body != nil {
		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "corpo da requisição inválido: " + err.Error()})
			return
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			var bodyMap map[string]any
			if err := json.Unmarshal(body, &bodyMap); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "corpo JSON inválido: " + err.Error()})
				return
			}
			for k, v := range bodyMap {
				params[strings.ToUpper(k)] = v
			}
		}
	}

	result, err := route.Handler(params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if raw, ok := result.(RawResponse); ok {
		writeRaw(w, raw)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// dispatchRaw monta o Request de baixo nível e chama route.Raw, depois
// escreve o Response devolvido. Corpo maior que maxBodyBytes (já limitado
// por limitBody no Handler) vira 400 sem chamar o handler.
func (s *Server) dispatchRaw(w http.ResponseWriter, r *http.Request, route Route) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "corpo da requisição inválido: " + err.Error()})
		return
	}

	pathParams := map[string]string{}
	for _, name := range pathParamNames(NormalizaPath(route.Path)) {
		if v := r.PathValue(name); v != "" {
			pathParams[name] = v
		}
	}

	queryOrder, queryPairs := parseQueryOrder(r.URL.RawQuery)
	req := &Request{
		Method:     r.Method,
		Path:       r.URL.Path,
		PathParams: pathParams,
		Query:      r.URL.Query(),
		QueryOrder: queryOrder,
		QueryPairs: queryPairs,
		Header:     r.Header.Clone(),
		Body:       body,
	}

	resp := route.Raw(req)
	if resp == nil {
		resp = &Response{}
	}
	writeResponse(w, resp)
}

// parseQueryOrder percorre a query string crua e devolve, na ordem exata
// em que os pares aparecem na URL: (1) uma lista com a chave de cada par
// (repetições preservadas — "?x=1&y=2&x=3" -> ["x","y","x"]) e (2) a lista
// de QueryPair decodificados. url.Values é um mapa e perde repetição e
// intercalação — informação que o WSRESTFUL clássico precisa para
// montar aQueryString.
func parseQueryOrder(rawQuery string) (order []string, pairs []QueryPair) {
	if rawQuery == "" {
		return nil, nil
	}
	for _, part := range strings.Split(rawQuery, "&") {
		if part == "" {
			continue
		}
		key, value := part, ""
		if i := strings.IndexByte(key, '='); i >= 0 {
			value = key[i+1:]
			key = key[:i]
		}
		if decoded, err := url.QueryUnescape(key); err == nil {
			key = decoded
		}
		if decoded, err := url.QueryUnescape(value); err == nil {
			value = decoded
		}
		order = append(order, key)
		pairs = append(pairs, QueryPair{Key: key, Value: value})
	}
	return order, pairs
}

// writeResponse escreve um Response de Route.Raw: status (0 = 200),
// headers extras e corpo cru. Content-Type vazio vira application/json.
func writeResponse(w http.ResponseWriter, resp *Response) {
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	contentType := resp.ContentType
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	for k, v := range resp.Header {
		w.Header().Set(k, v)
	}
	w.WriteHeader(status)
	_, _ = w.Write(resp.Body)
}

// NormalizaPath converte um path de rota para o formato do ServeMux do
// Go: garante "/" inicial e traduz segmentos ":param" (estilo Gin/TOTVS)
// para "{param}". Paths já no formato "{param}" passam intactos.
func NormalizaPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	var b strings.Builder
	b.Grow(len(p) + 8)
	for i := 0; i < len(p); {
		c := p[i]
		if c == '/' && i+1 < len(p) && p[i+1] == ':' {
			// Segmento ":nome": copia o "/" e emite "{nome}".
			j := i + 2
			for j < len(p) && p[j] != '/' {
				j++
			}
			name := p[i+2 : j]
			if name != "" {
				b.WriteByte('/')
				b.WriteByte('{')
				b.WriteString(name)
				b.WriteByte('}')
				i = j
				continue
			}
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeRaw(w http.ResponseWriter, raw RawResponse) {
	status := raw.Status
	if status == 0 {
		status = http.StatusOK
	}
	contentType := raw.ContentType
	if contentType == "" {
		contentType = "text/plain; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_, _ = w.Write(raw.Body)
}

// pathParamNames extrai os nomes `{nome}` de um path no formato do
// ServeMux (Go 1.22+), ex.: "/clientes/{id}/pedidos/{numero}" -> [id numero].
func pathParamNames(path string) []string {
	var names []string
	for {
		start := strings.IndexByte(path, '{')
		if start == -1 {
			break
		}
		end := strings.IndexByte(path[start:], '}')
		if end == -1 {
			break
		}
		name := path[start+1 : start+end]
		name = strings.TrimSuffix(name, "...")
		names = append(names, name)
		path = path[start+end+1:]
	}
	return names
}

// Serve sobe o servidor HTTP em addr (ex.: "127.0.0.1:8080" ou ":8080") e
// bloqueia até Shutdown ser chamado ou ocorrer um erro fatal de I/O.
// Usa Handler(): o mux é montado sob demanda na primeira requisição e
// remontado quando rotas novas são registradas.
func (s *Server) Serve(addr string) error {
	s.mu.Lock()
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
	srv := s.httpServer
	s.mu.Unlock()

	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// limitBody envolve o handler limitando o corpo de cada requisição a
// maxBodyBytes (FULL-REVIEW A2), impedindo exaustão de memória por uploads
// arbitrariamente grandes.
func limitBody(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		h.ServeHTTP(w, r)
	})
}

// Shutdown encerra graciosamente o servidor iniciado por Serve.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	srv := s.httpServer
	s.mu.Unlock()
	if srv == nil {
		return fmt.Errorf("rest: servidor não iniciado")
	}
	return srv.Shutdown(ctx)
}
