package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

// Onda 3b.3 — navegação: PoLink, PoBreadcrumb (itens com página
// resolvem o objeto no evento navigate) e PoContextTabs.

func TestPo3b3Mount(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg1 := PoPage():New(oApp, "P1")
	Local oPg2 := PoPage():New(oApp, "P2")
	Local oPg3 := PoPage():New(oApp, "P3")
	Local oLk  := PoLink():New(oPg1, "Site", "https://x")
	oLk:SetOpenNewTab(.T.)
	Local oBc  := PoBreadcrumb():New(oPg2, {{"Home", oPg1}, {"Docs", "https://d"}})
	Local oCt  := PoContextTabs():New(oPg3)
	oCt:AddTab("Aba 1", oPg1)
	oCt:AddTab("Aba 2", Nil)
	oCt:CloseTab(1)
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
	// root = primeira página (P1)
	lk := m.Tree.Children[0]
	if lk.Type != "link" || lk.Props["label"] != "Site" || lk.Props["url"] != "https://x" || lk.Props["openNewTab"] != true {
		t.Fatalf("link = %v", lk.Props)
	}
}

func TestPo3b3BreadcrumbNavigate(t *testing.T) {
	// ids: p1=1, p2=2, breadcrumb=3
	f := newFakeUI(
		fakeEv{3, "navigate", []byte(`{"page":1,"label":"Home"}`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg1 := PoPage():New(oApp, "P1")
	Local oPg2 := PoPage():New(oApp, "P2")
	Local oBc  := PoBreadcrumb():New(oPg2, {{"Home", oPg1}})
	oBc:OnNavigate({|o, pg, lbl| ConOut("NAV=" + lbl + ":" + ValType(pg)), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	s := out.String()
	if !strings.Contains(s, "NAV=Home:O") {
		t.Fatalf("navigate = %q", s)
	}
}

func TestPo3b3ContextTabs(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oCt  := PoContextTabs():New(oPg)
	oCt:AddTab("Um", Nil)
	oCt:AddTab("Dois", Nil)
	ConOut("N=" + cValToChar(Len(oCt:Get("tabs"))))
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	_ = v
}
