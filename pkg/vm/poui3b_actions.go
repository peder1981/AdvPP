package vm

import (
	"fmt"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3b.2 — ações: PoButtonGroup, PoDropdown, PoPopup, PoPopover,
// PoContextMenu e PoToolbar. Ações são codeblocks AdvPL disparados por
// eventos "act:N" postados pelo browser (dispatch genérico do poui_loop).

// po3bActionType: tipos desta onda.
func po3bActionType(typ string) bool {
	switch typ {
	case "buttongroup", "dropdown", "popup", "popover", "contextmenu", "toolbar":
		return true
	}
	return false
}

// po3bAddAction anexa {label, icon?, action:"act:N"} à lista props[prop]
// e registra o codeblock em handlers["act:N"]. Devolve o índice.
// iconePrimeiro: PoButtonGroup::AddButton(cLabel, cIcone, bAcao);
// demais: AddAction/AddItem(cLabel, bAcao[, cIcone]).
func po3bAddAction(a *poAppState, c *poComp, prop string, args []advplrt.Value, iconePrimeiro bool) int {
	label := advplrt.ToString(getArg(args, 0))
	var cb advplrt.Value
	var icone string
	if iconePrimeiro {
		icone = advplrt.ToString(getArg(args, 1))
		cb = getArg(args, 2)
	} else {
		cb = getArg(args, 1)
		icone = advplrt.ToString(getArg(args, 2))
	}
	lst, _ := c.props[prop].([]map[string]any)
	idx := len(lst)
	ev := fmt.Sprintf("act:%d", idx)
	if b, ok := cb.(*advplrt.CodeBlockValue); ok {
		c.handlers[ev] = b
	}
	item := map[string]any{"label": label, "action": ev}
	if icone != "" {
		item["icon"] = icone
	}
	lst = append(lst, item)
	a.setProp(c, prop, lst)
	return idx
}

// po3bActionList lê a lista de ações/itens do componente.
func po3bActionList(c *poComp, prop string) []map[string]any {
	lst, _ := c.props[prop].([]map[string]any)
	return lst
}

// po3bGroup2Method despacha os métodos dos componentes de ação.
func (v *VM) po3bGroup2Method(c *poComp, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	if !po3bActionType(c.typ) {
		return false, advplrt.Nil, nil
	}
	a := c.app
	ret := advplrt.Value(advplrt.Nil)
	switch c.typ {
	case "buttongroup":
		switch method {
		case "ADDBUTTON":
			po3bAddAction(a, c, "buttons", args, true)
		case "SETSELECTED":
			a.setProp(c, "selected", poToJSON(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	case "dropdown":
		switch method {
		case "ADDACTION":
			po3bAddAction(a, c, "actions", args, false)
		case "SETPOSITION":
			pos := strings.ToLower(advplrt.ToString(getArg(args, 0)))
			switch pos {
			case "left", "right", "top", "bottom":
				a.setProp(c, "position", pos)
			default:
				return true, advplrt.Nil, fmt.Errorf("PoDropdown:SetPosition: valor inválido %q", pos)
			}
		default:
			return false, advplrt.Nil, nil
		}
	case "popup":
		switch method {
		case "ADDACTION":
			po3bAddAction(a, c, "actions", args, false)
		case "OPEN":
			a.setProp(c, "open", true)
		case "CLOSE":
			a.setProp(c, "open", false)
		default:
			return false, advplrt.Nil, nil
		}
	case "popover":
		switch method {
		case "SETTRIGGER":
			tr := strings.ToLower(advplrt.ToString(getArg(args, 0)))
			switch tr {
			case "click", "hover", "focus":
				a.setProp(c, "trigger", tr)
			default:
				return true, advplrt.Nil, fmt.Errorf("PoPopover:SetTrigger: valor inválido %q", tr)
			}
		case "SETPOSITION":
			pos := strings.ToLower(advplrt.ToString(getArg(args, 0)))
			switch pos {
			case "top", "bottom", "left", "right":
				a.setProp(c, "position", pos)
			default:
				return true, advplrt.Nil, fmt.Errorf("PoPopover:SetPosition: valor inválido %q", pos)
			}
		default:
			return false, advplrt.Nil, nil
		}
	case "contextmenu":
		switch method {
		case "ADDITEM":
			po3bAddAction(a, c, "items", args, false)
		case "SETSELECTED":
			a.setProp(c, "selected", poToJSON(getArg(args, 0)))
		case "ONSELECT":
			c.handlers["itemselected"] = getArg(args, 0)
		case "ONCHANGEEXPANDED":
			c.handlers["expandedchange"] = getArg(args, 0)
		default:
			return false, advplrt.Nil, nil
		}
	case "toolbar":
		switch method {
		case "ADDACTION":
			po3bAddAction(a, c, "actions", args, false)
		case "SETPROFILE":
			a.setProp(c, "profile", map[string]any{
				"name":  advplrt.ToString(getArg(args, 0)),
				"email": advplrt.ToString(getArg(args, 1)),
			})
		case "SETNOTIFICATIONS":
			a.setProp(c, "notificationNumber", poToJSON(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	default:
		return false, advplrt.Nil, nil
	}
	_ = ret
	v.push(ret)
	return true, ret, nil
}

// poCompDeArg resolve um argumento que é um componente PO-UI (objeto
// cuja Native é *poComp) — usado para o alvo de PoPopup/PoPopover.
func poCompDeArg(x advplrt.Value) *poComp {
	o, ok := advplrt.Deref(x).(*advplrt.ObjectValue)
	if !ok {
		return nil
	}
	c, _ := o.Native.(*poComp)
	return c
}
