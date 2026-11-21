package webui

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"regexp"
	"runtime"
	"sync"
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

// UIPostWait injeta um evento esperando vaga na fila (saída de processo
// filho: nenhuma linha pode se perder). Sessão encerrada ou cancel fechado
// (o laço do programa terminou): devolve false sem postar.
func (p *Provider) UIPostWait(id int, ev string, value []byte, cancel <-chan struct{}) bool {
	select {
	case p.s.ui <- uiEvent{ID: id, Event: ev, Value: value}:
		return true
	case <-p.s.quit:
		return false
	case <-cancel:
		return false
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
	case <-s.quit:
		http.Error(w, "session closed", http.StatusGone)
		return
	default:
	}
	// Sem esperar vaga: com a fila cheia (programa fora do laço de eventos,
	// ex.: preso num MsgYesNo), responde na hora em vez de segurar a conexão.
	select {
	case s.ui <- uiEvent{ID: body.ID, Event: body.Event, Value: body.Value}:
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "fila de eventos cheia", http.StatusServiceUnavailable)
	}
}

var (
	iconesOnce sync.Once
	icones     map[string]bool
)

// Icones devolve os nomes de ícone Animalia (sem o prefixo "an-") definidos
// no CSS do PO-UI embutido — a fonte de verdade de quais ícones aparecem.
func Icones() map[string]bool {
	iconesOnce.Do(func() {
		icones = map[string]bool{}
		re := regexp.MustCompile(`\.an-([a-z0-9-]+):+before`)
		ents, _ := fs.Glob(distFS, "dist/styles-*.css")
		for _, e := range ents {
			data, err := distFS.ReadFile(e)
			if err != nil {
				continue
			}
			for _, m := range re.FindAllSubmatch(data, -1) {
				icones[string(m[1])] = true
			}
		}
	})
	return icones
}
