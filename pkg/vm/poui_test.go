package vm

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
	"github.com/advpl/compiler/pkg/preprocessor"
)

// compilarPrg compila um fonte AdvPL pelo mesmo pipeline do advplc
// (pré-processador -> lexer -> parser -> codegen).
func compilarPrg(t *testing.T, src string) *compiler.Bytecode {
	t.Helper()
	pp := preprocessor.NewPreprocessor(nil)
	processed, err := pp.Process(src, "teste.prw")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := lexer.Tokenize(processed, "teste.prw")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.NewParser(tokens, "teste.prw", pp.GetDefines())
	prog, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	prog.RootBoundaryLine = pp.RootBoundaryLine
	bc, err := compiler.Compile(prog)
	if err != nil {
		t.Fatal(err)
	}
	return bc
}

// rodaPrg executa o programa sem UI e devolve a VM, a saída e o erro.
func rodaPrg(t *testing.T, src string) (*VM, string, error) {
	t.Helper()
	v := NewVM(compilarPrg(t, src), false)
	var out bytes.Buffer
	v.SetOutputWriter(&out)
	_, err := v.Run()
	return v, out.String(), err
}

type noMount struct {
	ID       int            `json:"id"`
	Type     string         `json:"type"`
	Props    map[string]any `json:"props"`
	Children []noMount      `json:"children"`
}

const telaCompleta = `
User Function Main()
	Local oApp := PoApp():New("Teste")
	Local oPg  := PoPage():New(oApp, "Pg")
	Local oBtn, oInp, oTab, oTag
	oPg:AddAction("Ok", {|| .T.})
	oBtn := PoButton():New(oPg, "Clique")
	oInp := PoInput():New(oPg, "Nome", "Ana")
	oTab := PoTable():New(oPg)
	oTab:SetColumns({{"a", "A"}, {"n", "N", "number"}})
	oTab:SetItems({{"x", 1}})
	oTag := PoTag():New(oPg, "ok", "success")
Return
`

func TestPoMountJSON(t *testing.T) {
	v, _, err := rodaPrg(t, telaCompleta)
	if err != nil {
		t.Fatal(err)
	}
	if v.poApp == nil {
		t.Fatal("PoApp não registrou estado na VM")
	}
	var m struct {
		Op    string `json:"op"`
		Title string `json:"title"`
		Tree  noMount
	}
	if err := json.Unmarshal(v.poApp.mountJSON(), &m); err != nil {
		t.Fatal(err)
	}
	if m.Op != "mount" || m.Title != "Teste" {
		t.Fatalf("op/title = %q/%q", m.Op, m.Title)
	}
	if m.Tree.Type != "page" || m.Tree.ID != 1 || m.Tree.Props["title"] != "Pg" {
		t.Fatalf("página: %+v", m.Tree)
	}
	acts, _ := json.Marshal(m.Tree.Props["actions"])
	if string(acts) != `[{"index":0,"label":"Ok"}]` {
		t.Errorf("actions = %s", acts)
	}
	tipos := []string{"button", "input", "table", "tag"}
	if len(m.Tree.Children) != len(tipos) {
		t.Fatalf("filhos = %d", len(m.Tree.Children))
	}
	for i, c := range m.Tree.Children {
		if c.Type != tipos[i] || c.ID != i+2 {
			t.Errorf("filho %d = %s/%d", i, c.Type, c.ID)
		}
	}
	if m.Tree.Children[0].Props["label"] != "Clique" {
		t.Errorf("button label = %v", m.Tree.Children[0].Props["label"])
	}
	if m.Tree.Children[1].Props["label"] != "Nome" || m.Tree.Children[1].Props["value"] != "Ana" {
		t.Errorf("input = %v", m.Tree.Children[1].Props)
	}
	items, _ := json.Marshal(m.Tree.Children[2].Props["items"])
	if string(items) != `[{"a":"x","n":1}]` {
		t.Errorf("items = %s", items)
	}
	cols, _ := json.Marshal(m.Tree.Children[2].Props["columns"])
	if string(cols) != `[{"label":"A","property":"a","type":"string"},{"label":"N","property":"n","type":"number"}]` {
		t.Errorf("columns = %s", cols)
	}
	if m.Tree.Children[3].Props["value"] != "ok" || m.Tree.Children[3].Props["type"] != "success" {
		t.Errorf("tag = %v", m.Tree.Children[3].Props)
	}
}

func TestPoSetAntesDeAtivarNaoGeraPatch(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oInp := PoInput():New(oPg, "Nome", "Ana")
	oInp:SetValue("Bia")
	oInp:SetDisabled(.T.)
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(v.poApp.mountJSON()), `"value":"Bia"`) {
		t.Errorf("mount sem o valor novo: %s", v.poApp.mountJSON())
	}
	if !strings.Contains(string(v.poApp.mountJSON()), `"disabled":true`) {
		t.Errorf("mount sem disabled: %s", v.poApp.mountJSON())
	}
	if len(v.poApp.batch) != 0 {
		t.Errorf("batch = %v, esperado vazio antes do Activate", v.poApp.batch)
	}
}

func TestPoGetERetornos(t *testing.T) {
	_, out, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oInp := PoInput():New(oPg, "Nome", "Ana")
	ConOut(oInp:GetValue() + "|" + oInp:Get("label") + "|" + cValToChar(oInp:Id()))
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Ana|Nome|2") {
		t.Errorf("saída = %q", out)
	}
}

func TestPoPaiInvalido(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oBtn := PoButton():New(oApp, "x")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "pai inválido") {
		t.Errorf("erro = %v, esperado 'pai inválido'", err)
	}
}

func TestPoVariasPaginas(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oP1  := PoPage():New(oApp, "Um")
	Local oP2  := PoPage():New(oApp, "Dois")
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.poApp.pages) != 2 {
		t.Fatalf("páginas = %d", len(v.poApp.pages))
	}
	if !strings.Contains(string(v.poApp.mountJSON()), `"title":"Um"`) || strings.Contains(string(v.poApp.mountJSON()), `"Dois"`) {
		t.Errorf("mount deveria trazer só a página corrente: %s", v.poApp.mountJSON())
	}
}

func TestPoActivateForaDoModoWeb(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	oApp:Activate()
Return
`)
	if err == nil || !strings.Contains(err.Error(), "requer o modo web") {
		t.Errorf("erro = %v", err)
	}
}

// Compatibilidade: um programa que já tem uma função chamada PoTag (ou
// qualquer outro nome Po*) continua chamando a sua função, não a classe.
func TestPoNomeCedeAFuncaoDoUsuario(t *testing.T) {
	_, out, err := rodaPrg(t, `
User Function Main()
	ConOut("r=" + PoTag("x"))
Return

Static Function PoTag(c)
Return "minha-" + c
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "r=minha-x") {
		t.Errorf("saída = %q", out)
	}
}
