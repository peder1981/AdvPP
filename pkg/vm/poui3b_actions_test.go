package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

// Onda 3b.2 — ações: PoButtonGroup, PoDropdown, PoPopup, PoPopover,
// PoContextMenu e PoToolbar. Mount, AddAction/AddItem e disparo dos
// eventos act:N / itemselected.

func TestPo3b2Mount(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oBtn := PoButton("Alvo", {||}), oBG, oDD, oPop, oPov, oCM, oTB
	oBtn := PoButton():New(oPg, "Alvo")
	oBG := PoButtonGroup():New(oPg, "single")
	oBG:AddButton("Um", "an an-star", {|| ConOut("BG1")})
	oBG:AddButton("Dois", "", {|| ConOut("BG2")})
	oBG:SetSelected({0})
	oDD := PoDropdown():New(oPg, "Menu")
	oDD:AddAction("Acao1", {|| ConOut("DD1")}, "an an-gear")
	oPop := PoPopup():New(oPg, oBtn)
	oPop:AddAction("Pop1", {|| ConOut("POP1")})
	oPov := PoPopover():New(oPg, oBtn, "Dica")
	oPov:SetTrigger("hover")
	oPov:SetPosition("top")
	oCM := PoContextMenu():New(oPg, "Ctx")
	oCM:AddItem("Item1", {|| ConOut("CM1")})
	oCM:SetSelected(0)
	oTB := PoToolbar():New(oPg, "Barra")
	oTB:AddAction("TA", {|| ConOut("TA1")}, "an an-bell")
	oTB:SetProfile("Zé", "ze@x.com")
	oTB:SetNotifications(3)
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
	kids := m.Tree.Children
	if len(kids) != 7 {
		t.Fatalf("children = %d", len(kids))
	}
	bg := kids[1]
	if bg.Type != "buttongroup" || bg.Props["toggle"] != "single" {
		t.Fatalf("buttongroup = %v", bg.Props)
	}
	btns, _ := bg.Props["buttons"].([]any)
	if len(btns) != 2 {
		t.Fatalf("buttons = %v", bg.Props["buttons"])
	}
	b0, _ := btns[0].(map[string]any)
	if b0["label"] != "Um" || b0["icon"] != "an an-star" || b0["action"] != "act:0" {
		t.Fatalf("button0 = %v", b0)
	}
	pop := kids[3]
	if pop.Type != "popup" || pop.Props["target"] == nil {
		t.Fatalf("popup = %v", pop.Props)
	}
	pov := kids[4]
	if pov.Type != "popover" || pov.Props["trigger"] != "hover" || pov.Props["position"] != "top" {
		t.Fatalf("popover = %v", pov.Props)
	}
	tb := kids[6]
	if tb.Type != "toolbar" {
		t.Fatalf("toolbar = %v", tb.Type)
	}
	prof, _ := tb.Props["profile"].(map[string]any)
	if prof["name"] != "Zé" || prof["email"] != "ze@x.com" {
		t.Fatalf("profile = %v", tb.Props["profile"])
	}
	if tb.Props["notificationNumber"] != float64(3) {
		t.Fatalf("notifications = %v", tb.Props["notificationNumber"])
	}
}

func TestPo3b2Eventos(t *testing.T) {
	// act:N dispara o codeblock; itemselected do contextmenu idem.
	// ids: page=1, bg=2, cm=3, tb=4
	f := newFakeUI(
		fakeEv{2, "act:1", nil},
		fakeEv{3, "act:0", nil},        // clique no item -> ação do item
		fakeEv{3, "itemselected", []byte("0")}, // e o evento de seleção
		fakeEv{4, "act:0", nil},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oBG  := PoButtonGroup():New(oPg, "none")
	oBG:AddButton("A", "", {|| ConOut("BG_A")})
	oBG:AddButton("B", "", {|| ConOut("BG_B")})
	Local oCM  := PoContextMenu():New(oPg, "C")
	oCM:AddItem("I", {|| ConOut("CM_I")})
	oCM:OnSelect({|o, n| ConOut("SEL=" + cValToChar(n))})
	Local oTB  := PoToolbar():New(oPg, "T")
	oTB:AddAction("X", {|| ConOut("TB_X"), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	s := out.String()
	for _, want := range []string{"BG_B", "CM_I", "SEL=", "TB_X"} {
		if !strings.Contains(s, want) {
			t.Errorf("faltou %q em %q", want, s)
		}
	}
}
