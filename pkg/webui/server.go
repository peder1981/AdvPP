// Package webui implementa o modo "advplc serve": executa o programa
// AdvPL/TLPP no servidor (mesma VM, mesmo banco ADVPP.db) e renderiza a
// interface no browser do usuário. Fase 2 do renderer web: app PO-UI/Angular
// embutido (console, diálogos e FWMBrowse→po-table + SX3→po-dynamic-form).
//
// Protocolo (backend stdlib apenas, sem WebSocket):
//
//	GET  /            → app PO-UI embutido (embed.FS)
//	GET  /events?s=ID → stream SSE: {type:"output"|"dialog"|"browse"|"menu"|"input"|"done"|"error", ...}
//	POST /reply?s=ID  → resposta: {"id":N,"result":"ok"|"yes"|"no"|<ação JSON do browse>}
//
// Cada conexão /events cria uma sessão com VM própria (isolada, como um
// work process) — recarregar a página reexecuta o programa.
package webui

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// dist é o app PO-UI/Angular compilado (fase 2) — regenerar com `make web`.
//
//go:embed all:dist
var distFS embed.FS

type event struct {
	Type  string          `json:"type"` // output | dialog | browse | menu | input | done | error
	ID    int             `json:"id,omitempty"`
	Kind  string          `json:"kind,omitempty"` // info | stop | alert | yesno
	Title string          `json:"title,omitempty"`
	Text  string          `json:"text,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"` // payload estruturado (browse)
}

type session struct {
	events  chan event
	mu      sync.Mutex
	waiting map[int]chan string
	nextID  int
	// done vira true quando run() termina de verdade (evento "done" já
	// enfileirado). Usado pelo handler de /events pra decidir, na queda da
	// conexão, se a sessão pode ser descartada (programa realmente acabou)
	// ou se precisa ficar viva esperando uma reconexão retomar (ver New
	// no handler de /events).
	done atomic.Bool
	// quit fecha quando a sessão é abandonada (browser sumiu e não voltou
	// dentro de abandonAfter): diálogos pendentes encerram a goroutine do
	// programa (runtime.Goexit roda os defers, ex.: fechar as conexões de
	// banco da sessão) em vez de ela ficar presa para sempre.
	quit     chan struct{}
	quitOnce sync.Once
	conns    atomic.Int32 // conexões SSE abertas nesta sessão
	// ui recebe os eventos de componentes (POST /ui e timers) consumidos
	// pelo laço de PoApp:Activate (ver ui.go).
	ui chan uiEvent
	// sid é a chave desta sessão no mapa sessions (preenchida na criação).
	sid string
	// poui é o canal do app PO-UI aberto por FwCallApp nesta sessão
	// (nil = nenhum; ver poui_canal.go). pouiGone marca que o app foi
	// fechado — requisições seguintes a /poui/{app} recebem 410.
	poui     *pouiCanal
	pouiGone bool
}

func newSession() *session {
	return &session{
		events:  make(chan event, 64),
		waiting: make(map[int]chan string),
		quit:    make(chan struct{}),
		ui:      make(chan uiEvent, uiBuffer),
	}
}

// abandonAfter: quanto tempo uma sessão sem nenhum browser conectado
// espera uma reconexão antes de ser encerrada. ADVPP_WEBUI_ABANDON_SECONDS.
var abandonAfter = func() time.Duration {
	if n, err := strconv.Atoi(os.Getenv("ADVPP_WEBUI_ABANDON_SECONDS")); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return 5 * time.Minute
}()

func (s *session) abandon() { s.quitOnce.Do(func() { close(s.quit) }) }

// send enfileira um evento; numa sessão abandonada encerra a goroutine.
func (s *session) send(ev event) {
	select {
	case s.events <- ev:
	case <-s.quit:
		runtime.Goexit()
	}
}

// post enfileira um evento; numa sessão abandonada só descarta.
func (s *session) post(ev event) {
	select {
	case s.events <- ev:
	case <-s.quit:
	}
}

// wait bloqueia até a resposta do browser; numa sessão abandonada encerra
// a goroutine (Goexit não é capturável por recover/ErrorBlock da VM).
func (s *session) wait(ch chan string) string {
	select {
	case r := <-ch:
		return r
	case <-s.quit:
		runtime.Goexit()
		return ""
	}
}

// ask envia um diálogo ao browser e bloqueia até a resposta do usuário.
func (s *session) ask(kind, msg, title string) string {
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	ch := make(chan string, 1)
	s.waiting[id] = ch
	s.mu.Unlock()

	s.send(event{Type: "dialog", ID: id, Kind: kind, Title: title, Text: msg})
	return s.wait(ch)
}

// askData envia um evento com payload estruturado (ex.: browse) e bloqueia
// até a resposta do browser — mesma mecânica de ask, com JSON no lugar de texto.
func (s *session) askData(eventType string, data json.RawMessage) string {
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	ch := make(chan string, 1)
	s.waiting[id] = ch
	s.mu.Unlock()

	s.send(event{Type: eventType, ID: id, Data: data})
	return s.wait(ch)
}

func (s *session) reply(id int, result string) {
	s.mu.Lock()
	ch := s.waiting[id]
	delete(s.waiting, id)
	s.mu.Unlock()
	if ch != nil {
		ch <- result
	}
}

// Provider implementa vm.UIProvider sobre uma sessão do browser.
type Provider struct{ s *session }

func (p *Provider) MsgInfo(msg, title string)  { p.s.ask("info", msg, title) }
func (p *Provider) MsgStop(msg, title string)  { p.s.ask("stop", msg, title) }
func (p *Provider) MsgAlert(msg, title string) { p.s.ask("alert", msg, title) }
func (p *Provider) MsgYesNo(msg, title string) bool {
	return p.s.ask("yesno", msg, title) == "yes"
}

// menuSpec/inputSpec espelham o que o frontend espera em ev.Data (ver
// web/src/app/app.ts) — mesma convenção de BrowseSpec/DialogSpec.
type menuSpec struct {
	Title string   `json:"title"`
	Items []string `json:"items"`
}

type inputSpec struct {
	Prompt string `json:"prompt"`
	Def    string `json:"def"`
	Pw     bool   `json:"pw,omitempty"`
}

// Menu implementa vm.UIProvider: envia a lista de opções ao browser e
// bloqueia até o usuário escolher uma (ou fechar sem escolher).
func (p *Provider) Menu(items []string, title string) int {
	data, _ := json.Marshal(menuSpec{Title: title, Items: items})
	result := p.s.askData("menu", data)
	n, err := strconv.Atoi(result)
	if err != nil {
		return 0
	}
	return n
}

// InputText implementa vm.UIProvider: pede um texto ao browser e bloqueia
// até a resposta (ou o valor default, se o usuário cancelar). Se bIsPassword
// for true, o campo oculta caracteres digitados.
func (p *Provider) InputText(prompt, def string, bIsPassword bool) string {
	data, _ := json.Marshal(inputSpec{Prompt: prompt, Def: def, Pw: bIsPassword})
	result := p.s.askData("input", data)
	if result == "" {
		return def
	}
	return result
}

// Browse implementa vm.BrowseUI: envia o spec do FWMBrowse ao browser e
// bloqueia até o usuário devolver uma ação (save/delete/close em JSON).
func (p *Provider) Browse(spec []byte) []byte {
	return []byte(p.s.askData("browse", spec))
}

// Dialog implementa vm.DialogUI: envia um MSDIALOG legado (fase 4) ao
// browser e bloqueia até o usuário agir (button/close em JSON).
func (p *Provider) Dialog(spec []byte) []byte {
	return []byte(p.s.askData("msdialog", spec))
}

// ocultaLog: ADVPP_WEBUI_HIDE_LOG=1 não manda o ConOut ao browser (o painel
// "Log técnico" some); a saída vai só para o stdout do processo. Para produção
// exposta, onde o log revela detalhes internos (banco, migrações) a qualquer
// visitante.
var ocultaLog = os.Getenv("ADVPP_WEBUI_HIDE_LOG") == "1"

// outWriter transmite a saída de console (ConOut) para o browser.
type OutWriter struct{ s *session }

func (w *OutWriter) Write(b []byte) (int, error) {
	text := string(b)
	if len(text) > 0 && text[len(text)-1] == '\n' {
		text = text[:len(text)-1]
	}
	if ocultaLog {
		fmt.Fprintln(os.Stdout, text) // só no log do processo (journal)
		return len(b), nil
	}
	select {
	case w.s.events <- event{Type: "output", Text: text}:
	case <-w.s.quit: // ninguém mais lê: descarta
	}
	return len(b), nil
}

// RunFunc executa o programa de uma sessão. Recebe o provider de UI e o
// writer de console já ligados ao browser; retorna o erro de execução.
type RunFunc func(ui *Provider, console *OutWriter) error

// Server é o servidor do modo web. Mantém as sessões ativas para permitir
// broadcast (hot reload da fase 3: --watch).
type Server struct {
	sourceName string
	resolve    AppResolver
	mu         sync.Mutex
	sessions   map[string]*session

	restMu      sync.RWMutex
	restPrefix  string
	restHandler http.Handler
}

// New cria o servidor do modo web. run é chamado uma vez por sessão.
func New(sourceName string, run RunFunc) *Server {
	// Um programa só: nenhuma rota /app/<nome> existe.
	resolve := func(app string) (RunFunc, error) {
		if app != "" {
			return nil, ErrAppNaoEncontrado
		}
		return run, nil
	}
	return &Server{sourceName: sourceName, resolve: resolve, sessions: map[string]*session{}}
}

// Serve sobe o servidor HTTP e bloqueia (compatibilidade com a fase 1).
func Serve(addr, sourceName string, run RunFunc) error {
	return New(sourceName, run).Serve(addr)
}

// Broadcast envia um evento a todas as sessões conectadas sem bloquear.
// kind "reload" faz o browser recarregar (reexecutando o programa).
func (srv *Server) Broadcast(kind, text string) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	for _, s := range srv.sessions {
		select {
		case s.events <- event{Type: kind, Text: text}:
		default: // sessão com canal cheio: descarta em vez de travar o watcher
		}
	}
}

// MountREST monta um handler REST sob prefixo (ex.: "/rest") no mesmo
// servidor do modo web — as rotas passam pela mesma checagem de
// host/origem (sameOrigin) das demais. h deve ser um handler estável que
// delega para o servidor REST corrente (o --watch troca o servidor por
// baixo via ponteiro atômico do chamador). Deve ser chamado antes de
// Serve/Handler.
func (srv *Server) MountREST(prefixo string, h http.Handler) {
	srv.restMu.Lock()
	srv.restPrefix = "/" + strings.Trim(prefixo, "/")
	srv.restHandler = h
	srv.restMu.Unlock()
}

// Serve sobe o servidor HTTP e bloqueia.
func (srv *Server) Serve(addr string) error {
	fmt.Printf("AdvPP web: http://%s  (fonte: %s)\n", addr, srv.sourceName)
	hs := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	return hs.ListenAndServe()
}

// Handler monta as rotas do modo web (com a checagem de origem); Serve o
// usa, e testes podem montá-lo num httptest.Server.
func (srv *Server) Handler() http.Handler {
	mu := &srv.mu
	sessions := srv.sessions
	sourceName := srv.sourceName

	mux := http.NewServeMux()

	staticFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err) // "dist" é embutido no binário: só falha com build quebrado
	}
	mux.Handle("/", http.FileServerFS(staticFS))
	// /app/<nome>: a mesma SPA; o frontend lê o app da URL (vários apps).
	mux.HandleFunc("GET /app/{nome}", func(w http.ResponseWriter, r *http.Request) {
		if !NomeDeAppValido(r.PathValue("nome")) {
			http.Error(w, "nome de app inválido", http.StatusBadRequest)
			return
		}
		http.ServeFileFS(w, r, staticFS, "index.html")
	})

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		// Canal lateral do app PO-UI (aba nova do FwCallApp): SSE próprio,
		// não cria sessão nem executa programa.
		if srv.handlePouiSSE(w, r) {
			return
		}
		sid := r.URL.Query().Get("s")
		if sid == "" {
			http.Error(w, "missing session", http.StatusBadRequest)
			return
		}

		// O sid só muda quando a página recarrega de verdade (App gera um
		// novo Math.random() no constructor — ver web/src/app/app.ts). O
		// EventSource do browser, por outro lado, reconecta sozinho em
		// qualquer queda de conexão (proxy, aba suspensa, blip de rede),
		// reusando o MESMO sid, sem recarregar a página. Se essa reconexão
		// cair aqui como sessão nova, o programa reexecuta do zero por
		// baixo dos panos — inclusive enquanto uma goroutine anterior
		// segue presa esperando resposta de um diálogo (FWMenuSelect,
		// FWGetText, MsgYesNo...) que nunca mais vai chegar. Por isso:
		// sid já conhecido e ainda vivo (done == false) reaproveita a
		// mesma sessão em vez de criar uma nova.
		// App da sessão (vários apps): validado antes de qualquer acesso a
		// disco; resolvido só para sessão nova.
		app := r.URL.Query().Get("app")
		if app != "" && !NomeDeAppValido(app) {
			http.Error(w, "nome de app inválido", http.StatusBadRequest)
			return
		}

		mu.Lock()
		s, resumed := sessions[sid]
		if !resumed && len(sessions) >= maxSessions {
			mu.Unlock()
			http.Error(w, "too many sessions", http.StatusServiceUnavailable)
			return
		}
		var run RunFunc
		if !resumed {
			var err error
			if run, err = srv.resolve(app); err != nil {
				mu.Unlock()
				if errors.Is(err, ErrAppNaoEncontrado) {
					http.Error(w, err.Error(), http.StatusNotFound)
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
		}
		if !resumed {
			s = newSession()
			s.sid = sid
			sessions[sid] = s
		}
		mu.Unlock()
		s.conns.Add(1)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		// Envia os headers (200 + text/event-stream) imediatamente. Sem
		// isso, numa sessão retomada (sem goroutine nova, sem primeiro
		// evento síncrono), o cliente ficaria esperando a resposta HTTP
		// em si até o próximo evento — que só chega quando o usuário
		// responder ao diálogo pendente. EventSource/curl não veem uma
		// conexão SSE aberta enquanto os headers não saem.
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		if !resumed {
			// Executa o programa em goroutine própria; eventos fluem pelo canal
			go func() {
				if !ocultaLog {
					s.post(event{Type: "output", Text: "── executando " + sourceName + " ──"})
				}
				if err := run(&Provider{s}, &OutWriter{s}); err != nil {
					s.post(event{Type: "error", Text: err.Error()})
				}
				s.post(event{Type: "done"})
				s.done.Store(true)
			}()
		}

		enc := json.NewEncoder(w)
		for {
			select {
			case ev := <-s.events:
				fmt.Fprintf(w, "data: ")
				enc.Encode(ev)
				fmt.Fprintf(w, "\n")
				flusher.Flush()
				// não encerra no "done": a conexão fica aberta para eventos
				// posteriores (ex.: reload do --watch)
			case <-r.Context().Done():
				// Só descarta a sessão se o programa já terminou de verdade
				// (done == true). Enquanto estiver rodando/bloqueada num
				// diálogo, ela fica no mapa pra uma eventual reconexão
				// retomar (ver comentário acima) — o goroutine dela segue
				// vivo, com o canal de eventos aberto, esperando o /reply.
				if s.conns.Add(-1) == 0 {
					if s.done.Load() {
						mu.Lock()
						delete(sessions, sid)
						mu.Unlock()
					} else {
						// Programa ainda rodando e nenhum browser: espera
						// uma reconexão por abandonAfter; sem ela, encerra.
						time.AfterFunc(abandonAfter, func() {
							if s.conns.Load() == 0 && !s.done.Load() {
								mu.Lock()
								if sessions[sid] == s {
									delete(sessions, sid)
								}
								mu.Unlock()
								s.abandon()
							}
						})
					}
				}
				return
			}
		}
	})

	mux.HandleFunc("POST /reply", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxReplyBytes)
		sid := r.URL.Query().Get("s")
		mu.Lock()
		s := sessions[sid]
		mu.Unlock()
		if s == nil {
			http.Error(w, "unknown session", http.StatusNotFound)
			return
		}
		var body struct {
			ID     int    `json:"id"`
			Result string `json:"result"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.reply(body.ID, body.Result)
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /ui", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		s := sessions[r.URL.Query().Get("s")]
		mu.Unlock()
		handleUIPost(s, w, r)
	})

	// Canal do app PO-UI (FwCallApp/--poui-app): /poui/{app}/ serve os
	// assets do app aberto na sessão, /poui-ctx o contexto e /poui-ui o
	// jsToAdvpl. Todas passam pelo sameOrigin do mux.
	mux.HandleFunc("GET /poui/{app}/{resto...}", func(w http.ResponseWriter, r *http.Request) {
		srv.handlePouiApp(w, r, r.PathValue("app"), r.PathValue("resto"))
	})
	mux.HandleFunc("GET /poui-ctx", func(w http.ResponseWriter, r *http.Request) {
		srv.handlePouiCtx(w, r)
	})
	mux.HandleFunc("POST /poui-ui", func(w http.ResponseWriter, r *http.Request) {
		srv.handlePouiUI(w, r)
	})

	srv.restMu.RLock()
	if srv.restHandler != nil && srv.restPrefix != "" {
		// O prefixo é removido antes do dispatch: as rotas REST são
		// registradas com paths absolutos ("/api/est/v1/...") e chegam
		// como "/rest/api/est/v1/...".
		prefix := srv.restPrefix
		mux.Handle(prefix+"/", http.StripPrefix(prefix, srv.restHandler))
		mux.Handle(prefix, http.RedirectHandler(prefix+"/", http.StatusMovedPermanently))
	}
	srv.restMu.RUnlock()

	return sameOrigin(mux)
}

// Limites do modo web: sessões simultâneas (cada uma é uma VM rodando o
// programa) e corpo do /reply (resposta de diálogo/browse).
const (
	maxSessions   = 64
	maxReplyBytes = 8 << 20
)

// sameOrigin barra requisições vindas de outro site. Sem isso, qualquer
// página aberta no browser do usuário podia disparar GET /events (o
// programa executa a cada requisição — EventSource/img não precisam de
// CORS para *enviar*) e, via DNS rebinding (attacker.com resolvendo para
// 127.0.0.1), ler os eventos e responder diálogos pelo /reply.
//   - Host precisa ser loopback/IP literal ou o host de ADVPL_SERVE_HOST
//     (rebinding sempre chega com o nome de domínio do atacante);
//   - Origin, quando presente, precisa ter o mesmo host:porta da requisição.
func sameOrigin(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHost(r.Host) {
			http.Error(w, "host não permitido", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" {
			u, err := url.Parse(o)
			if err != nil || u.Host != r.Host {
				http.Error(w, "origem não permitida", http.StatusForbidden)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

func allowedHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil {
		return true
	}
	// Nomes extras (ex.: hostname da máquina na LAN):
	// ADVPL_SERVE_HOST e ADVPP_WEBUI_ALLOWED_HOSTS (separados por vírgula).
	extra := os.Getenv("ADVPL_SERVE_HOST") + "," + os.Getenv("ADVPP_WEBUI_ALLOWED_HOSTS")
	for _, a := range strings.Split(extra, ",") {
		if a = strings.TrimSpace(a); a != "" && strings.EqualFold(a, host) {
			return true
		}
	}
	return false
}
