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
	// UIPostWait espera vaga na fila (saída de processo). Devolve false sem
	// postar se a sessão encerrou ou se cancel fechou (laço terminado).
	UIPostWait(id int, event string, value []byte, cancel <-chan struct{}) bool
	UINext() (id int, event string, value []byte)
}

// poComp é um componente da árvore (página, botão, campo...).
type poComp struct {
	id       int
	typ      string // page | menu | container | widget | divider | tabs | tab | button | input | table | tag
	props    map[string]any
	children []*poComp
	handlers map[string]advplrt.Value // "click", "change", "select", "action:<n>"
	obj      *advplrt.ObjectValue
	app      *poAppState
	columns  []string // PoTable: propriedades na ordem das colunas
	rows     []advplrt.Value
	maxPts   int // PoChart: pontos mantidos (0 = sem limite)
}

// poAppState é o estado de um PoApp (uma por VM ativa por vez).
type poAppState struct {
	title     string
	pages     []*poComp
	cur       *poComp // página corrente (a que vai no mount)
	menu      *poComp
	remount   bool // Navigate durante um handler: reenvia o mount ao fim dele
	byID      map[int]*poComp
	nextID    int
	active    bool
	closed    bool
	batch     []map[string]any
	timers    map[int]*poTimer
	nextTimer int
	spawns    map[int]*poSpawn
	nextSpawn int
	confirms  map[int][2]advplrt.Value // id -> {bSim, bNao}; apagado na 1ª resposta
	obj       *advplrt.ObjectValue     // o próprio PoApp (argumento dos timers)
	ui        ComponentUI              // ligado durante o Activate
}

func newPoAppState(title string) *poAppState {
	return &poAppState{title: title, byID: map[int]*poComp{}, timers: map[int]*poTimer{},
		spawns: map[int]*poSpawn{}, confirms: map[int][2]advplrt.Value{}}
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

// setProps grava várias propriedades num único patch (ex.: PoChart:AddPoint).
func (a *poAppState) setProps(c *poComp, props map[string]any) {
	for k, v := range props {
		c.props[k] = v
	}
	if a.active {
		a.batch = append(a.batch, map[string]any{"op": "patch", "id": c.id, "props": props})
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

// mountJSON é a operação "mount" com o menu e a página corrente.
func (a *poAppState) mountJSON() []byte {
	op := struct {
		Op    string  `json:"op"`
		Title string  `json:"title"`
		Menu  *poNode `json:"menu"`
		Tree  *poNode `json:"tree"`
	}{Op: "mount", Title: a.title}
	if a.menu != nil {
		n := a.menu.node()
		op.Menu = &n
	}
	if a.cur != nil {
		n := a.cur.node()
		op.Tree = &n
	}
	data, _ := json.Marshal(op)
	return data
}

// addOp acrescenta uma operação não-patch (notify, confirm, loading) ao lote,
// na ordem em que o programa a gerou. Antes do Activate, sai logo após o mount.
func (a *poAppState) addOp(op map[string]any) {
	a.batch = append(a.batch, op)
}

// isContainer: tipos que aceitam componentes de conteúdo como filhos.
func (a *poAppState) isContainer(c *poComp) bool {
	switch c.typ {
	case "page", "container", "widget", "tab":
		return true
	}
	return false
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
