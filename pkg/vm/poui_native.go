package vm

import (
	"fmt"
	"regexp"
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
	// fase 2: estrutura
	"POMENU":      {"PoMenu", "menu"},
	"POCONTAINER": {"PoContainer", "container"},
	"POWIDGET":    {"PoWidget", "widget"},
	"PODIVIDER":   {"PoDivider", "divider"},
	"POTABS":      {"PoTabs", "tabs"},
	"POTAB":       {"PoTab", "tab"},
	// fase 2: dados e entradas
	"POTREE":     {"PoTree", "tree"},
	"POLIST":     {"PoList", "list"},
	"POINFO":     {"PoInfo", "info"},
	"POPROGRESS": {"PoProgress", "progress"},
	"POGAUGE":    {"PoGauge", "gauge"},
	"POCHART":    {"PoChart", "chart"},
	"POPASSWORD": {"PoPassword", "password"},
	"POTEXTAREA": {"PoTextarea", "textarea"},
	"PONUMBER":   {"PoNumber", "number"},
	"POSELECT":   {"PoSelect", "select"},
	"POCHECKBOX": {"PoCheckbox", "checkbox"},
	"POSWITCH":   {"PoSwitch", "switch"},
}

// reNomeApp: mesma regra de webui.NomeDeAppValido (pkg/vm não importa webui).
var reNomeApp = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// iconesValidos: nomes de ícone Animalia do PO-UI embutido (sem o prefixo
// "an-"), definidos pelo modo web (webui.Icones). nil = sem validação.
var iconesValidos map[string]bool

// DefinirIconesValidos informa os ícones aceitos por PoMenu:AddItem.
func DefinirIconesValidos(m map[string]bool) { iconesValidos = m }

// poIcone normaliza "an an-house", "an-house" ou "house" para "an an-house"
// e recusa ícone inexistente (com a lista definida).
func poIcone(orig string) (string, error) {
	f := strings.Fields(orig)
	if len(f) == 0 {
		return "", nil
	}
	nome := strings.TrimPrefix(f[len(f)-1], "an-")
	if iconesValidos != nil && !iconesValidos[nome] {
		return "", fmt.Errorf("PoMenu:AddItem: ícone inválido %q (use um ícone Animalia do PO-UI, ex.: \"an an-house\")", orig)
	}
	return "an an-" + nome, nil
}

// poChartTypes: tipos aceitos por PoChart (PoChartType do PO-UI).
var poChartTypes = map[string]bool{"line": true, "column": true, "bar": true, "pie": true, "donut": true, "area": true}

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
		a.obj = obj
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
		if a.active {
			return fmt.Errorf("PoApp:Activate: a tela já está ativa (Activate chamado dentro de um evento?)")
		}
		ui, ok := v.uiProvider.(ComponentUI)
		if !ok {
			return fmt.Errorf("PoApp: requer o modo web (advplc serve)")
		}
		if err := v.poActivate(a, ui); err != nil {
			return err
		}
	case "CLOSE":
		a.closed = true
	case "SETTIMER":
		id, err := a.setTimer(int(advplrt.ToFloat(getArg(args, 0))), getArg(args, 1))
		if err != nil {
			return err
		}
		v.push(advplrt.NewNumber(float64(id)))
		return nil
	case "KILLTIMER":
		a.killTimer(int(advplrt.ToFloat(getArg(args, 0))))
	case "NAVIGATE":
		pg, ok := paiNative[*poComp](asObj(getArg(args, 0)))
		if !ok || pg.typ != "page" || pg.app != a {
			return fmt.Errorf("PoApp:Navigate: página de outra aplicação")
		}
		a.cur = pg
		a.remount = a.active
	case "SPAWN":
		var argv []string
		if arr, ok := getArg(args, 1).(*advplrt.ArrayValue); ok {
			for _, el := range arr.Elements {
				argv = append(argv, advplrt.ToString(el))
			}
		}
		id := a.spawn(advplrt.ToString(getArg(args, 0)), argv, getArg(args, 2), getArg(args, 3))
		v.push(advplrt.NewNumber(float64(id)))
		return nil
	case "KILLSPAWN":
		a.killSpawn(int(advplrt.ToFloat(getArg(args, 0))))
	case "CONFIRM":
		a.nextID++
		a.confirms[a.nextID] = [2]advplrt.Value{getArg(args, 1), getArg(args, 2)}
		a.addOp(map[string]any{"op": "confirm", "id": a.nextID, "text": advplrt.ToString(getArg(args, 0)), "title": argTexto(args, 3, "Confirmação")})
	case "OPEN":
		app := advplrt.ToString(getArg(args, 0))
		if !reNomeApp.MatchString(app) {
			return fmt.Errorf("PoApp:Open: nome de app inválido %q (letras, dígitos, _ e -)", app)
		}
		a.addOp(map[string]any{"op": "open", "app": app})
	case "LOADING":
		a.addOp(map[string]any{"op": "loading", "on": advplrt.ToBool(getArg(args, 0)), "text": argTexto(args, 1, "")})
	case "NOTIFY":
		a.addOp(map[string]any{"op": "notify", "text": advplrt.ToString(getArg(args, 0)), "type": argTexto(args, 1, "information")})
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
	switch info.typ {
	case "page", "menu":
		a, ok := paiNative[*poAppState](pai)
		if !ok {
			return fmt.Errorf("%s: pai inválido (esperado PoApp)", obj.ClassName)
		}
		if info.typ == "menu" {
			if a.menu != nil {
				return fmt.Errorf("PoApp: só um PoMenu por aplicação")
			}
			c = a.add("menu", obj)
			a.menu = c
			c.props["items"] = []map[string]any{}
			break
		}
		c = a.add("page", obj)
		a.pages = append(a.pages, c)
		if a.cur == nil {
			a.cur = c
		}
		c.props["title"] = advplrt.ToString(getArg(args, 1))
		c.props["actions"] = []map[string]any{}
	default:
		p, ok := paiNative[*poComp](pai)
		if info.typ == "tab" {
			ok = ok && p.typ == "tabs"
		} else {
			ok = ok && p.app.isContainer(p)
		}
		if !ok {
			return fmt.Errorf("%s: pai inválido", obj.ClassName)
		}
		c = p.app.add(info.typ, obj)
		if info.typ == "tab" {
			c.props["label"] = advplrt.ToString(getArg(args, 1))
			c.props["active"] = len(p.children) == 0
		}
		p.children = append(p.children, c)
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
		case "container", "widget":
			c.props["title"] = argTexto(args, 1, "")
		case "divider":
			c.props["label"] = argTexto(args, 1, "")
		case "tree", "list":
			c.props["items"] = []map[string]any{}
		case "info":
			c.props["label"] = advplrt.ToString(getArg(args, 1))
			c.props["value"] = poToJSON(getArg(args, 2))
		case "progress":
			c.props["value"] = 0.0
			c.props["text"] = argTexto(args, 1, "")
		case "gauge":
			c.props["title"] = advplrt.ToString(getArg(args, 1))
			c.props["value"] = 0.0
			c.props["description"] = ""
		case "chart":
			tipo := argTexto(args, 2, "line")
			if !poChartTypes[tipo] {
				return fmt.Errorf("PoChart: tipo inválido %q (line, column, bar, pie, donut, area)", tipo)
			}
			c.props["title"] = advplrt.ToString(getArg(args, 1))
			c.props["type"] = tipo
			c.props["categories"] = []any{}
			c.props["series"] = []map[string]any{}
		case "password", "textarea":
			c.props["label"] = advplrt.ToString(getArg(args, 1))
			c.props["value"] = argTexto(args, 2, "")
		case "number":
			c.props["label"] = advplrt.ToString(getArg(args, 1))
			c.props["value"] = poToJSON(getArg(args, 2))
		case "select":
			c.props["label"] = advplrt.ToString(getArg(args, 1))
			c.props["options"] = poOpcoes(getArg(args, 2))
			c.props["value"] = argTexto(args, 3, "")
		case "checkbox", "switch":
			c.props["label"] = advplrt.ToString(getArg(args, 1))
			c.props["value"] = advplrt.ToBool(getArg(args, 2))
		}
	}
	obj.Native = c
	v.push(obj)
	return nil
}

// poOpcoes: opções de PoSelect — texto, ou {cRotulo, cValor}.
func poOpcoes(x advplrt.Value) []map[string]any {
	out := []map[string]any{}
	arr, _ := x.(*advplrt.ArrayValue)
	if arr == nil {
		return out
	}
	for _, el := range arr.Elements {
		if par, ok := el.(*advplrt.ArrayValue); ok && len(par.Elements) >= 2 {
			out = append(out, map[string]any{"label": advplrt.ToString(par.Elements[0]), "value": advplrt.ToString(par.Elements[1])})
		} else {
			t := advplrt.ToString(el)
			out = append(out, map[string]any{"label": t, "value": t})
		}
	}
	return out
}

// poArvore: itens de PoTree, recursivo — {cRotulo, cValor[, aFilhos]}.
func poArvore(x advplrt.Value) []map[string]any {
	out := []map[string]any{}
	arr, _ := x.(*advplrt.ArrayValue)
	if arr == nil {
		return out
	}
	for _, el := range arr.Elements {
		it, ok := el.(*advplrt.ArrayValue)
		if !ok || len(it.Elements) < 2 {
			continue
		}
		n := map[string]any{"label": advplrt.ToString(it.Elements[0]), "value": advplrt.ToString(it.Elements[1])}
		if len(it.Elements) > 2 {
			if filhos := poArvore(it.Elements[2]); len(filhos) > 0 {
				n["subItems"] = filhos
			}
		}
		out = append(out, n)
	}
	return out
}

// poLimita: valor de progresso/gauge entre 0 e 100.
func poLimita(x advplrt.Value) float64 {
	f := advplrt.ToFloat(x)
	if f < 0 {
		return 0
	}
	if f > 100 {
		return 100
	}
	return f
}

// asObj: o valor como objeto, ou nil.
func asObj(x advplrt.Value) *advplrt.ObjectValue {
	o, _ := x.(*advplrt.ObjectValue)
	return o
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
		if c.typ == "progress" || c.typ == "gauge" {
			a.setProp(c, "value", poLimita(getArg(args, 0)))
		} else {
			a.setProp(c, "value", poToJSON(getArg(args, 0)))
		}
	case "SETDESCRIPTION":
		a.setProp(c, "description", advplrt.ToString(getArg(args, 0)))
	case "SETCATEGORIES", "SETSERIES", "SETMAXPOINTS", "ADDPOINT":
		if c.typ != "chart" {
			return fmt.Errorf("%s: só em PoChart", method)
		}
		v.poChartMethod(c, method, args)
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
	case "SETCOLS":
		n := int(advplrt.ToFloat(getArg(args, 0)))
		if n < 1 || n > 12 {
			return fmt.Errorf("SetCols: use de 1 a 12")
		}
		a.setProp(c, "cols", n)
	case "ADDITEM":
		if c.typ != "menu" {
			return fmt.Errorf("AddItem: só em PoMenu")
		}
		icone, err := poIcone(argTexto(args, 1, ""))
		if err != nil {
			return err
		}
		items := c.props["items"].([]map[string]any)
		n := len(items)
		c.handlers[fmt.Sprintf("action:%d", n)] = getArg(args, 2)
		a.setProp(c, "items", append(items, map[string]any{"label": advplrt.ToString(getArg(args, 0)), "icon": icone, "index": n}))
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
		if c.typ == "tree" {
			a.setProp(c, "items", poArvore(getArg(args, 0)))
			break
		}
		if c.typ == "list" {
			items := []map[string]any{}
			c.rows = nil
			if arr, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
				for _, el := range arr.Elements {
					linha, _ := el.(*advplrt.ArrayValue)
					it := map[string]any{"title": ""}
					if linha != nil && len(linha.Elements) > 0 {
						it["title"] = advplrt.ToString(linha.Elements[0])
					}
					// sem detalhe: nem envia a chave (a lista não mostra linha vazia)
					if linha != nil && len(linha.Elements) > 1 {
						if d := advplrt.ToString(linha.Elements[1]); d != "" {
							it["detail"] = d
						}
					}
					items = append(items, it)
					c.rows = append(c.rows, el)
				}
			}
			a.setProp(c, "items", items)
			break
		}
		if c.typ != "table" {
			return fmt.Errorf("SetItems: só em PoTable, PoTree e PoList")
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

// poChartMethod: SetCategories, SetSeries, SetMaxPoints e AddPoint.
func (v *VM) poChartMethod(c *poComp, method string, args []advplrt.Value) {
	a := c.app
	switch method {
	case "SETCATEGORIES":
		cats, _ := poToJSON(getArg(args, 0)).([]any)
		a.setProp(c, "categories", poJanela(cats, c.maxPts))
	case "SETSERIES":
		series := []map[string]any{}
		if arr, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
			for _, el := range arr.Elements {
				s, ok := el.(*advplrt.ArrayValue)
				if !ok || len(s.Elements) < 1 {
					continue
				}
				dados := []any{}
				if len(s.Elements) > 1 {
					dados, _ = poToJSON(s.Elements[1]).([]any)
				}
				series = append(series, map[string]any{"label": advplrt.ToString(s.Elements[0]), "data": poJanela(dados, c.maxPts)})
			}
		}
		a.setProp(c, "series", series)
	case "SETMAXPOINTS":
		c.maxPts = int(advplrt.ToFloat(getArg(args, 0)))
	case "ADDPOINT":
		cats := append(c.props["categories"].([]any), poToJSON(getArg(args, 0)))
		vals, _ := poToJSON(getArg(args, 1)).([]any)
		old := c.props["series"].([]map[string]any)
		series := make([]map[string]any, len(old))
		for i, s := range old {
			var val any
			if i < len(vals) {
				val = vals[i]
			}
			series[i] = map[string]any{"label": s["label"], "data": poJanela(append(s["data"].([]any), val), c.maxPts)}
		}
		a.setProps(c, map[string]any{"categories": poJanela(cats, c.maxPts), "series": series})
	}
	// sem push: callPoCompMethod empilha o retorno (Nil) uma única vez
}

// poJanela mantém só os últimos max itens (max <= 0: todos). Copia, para
// não compartilhar o array com um patch já enviado.
func poJanela(xs []any, max int) []any {
	if xs == nil {
		xs = []any{}
	}
	if max > 0 && len(xs) > max {
		xs = xs[len(xs)-max:]
	}
	return append([]any{}, xs...)
}
