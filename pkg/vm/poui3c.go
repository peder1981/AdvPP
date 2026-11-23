package vm

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3c — serviços e diretivas: PoApp:SetTheme/SetLanguage/
// OnMediaChange/Storage*/OnNetworkChange/StartGuide/OnGuideEnd e, nos
// componentes, SetTooltip/SetDraggable/OnDrop. O armazenamento fica no
// localStorage do browser (namespace advpp:poui:<sid>:<chave>); StorageGet
// é síncrono via round-trip com timeout de 5 s (Nil no timeout).

var poIdiomas = map[string]bool{"pt": true, "en": true, "es": true, "ru": true}
var poTemas = map[string]bool{"light": true, "dark": true}
var poTooltipPos = map[string]bool{"top": true, "bottom": true, "left": true, "right": true}

// poStorageGetTimeout: teto do round-trip do StorageGet (testes reduzem).
var poStorageGetTimeout = 5 * time.Second

// po3cAppMethod despacha os métodos de serviço do PoApp. Devolve true
// quando o método foi tratado.
func (v *VM) po3cAppMethod(a *poAppState, obj *advplrt.ObjectValue, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	switch method {
	case "SETTHEME":
		raw := advplrt.ToString(getArg(args, 0))
		if tema := strings.ToLower(strings.TrimSpace(raw)); poTemas[tema] {
			a.addOp(map[string]any{"op": "theme", "value": tema})
		} else {
			// JSON de tokens (PoThemeTokens) ou PoTheme completo.
			var x any
			if err := json.Unmarshal([]byte(raw), &x); err != nil {
				return true, advplrt.Nil, fmt.Errorf("PoApp:SetTheme: tema inválido %q (light, dark ou JSON de tokens)", raw)
			}
			a.addOp(map[string]any{"op": "theme", "value": x})
		}
	case "SETLANGUAGE":
		lang := strings.ToLower(strings.TrimSpace(advplrt.ToString(getArg(args, 0))))
		if !poIdiomas[lang] {
			return true, advplrt.Nil, fmt.Errorf("PoApp:SetLanguage: idioma inválido %q (pt, en, es, ru)", lang)
		}
		a.addOp(map[string]any{"op": "language", "value": lang})
	case "ONMEDIACHANGE":
		a.handlers["mediachange"] = getArg(args, 0)
	case "ONNETWORKCHANGE":
		a.handlers["networkchange"] = getArg(args, 0)
	case "ONGUIDEEND":
		a.handlers["guideend"] = getArg(args, 0)
	case "STORAGESET":
		a.addOp(map[string]any{"op": "storageset", "key": advplrt.ToString(getArg(args, 0)), "value": poToJSON(getArg(args, 1))})
	case "STORAGEREMOVE":
		a.addOp(map[string]any{"op": "storageremove", "key": advplrt.ToString(getArg(args, 0))})
	case "STORAGEGET":
		return true, v.poStorageGet(a, advplrt.ToString(getArg(args, 0))), nil
	case "STARTGUIDE":
		a.addOp(map[string]any{"op": "guide", "steps": poGuideSteps(getArg(args, 0))})
	default:
		return false, advplrt.Nil, nil
	}
	return true, advplrt.Nil, nil
}

// poStorageGet pede o valor ao browser e espera a resposta "storageresult"
// (ou "storagetimeout" do watchdog). Eventos de outros componentes lidos
// enquanto isso ficam em a.deferred e são despachados depois do handler.
func (v *VM) poStorageGet(a *poAppState, key string) advplrt.Value {
	if a.ui == nil || !a.active {
		return advplrt.Nil
	}
	a.nextReq++
	req := a.nextReq
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-time.After(poStorageGetTimeout):
			val, _ := json.Marshal(map[string]any{"reqId": req})
			a.ui.UIPostWait(a.appID, "storagetimeout", val, done)
		case <-done:
		}
	}()
	// Pelo batch + flush: sai imediatamente, na ordem das demais ops.
	a.addOp(map[string]any{"op": "storageget", "reqId": req, "key": key})
	a.flush()
	for {
		id, ev, val := a.ui.UINext()
		if id == a.appID && (ev == "storageresult" || ev == "storagetimeout") {
			var body struct {
				ReqID int64           `json:"reqId"`
				Value json.RawMessage `json:"value"`
			}
			if json.Unmarshal(val, &body) == nil && body.ReqID == int64(req) {
				if ev == "storagetimeout" || len(body.Value) == 0 {
					return advplrt.Nil
				}
				return poFromJSONPoEvent(body.Value)
			}
		}
		a.deferred = append(a.deferred, poPending{id: id, ev: ev, val: val})
	}
}

// poGuideSteps converte aPassos={{oComponente,cTitulo,cTexto},...} para
// []{target,title,content}; target é o id do componente (0 = sem alvo).
func poGuideSteps(x advplrt.Value) []map[string]any {
	out := []map[string]any{}
	arr, _ := advplrt.Deref(x).(*advplrt.ArrayValue)
	if arr == nil {
		return out
	}
	for _, el := range arr.Elements {
		par, ok := advplrt.Deref(el).(*advplrt.ArrayValue)
		if !ok || len(par.Elements) < 3 {
			continue
		}
		step := map[string]any{
			"target":  0,
			"title":   advplrt.ToString(par.Elements[1]),
			"content": advplrt.ToString(par.Elements[2]),
		}
		if c := poCompDeArg(par.Elements[0]); c != nil {
			step["target"] = c.id
		}
		out = append(out, step)
	}
	return out
}

// poReorder move drag para a posição pos (0-based, já descontada a
// remoção) dentro dos filhos de c. Devolve false se drag não é filho.
func poReorder(c *poComp, drag *poComp, pos int) bool {
	idx := -1
	for i, ch := range c.children {
		if ch == drag {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	c.children = append(c.children[:idx], c.children[idx+1:]...)
	if pos < 0 {
		pos = 0
	}
	if pos > len(c.children) {
		pos = len(c.children)
	}
	c.children = append(c.children, nil)
	copy(c.children[pos+1:], c.children[pos:])
	c.children[pos] = drag
	return true
}

// po3cMethod despacha SetTooltip/SetDraggable/OnDrop dos componentes.
func (v *VM) po3cMethod(c *poComp, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	a := c.app
	switch method {
	case "SETTOOLTIP":
		pos := argTexto(args, 1, "top")
		if !poTooltipPos[pos] {
			return true, advplrt.Nil, fmt.Errorf("SetTooltip: posição inválida %q (top, bottom, left, right)", pos)
		}
		a.setProps(c, map[string]any{"tooltip": advplrt.ToString(getArg(args, 0)), "tooltipPosition": pos})
	case "SETDRAGGABLE":
		a.setProp(c, "draggable", advplrt.ToBool(getArg(args, 0)))
	case "ONDROP":
		if !a.isContainer(c) {
			return true, advplrt.Nil, fmt.Errorf("OnDrop: só em contêineres")
		}
		c.handlers["drop"] = getArg(args, 0)
	default:
		return false, advplrt.Nil, nil
	}
	v.push(advplrt.Nil)
	return true, advplrt.Nil, nil
}
