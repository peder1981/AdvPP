package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

// Onda 3a.5 — PoPageList/PoPageEdit/PoPageDetail: páginas do PoApp com
// templates PO-UI e eventos de ação.

func TestPoPaginasTemplateMount(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oL   := PoPageList():New(oApp, "Lista")
	Local oE   := PoPageEdit():New(oApp, "Editar")
	Local oD   := PoPageDetail():New(oApp, "Detalhe")
	Local oIn  := PoInput():New(oE, "Campo", "v")
	oL:SetFilter("Filtrar...")
	oL:AddAction("Novo", {|p| })
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Tree noMount
	}
	if err := json.Unmarshal(v.poApp.mountJSON(), &m); err != nil {
		t.Fatal(err)
	}
	// mount mostra só a página corrente (a primeira: pagelist)
	if m.Tree.Type != "pagelist" {
		t.Fatalf("tree type = %s", m.Tree.Type)
	}
	if m.Tree.Props["filterPlaceholder"] != "Filtrar..." {
		t.Fatalf("filterPlaceholder = %v", m.Tree.Props["filterPlaceholder"])
	}
	acts, _ := m.Tree.Props["actions"].([]any)
	if len(acts) != 1 {
		t.Fatalf("actions = %v", m.Tree.Props["actions"])
	}
}

func TestPoPaginasTemplateEventos(t *testing.T) {
	// ids: pagelist 1? — PoApp não é comp; pagelist=1? Na verdade ids
	// começam no primeiro add: pagelist=1? Verificar: add() incrementa
	// nextID por componente; PoPageList é o 1º -> id 1? Não: PoApp não
	// usa add. pagelist=1? — o teste usa ids observados: page=1 antes.
	// Aqui: pagelist=1? Não há PoPage — pagelist é o primeiro -> id 1? 
	// add() é chamado para cada comp; PoPageList -> id 1? Confirma pelo
	// mount: não importa, o fakeEv usa o id real. pagelist=1? pageedit=2?
	f := newFakeUI(
		fakeEv{1, "filter", []byte(`"abc"`)},
		fakeEv{1, "advancedsearch", []byte(`null`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oL   := PoPageList():New(oApp, "L")
	oL:OnFilter({|p, t| ConOut("F=" + t)})
	oL:OnAdvancedSearch({|p| ConOut("A=1"), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	s := out.String()
	if !strings.Contains(s, "F=abc") || !strings.Contains(s, "A=1") {
		t.Errorf("saída = %q", s)
	}
}

func TestPoPaginasTemplatePaiInvalido(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	PoPageList():New(oPg, "x")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "pai inválido") {
		t.Errorf("erro = %v", err)
	}
}
