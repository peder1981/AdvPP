// Package webui implementa o modo "advplc serve": executa o programa
// AdvPL/TLPP no servidor (mesma VM, mesmo banco ADVPP.db) e renderiza a
// interface no browser do usuário — fase 1 do renderer web: console e
// diálogos (MsgInfo/MsgStop/MsgAlert/MsgYesNo/Alert) em HTML puro.
//
// Protocolo (stdlib apenas, sem WebSocket):
//   GET  /            → página HTML embutida
//   GET  /events?s=ID → stream SSE: {type:"output"|"dialog"|"done"|"error", ...}
//   POST /reply?s=ID  → resposta de diálogo: {"id":N,"result":"ok"|"yes"|"no"}
//
// Cada conexão /events cria uma sessão com VM própria (isolada, como um
// work process) — recarregar a página reexecuta o programa.
package webui

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

//go:embed index.html
var indexHTML []byte

type event struct {
	Type  string `json:"type"` // output | dialog | done | error
	ID    int    `json:"id,omitempty"`
	Kind  string `json:"kind,omitempty"` // info | stop | alert | yesno
	Title string `json:"title,omitempty"`
	Text  string `json:"text,omitempty"`
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

// Serve sobe o servidor HTTP e bloqueia. run é chamado uma vez por sessão.
func Serve(addr, sourceName string, run RunFunc) error {
	var mu sync.Mutex
	sessions := map[string]*session{}

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

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
				if ev.Type == "done" {
					return
				}
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
