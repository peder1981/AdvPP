package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

func mountDe(t *testing.T, v *VM) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(v.poApp.mountJSON(), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPoMenuNoMount(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp  := PoApp():New("T")
	Local oMenu := PoMenu():New(oApp)
	Local oPg   := PoPage():New(oApp, "P")
	oMenu:AddItem("Início", "an an-house", {|| .T.})
	oMenu:AddItem("Sobre", "", {|| .T.})
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	menu, _ := mountDe(t, v)["menu"].(map[string]any)
	if menu == nil || menu["type"] != "menu" {
		t.Fatalf("menu = %v", menu)
	}
	items, _ := json.Marshal(menu["props"].(map[string]any)["items"])
	if string(items) != `[{"icon":"an an-house","index":0,"label":"Início"},{"icon":"","index":1,"label":"Sobre"}]` {
		t.Errorf("items = %s", items)
	}
}

func TestPoMenuUnico(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oM1  := PoMenu():New(oApp)
	Local oM2  := PoMenu():New(oApp)
Return
`)
	if err == nil || !strings.Contains(err.Error(), "só um PoMenu") {
		t.Errorf("erro = %v", err)
	}
}

func TestPoContainersAninhados(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp  := PoApp():New("T")
	Local oPg   := PoPage():New(oApp, "P")
	Local oCt   := PoContainer():New(oPg, "Caixa")
	Local oWg   := PoWidget():New(oCt, "Cartão")
	Local oDv   := PoDivider():New(oWg, "Seção")
	Local oTabs := PoTabs():New(oWg)
	Local oT1   := PoTab():New(oTabs, "A")
	Local oT2   := PoTab():New(oTabs, "B")
	Local oBtn  := PoButton():New(oT2, "Dentro")
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	s := string(v.poApp.mountJSON())
	for _, frag := range []string{`"type":"container"`, `"title":"Caixa"`, `"type":"widget"`, `"type":"divider"`, `"label":"Seção"`, `"type":"tabs"`, `"label":"Dentro"`} {
		if !strings.Contains(s, frag) {
			t.Errorf("mount sem %s: %s", frag, s)
		}
	}
	if strings.Count(s, `"active":true`) != 1 || strings.Count(s, `"active":false`) != 1 {
		t.Errorf("abas: só a primeira ativa: %s", s)
	}
}

func TestPoPaiDeTabInvalido(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oT   := PoTab():New(oPg, "x")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "pai inválido") {
		t.Errorf("erro = %v", err)
	}
}

func TestSetColsInvalido(t *testing.T) {
	for _, n := range []string{"0", "13"} {
		_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oB   := PoButton():New(oPg, "x")
	oB:SetCols(`+n+`)
Return
`)
		if err == nil || !strings.Contains(err.Error(), "use de 1 a 12") {
			t.Errorf("SetCols(%s): erro = %v", n, err)
		}
	}
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oB   := PoButton():New(oPg, "x")
	oB:SetCols(6)
Return
`)
	if err != nil || !strings.Contains(string(v.poApp.mountJSON()), `"cols":6`) {
		t.Errorf("SetCols(6): %v %s", err, v.poApp.mountJSON())
	}
}

func TestNavigatePaginaAlheia(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oA1 := PoApp():New("A")
	Local oP1 := PoPage():New(oA1, "P1")
	Local oA2 := PoApp():New("B")
	Local oP2 := PoPage():New(oA2, "P2")
	oA1:Navigate(oP2)
Return
`)
	if err == nil || !strings.Contains(err.Error(), "página de outra aplicação") {
		t.Errorf("erro = %v", err)
	}
}

func TestNavigateRemonta(t *testing.T) {
	// ids: menu 1, página 1 = 2, página 2 = 3
	f := newFakeUI(fakeEv{1, "action", []byte(`0`)}, fakeEv{1, "action", []byte(`1`)})
	_, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp  := PoApp():New("T")
	Local oMenu := PoMenu():New(oApp)
	Local oP1   := PoPage():New(oApp, "Um")
	Local oP2   := PoPage():New(oApp, "Dois")
	oMenu:AddItem("Ir", "", {|| oApp:Navigate(oP2), oApp:Notify("ok", "success")})
	oMenu:AddItem("Sair", "", {|| oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if got := strings.Join(f.ops(), ","); got != "mount,mount,batch,close" {
		t.Fatalf("ops = %s", got)
	}
	if !strings.Contains(string(f.sent[1]), `"title":"Dois"`) {
		t.Errorf("2º mount = %s", f.sent[1])
	}
	if string(f.sent[2]) != `{"op":"batch","ops":[{"op":"notify","text":"ok","type":"success"}]}` {
		t.Errorf("batch = %s", f.sent[2])
	}
}

func TestNotifyAntesDoActivate(t *testing.T) {
	f := newFakeUI(fakeEv{2, "click", nil})
	_, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oB   := PoButton():New(oPg, "x", {|| oApp:Close()})
	oApp:Notify("bem-vindo")
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if got := strings.Join(f.ops(), ","); got != "mount,batch,close" {
		t.Fatalf("ops = %s", got)
	}
	if !strings.Contains(string(f.sent[1]), `"type":"information"`) {
		t.Errorf("batch = %s", f.sent[1])
	}
}
