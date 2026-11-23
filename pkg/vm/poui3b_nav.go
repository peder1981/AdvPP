package vm

import (
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3b.3 — navegação (parcial): PoLink, PoBreadcrumb e PoContextTabs.
// PoNavbar/PoHeader/PoMenuPanel são chrome do PoApp e ficam para a
// próxima entrega (exigem nó próprio no mountJSON).

// po3bNavType: tipos desta entrega.
func po3bNavType(typ string) bool {
	switch typ {
	case "link", "breadcrumb", "contexttabs", "contexttab":
		return true
	}
	return false
}

// po3bNavItem: {{cLabel, cLinkOuAcao}} -> {label, link|action:"nav:N"}.
func po3bNavItems(a *poAppState, c *poComp, x advplrt.Value) []map[string]any {
	out := []map[string]any{}
	arr, _ := advplrt.Deref(x).(*advplrt.ArrayValue)
	if arr == nil {
		return out
	}
	for _, el := range arr.Elements {
		par, ok := advplrt.Deref(el).(*advplrt.ArrayValue)
		if !ok || len(par.Elements) < 2 {
			continue
		}
		item := map[string]any{"label": advplrt.ToString(par.Elements[0])}
		// cPagina: se for componente de página, guarda o id; o clique
		// posta "navigate" {page:id} e o loop resolve para o objeto.
		if pg := poCompDeArg(par.Elements[1]); pg != nil {
			item["page"] = pg.id
		} else {
			item["link"] = advplrt.ToString(par.Elements[1])
		}
		out = append(out, item)
	}
	return out
}

// po3bNavMethod despacha os métodos dos componentes de navegação.
func (v *VM) po3bNavMethod(c *poComp, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	if !po3bNavType(c.typ) {
		return false, advplrt.Nil, nil
	}
	a := c.app
	ret := advplrt.Value(advplrt.Nil)
	switch c.typ {
	case "link":
		switch method {
		case "SETOPENNEWTAB":
			a.setProp(c, "openNewTab", advplrt.ToBool(getArg(args, 0)))
		case "ONCLICK":
			c.handlers["click"] = getArg(args, 0)
		default:
			return false, advplrt.Nil, nil
		}
	case "breadcrumb":
		switch method {
		case "SETITEMS":
			a.setProp(c, "items", po3bNavItems(a, c, getArg(args, 0)))
		case "ONNAVIGATE":
			c.handlers["navigate"] = getArg(args, 0)
		default:
			return false, advplrt.Nil, nil
		}
	case "contexttabs":
		switch method {
		case "ADDTAB":
			label := advplrt.ToString(getArg(args, 0))
			pg := poCompDeArg(getArg(args, 1))
			tabs, _ := c.props["tabs"].([]map[string]any)
			item := map[string]any{"label": label}
			if pg != nil {
				item["page"] = pg.id
			}
			tabs = append(tabs, item)
			a.setProp(c, "tabs", tabs)
		case "CLOSETAB":
			n := int(advplrt.ToFloat(getArg(args, 0)))
			tabs, _ := c.props["tabs"].([]map[string]any)
			if n >= 0 && n < len(tabs) {
				tabs = append(tabs[:n], tabs[n+1:]...)
				a.setProp(c, "tabs", tabs)
			}
		case "ONCLOSE":
			c.handlers["close"] = getArg(args, 0)
		case "ONACTIVATE":
			c.handlers["activate"] = getArg(args, 0)
		default:
			return false, advplrt.Nil, nil
		}
	case "contexttab":
		// PoContextTab não é usado diretamente — as abas são dados do
		// PoContextTabs. Mantido no registro por compatibilidade de nome.
		return false, advplrt.Nil, nil
	default:
		return false, advplrt.Nil, nil
	}
	v.push(ret)
	return true, ret, nil
}
