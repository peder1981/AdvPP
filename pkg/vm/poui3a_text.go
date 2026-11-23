package vm

import (
	"github.com/advpl/compiler/pkg/tools/shared"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3a.3 — texto especializado: PoEmail, PoUrl, PoDecimal, PoRichText,
// PoSearch. PoPassword/PoTextarea/PoNumber já existem desde a fase 2.
// PoRichText devolve o HTML cru do editor (sem sanitização — quem exibe
// sanitiza, conforme o plano).

// po3aTextMethod despacha os métodos específicos dos campos de texto.
// Devolve true quando o método foi tratado.
func (v *VM) po3aTextMethod(c *poComp, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	a := c.app
	switch c.typ {
	case "decimal":
		switch method {
		case "SETDECIMALS":
			a.setProp(c, "decimalsLength", int(advplrt.ToFloat(getArg(args, 0))))
		case "SETMIN":
			a.setProp(c, "min", advplrt.ToFloat(getArg(args, 0)))
		case "SETMAX":
			a.setProp(c, "max", advplrt.ToFloat(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	case "richtext":
		switch method {
		case "SETHEIGHT":
			a.setProp(c, "height", advplrt.ToString(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	case "search":
		switch method {
		case "SETITEMS":
			a.setProp(c, "items", poToJSON(getArg(args, 0)))
		case "SETFILTERKEYS":
			a.setProp(c, "filterKeys", poToJSON(getArg(args, 0)))
		case "ONFILTER":
			c.handlers["filter"] = getArg(args, 0)
		default:
			return false, advplrt.Nil, nil
		}
	default:
		return false, advplrt.Nil, nil
	}
	return true, advplrt.Nil, nil
}

// poRestPrefixURL: URL de upload/download — relativa ganha o RestPrefix.
func poRestPrefixURL(x advplrt.Value) string {
	u := advplrt.ToString(x)
	if u == "" {
		return ""
	}
	if len(u) > 8 && (u[:7] == "http://" || u[:8] == "https://") {
		return u
	}
	if u[0] != '/' {
		u = "/" + u
	}
	return shared.ResolveRestPrefix() + u
}
