package vm

import (
	"fmt"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3a.6 — templates dinâmicos ligados ao REST da onda 1. Não há tags
// po-page-dynamic-* nesta versão do PO-UI; as classes AdvPL montam o
// equivalente (lista/tabela + form dinâmico) no frontend, consumindo o
// contrato {items,hasNext} do GET e <url>/<id> para POST/PUT/DELETE.
//
// aFields = {{cProp, cLabel, cTipo, lKey, lFilter, lVisible}, ...}
// cTipo em string|number|date|boolean|currency -> {property,label,type,
// key,filter,visible} do po-dynamic-form/po-table.

// poDynField converte uma linha de aFields para o mapa do protocolo.
func poDynField(el advplrt.Value) (map[string]any, error) {
	arr, ok := advplrt.Deref(el).(*advplrt.ArrayValue)
	if !ok || len(arr.Elements) < 2 {
		return nil, fmt.Errorf("SetFields: cada campo é {cProp, cLabel[, cTipo, lKey, lFilter, lVisible]}")
	}
	tipo := argTexto(arr.Elements, 2, "string")
	switch strings.ToLower(tipo) {
	case "string", "number", "date", "boolean", "currency", "decimal", "integer", "text":
	default:
		return nil, fmt.Errorf("SetFields: tipo inválido %q (string, number, date, boolean, currency)", tipo)
	}
	return map[string]any{
		"property": advplrt.ToString(arr.Elements[0]),
		"label":    advplrt.ToString(arr.Elements[1]),
		"type":     strings.ToLower(tipo),
		"key":      argBool(arr.Elements, 3),
		"filter":   argBool(arr.Elements, 4),
		"visible":  argBoolDef(arr.Elements, 5, true),
	}, nil
}

func argBool(args []advplrt.Value, i int) bool {
	if x := getArg(args, i); !advplrt.IsNil(x) {
		return advplrt.ToBool(x)
	}
	return false
}

func argBoolDef(args []advplrt.Value, i int, def bool) bool {
	if x := getArg(args, i); !advplrt.IsNil(x) {
		return advplrt.ToBool(x)
	}
	return def
}

// poDynFields converte o array aFields inteiro.
func poDynFields(x advplrt.Value) ([]map[string]any, error) {
	out := []map[string]any{}
	arr, _ := advplrt.Deref(x).(*advplrt.ArrayValue)
	if arr == nil {
		return out, nil
	}
	for _, el := range arr.Elements {
		f, err := poDynField(el)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// poDynActions: SetActions(o) — objeto {cAcao: cRotulo} ou array de
// {cAcao, cRotulo} -> map para o frontend.
func poDynActions(x advplrt.Value) map[string]any {
	out := map[string]any{}
	if obj, ok := advplrt.Deref(x).(*advplrt.ObjectValue); ok {
		for _, k := range obj.Keys {
			out[strings.ToLower(k)] = advplrt.ToString(obj.Props[k])
		}
		return out
	}
	if arr, ok := advplrt.Deref(x).(*advplrt.ArrayValue); ok {
		for _, el := range arr.Elements {
			par, ok := advplrt.Deref(el).(*advplrt.ArrayValue)
			if ok && len(par.Elements) >= 2 {
				out[strings.ToLower(advplrt.ToString(par.Elements[0]))] = advplrt.ToString(par.Elements[1])
			}
		}
	}
	return out
}

// poDynPage: tipos dinâmicos são páginas do PoApp.
func poDynPage(typ string) bool {
	switch typ {
	case "dyntable", "dynedit", "dyndetail", "dynsearch":
		return true
	}
	return false
}

// po3aDynMethod despacha os métodos dos templates dinâmicos.
func (v *VM) po3aDynMethod(c *poComp, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	if !poDynPage(c.typ) {
		return false, advplrt.Nil, nil
	}
	a := c.app
	switch method {
	case "SETFIELDS":
		fields, err := poDynFields(getArg(args, 0))
		if err != nil {
			return true, advplrt.Nil, err
		}
		a.setProp(c, "fields", fields)
	case "SETACTIONS":
		a.setProp(c, "actions", poDynActions(getArg(args, 0)))
	case "SETQUICKSEARCHFIELD":
		a.setProp(c, "quickSearchField", advplrt.ToString(getArg(args, 0)))
	case "SETKEEPFILTERS":
		a.setProp(c, "keepFilters", advplrt.ToBool(getArg(args, 0)))
	case "SETFILTERS":
		// PoPageDynamicSearch:SetFilters(a) — mesmos campos de aFields.
		fields, err := poDynFields(getArg(args, 0))
		if err != nil {
			return true, advplrt.Nil, err
		}
		a.setProp(c, "filters", fields)
	case "SETQUICKFILTER":
		a.setProp(c, "quickFilter", advplrt.ToString(getArg(args, 0)))
	case "SETDUPLICATES":
		// PoPageDynamicEdit:SetDuplicates(a) — campos que não podem repetir.
		a.setProp(c, "duplicates", poToJSON(getArg(args, 0)))
	case "SETID":
		// PoPageDynamicDetail/Edit:SetId(cId) — registro a carregar. O
		// template do PO-UI lê o id da rota; o frontend entrega uma rota
		// com ele. Vazio: detalhe sem registro, edição em modo inclusão.
		a.setProp(c, "recordId", advplrt.ToString(getArg(args, 0)))
	case "ONCUSTOMACTION":
		c.handlers["customaction"] = getArg(args, 0)
	case "ONQUICKSEARCH":
		c.handlers["quicksearch"] = getArg(args, 0)
	case "ONADVANCEDSEARCH":
		c.handlers["advancedsearch"] = getArg(args, 0)
	case "ONCHANGEDISCLAIMERS":
		c.handlers["changedisclaimers"] = getArg(args, 0)
	case "ONBEFORESAVE":
		c.handlers["beforesave"] = getArg(args, 0)
	case "ONBEFOREREMOVE":
		c.handlers["beforeremove"] = getArg(args, 0)
	default:
		return false, advplrt.Nil, nil
	}
	return true, advplrt.Nil, nil
}
