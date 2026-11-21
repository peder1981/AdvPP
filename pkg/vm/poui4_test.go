package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEditorMountPadrao(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oEd  := AdvCodeEditor():New(oPg, "User Function X()" + Chr(10) + "Return")
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	s := string(v.poApp.mountJSON())
	for _, frag := range []string{`"type":"editor"`, `"language":"advpl"`, `"markers":[]`, `"readonly":false`, `"height":"400px"`, `"value":"User Function X()\nReturn"`} {
		if !strings.Contains(s, frag) {
			t.Errorf("mount sem %s: %s", frag, s)
		}
	}
}

func TestEditorLinguagemInvalida(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oEd  := AdvCodeEditor():New(oPg, "", "cobol")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "linguagem inválida") {
		t.Errorf("erro = %v", err)
	}
}

func TestEditorMarkers(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oEd  := AdvCodeEditor():New(oPg, "x", "tlpp")
	oEd:SetMarkers({{2, 8, "esperado )"}, {5, 1, "aviso", "warning"}})
	oEd:SetReadOnly(.T.)
	oEd:SetHeight("70vh")
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Tree struct {
			Children []struct{ Props map[string]any }
		}
	}
	json.Unmarshal(v.poApp.mountJSON(), &m)
	p := m.Tree.Children[0].Props
	mk, _ := json.Marshal(p["markers"])
	if string(mk) != `[{"col":8,"line":2,"message":"esperado )","severity":"error"},{"col":1,"line":5,"message":"aviso","severity":"warning"}]` {
		t.Errorf("markers = %s", mk)
	}
	if p["readonly"] != true || p["height"] != "70vh" || p["language"] != "tlpp" {
		t.Errorf("props = %v", p)
	}
}

func TestEditorSaveEChange(t *testing.T) {
	// ids: página 1, editor 2
	f := newFakeUI(fakeEv{2, "change", []byte(`"a"`)}, fakeEv{2, "save", []byte(`"novo texto"`)})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oEd  := AdvCodeEditor():New(oPg, "velho")
	oEd:OnChange({|o, c| ConOut("change=" + c)})
	oEd:OnSave({|o, c| ConOut("save=" + c + " get=" + o:GetText()), oEd:ClearMarkers(), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	for _, frag := range []string{"change=a", "save=novo texto get=novo texto"} {
		if !strings.Contains(out.String(), frag) {
			t.Errorf("saída sem %q: %q", frag, out.String())
		}
	}
	// save vindo do browser não ecoa o texto de volta; ClearMarkers gera patch
	if len(f.sent) < 2 || strings.Contains(string(f.sent[1]), "novo texto") || !strings.Contains(string(f.sent[1]), `"markers":[]`) {
		t.Errorf("envios = %q", f.sent)
	}
}

func TestEditorTextoGrande(t *testing.T) {
	grande := strings.Repeat("Ação é ç\r\n", 100000) // ~1,1 MB
	payload, _ := json.Marshal(grande)
	f := newFakeUI(fakeEv{2, "save", payload})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oEd  := AdvCodeEditor():New(oPg, "")
	oEd:OnSave({|o, c| ConOut("len=" + cValToChar(Len(o:GetText())) + " igual=" + cValToChar(o:GetText() == c)), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	want := "len=" + itoa(len(grande)) + " igual=.T."
	if !strings.Contains(out.String(), want) {
		t.Errorf("saída = %q, esperado %q", out.String(), want)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }
