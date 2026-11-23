package vm

import (
	"encoding/json"
	"strconv"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3a.6 — hooks síncronos OnBeforeSave/OnBeforeRemove. O frontend
// envia {"reqId":N,"data":{...registro}}; o handler AdvPL devolve
// lOk e o VM responde com op "beforeResult" {reqId, ok} no próximo
// flush. Sem handler ou erro -> ok=false (recusa), nunca trava.

// poBeforeHook executa o handler do hook e agenda a resposta.
func (v *VM) poBeforeHook(a *poAppState, c *poComp, ev string, val []byte) {
	var body struct {
		ReqID int64           `json:"reqId"`
		Data  json.RawMessage `json:"data"`
	}
	reqID := int64(0)
	if err := json.Unmarshal(val, &body); err == nil {
		reqID = body.ReqID
	}
	h, ok := c.handlers[ev].(*advplrt.CodeBlockValue)
	if !ok {
		// Sem OnBefore*: o PO-UI prossegue (não há veto).
		a.addOp(map[string]any{"op": "beforeResult", "reqId": reqID, "ok": true})
		return
	}
	var registro advplrt.Value = advplrt.Nil
	if len(body.Data) > 0 {
		registro = poFromJSONPoEvent(body.Data)
	}
	pilha := len(v.stack)
	ret, err := v.callBlockSync(h, c.obj, registro)
	if err != nil {
		v.stack = v.stack[:pilha]
		v.writeOut("PoApp: erro no hook " + ev + " do componente " + strconv.Itoa(c.id) + ": " + err.Error())
		a.addOp(map[string]any{"op": "beforeResult", "reqId": reqID, "ok": false})
		return
	}
	a.addOp(map[string]any{"op": "beforeResult", "reqId": reqID, "ok": advplrt.ToBool(ret)})
}
