package vm

import (
	"encoding/json"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// ComponentUI é a extensão opcional de UIProvider para os componentes Po*
// (modo web, advplc serve). O laço de PoApp:Activate envia operações
// (mount/batch/close) com UISend e lê eventos do browser e dos timers com
// UINext; timers injetam eventos com UIPost.
type ComponentUI interface {
	UISend(op []byte)
	UIPost(id int, event string, value []byte)
	UINext() (id int, event string, value []byte)
}

// poComp é um componente da árvore (página, botão, campo...).
type poComp struct {
	id       int
	typ      string // page | button | input | table | tag
	props    map[string]any
	children []*poComp
	handlers map[string]advplrt.Value // "click", "change", "select", "action:<n>"
	obj      *advplrt.ObjectValue
	app      *poAppState
	columns  []string // PoTable: propriedades na ordem das colunas
	rows     []advplrt.Value
}

// poAppState é o estado de um PoApp (uma por VM ativa por vez).
type poAppState struct {
	title     string
	page      *poComp
	byID      map[int]*poComp
	nextID    int
	active    bool
	closed    bool
	batch     []map[string]any
	timers    map[int]*poTimer
	nextTimer int
	obj       *advplrt.ObjectValue // o próprio PoApp (argumento dos timers)
	ui        ComponentUI          // ligado durante o Activate
}

func newPoAppState(title string) *poAppState {
	return &poAppState{title: title, byID: map[int]*poComp{}, timers: map[int]*poTimer{}}
}

// add registra um componente novo com o próximo id.
func (a *poAppState) add(typ string, obj *advplrt.ObjectValue) *poComp {
	a.nextID++
	c := &poComp{id: a.nextID, typ: typ, props: map[string]any{}, handlers: map[string]advplrt.Value{}, obj: obj, app: a}
	a.byID[c.id] = c
	return c
}

// setProp grava uma propriedade; com a tela ativa, acumula um patch no lote.
func (a *poAppState) setProp(c *poComp, prop string, val any) {
	c.props[prop] = val
	if a.active {
		a.batch = append(a.batch, map[string]any{"op": "patch", "id": c.id, "props": map[string]any{prop: val}})
	}
}

type poNode struct {
	ID       int            `json:"id"`
	Type     string         `json:"type"`
	Props    map[string]any `json:"props"`
	Children []poNode       `json:"children"`
}

func (c *poComp) node() poNode {
	n := poNode{ID: c.id, Type: c.typ, Props: c.props, Children: []poNode{}}
	for _, ch := range c.children {
		n.Children = append(n.Children, ch.node())
	}
	return n
}

// mountJSON é a operação "mount" com a página corrente.
func (a *poAppState) mountJSON() []byte {
	op := struct {
		Op    string  `json:"op"`
		Title string  `json:"title"`
		Tree  *poNode `json:"tree"`
	}{Op: "mount", Title: a.title}
	if a.page != nil {
		n := a.page.node()
		op.Tree = &n
	}
	data, _ := json.Marshal(op)
	return data
}

// poToJSON converte um valor AdvPL para o JSON do protocolo.
func poToJSON(val advplrt.Value) any {
	switch x := val.(type) {
	case *advplrt.StringValue:
		return x.Val
	case *advplrt.NumberValue:
		return x.Val
	case *advplrt.BoolValue:
		return x.Val
	case *advplrt.DateValue:
		if x.Val.IsZero() {
			return ""
		}
		return x.Val.Format("2006-01-02")
	case *advplrt.ArrayValue:
		out := make([]any, len(x.Elements))
		for i, e := range x.Elements {
			out[i] = poToJSON(e)
		}
		return out
	case nil, *advplrt.NilValue:
		return nil
	default:
		return advplrt.ToString(val)
	}
}

// poFromJSON converte um valor do protocolo de volta para AdvPL.
func poFromJSON(x any) advplrt.Value {
	switch v := x.(type) {
	case string:
		return advplrt.NewString(v)
	case float64:
		return advplrt.NewNumber(v)
	case int:
		return advplrt.NewNumber(float64(v))
	case bool:
		return advplrt.NewBool(v)
	case []any:
		el := make([]advplrt.Value, len(v))
		for i, e := range v {
			el[i] = poFromJSON(e)
		}
		return advplrt.NewArray(el)
	default:
		return advplrt.Nil
	}
}
