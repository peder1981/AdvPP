package vm

import (
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3b.4 — componentes de exibição: PoAvatar, PoBadge, PoImage,
// PoSkeleton, PoHelper, PoCalendar, PoDynamicView, PoDisclaimerGroup,
// PoFilterChip. PoToaster é o PoNotificationService (PoApp:Notify estendido
// em poui_native.go) e /poui-asset resolve src relativa de imagem.

// po3bDispType: tipos de exibição.
func po3bDispType(typ string) bool {
	switch typ {
	case "avatar", "badge", "image", "skeleton", "helper",
		"calendar", "dynamicview", "disclaimergroup", "filterchip":
		return true
	}
	return false
}

// po3bDispProps: props iniciais por tipo (New). args[0] é o pai.
func po3bDispProps(c *poComp, args []advplrt.Value) {
	switch c.typ {
	case "avatar":
		c.props["src"] = advplrt.ToString(getArg(args, 1))
		c.props["size"] = argTexto(args, 2, "medium")
	case "badge":
		c.props["value"] = advplrt.ToFloat(getArg(args, 1))
		c.props["color"] = argTexto(args, 2, "")
	case "image":
		c.props["src"] = advplrt.ToString(getArg(args, 1))
		c.props["alt"] = argTexto(args, 2, "")
	case "skeleton":
		c.props["type"] = argTexto(args, 1, "")
		c.props["size"] = argTexto(args, 2, "")
	case "helper":
		c.props["helper"] = advplrt.ToString(getArg(args, 1))
		c.props["title"] = argTexto(args, 2, "")
	case "calendar":
		c.props["value"] = poDataISO(getArg(args, 1))
	case "dynamicview":
		c.props["fields"] = []any{}
		c.props["value"] = map[string]any{}
		c.props["load"] = ""
	case "disclaimergroup":
		c.props["title"] = argTexto(args, 1, "")
		c.props["disclaimers"] = po3bDispDisclaimers(getArg(args, 2))
	case "filterchip":
		c.props["label"] = argTexto(args, 1, "")
		c.props["selected"] = false
	}
}

// po3bDispDisclaimers: aDisclaimers (strings ou {texto/tipo}) ->
// []any de {text, type?} do po-disclaimer-group.
func po3bDispDisclaimers(x advplrt.Value) []any {
	out := []any{}
	arr, ok := advplrt.Deref(x).(*advplrt.ArrayValue)
	if !ok {
		return out
	}
	for _, el := range arr.Elements {
		// PoDisclaimer do PO-UI: "value" é obrigatório (sem ele o
		// po-disclaimer-group descarta o item); "label" é o texto exibido.
		texto := advplrt.ToString(el)
		if o, ok := advplrt.Deref(el).(*advplrt.ObjectValue); ok {
			texto = advplrt.ToString(o.Props["TEXTO"])
		}
		out = append(out, map[string]any{"value": texto, "label": texto})
	}
	return out
}

// po3bDispMethod despacha os métodos dos componentes de exibição.
func (v *VM) po3bDispMethod(c *poComp, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	if !po3bDispType(c.typ) {
		return false, advplrt.Nil, nil
	}
	a := c.app
	ret := advplrt.Value(advplrt.Nil)
	switch c.typ {
	case "avatar":
		switch method {
		case "SETSRC":
			a.setProp(c, "src", advplrt.ToString(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	case "badge":
		switch method {
		case "SETVALUE":
			a.setProp(c, "value", advplrt.ToFloat(getArg(args, 0)))
		case "SETCOLOR":
			a.setProp(c, "color", argTexto(args, 1, ""))
		case "SETSTATUS":
			a.setProp(c, "status", argTexto(args, 1, ""))
		default:
			return false, advplrt.Nil, nil
		}
	case "image":
		switch method {
		case "SETSRC":
			a.setProp(c, "src", advplrt.ToString(getArg(args, 0)))
		case "SETHEIGHT":
			a.setProp(c, "height", advplrt.ToFloat(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	case "skeleton":
		switch method {
		case "SETVARIANT":
			a.setProp(c, "variant", argTexto(args, 1, ""))
		default:
			return false, advplrt.Nil, nil
		}
	case "helper":
		switch method {
		case "SETTYPE":
			a.setProp(c, "type", argTexto(args, 1, ""))
		default:
			return false, advplrt.Nil, nil
		}
	case "calendar":
		switch method {
		case "SETMODE":
			a.setProp(c, "mode", strings.ToLower(argTexto(args, 0, "single")))
		case "SETVALUE":
			a.setProp(c, "value", poDataISO(getArg(args, 0)))
		case "GETVALUE":
			s, _ := c.props["value"].(string)
			ret = poDataDeValor(advplrt.NewString(s))
		default:
			return false, advplrt.Nil, nil
		}
	case "dynamicview":
		switch method {
		case "SETVALUE":
			if o, ok := advplrt.Deref(getArg(args, 0)).(*advplrt.ObjectValue); ok {
				m := map[string]any{}
				for _, k := range o.Keys {
					m[strings.ToLower(k)] = poToJSON(o.Props[k])
				}
				a.setProp(c, "value", m)
			}
		case "SETFIELDS":
			a.setProp(c, "fields", poDispFields(getArg(args, 0)))
		case "SETLOAD":
			a.setProp(c, "load", advplrt.ToString(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	case "disclaimergroup":
		switch method {
		case "SETITEMS":
			a.setProp(c, "disclaimers", po3bDispDisclaimers(getArg(args, 0)))
		case "SETTITLE":
			a.setProp(c, "title", argTexto(args, 0, ""))
		case "ONREMOVEALL":
			c.handlers["removeall"] = getArg(args, 0)
		default:
			return false, advplrt.Nil, nil
		}
	case "filterchip":
		switch method {
		case "SETSELECTED":
			a.setProp(c, "selected", advplrt.ToBool(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	}
	v.push(ret)
	return true, ret, nil
}

// poDispFields: aCampos ({cProp,cLabel,cTipo,...} ou objeto) -> fields do
// po-dynamic-view ({property,label,type,...}).
func poDispFields(x advplrt.Value) []any {
	out := []any{}
	arr, _ := advplrt.Deref(x).(*advplrt.ArrayValue)
	if arr == nil {
		return out
	}
	for _, el := range arr.Elements {
		if arr2, ok := advplrt.Deref(el).(*advplrt.ArrayValue); ok && len(arr2.Elements) >= 2 {
			out = append(out, map[string]any{
				"property": advplrt.ToString(arr2.Elements[0]),
				"label":    advplrt.ToString(arr2.Elements[1]),
				"type":     strings.ToLower(argTexto(arr2.Elements, 2, "string")),
			})
		} else if o, ok := advplrt.Deref(el).(*advplrt.ObjectValue); ok {
			f := map[string]any{}
			for _, k := range o.Keys {
				f[strings.ToLower(k)] = poToJSON(o.Props[k])
			}
			out = append(out, f)
		}
	}
	return out
}
