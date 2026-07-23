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
	"sort"
	"strings"
	"sync"
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
	// serializado como corpo JSON da resposta (200) ou um erro (500).
	Handler func(params map[string]any) (any, error)
}

// Server é um servidor REST servindo um conjunto fixo de rotas sobre HTTP.
type Server struct {
	Name    string
	Version string

	mu     sync.Mutex
	routes []Route

	httpServer *http.Server
}

// NewServer cria um servidor REST vazio.
func NewServer(name, version string) *Server {
	return &Server{Name: name, Version: version}
}

// AddRoute registra (ou substitui, se method+path já existirem) uma rota.
func (s *Server) AddRoute(r Route) {
	s.mu.Lock()
	defer s.mu.Unlock()
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	r.Method = method
	for i, existing := range s.routes {
		if existing.Method == r.Method && existing.Path == r.Path {
			s.routes[i] = r
			return
		}
	}
	s.routes = append(s.routes, r)
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
// suportados desde 1.22) a partir das rotas registradas.
func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	for _, route := range s.Routes() {
		route := route
		pattern := route.Path
		if route.Method != "" {
			pattern = route.Method + " " + route.Path
		}
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			s.dispatch(w, r, route)
		})
	}
	// Introspecção: lista as rotas registradas, útil para debug manual e
	// para o cliente descobrir a API sem precisar da doc externa.
	mux.HandleFunc("GET /_routes", func(w http.ResponseWriter, r *http.Request) {
		list := make([]map[string]string, 0, len(s.routes))
		for _, route := range s.Routes() {
			list = append(list, map[string]string{"method": route.Method, "path": route.Path})
		}
		writeJSON(w, http.StatusOK, list)
	})
	return mux
}

// dispatch mescla path params + query string + corpo JSON num único mapa
// (nessa ordem de precedência — o corpo, por ser o payload mais específico,
// vence em caso de colisão de chave) e chama o Handler da rota.
func (s *Server) dispatch(w http.ResponseWriter, r *http.Request, route Route) {
	params := map[string]any{}

	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			params[strings.ToUpper(k)] = v[0]
		}
	}

	// Path params: Go 1.22+ expõe cada `{nome}` do pattern via r.PathValue.
	for _, name := range pathParamNames(route.Path) {
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
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
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
func (s *Server) Serve(addr string) error {
	// buildMux() -> Routes() precisa de s.mu — montar o mux ANTES de
	// travar para gravar s.httpServer, senão é deadlock (mutex não é
	// reentrante).
	mux := s.buildMux()
	s.mu.Lock()
	s.httpServer = &http.Server{Addr: addr, Handler: mux}
	srv := s.httpServer
	s.mu.Unlock()

	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
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
