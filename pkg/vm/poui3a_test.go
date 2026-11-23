package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

// Onda 3a.1 — componentes de seleção. Cobre mount (props iniciais),
// patch (setters) e eventos (change/selected) conforme a receita do plano.

const telaSelecao = `
User Function Main()
	Local oApp := PoApp():New("Sel")
	Local oPg  := PoPage():New(oApp, "Pg")
	Local oCmb := PoCombo():New(oPg, "UF", {{"SP", "São Paulo"}, {"RJ", "Rio"}}, "SP")
	Local oMul := PoMultiselect():New(oPg, "UFs", {{"SP", "São Paulo"}, {"RJ", "Rio"}}, {"SP"})
	Local oRad := PoRadioGroup():New(oPg, "Sexo", {{"M", "Masc"}, {"F", "Fem"}}, "M")
	Local oChk := PoCheckboxGroup():New(oPg, "Tipos", {{"A", "Tipo A"}, {"B", "Tipo B"}}, {"A"})
	Local oLuk := PoLookup():New(oPg, "Cliente", "/clientes", "A1_COD", "A1_NOME")
	oLuk:SetColumns({{"A1_COD", "Código"}, {"A1_NOME", "Nome"}})
Return
`

func TestPoSelecaoMount(t *testing.T) {
	v, _, err := rodaPrg(t, telaSelecao)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Tree noMount
	}
	if err := json.Unmarshal(v.poApp.mountJSON(), &m); err != nil {
		t.Fatal(err)
	}
	// ids: página 1, combo 2, multiselect 3, radiogroup 4, checkboxgroup 5, lookup 6
	if len(m.Tree.Children) != 5 {
		t.Fatalf("filhos = %d", len(m.Tree.Children))
	}
	cmb := m.Tree.Children[0]
	if cmb.Type != "combo" || cmb.Props["label"] != "UF" || cmb.Props["value"] != "SP" {
		t.Fatalf("combo = %+v", cmb)
	}
	opts, _ := json.Marshal(cmb.Props["options"])
	if string(opts) != `[{"label":"São Paulo","value":"SP"},{"label":"Rio","value":"RJ"}]` {
		t.Errorf("combo options = %s", opts)
	}
	mul := m.Tree.Children[1]
	if mul.Type != "multiselect" {
		t.Fatalf("multiselect type = %s", mul.Type)
	}
	vals, _ := json.Marshal(mul.Props["value"])
	if string(vals) != `["SP"]` {
		t.Errorf("multiselect value = %s", vals)
	}
	rad := m.Tree.Children[2]
	if rad.Type != "radiogroup" || rad.Props["columns"] != float64(1) {
		t.Fatalf("radiogroup = %+v", rad)
	}
	chk := m.Tree.Children[3]
	if chk.Type != "checkboxgroup" {
		t.Fatalf("checkboxgroup type = %s", chk.Type)
	}
	luk := m.Tree.Children[4]
	if luk.Type != "lookup" {
		t.Fatalf("lookup type = %s", luk.Type)
	}
	if luk.Props["filterUrl"] != "/rest/clientes" {
		t.Errorf("lookup filterUrl = %v", luk.Props["filterUrl"])
	}
	if luk.Props["fieldValue"] != "A1_COD" || luk.Props["fieldLabel"] != "A1_NOME" {
		t.Errorf("lookup fields = %v", luk.Props)
	}
	cols, _ := json.Marshal(luk.Props["columns"])
	if string(cols) != `[{"label":"Código","property":"A1_COD","type":"string"},{"label":"Nome","property":"A1_NOME","type":"string"}]` {
		t.Errorf("lookup columns = %s", cols)
	}
}

func TestPoSelecaoSettersSemActivate(t *testing.T) {
	v, out, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oCmb := PoCombo():New(oPg, "UF", {{"SP", "São Paulo"}}, "")
	Local oRad := PoRadioGroup():New(oPg, "S", {{"M", "M"}}, "M")
	Local oLuk := PoLookup():New(oPg, "Cli", "/cli", "COD", "NOM")
	oCmb:SetOptions({{"RJ", "Rio"}, {"MG", "Minas"}})
	oCmb:SetValue("RJ")
	oRad:SetColumns(3)
	oLuk:SetFilterParams({{"filial", "01"}})
	ConOut("V=" + oCmb:GetValue() + "|C=" + cValToChar(oRad:Get("columns")))
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "V=RJ|C=3") {
		t.Errorf("saída = %q", out)
	}
	// filterParams {{cProp,xVal}} vira objeto plano para o PoLookupFilter.
	var m struct {
		Tree noMount
	}
	if err := json.Unmarshal(v.poApp.mountJSON(), &m); err != nil {
		t.Fatal(err)
	}
	fp, _ := json.Marshal(m.Tree.Children[2].Props["filterParams"])
	if string(fp) != `{"filial":"01"}` {
		t.Errorf("filterParams = %s", fp)
	}
}

func TestPoSelecaoEventos(t *testing.T) {
	// ids: página 1, multiselect 2, lookup 3
	f := newFakeUI(
		fakeEv{2, "change", []byte(`["SP","RJ"]`)},
		fakeEv{3, "selected", []byte(`{"A1_COD":"0001","A1_NOME":"Fulano"}`)},
		fakeEv{3, "change", []byte(`"0001"`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("Sel")
	Local oPg  := PoPage():New(oApp, "Pg")
	Local oMul := PoMultiselect():New(oPg, "UFs", {{"SP", "SP"}, {"RJ", "RJ"}}, {})
	Local oLuk := PoLookup():New(oPg, "Cli", "/cli", "COD", "NOM")
	oMul:OnChange({|o, cVal| ConOut("MUL=" + cValToChar(Len(cVal)) + ":" + cVal[1] + cVal[2])})
	oLuk:OnSelected({|o, oLin| ConOut("SEL=" + oLin["A1_NOME"])})
	oLuk:OnChange({|o, cVal| ConOut("LUK=" + cVal), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	s := out.String()
	if !strings.Contains(s, "MUL=2:SPRJ") {
		t.Errorf("multiselect change = %q", s)
	}
	if !strings.Contains(s, "SEL=Fulano") {
		t.Errorf("lookup selected = %q", s)
	}
	if !strings.Contains(s, "LUK=0001") {
		t.Errorf("lookup change = %q", s)
	}
}

func TestPoSelecaoPaiInvalido(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oCmb := PoCombo():New(oApp, "x", {}, "")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "pai inválido") {
		t.Errorf("erro = %v, esperado 'pai inválido'", err)
	}
}
