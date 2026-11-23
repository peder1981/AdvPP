package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

// Onda 3b.1 — agrupamento: PoAccordion(+Item), PoModal, PoStepper(+Step),
// PoSlide. Mount, pais válidos/inválidos e métodos.

func TestPo3bMount(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oAc  := PoAccordion():New(oPg)
	oAc:SetShowManager(.T.)
	Local oIt  := PoAccordionItem():New(oAc, "Secao 1")
	Local oMd  := PoModal():New(oPg, "Modal T")
	oMd:SetSize("lg")
	oMd:SetPrimaryAction("OK", {|| ConOut("ok")})
	Local oSt  := PoStepper():New(oPg)
	Local oSp  := PoStep():New(oSt, "Passo 1")
	Local oSl  := PoSlide():New(oPg, {{"a.png","Alt","/x"},{"b.png"}})
	oSl:SetInterval(3000)
	oSl:SetHeight(200)
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
	if m.Tree.Type != "page" {
		t.Fatalf("root = %s", m.Tree.Type)
	}
	kids := m.Tree.Children
	if len(kids) != 4 {
		t.Fatalf("page children = %d: %v", len(kids), kids)
	}
	ac := kids[0]
	if ac.Type != "accordion" || ac.Props["showManager"] != true {
		t.Fatalf("accordion = %v", ac)
	}
	it := ac.Children[0]
	if it.Type != "accordionitem" || it.Props["label"] != "Secao 1" {
		t.Fatalf("item = %v", it)
	}
	md := kids[1]
	if md.Type != "modal" || md.Props["title"] != "Modal T" || md.Props["size"] != "lg" || md.Props["open"] != false {
		t.Fatalf("modal = %v", md.Props)
	}
	pa, _ := md.Props["primaryAction"].(map[string]any)
	if pa["label"] != "OK" || pa["action"] != "click:primaryaction" {
		t.Fatalf("primaryAction = %v", md.Props["primaryAction"])
	}
	st := kids[2]
	if st.Type != "stepper" || st.Children[0].Type != "step" {
		t.Fatalf("stepper = %v", st)
	}
	sl := kids[3]
	if sl.Type != "slide" {
		t.Fatalf("slide = %v", sl.Type)
	}
	slides, _ := sl.Props["slides"].([]any)
	if len(slides) != 2 {
		t.Fatalf("slides = %v", sl.Props["slides"])
	}
	s0, _ := slides[0].(map[string]any)
	if s0["image"] != "a.png" || s0["alt"] != "Alt" || s0["link"] != "/x" {
		t.Fatalf("slide0 = %v", s0)
	}
}

func TestPo3bStepper(t *testing.T) {
	_, outStr, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oSt  := PoStepper():New(oPg)
	PoStep():New(oSt, "Um")
	PoStep():New(oSt, "Dois")
	PoStep():New(oSt, "Tres")
	oSt:Next()
	oSt:Next()
	oSt:Next() // no fim: não passa de 2
	ConOut("G=" + cValToChar(oSt:GetStep()))
	Local n := oSt:GetStep()
	ConOut("N=" + cValToChar(n))
	ConOut("O=" + oSt:Get("orientation"))
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	out := outStr
	if !strings.Contains(out, "N=2") {
		t.Fatalf("step = %s", out)
	}
	if !strings.Contains(out, "O=horizontal") {
		t.Fatalf("orientation = %s", out)
	}
}

func TestPo3bEventos(t *testing.T) {
	// accordionitem expand/collapse, modal close e changestep passam pelo
	// dispatch genérico de handlers.
	// ids: page=1, accordion=2, item=3, modal=4, stepper=5
	f := newFakeUI(
		fakeEv{3, "expand", nil},
		fakeEv{4, "close", nil},
		fakeEv{5, "changestep", []byte("2")},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oAc  := PoAccordion():New(oPg)
	Local oIt  := PoAccordionItem():New(oAc, "S")
	oIt:OnExpand({|o| ConOut("EXPAND")})
	Local oMd  := PoModal():New(oPg, "M")
	oMd:OnClose({|o| ConOut("MODALCLOSE")})
	Local oSt  := PoStepper():New(oPg)
	PoStep():New(oSt, "A")
	PoStep():New(oSt, "B")
	oSt:OnChangeStep({|o, n| ConOut("STEP=" + cValToChar(n)), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	s := out.String()
	for _, want := range []string{"EXPAND", "MODALCLOSE", "STEP="} {
		if !strings.Contains(s, want) {
			t.Errorf("faltou %q em %q", want, s)
		}
	}
}

func TestPo3bPaisInvalidos(t *testing.T) {
	// accordionitem fora de accordion, step fora de stepper e modal fora
	// de página devem falhar na construção.
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oBox := PoContainer():New(oPg, "C")
	Local oIt  := PoAccordionItem():New(oBox, "X")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "pai inválido") {
		t.Fatalf("esperava pai inválido, veio %v", err)
	}
	_, _, err = rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oBox := PoContainer():New(oPg, "C")
	Local oSp  := PoStep():New(oBox, "X")
Return
`)
	if err == nil {
		t.Fatal("PoStep fora de PoStepper deveria falhar")
	}
}
