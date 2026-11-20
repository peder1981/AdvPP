package vm

import (
	"encoding/json"
	"fmt"
	"strconv"
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
	ui.UISend(a.mountJSON())
	a.active = true
	a.flush() // ops geradas antes do Activate (ex.: Notify de boas-vindas)
	defer func() {
		a.stopTimers()
		a.active = false
		a.batch = nil
	}()
	for id, t := range a.timers {
		a.startTimer(id, t)
	}
	for !a.closed {
		id, ev, val := ui.UINext()
		v.poDispatch(a, id, ev, val)
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
	if ev == "timer" {
		t, ok := a.timers[id]
		if !ok {
			return
		}
		h, args = t.block, []advplrt.Value{a.obj}
	} else {
		c, ok := a.byID[id]
		if !ok {
			return
		}
		switch ev {
		case "click":
			h, args = c.handlers["click"], []advplrt.Value{c.obj}
		case "change":
			txt := poJSONTexto(val)
			c.props["value"] = txt // veio do browser: não gera patch
			h, args = c.handlers["change"], []advplrt.Value{c.obj, advplrt.NewString(txt)}
		case "select":
			i, ok := poJSONIndice(val)
			if !ok || i >= len(c.rows) {
				return
			}
			h, args = c.handlers["select"], []advplrt.Value{c.obj, c.rows[i]}
		case "action":
			i, ok := poJSONIndice(val)
			if !ok {
				return
			}
			h, args = c.handlers[fmt.Sprintf("action:%d", i)], []advplrt.Value{c.obj}
		default:
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
