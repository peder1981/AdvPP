package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

// Onda 3a.3 — texto especializado: PoEmail/PoUrl/PoDecimal/PoRichText/PoSearch.

const telaTexto = `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oEm  := PoEmail():New(oPg, "Email", "a@b.c")
	Local oUr  := PoUrl():New(oPg, "Site", "https://x")
	Local oDe  := PoDecimal():New(oPg, "Valor", 1.5, 2)
	Local oRt  := PoRichText():New(oPg, "Html", "<b>x</b>")
	Local oSe  := PoSearch():New(oPg, "Buscar")
	oDe:SetDecimals(3)
	oDe:SetMin(0)
	oDe:SetMax(999)
	oDe:SetValue(12.5)
	oRt:SetHeight("200px")
	oSe:SetItems({{"codigo","Nome"},{"x","Y"}})
	oSe:SetFilterKeys({"codigo"})
Return
`

func TestPoTextoMount(t *testing.T) {
	v, _, err := rodaPrg(t, telaTexto)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Tree noMount
	}
	if err := json.Unmarshal(v.poApp.mountJSON(), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Tree.Children) != 5 {
		t.Fatalf("filhos = %d", len(m.Tree.Children))
	}
	de := m.Tree.Children[2]
	if de.Type != "decimal" || de.Props["decimalsLength"] != float64(3) {
		t.Fatalf("decimal = %+v", de)
	}
	rt := m.Tree.Children[3]
	if rt.Type != "richtext" || rt.Props["height"] != "200px" {
		t.Fatalf("richtext = %+v", rt)
	}
	se := m.Tree.Children[4]
	if se.Type != "search" {
		t.Fatalf("search type = %s", se.Type)
	}
}

func TestPoTextoSetValueGetValue(t *testing.T) {
	_, out, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oEm  := PoEmail():New(oPg, "E", "a@b.c")
	Local oDe  := PoDecimal():New(oPg, "D", 0, 2)
	oEm:SetValue("novo@x.y")
	oDe:SetValue(7.25)
	ConOut("E=" + cValToChar(oEm:GetValue()) + "|D=" + cValToChar(oDe:GetValue()))
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "E=novo@x.y|D=7.25") {
		t.Errorf("saída = %q", out)
	}
}

func TestPoTextoEventos(t *testing.T) {
	// ids: página 1, email 2, decimal 3, search 4
	f := newFakeUI(
		fakeEv{3, "change", []byte(`12.5`)},
		fakeEv{4, "filter", []byte(`"termo"`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oEm  := PoEmail():New(oPg, "E")
	Local oDe  := PoDecimal():New(oPg, "D")
	Local oSe  := PoSearch():New(oPg, "ph")
	oDe:OnChange({|o, n| ConOut("D=" + ValType(n) + cValToChar(n))})
	oSe:OnFilter({|o, t| ConOut("F=" + ValType(t) + t), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	s := out.String()
	if !strings.Contains(s, "D=N12.5") {
		t.Errorf("decimal change = %q", s)
	}
	if !strings.Contains(s, "F=Ctermo") {
		t.Errorf("search filter = %q", s)
	}
}

func TestPoTextoPaiInvalido(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oBtn := PoButton():New(PoPage():New(oApp, "P"), "b")
	PoEmail():New(oBtn, "e", "x")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "pai inválido") {
		t.Errorf("erro = %v, esperado 'pai inválido'", err)
	}
}
