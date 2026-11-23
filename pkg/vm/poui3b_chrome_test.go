package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

// Onda 3b.3 — chrome do PoApp: PoNavbar, PoHeader e PoMenuPanel são
// singletons serializados no mountJSON (fora da árvore da página).

func TestPo3b3ChromeMount(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P1")
	Local oNv  := PoNavbar():New(oApp)
	Local oHd  := PoHeader():New(oApp)
	Local oMp  := PoMenuPanel():New(oApp)
	oNv:SetLogo("logo.png")
	oNv:AddItem("Home", .T.)
	oNv:AddIconAction("an an-cog", "Conf", .T.)
	oHd:SetUser("Maria", "m@x.com")
	oHd:AddAction("Sair", .T.)
	oMp:AddItem("Menu A", "an an-star", .T.)
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Nav   noMount
		Head  noMount
		Panel noMount
	}
	if err := json.Unmarshal(v.poApp.mountJSON(), &m); err != nil {
		t.Fatal(err)
	}
	if m.Nav.Type != "navbar" || m.Nav.Props["logo"] != "logo.png" {
		t.Fatalf("nav = %+v", m.Nav.Props)
	}
	items, _ := m.Nav.Props["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("nav items = %v", items)
	}
	icons, _ := m.Nav.Props["iconActions"].([]any)
	if len(icons) != 1 {
		t.Fatalf("nav iconActions = %v", icons)
	}
	if m.Head.Type != "header" {
		t.Fatalf("head = %+v", m.Head.Props)
	}
	user, _ := m.Head.Props["user"].(map[string]any)
	if user["name"] != "Maria" || user["email"] != "m@x.com" {
		t.Fatalf("head user = %v", user)
	}
	if m.Panel.Type != "menupanel" {
		t.Fatalf("panel = %+v", m.Panel.Props)
	}
	pitems, _ := m.Panel.Props["items"].([]any)
	if len(pitems) != 1 {
		t.Fatalf("panel items = %v", pitems)
	}
}

func TestPo3b3ChromeSingleton(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P1")
	Local oNv  := PoNavbar():New(oApp)
	Local oNv2 := PoNavbar():New(oApp)
Return
`)
	if err == nil || !strings.Contains(err.Error(), "PoApp") {
		t.Fatalf("esperava erro de singleton, got %v", err)
	}
}

func TestPo3b3ChromeAction(t *testing.T) {
	// ids: page=1, navbar=2 — clique no item dispara "act:N".
	f := newFakeUI(
		fakeEv{2, "act:0", []byte(`null`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P1")
	Local oNv  := PoNavbar():New(oApp)
	oNv:AddItem("Home", {|| ConOut("CLICK=HOME"), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if s := out.String(); !strings.Contains(s, "CLICK=HOME") {
		t.Fatalf("out = %q", s)
	}
}
