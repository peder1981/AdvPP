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
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
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
		mu.Lock()
		s, resumed := sessions[sid]
		if !resumed {
			s = newSession()
			sessions[sid] = s
		}
		mu.Unlock()

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
				s.events <- event{Type: "output", Text: "── executando " + sourceName + " ──"}
				if err := run(&Provider{s}, &OutWriter{s}); err != nil {
					s.events <- event{Type: "error", Text: err.Error()}
				}
				s.events <- event{Type: "done"}
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
				if s.done.Load() {
					mu.Lock()
					delete(sessions, sid)
					mu.Unlock()
				}
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
