package vm

import (
	"fmt"
	"strings"

	"github.com/advpl/compiler/pkg/tools/shared"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3a.1 — componentes de seleção (PoCombo, PoMultiselect, PoRadioGroup,
// PoCheckboxGroup, PoLookup). aOpcoes segue o formato do plano:
// {{cValor, cLabel}, ...} — par {valor, rótulo}; string solta vira
// {valor=texto, label=texto}.

// poOpcoesValorLabel converte aOpcoes {{cValor, cLabel}} em
// []{label, value} do protocolo.
func poOpcoesValorLabel(x advplrt.Value) []map[string]any {
	out := []map[string]any{}
	arr, _ := advplrt.Deref(x).(*advplrt.ArrayValue)
	if arr == nil {
		return out
	}
	for _, el := range arr.Elements {
		if par, ok := advplrt.Deref(el).(*advplrt.ArrayValue); ok && len(par.Elements) >= 2 {
			out = append(out, map[string]any{
				"label": advplrt.ToString(par.Elements[1]),
				"value": advplrt.ToString(par.Elements[0]),
			})
		} else {
			t := advplrt.ToString(el)
			out = append(out, map[string]any{"label": t, "value": t})
		}
	}
	return out
}

// poValores converte aValores (array de strings) em []string para as
// props "value" de multiselect/checkboxgroup.
func poValores(x advplrt.Value) []string {
	out := []string{}
	arr, _ := advplrt.Deref(x).(*advplrt.ArrayValue)
	if arr == nil {
		return out
	}
	for _, el := range arr.Elements {
		out = append(out, advplrt.ToString(el))
	}
	return out
}

// poFilterURL prefixa a URL relativa do serviço com o RestPrefix do serve
// (plano 3a.1: o PO-UI chama o REST direto do navegador). URL absoluta
// (http(s)://) passa direto.
func poFilterURL(x advplrt.Value) string {
	u := strings.TrimSpace(advplrt.ToString(x))
	if u == "" || strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	return shared.ResolveRestPrefix() + u
}

// poFilterParams normaliza o argumento de SetFilterParams: array de pares
// {{cProp, xVal}} vira map[string]any; JsonObject/objeto passa por poToJSON.
func poFilterParams(x advplrt.Value) any {
	if arr, ok := advplrt.Deref(x).(*advplrt.ArrayValue); ok {
		m := map[string]any{}
		pares := true
		for _, el := range arr.Elements {
			par, ok := advplrt.Deref(el).(*advplrt.ArrayValue)
			if !ok || len(par.Elements) != 2 {
				pares = false
				break
			}
			m[advplrt.ToString(par.Elements[0])] = poToJSON(par.Elements[1])
		}
		if pares {
			return m
		}
	}
	return poToJSON(x)
}

// po3aMethod despacha os métodos específicos dos componentes de seleção
// da onda 3a.1. Devolve true quando o método foi tratado.
func (v *VM) po3aMethod(c *poComp, method string, args []advplrt.Value) (bool, error) {
	a := c.app
	switch method {
	case "SETOPTIONS":
		switch c.typ {
		case "combo", "multiselect", "radiogroup", "checkboxgroup":
			a.setProp(c, "options", poOpcoesValorLabel(getArg(args, 0)))
		default:
			return false, nil
		}
	case "SETFILTERSERVICE":
		// PoCombo/PoMultiselect:SetFilterService(cUrl[, cCampoLabel, cCampoValor])
		// — PoLookup recebe a URL no construtor, mas aceita o mesmo ajuste.
		if c.typ != "combo" && c.typ != "multiselect" && c.typ != "lookup" {
			return false, nil
		}
		a.setProp(c, "filterUrl", poFilterURL(getArg(args, 0)))
		if s := advplrt.ToString(getArg(args, 1)); s != "" {
			a.setProp(c, "fieldLabel", s)
		}
		if s := advplrt.ToString(getArg(args, 2)); s != "" {
			a.setProp(c, "fieldValue", s)
		}
	case "SETFILTERPARAMS":
		// PoLookup:SetFilterParams(o) — o PoLookupFilter do PO-UI só aceita
		// objeto plano; {{cProp, xVal}} vira mapa {prop: valor}.
		if c.typ != "lookup" {
			return false, nil
		}
		a.setProp(c, "filterParams", poFilterParams(getArg(args, 0)))
	case "SETCOLUMNS":
		switch c.typ {
		case "radiogroup", "checkboxgroup":
			// SetColumns(n): quantidade de colunas do grupo.
			n := int(advplrt.ToFloat(getArg(args, 0)))
			if n < 1 {
				return true, nil
			}
			a.setProp(c, "columns", n)
		case "lookup":
			// SetColumns({{cProp, cRotulo[, cTipo]}, ...}) — mesmo formato
			// de PoTable:SetColumns, mapeado para {property,label,type}.
			cols := []map[string]any{}
			if arr, ok := advplrt.Deref(getArg(args, 0)).(*advplrt.ArrayValue); ok {
				for _, el := range arr.Elements {
					col, ok := advplrt.Deref(el).(*advplrt.ArrayValue)
					if !ok || len(col.Elements) < 2 {
						return true, fmt.Errorf("SetColumns: cada coluna é {cProp, cRotulo[, cTipo]}")
					}
					cols = append(cols, map[string]any{
						"property": advplrt.ToString(col.Elements[0]),
						"label":    advplrt.ToString(col.Elements[1]),
						"type":     argTexto(col.Elements, 2, "string"),
					})
				}
			}
			a.setProp(c, "columns", cols)
		default:
			return false, nil
		}
	case "ONSELECTED":
		// PoLookup:OnSelected(oLinha) — a linha vem do REST, não de c.rows.
		if c.typ != "lookup" {
			return false, nil
		}
		c.handlers["selected"] = getArg(args, 0)
	default:
		return false, nil
	}
	return true, nil
}
