package vm

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// poTimer é um timer de PoApp:SetTimer. A goroutine só posta eventos
// "timer" no laço; quem executa o bloco é a VM, no Activate.
type poTimer struct {
	ms    int
	block advplrt.Value
	stop  chan struct{}
	vivo  bool
}

const poTimerMinMs = 100

func (a *poAppState) setTimer(ms int, block advplrt.Value) (int, error) {
	if ms < poTimerMinMs {
		return 0, fmt.Errorf("PoApp:SetTimer: intervalo mínimo 100 ms")
	}
	a.nextTimer++
	t := &poTimer{ms: ms, block: block}
	a.timers[a.nextTimer] = t
	if a.active {
		a.startTimer(a.nextTimer, t)
	}
	return a.nextTimer, nil
}

func (a *poAppState) startTimer(id int, t *poTimer) {
	if t.vivo {
		return
	}
	t.vivo = true
	t.stop = make(chan struct{})
	ui, stop := a.ui, t.stop
	go func() {
		tk := time.NewTicker(time.Duration(t.ms) * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tk.C:
				ui.UIPost(id, "timer", nil)
			}
		}
	}()
}

func (a *poAppState) killTimer(id int) {
	if t, ok := a.timers[id]; ok {
		if t.vivo {
			close(t.stop)
			t.vivo = false
		}
		delete(a.timers, id)
	}
}

func (a *poAppState) stopTimers() {
	for _, t := range a.timers {
		if t.vivo {
			close(t.stop)
			t.vivo = false
		}
	}
}

// poActivate monta a tela e roda o laço de eventos até Close() ou o fim da
// sessão (UINext encerra a goroutine; os defers param os timers).
func (v *VM) poActivate(a *poAppState, ui ComponentUI) error {
	a.ui = ui
	a.closed = false
	a.remount = false
	// id reservado para os eventos de app (mediachange, networkchange,
	// guideend, storage): fora da sequência dos componentes e dos
	// confirms — poAppNodeID é grande o bastante para nunca colidir.
	a.appID = poAppNodeID
	ui.UISend(a.mountJSON())
	a.active = true
	a.flush() // ops geradas antes do Activate (ex.: Notify de boas-vindas)
	defer func() {
		a.killTerms()
		a.killSpawns()
		a.stopTimers()
		a.active = false
		a.batch = nil
	}()
	for id, t := range a.timers {
		a.startTimer(id, t)
	}
	for id, sp := range a.spawns {
		a.startSpawn(id, sp)
	}
	for _, c := range a.byID {
		if c.term != nil {
			a.startTerm(c)
		}
	}
	a.flush() // running:true dos terminais
	for !a.closed {
		id, ev, val := ui.UINext()
		v.poDispatch(a, id, ev, val)
		// Eventos lidos durante um round-trip síncrono (StorageGet)
		// voltam para a fila na ordem em que chegaram.
		for len(a.deferred) > 0 {
			d := a.deferred[0]
			a.deferred = a.deferred[1:]
			v.poDispatch(a, d.id, d.ev, d.val)
		}
		if a.remount {
			// Navigate: a página nova vai inteira; patches perdem o sentido,
			// as demais ops (notify...) seguem depois do mount.
			a.remount = false
			ui.UISend(a.mountJSON())
			resto := a.batch[:0]
			for _, op := range a.batch {
				if op["op"] != "patch" {
					resto = append(resto, op)
				}
			}
			a.batch = resto
		}
		a.flush()
	}
	ui.UISend([]byte(`{"op":"close"}`))
	return nil
}

// flush envia o lote acumulado, se houver.
func (a *poAppState) flush() {
	if len(a.batch) == 0 {
		return
	}
	data, _ := json.Marshal(map[string]any{"op": "batch", "ops": a.batch})
	a.batch = nil
	a.ui.UISend(data)
}

// poDispatch executa o handler do evento. Evento sem destino ou sem handler
// é ignorado; erro no handler vai para o console e o laço segue.
func (v *VM) poDispatch(a *poAppState, id int, ev string, val []byte) {
	var h advplrt.Value
	var args []advplrt.Value
	switch ev {
	case "timer":
		t, ok := a.timers[id]
		if !ok {
			return
		}
		h, args = t.block, []advplrt.Value{a.obj}
	case "line", "exit":
		h, args = a.poSpawnEvento(id, ev, val)
	case "notifyact":
		cb, ok := a.notifyActs[id]
		if !ok {
			return
		}
		delete(a.notifyActs, id)
		h, args = cb, []advplrt.Value{a.obj}
	case "confirm":
		cb, ok := a.confirms[id]
		if !ok {
			return // resposta repetida ou id desconhecido
		}
		delete(a.confirms, id)
		h, args = cb[1], []advplrt.Value{a.obj}
		if poJSONTexto(val) == "true" {
			h = cb[0]
		}
	default:
		if id == a.appID {
			// Onda 3c — eventos de app (mediachange, networkchange,
			// guideend): handler em a.handlers, args (oApp, valor).
			hb, ok2 := a.handlers[ev].(*advplrt.CodeBlockValue)
			if !ok2 {
				return
			}
			h, args = hb, []advplrt.Value{a.obj, poFromJSONPoEvent(val)}
			break
		}
		c, ok := a.byID[id]
		if !ok {
			return
		}
		if c.term != nil && (ev == "input" || ev == "resize" || ev == "termexit") {
			h, args = a.poTermEvento(c, ev, val)
			break
		}
		switch ev {
		case "click":
			h, args = c.handlers["click"], []advplrt.Value{c.obj}
		case "save": // AdvCodeEditor: Ctrl+S com o texto completo
			txt := poJSONTexto(val)
			c.props["value"] = txt
			h, args = c.handlers["save"], []advplrt.Value{c.obj, advplrt.NewString(txt)}
		case "change":
			novo := poValorDoBrowser(c.typ, val)
			c.props["value"] = poToJSON(novo) // veio do browser: não gera patch
			h, args = c.handlers["change"], []advplrt.Value{c.obj, novo}
		case "select":
			if c.typ == "tree" {
				h, args = c.handlers["select"], []advplrt.Value{c.obj, advplrt.NewString(poJSONTexto(val))}
				break
			}
			i, ok := poJSONIndice(val)
			if !ok || i >= len(c.rows) {
				return
			}
			h, args = c.handlers["select"], []advplrt.Value{c.obj, c.rows[i]}
		case "selected":
			// PoLookup:OnSelected(oLinha) — a linha vem do REST como objeto.
			h, args = c.handlers["selected"], []advplrt.Value{c.obj, poFromJSONPoEvent(val)}
		case "filter":
			// PoSearch:OnFilter(cTexto)
			h, args = c.handlers["filter"], []advplrt.Value{c.obj, advplrt.NewString(poJSONTexto(val))}
		case "upload":
			// PoUpload:OnUpload(aArquivos) — resposta do POST /poui-upload.
			h, args = c.handlers["upload"], []advplrt.Value{c.obj, poFromJSONPoEvent(val)}
		case "advancedsearch":
			// PoPageList:OnAdvancedSearch()
			h, args = c.handlers["advancedsearch"], []advplrt.Value{c.obj}
		case "savenew", "cancel", "edit", "remove", "back":
			// PoPageEdit/PoPageDetail: OnSaveNew/OnCancel/OnEdit/OnRemove/OnBack
			h, args = c.handlers[ev], []advplrt.Value{c.obj}
		case "login", "submit":
			// Onda 3b.5 — PoPageLogin/PoPageChangePassword: credenciais
			// síncronas; a senha não é gravada em props.
			v.poLoginHook(a, c, ev, val)
			return
		case "beforesave", "beforeremove", "canactivate":
			// Onda 3a.6 — round-trip síncrono: o frontend espera
			// {"op":"beforeResult","reqId":N,"ok":bool} antes de gravar.
			// O handler roda na goroutine do laço e o resultado sai no
			// flush logo após o dispatch; timeout no frontend (30 s)
			// equivale a recusar.
			v.poBeforeHook(a, c, ev, val)
			return
		case "drop":
			// Onda 3c.3 — {drag,pos}: reordena os filhos do container e
			// chama OnDrop(oArrastado, nNovaPosicao).
			var body struct {
				Drag int `json:"drag"`
				Pos  int `json:"pos"`
			}
			if json.Unmarshal(val, &body) != nil {
				return
			}
			drag, ok := a.byID[body.Drag]
			if !ok || drag.app != a || !poReorder(c, drag, body.Pos) {
				return
			}
			h, args = c.handlers["drop"], []advplrt.Value{drag.obj, advplrt.NewNumber(float64(body.Pos))}
		case "action":
			i, ok := poJSONIndice(val)
			if !ok {
				return
			}
			h, args = c.handlers[fmt.Sprintf("action:%d", i)], []advplrt.Value{c.obj}
		case "navigate":
			// PoBreadcrumb:OnNavigate(oPagina) — {page:id} resolve para o
			// objeto da página; {label} vai como 2º argumento.
			var nav struct {
				Page  int    `json:"page"`
				Label string `json:"label"`
			}
			_ = json.Unmarshal(val, &nav)
			var navArgs []advplrt.Value
			navArgs = append(navArgs, c.obj)
			if alvo, ok := a.byID[nav.Page]; ok {
				navArgs = append(navArgs, alvo.obj)
			} else {
				navArgs = append(navArgs, advplrt.Nil)
			}
			navArgs = append(navArgs, advplrt.NewString(nav.Label))
			h, args = c.handlers["navigate"], navArgs
		default:
			// Eventos genéricos registrados via On* (onda 3b em diante):
			// expand/collapse/close/changestep/click:* recebem (oComp, valor).
			if hb, ok2 := c.handlers[ev].(*advplrt.CodeBlockValue); ok2 {
				h, args = hb, []advplrt.Value{c.obj, poFromJSONPoEvent(val)}
				break
			}
			return
		}
	}
	if _, ok := h.(*advplrt.CodeBlockValue); !ok {
		return
	}
	pilha := len(v.stack)
	if _, err := v.callBlockSync(h, args...); err != nil {
		// callBlockSync desfaz os frames mas não a pilha de operandos: sem
		// isto, um handler que falha a cada timer cresce a pilha sem limite.
		v.stack = v.stack[:pilha]
		v.writeOut(fmt.Sprintf("PoApp: erro no evento %s do componente %d: %v", ev, id, err))
	}
}

// poValorDoBrowser converte o valor de um "change" conforme o componente:
// number -> N, checkbox/switch -> L, multiselect/checkboxgroup -> array
// de strings, demais -> C.
func poValorDoBrowser(typ string, val []byte) advplrt.Value {
	txt := poJSONTexto(val)
	switch typ {
	case "number", "decimal":
		f, _ := strconv.ParseFloat(strings.TrimSpace(txt), 64)
		return advplrt.NewNumber(f)
	case "checkbox", "switch", "filterchip":
		return advplrt.NewBool(txt == "true")
	case "datepicker", "calendar":
		return poDataDeValor(advplrt.NewString(txt))
	case "disclaimergroup":
		// OnChange(a) — array de disclaimers vem do browser.
		return poFromJSONPoEvent(val)
	case "daterange":
		return poRangeDoBrowser(val)
	case "multiselect", "checkboxgroup", "lookup":
		var xs []any
		if json.Unmarshal(val, &xs) != nil {
			if typ == "lookup" {
				break
			}
			return advplrt.NewArray(nil)
		}
		el := make([]advplrt.Value, len(xs))
		for i, e := range xs {
			if m, ok := e.(map[string]any); ok {
				if v, has := m["value"]; has {
					e = v
				}
			}
			el[i] = poFromJSON(e)
		}
		return advplrt.NewArray(el)
	}
	return advplrt.NewString(txt)
}

// poFromJSONPoEvent: o "value" do POST /ui como valor AdvPL — objetos e
// arrays viram estruturas AdvPL; escalares caem no poFromJSON comum.
func poFromJSONPoEvent(val []byte) advplrt.Value {
	var x any
	if json.Unmarshal(val, &x) != nil || x == nil {
		return advplrt.Nil
	}
	return poFromJSON(x)
}

// poJSONTexto: valor JSON do browser como texto ("abc" -> abc, 42 -> 42).
func poJSONTexto(val []byte) string {
	var x any
	if json.Unmarshal(val, &x) != nil || x == nil {
		return ""
	}
	switch t := x.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return string(val)
	}
}

// poJSONIndice: índice 0-based vindo do browser.
func poJSONIndice(val []byte) (int, bool) {
	var f float64
	if json.Unmarshal(val, &f) != nil || f < 0 {
		return 0, false
	}
	return int(f), true
}
