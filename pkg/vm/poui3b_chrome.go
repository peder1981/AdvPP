package vm

import (
	"fmt"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3b.3 — chrome do PoApp: PoNavbar, PoHeader e PoMenuPanel. Como o
// PoMenu, são únicos por aplicação e renderizados acima do conteúdo
// (nós nav/head/panel do mountJSON).

// po3bChromeType: tipos de chrome.
func po3bChromeType(typ string) bool {
	switch typ {
	case "navbar", "header", "menupanel":
		return true
	}
	return false
}

// po3bChromeNew registra o componente como chrome do app (único).
func po3bChromeNew(a *poAppState, class, typ string, obj *advplrt.ObjectValue, args []advplrt.Value) (*poComp, error) {
	var slot **poComp
	switch typ {
	case "navbar":
		slot = &a.navbar
	case "header":
		slot = &a.header
	case "menupanel":
		slot = &a.menupanel
	}
	if *slot != nil {
		return nil, fmt.Errorf("PoApp: só um %s por aplicação", class)
	}
	c := a.add(typ, obj)
	*slot = c
	return c, nil
}

// po3bChromeProps: props iniciais de cada chrome.
func po3bChromeProps(c *poComp, args []advplrt.Value) {
	switch c.typ {
	case "navbar":
		c.props["items"] = []map[string]any{}
		c.props["iconActions"] = []map[string]any{}
		c.props["logo"] = ""
	case "header":
		c.props["actions"] = []map[string]any{}
		c.props["user"] = map[string]any{}
	case "menupanel":
		c.props["items"] = []map[string]any{}
		c.props["logo"] = ""
	}
}

// po3bChromeItem anexa item de menu {label, icon?, action:"act:N"}.
func po3bChromeItem(a *poAppState, c *poComp, prop string, args []advplrt.Value) {
	label := advplrt.ToString(getArg(args, 0))
	var cb advplrt.Value
	var icone string
	switch c.typ {
	case "navbar":
		// AddItem(cLabel, bAcao) / AddIconAction(cIcone, cLabel, bAcao)
		if prop == "iconActions" {
			icone = advplrt.ToString(getArg(args, 0))
			label = advplrt.ToString(getArg(args, 1))
			cb = getArg(args, 2)
		} else {
			cb = getArg(args, 1)
		}
	case "menupanel":
		// AddItem(cLabel, cIcone, bAcao)
		icone = advplrt.ToString(getArg(args, 1))
		cb = getArg(args, 2)
	default: // header AddAction(cLabel, bAcao)
		cb = getArg(args, 1)
	}
	lst, _ := c.props[prop].([]map[string]any)
	ev := fmt.Sprintf("act:%d", len(lst))
	if b, ok := cb.(*advplrt.CodeBlockValue); ok {
		c.handlers[ev] = b
	}
	item := map[string]any{"label": label, "action": ev}
	if icone != "" {
		item["icon"] = icone
	}
	a.setProp(c, prop, append(lst, item))
}

// po3bChromeMethod despacha os métodos do chrome.
func (v *VM) po3bChromeMethod(c *poComp, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	if !po3bChromeType(c.typ) {
		return false, advplrt.Nil, nil
	}
	a := c.app
	ret := advplrt.Value(advplrt.Nil)
	switch c.typ {
	case "navbar":
		switch method {
		case "SETLOGO":
			a.setProp(c, "logo", advplrt.ToString(getArg(args, 0)))
		case "ADDITEM":
			po3bChromeItem(a, c, "items", args)
		case "ADDICONACTION":
			po3bChromeItem(a, c, "iconActions", args)
		default:
			return false, advplrt.Nil, nil
		}
	case "header":
		switch method {
		case "SETUSER":
			a.setProp(c, "user", map[string]any{
				"name":  advplrt.ToString(getArg(args, 0)),
				"email": advplrt.ToString(getArg(args, 1)),
			})
		case "ADDACTION":
			po3bChromeItem(a, c, "actions", args)
		default:
			return false, advplrt.Nil, nil
		}
	case "menupanel":
		switch method {
		case "ADDITEM":
			po3bChromeItem(a, c, "items", args)
		case "SETLOGO":
			a.setProp(c, "logo", advplrt.ToString(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	default:
		return false, advplrt.Nil, nil
	}
	v.push(ret)
	return true, ret, nil
}
