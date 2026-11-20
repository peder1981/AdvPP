package webui

import (
	"encoding/json"
	"net/http"
	"runtime"
)

// uiBuffer: eventos de componentes pendentes por sessão. Acima disso,
// UIPost descarta (timer atrasado) em vez de travar quem posta.
const uiBuffer = 256

// uiEvent é um evento de componente (clique, mudança, timer...) a caminho
// do laço de PoApp:Activate.
type uiEvent struct {
	ID    int
	Event string
	Value json.RawMessage
}

// UISend envia uma operação de componentes (mount/batch/close) ao browser
// como evento SSE {"type":"ui","data":op}. Implementa vm.ComponentUI.
func (p *Provider) UISend(op []byte) {
	p.s.send(event{Type: "ui", Data: json.RawMessage(op)})
}

// UIPost injeta um evento no laço da sessão sem bloquear (timers).
// Sessão encerrada ou fila cheia: o evento é descartado.
func (p *Provider) UIPost(id int, ev string, value []byte) {
	select {
	case <-p.s.quit:
	case p.s.ui <- uiEvent{ID: id, Event: ev, Value: value}:
	default:
	}
}

// UINext bloqueia até o próximo evento de componente. Sessão abandonada:
// encerra a goroutine do programa (mesmo contrato de wait).
func (p *Provider) UINext() (int, string, []byte) {
	select {
	case ev := <-p.s.ui:
		return ev.ID, ev.Event, ev.Value
	case <-p.s.quit:
		runtime.Goexit()
		return 0, "", nil
	}
}

// handleUIPost trata POST /ui?s=<sid>: {"id":N,"event":"...","value":...}.
func handleUIPost(s *session, w http.ResponseWriter, r *http.Request) {
	if s == nil {
		http.Error(w, "unknown session", http.StatusNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxReplyBytes)
	var body struct {
		ID    int             `json:"id"`
		Event string          `json:"event"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.ID <= 0 || body.Event == "" {
		http.Error(w, "id e event são obrigatórios", http.StatusBadRequest)
		return
	}
	select {
	case s.ui <- uiEvent{ID: body.ID, Event: body.Event, Value: body.Value}:
		w.WriteHeader(http.StatusNoContent)
	case <-s.quit:
		http.Error(w, "session closed", http.StatusGone)
	case <-r.Context().Done():
	}
}
