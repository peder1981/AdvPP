package vm

import (
	"fmt"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// poClasses: nome da classe AdvPL (maiúsculo) -> ClassName e tipo do componente.
var poClasses = map[string]struct{ class, typ string }{
	"POAPP":    {"PoApp", ""},
	"POPAGE":   {"PoPage", "page"},
	"POBUTTON": {"PoButton", "button"},
	"POINPUT":  {"PoInput", "input"},
	"POTABLE":  {"PoTable", "table"},
	"POTAG":    {"PoTag", "tag"},
}

// isPoClass diz se a ClassName é de um componente Po* (despacho de métodos).
func isPoClass(className string) bool {
	_, ok := poClasses[strings.ToUpper(className)]
	return ok
}

// newPoObject cria a instância vazia; o estado nasce em :New().
func newPoObject(upperName string) *advplrt.ObjectValue {
	return advplrt.NewObject(poClasses[upperName].class, nil)
}

func (v *VM) callPoMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	if obj.ClassName == "PoApp" {
		return v.callPoAppMethod(obj, method, args)
	}
	if method == "NEW" {
		return v.poNew(obj, args)
	}
	c, ok := obj.Native.(*poComp)
	if !ok {
		return fmt.Errorf("%s: chame :New() antes de usar o componente", obj.ClassName)
	}
	return v.callPoCompMethod(c, method, args)
}

func (v *VM) callPoAppMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	if method == "NEW" {
		a := newPoAppState(advplrt.ToString(getArg(args, 0)))
		obj.Native = a
		v.poApp = a
		v.push(obj)
		return nil
	}
	a, ok := obj.Native.(*poAppState)
	if !ok {
		return fmt.Errorf("PoApp: chame :New() antes de usar")
	}
	switch method {
	case "ACTIVATE":
		ui, ok := v.uiProvider.(ComponentUI)
		if !ok {
			return fmt.Errorf("PoApp: requer o modo web (advplc serve)")
		}
		if err := v.poActivate(a, ui); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown method %s on PoApp", method)
	}
	v.push(advplrt.Nil)
	return nil
}

// poNew trata :New(oPai, ...) dos componentes.
func (v *VM) poNew(obj *advplrt.ObjectValue, args []advplrt.Value) error {
	info := poClasses[strings.ToUpper(obj.ClassName)]
	pai, _ := getArg(args, 0).(*advplrt.ObjectValue)
	var c *poComp
	if info.typ == "page" {
		a, ok := paiNative[*poAppState](pai)
		if !ok {
			return fmt.Errorf("%s: pai inválido (esperado PoApp)", obj.ClassName)
		}
		if a.page != nil {
			return fmt.Errorf("PoApp: só uma PoPage por aplicação nesta versão")
		}
		c = a.add("page", obj)
		a.page = c
		c.props["title"] = advplrt.ToString(getArg(args, 1))
		c.props["actions"] = []map[string]any{}
	} else {
		pg, ok := paiNative[*poComp](pai)
		if !ok || pg.typ != "page" {
			return fmt.Errorf("%s: pai inválido (esperado PoPage)", obj.ClassName)
		}
		c = pg.app.add(info.typ, obj)
		pg.children = append(pg.children, c)
		switch info.typ {
		case "button":
			c.props["label"] = advplrt.ToString(getArg(args, 1))
			if b, ok := getArg(args, 2).(*advplrt.CodeBlockValue); ok {
				c.handlers["click"] = b
			}
		case "input":
			c.props["label"] = advplrt.ToString(getArg(args, 1))
			c.props["value"] = argTexto(args, 2, "")
		case "table":
			c.props["columns"] = []map[string]any{}
			c.props["items"] = []map[string]any{}
		case "tag":
			c.props["value"] = advplrt.ToString(getArg(args, 1))
			c.props["type"] = argTexto(args, 2, "neutral")
		}
	}
	obj.Native = c
	v.push(obj)
	return nil
}

func paiNative[T any](pai *advplrt.ObjectValue) (T, bool) {
	var zero T
	if pai == nil {
		return zero, false
	}
	t, ok := pai.Native.(T)
	return t, ok
}

// argTexto: argumento como texto, ou def se ausente/NIL.
func argTexto(args []advplrt.Value, i int, def string) string {
	if x := getArg(args, i); !advplrt.IsNil(x) {
		return advplrt.ToString(x)
	}
	return def
}

func (v *VM) callPoCompMethod(c *poComp, method string, args []advplrt.Value) error {
	a := c.app
	var ret advplrt.Value = advplrt.Nil
	switch method {
	case "SET":
		a.setProp(c, advplrt.ToString(getArg(args, 0)), poToJSON(getArg(args, 1)))
	case "GET":
		ret = poFromJSON(c.props[advplrt.ToString(getArg(args, 0))])
	case "SETVALUE":
		a.setProp(c, "value", poToJSON(getArg(args, 0)))
	case "GETVALUE":
		ret = poFromJSON(c.props["value"])
	case "SETLABEL":
		a.setProp(c, "label", advplrt.ToString(getArg(args, 0)))
	case "SETVISIBLE":
		a.setProp(c, "visible", advplrt.ToBool(getArg(args, 0)))
	case "SETDISABLED":
		a.setProp(c, "disabled", advplrt.ToBool(getArg(args, 0)))
	case "ID":
		ret = advplrt.NewNumber(float64(c.id))
	case "ONCLICK":
		c.handlers["click"] = getArg(args, 0)
	case "ONCHANGE":
		c.handlers["change"] = getArg(args, 0)
	case "ONSELECT":
		c.handlers["select"] = getArg(args, 0)
	case "ADDACTION":
		if c.typ != "page" {
			return fmt.Errorf("AddAction: só em PoPage")
		}
		acts := c.props["actions"].([]map[string]any)
		n := len(acts)
		c.handlers[fmt.Sprintf("action:%d", n)] = getArg(args, 1)
		a.setProp(c, "actions", append(acts, map[string]any{"label": advplrt.ToString(getArg(args, 0)), "index": n}))
	case "SETCOLUMNS":
		if c.typ != "table" {
			return fmt.Errorf("SetColumns: só em PoTable")
		}
		cols := []map[string]any{}
		c.columns = nil
		if arr, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
			for _, el := range arr.Elements {
				col, ok := el.(*advplrt.ArrayValue)
				if !ok || len(col.Elements) < 2 {
					return fmt.Errorf("SetColumns: cada coluna é {cProp, cRotulo[, cTipo]}")
				}
				prop := advplrt.ToString(col.Elements[0])
				c.columns = append(c.columns, prop)
				cols = append(cols, map[string]any{"property": prop, "label": advplrt.ToString(col.Elements[1]), "type": argTexto(col.Elements, 2, "string")})
			}
		}
		a.setProp(c, "columns", cols)
	case "SETITEMS":
		if c.typ != "table" {
			return fmt.Errorf("SetItems: só em PoTable")
		}
		items := []map[string]any{}
		c.rows = nil
		if arr, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
			for _, el := range arr.Elements {
				linha, _ := el.(*advplrt.ArrayValue)
				it := map[string]any{}
				for i, prop := range c.columns {
					if linha != nil && i < len(linha.Elements) {
						it[prop] = poToJSON(linha.Elements[i])
					}
				}
				items = append(items, it)
				c.rows = append(c.rows, el)
			}
		}
		a.setProp(c, "items", items)
	default:
		return fmt.Errorf("unknown method %s on %s", method, c.obj.ClassName)
	}
	v.push(ret)
	return nil
}
