package vm

import (
	"encoding/json"
	"strconv"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3b.5 — páginas especiais: PoPageLogin, PoPageChangePassword,
// PoPageBlockedUser, PoPageSlide e PoPageJobScheduler. Senhas trafegam
// só no payload do evento síncrono (login/submit) — nunca em props,
// console, log ou Get() posterior (ver TestLoginSenhaNaoFicaNaProp).

// po3bPageType: tipos de página especial (filhos diretos do PoApp).
func po3bPageType(typ string) bool {
	switch typ {
	case "pagelogin", "pagechangepassword", "pageblockeduser",
		"pageslide", "pagejobscheduler":
		return true
	}
	return false
}

// po3bPageProps: props iniciais por tipo. args[0] é o PoApp.
func po3bPageProps(c *poComp, args []advplrt.Value) error {
	switch c.typ {
	case "pagelogin":
		c.props["product"] = advplrt.ToString(getArg(args, 1))
		c.props["literals"] = map[string]any{}
		c.props["recovery"] = false
	case "pagechangepassword":
		c.props["requirements"] = poStrArray(getArg(args, 1))
	case "pageblockeduser":
		c.props["reason"] = advplrt.ToString(getArg(args, 1))
		c.props["contactEmail"] = ""
	case "pageslide":
		c.props["title"] = advplrt.ToString(getArg(args, 1))
		c.props["open"] = false
	case "pagejobscheduler":
		c.props["title"] = advplrt.ToString(getArg(args, 1))
		c.props["url"] = poFilterURL(getArg(args, 2))
		c.props["parameters"] = []any{}
	}
	return nil
}

// poStrArray: array AdvPL de strings -> []any.
func poStrArray(x advplrt.Value) []any {
	out := []any{}
	if arr, ok := advplrt.Deref(x).(*advplrt.ArrayValue); ok {
		for _, el := range arr.Elements {
			out = append(out, advplrt.ToString(el))
		}
	}
	return out
}

// po3bPageMethod despacha métodos das páginas especiais.
func (v *VM) po3bPageMethod(c *poComp, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	if !po3bPageType(c.typ) {
		return false, advplrt.Nil, nil
	}
	a := c.app
	ret := advplrt.Value(advplrt.Nil)
	switch method {
	case "SETLITERALS":
		if c.typ != "pagelogin" {
			return false, advplrt.Nil, nil
		}
		if o, ok := advplrt.Deref(getArg(args, 0)).(*advplrt.ObjectValue); ok {
			m := map[string]any{}
			for _, k := range o.Keys {
				m[k] = poToJSON(o.Props[k])
			}
			a.setProp(c, "literals", m)
		}
	case "SETRECOVERY":
		a.setProp(c, "recovery", advplrt.ToBool(getArg(args, 0)))
	case "SETREQUIREMENTS":
		a.setProp(c, "requirements", poStrArray(getArg(args, 0)))
	case "SETCONTACTEMAIL":
		a.setProp(c, "contactEmail", advplrt.ToString(getArg(args, 0)))
	case "OPEN":
		if c.typ == "pageslide" {
			a.setProp(c, "open", true)
		}
	case "CLOSE":
		if c.typ == "pageslide" {
			a.setProp(c, "open", false)
		}
	case "SETPARAMETERS":
		if c.typ == "pagejobscheduler" {
			a.setProp(c, "parameters", poStrArray(getArg(args, 0)))
		}
	case "ONLOGIN", "ONSUBMIT", "ONCLOSE":
		// handlers["login"|"submit"|"close"]; login/submit são síncronos
		// (round-trip beforeResult), close é evento comum.
		key := map[string]string{"ONLOGIN": "login", "ONSUBMIT": "submit", "ONCLOSE": "close"}[method]
		c.handlers[key] = getArg(args, 0)
	default:
		return false, advplrt.Nil, nil
	}
	v.push(ret)
	return true, ret, nil
}

// poLoginHook: login/submit síncronos — {reqId, user, pass} do browser,
// handler (oComp, cUsuario, cSenha) -> lOk, resposta beforeResult.
// A senha existe só na variável local: nunca em props/log/console.
func (v *VM) poLoginHook(a *poAppState, c *poComp, ev string, val []byte) {
	var body struct {
		ReqID   int64  `json:"reqId"`
		User    string `json:"user"`
		Pass    string `json:"pass"`
		NewPass string `json:"new"`
	}
	reqID := int64(0)
	if err := json.Unmarshal(val, &body); err == nil {
		reqID = body.ReqID
	}
	h, ok := c.handlers[ev].(*advplrt.CodeBlockValue)
	if !ok {
		a.addOp(map[string]any{"op": "beforeResult", "reqId": reqID, "ok": false})
		return
	}
	pilha := len(v.stack)
	var ret advplrt.Value
	var err error
	if ev == "submit" {
		// OnSubmit(cAtual, cNova)
		ret, err = v.callBlockSync(h, c.obj, advplrt.NewString(body.Pass), advplrt.NewString(body.NewPass))
	} else {
		// OnLogin(cUsuario, cSenha)
		ret, err = v.callBlockSync(h, c.obj, advplrt.NewString(body.User), advplrt.NewString(body.Pass))
	}
	body.Pass, body.NewPass = "", "" // higiene: limpa as cópias locais
	if err != nil {
		v.stack = v.stack[:pilha]
		a.addOp(map[string]any{"op": "beforeResult", "reqId": reqID, "ok": false})
		return
	}
	a.addOp(map[string]any{"op": "beforeResult", "reqId": reqID, "ok": advplrt.ToBool(ret)})
}

var _ = strconv.Itoa
