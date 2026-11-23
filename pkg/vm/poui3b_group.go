package vm

import (
	"fmt"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3b.1 — agrupamento e sobreposição: PoAccordion/PoAccordionItem,
// PoModal, PoStepper/PoStep e PoSlide. Contratos conferidos nos d.ts do
// @po-ui/ng-components 21.x.

// po3bGroupType: tipos desta onda.
func po3bGroupType(typ string) bool {
	switch typ {
	case "accordion", "accordionitem", "modal", "stepper", "step", "slide":
		return true
	}
	return false
}

// po3bCanActivate: OnCanActivate usa o mesmo round-trip síncrono dos
// hooks OnBefore* da 3a.6 (op beforeResult {reqId, ok}, timeout 30 s no
// frontend = recusa).

// po3bGroupMethod despacha os métodos dos componentes de agrupamento.
func (v *VM) po3bGroupMethod(c *poComp, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	if !po3bGroupType(c.typ) {
		return false, advplrt.Nil, nil
	}
	a := c.app
	ret := advplrt.Value(advplrt.Nil)
	switch c.typ {
	case "accordion":
		switch method {
		case "SETSHOWMANAGER":
			a.setProp(c, "showManager", advplrt.ToBool(getArg(args, 0)))
		case "SETALLOWEXPANDALL":
			a.setProp(c, "allowExpandAll", advplrt.ToBool(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	case "accordionitem":
		switch method {
		case "EXPAND":
			a.setProp(c, "expanded", true)
		case "COLLAPSE":
			a.setProp(c, "expanded", false)
		case "ONEXPAND":
			c.handlers["expand"] = getArg(args, 0)
		case "ONCOLLAPSE":
			c.handlers["collapse"] = getArg(args, 0)
		default:
			return false, advplrt.Nil, nil
		}
	case "modal":
		switch method {
		case "OPEN":
			a.setProp(c, "open", true)
		case "CLOSE":
			a.setProp(c, "open", false)
		case "SETPRIMARYACTION", "SETSECONDARYACTION":
			key := "primaryAction"
			if method == "SETSECONDARYACTION" {
				key = "secondaryAction"
			}
			a.setProp(c, key, map[string]any{
				"label": advplrt.ToString(getArg(args, 0)),
				"action": poHandlerRef(a, c, "click:"+strings.ToLower(key), getArg(args, 1)),
			})
		case "SETSIZE":
			sz := strings.ToLower(advplrt.ToString(getArg(args, 0)))
			if sz != "sm" && sz != "md" && sz != "lg" && sz != "xl" && sz != "auto" {
				return true, advplrt.Nil, fmt.Errorf("PoModal:SetSize: tamanho inválido %q", sz)
			}
			a.setProp(c, "size", sz)
		case "ONCLOSE":
			c.handlers["close"] = getArg(args, 0)
		default:
			return false, advplrt.Nil, nil
		}
	case "stepper":
		switch method {
		case "NEXT":
			cur := poStepperStep(c)
			if cur < len(c.children)-1 {
				cur++
				a.setProp(c, "step", cur)
			}
		case "PREVIOUS":
			cur := poStepperStep(c)
			if cur > 0 {
				cur--
				a.setProp(c, "step", cur)
			}
		case "GETSTEP":
			ret = advplrt.NewNumber(float64(poStepperStep(c)))
		case "SETORIENTATION":
			ori := strings.ToLower(advplrt.ToString(getArg(args, 0)))
			if ori != "horizontal" && ori != "vertical" {
				return true, advplrt.Nil, fmt.Errorf("PoStepper:SetOrientation: valor inválido %q", ori)
			}
			a.setProp(c, "orientation", ori)
		case "ONCHANGESTEP":
			c.handlers["changestep"] = getArg(args, 0)
		default:
			return false, advplrt.Nil, nil
		}
	case "step":
		switch method {
		case "ONCANACTIVATE":
			c.handlers["canactivate"] = getArg(args, 0)
		default:
			return false, advplrt.Nil, nil
		}
	case "slide":
		switch method {
		case "SETINTERVAL":
			a.setProp(c, "interval", poToJSON(getArg(args, 0)))
		case "SETHEIGHT":
			a.setProp(c, "height", poToJSON(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	default:
		return false, advplrt.Nil, nil
	}
	v.push(ret)
	return true, ret, nil
}

// poSlideItems converte aSlides={{cImagem,cTitulo,cLink},...} para
// []PoSlideItem {image, alt, link} do PO-UI.
func poSlideItems(x advplrt.Value) []map[string]any {
	out := []map[string]any{}
	arr, _ := advplrt.Deref(x).(*advplrt.ArrayValue)
	if arr == nil {
		return out
	}
	for _, el := range arr.Elements {
		par, ok := advplrt.Deref(el).(*advplrt.ArrayValue)
		if !ok || len(par.Elements) < 1 {
			continue
		}
		item := map[string]any{"image": advplrt.ToString(par.Elements[0])}
		if len(par.Elements) > 1 {
			item["alt"] = advplrt.ToString(par.Elements[1])
		}
		if len(par.Elements) > 2 {
			item["link"] = advplrt.ToString(par.Elements[2])
		}
		out = append(out, item)
	}
	return out
}

// poStepperStep: passo corrente (0-based) do stepper.
func poStepperStep(c *poComp) int {
	if n, ok := c.props["step"].(float64); ok {
		return int(n)
	}
	if n, ok := c.props["step"].(int); ok {
		return n
	}
	return 0
}

// poHandlerRef registra o codeblock como handler nomeado do componente e
// devolve um marcador que o frontend usa para disparar o evento.
func poHandlerRef(a *poAppState, c *poComp, ev string, x advplrt.Value) string {
	if b, ok := x.(*advplrt.CodeBlockValue); ok {
		c.handlers[ev] = b
	}
	return ev
}
