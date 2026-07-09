// Package webui implementa o modo "advplc serve": executa o programa
// AdvPL/TLPP no servidor (mesma VM, mesmo banco ADVPP.db) e renderiza a
// interface no browser do usuário. Fase 2 do renderer web: app PO-UI/Angular
// embutido (console, diálogos e FWMBrowse→po-table + SX3→po-dynamic-form).
//
// Protocolo (backend stdlib apenas, sem WebSocket):
//   GET  /            → app PO-UI embutido (embed.FS)
//   GET  /events?s=ID → stream SSE: {type:"output"|"dialog"|"browse"|"done"|"error", ...}
//   POST /reply?s=ID  → resposta: {"id":N,"result":"ok"|"yes"|"no"|<ação JSON do browse>}
//
// Cada conexão /events cria uma sessão com VM própria (isolada, como um
// work process) — recarregar a página reexecuta o programa.
package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
)

// dist é o app PO-UI/Angular compilado (fase 2) — regenerar com `make web`.
//
//go:embed all:dist
var distFS embed.FS

type event struct {
	Type  string          `json:"type"` // output | dialog | browse | done | error
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
}

func newSession() *session {
	return &session{
		events:  make(chan event, 64),
		waiting: make(map[int]chan string),
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

	s.events <- event{Type: "dialog", ID: id, Kind: kind, Title: title, Text: msg}
	return <-ch
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

	s.events <- event{Type: eventType, ID: id, Data: data}
	return <-ch
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

// outWriter transmite a saída de console (ConOut) para o browser.
type OutWriter struct{ s *session }

func (w *OutWriter) Write(b []byte) (int, error) {
	text := string(b)
	if len(text) > 0 && text[len(text)-1] == '\n' {
		text = text[:len(text)-1]
	}
	w.s.events <- event{Type: "output", Text: text}
	return len(b), nil
}

// RunFunc executa o programa de uma sessão. Recebe o provider de UI e o
// writer de console já ligados ao browser; retorna o erro de execução.
type RunFunc func(ui *Provider, console *OutWriter) error

// Server é o servidor do modo web. Mantém as sessões ativas para permitir
// broadcast (hot reload da fase 3: --watch).
type Server struct {
	sourceName string
	run        RunFunc
	mu         sync.Mutex
	sessions   map[string]*session
}

// New cria o servidor do modo web. run é chamado uma vez por sessão.
func New(sourceName string, run RunFunc) *Server {
	return &Server{sourceName: sourceName, run: run, sessions: map[string]*session{}}
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

// Serve sobe o servidor HTTP e bloqueia.
func (srv *Server) Serve(addr string) error {
	mu := &srv.mu
	sessions := srv.sessions
	sourceName := srv.sourceName
	run := srv.run

	mux := http.NewServeMux()

	staticFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServerFS(staticFS))

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		sid := r.URL.Query().Get("s")
		if sid == "" {
			http.Error(w, "missing session", http.StatusBadRequest)
			return
		}
		s := newSession()
		mu.Lock()
		sessions[sid] = s
		mu.Unlock()
		defer func() {
			mu.Lock()
			delete(sessions, sid)
			mu.Unlock()
		}()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Executa o programa em goroutine própria; eventos fluem pelo canal
		go func() {
			s.events <- event{Type: "output", Text: "── executando " + sourceName + " ──"}
			if err := run(&Provider{s}, &OutWriter{s}); err != nil {
				s.events <- event{Type: "error", Text: err.Error()}
			}
			s.events <- event{Type: "done"}
		}()

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
				return
			}
		}
	})

	mux.HandleFunc("/reply", func(w http.ResponseWriter, r *http.Request) {
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

	fmt.Printf("AdvPP web: http://%s  (fonte: %s)\n", addr, sourceName)
	return http.ListenAndServe(addr, mux)
}
