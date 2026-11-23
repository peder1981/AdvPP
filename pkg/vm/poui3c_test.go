package vm

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Onda 3c — serviços e diretivas: SetTheme/SetLanguage/OnMediaChange,
// Storage*, OnNetworkChange, StartGuide/OnGuideEnd, SetTooltip,
// SetDraggable/OnDrop.

func TestPo3cTemaIdioma(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	oApp:SetTheme("dark")
	oApp:SetTheme("{" + Chr(34) + "color" + Chr(34) + ":{}}")
	oApp:SetLanguage("en")
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	ops := v.poApp.batch
	if len(ops) != 3 {
		t.Fatalf("ops = %v", ops)
	}
	if ops[0]["op"] != "theme" || ops[0]["value"] != "dark" {
		t.Fatalf("theme dark = %v", ops[0])
	}
	if ops[1]["op"] != "theme" {
		t.Fatalf("theme json = %v", ops[1])
	}
	if m, ok := ops[1]["value"].(map[string]any); !ok || m["color"] == nil {
		t.Fatalf("theme tokens = %v", ops[1]["value"])
	}
	if ops[2]["op"] != "language" || ops[2]["value"] != "en" {
		t.Fatalf("language = %v", ops[2])
	}
}

func TestPo3cTemaIdiomaInvalidos(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	PoPage():New(oApp, "P")
	oApp:SetTheme("neon")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "SetTheme") {
		t.Fatalf("esperava erro de tema, veio %v", err)
	}
	_, _, err = rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	PoPage():New(oApp, "P")
	oApp:SetLanguage("fr")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "SetLanguage") {
		t.Fatalf("esperava erro de idioma, veio %v", err)
	}
}

func TestPo3cStorage(t *testing.T) {
	// StorageSet/Remove viram ops; StorageGet é round-trip: o fakeUI
	// responde "storageresult" com o valor.
	poStorageGetTimeout = 2 * time.Second
	defer func() { poStorageGetTimeout = 5 * time.Second }()

	f := newFakeUI()
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oBtn := PoButton():New(oPg, "Pega")
	oBtn:OnClick({||
		oApp:StorageSet("tema", "escuro")
		Local x := oApp:StorageGet("tema")
		ConOut("GET=" + cValToChar(x))
		oApp:StorageRemove("tema")
		oApp:Close()
	})
	oApp:Activate()
Return
`, f)

	// ids: page=1, button=2; eventos de app usam poAppNodeID
	go func() {
		// espera o mount ser processado e o click chegar ao handler
		for i := 0; i < 200; i++ {
			f.mu.Lock()
			n := len(f.sent)
			f.mu.Unlock()
			if n >= 2 { // mount + primeiro flush
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		f.in <- fakeEv{2, "click", nil}
		// a resposta do storageget chega depois do click (FIFO)
		time.Sleep(20 * time.Millisecond)
		f.in <- fakeEv{poAppNodeID, "storageresult", []byte(`{"reqId":1,"value":"escuro"}`)}
	}()

	esperaFim(t, done, errc)
	s := out.String()
	if !strings.Contains(s, "GET=escuro") {
		t.Fatalf("saída = %q", s)
	}
	// ops storageset/storageremove/storageget foram enviadas
	var ops []string
	f.mu.Lock()
	for _, b := range f.sent {
		var m map[string]any
		json.Unmarshal(b, &m)
		if m["op"] == "batch" {
			for _, o := range m["ops"].([]any) {
				ops = append(ops, o.(map[string]any)["op"].(string))
			}
		} else {
			ops = append(ops, m["op"].(string))
		}
	}
	f.mu.Unlock()
	joined := strings.Join(ops, ",")
	for _, want := range []string{"storageset", "storageget", "storageremove"} {
		if !strings.Contains(joined, want) {
			t.Errorf("op %s ausente em %s", want, joined)
		}
	}
}

func TestPo3cStorageGetTimeout(t *testing.T) {
	poStorageGetTimeout = 50 * time.Millisecond
	defer func() { poStorageGetTimeout = 5 * time.Second }()

	f := newFakeUI()
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oBtn := PoButton():New(oPg, "Pega")
	oBtn:OnClick({||
		Local x := oApp:StorageGet("nada")
		ConOut("NIL=" + cValToChar(Empty(x)))
		oApp:Close()
	})
	oApp:Activate()
Return
`, f)
	go func() {
		for i := 0; i < 200; i++ {
			f.mu.Lock()
			n := len(f.sent)
			f.mu.Unlock()
			if n >= 2 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		f.in <- fakeEv{2, "click", nil}
	}()
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "NIL=") {
		t.Fatalf("saída = %q", out.String())
	}
}

func TestPo3cEventosApp(t *testing.T) {
	// mediachange/networkchange/guideend chegam com id=appID e caem em
	// a.handlers com args (oApp, valor).
	// comps: page=1, button=2; app events usam poAppNodeID
	f := newFakeUI(
		fakeEv{poAppNodeID, "mediachange", []byte(`"md"`)},
		fakeEv{poAppNodeID, "networkchange", []byte(`false`)},
		fakeEv{poAppNodeID, "guideend", []byte(`true`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	PoButton():New(oPg, "B")
	oApp:OnMediaChange({|oApp, m| ConOut("MEDIA=" + cValToChar(m))})
	oApp:OnNetworkChange({|oApp, l| ConOut("NET=" + cValToChar(l))})
	oApp:OnGuideEnd({|oApp, b| ConOut("GUIA=" + cValToChar(b)), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	s := out.String()
	for _, want := range []string{"MEDIA=md", "NET="} {
		if !strings.Contains(s, want) {
			t.Errorf("faltou %q em %q", want, s)
		}
	}
	if !strings.Contains(s, "GUIA=") {
		t.Errorf("faltou GUIA em %q", s)
	}
}

func TestPo3cStartGuide(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oBtn := PoButton():New(oPg, "Vai")
	oApp:StartGuide({{oBtn, "Titulo", "Texto do passo"}})
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	ops := v.poApp.batch
	if len(ops) != 1 || ops[0]["op"] != "guide" {
		t.Fatalf("ops = %v", ops)
	}
	steps, _ := ops[0]["steps"].([]map[string]any)
	if len(steps) != 1 || steps[0]["title"] != "Titulo" || steps[0]["content"] != "Texto do passo" {
		t.Fatalf("steps = %v", ops[0]["steps"])
	}
	if steps[0]["target"] != 2 {
		t.Fatalf("target = %v (button id=2)", steps[0]["target"])
	}
}

func TestPo3cTooltipEDraggable(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oBox := PoContainer():New(oPg, "C")
	Local oBtn := PoButton():New(oBox, "B")
	oBtn:SetTooltip("Dica", "left")
	oBtn:SetDraggable(.T.)
	oBox:OnDrop({|o, n| ConOut("DROP")})
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
	box := m.Tree.Children[0]
	btn := box.Children[0]
	if btn.Props["tooltip"] != "Dica" || btn.Props["tooltipPosition"] != "left" {
		t.Fatalf("tooltip = %v", btn.Props)
	}
	if btn.Props["draggable"] != true {
		t.Fatalf("draggable = %v", btn.Props)
	}
}

func TestPo3cDropEvento(t *testing.T) {
	// container=2 (page=1); filhos button=3 e input=4. Drop do 4 na pos 0.
	f := newFakeUI(
		fakeEv{2, "drop", []byte(`{"drag":4,"pos":0}`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oBox := PoContainer():New(oPg, "C")
	Local oB1  := PoButton():New(oBox, "Um")
	Local oB2  := PoButton():New(oBox, "Dois")
	oBox:OnDrop({|o, n|
		ConOut("ARR=" + cValToChar(o:Get("label")) + " POS=" + cValToChar(n))
		oApp:Close()
	})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	s := out.String()
	if !strings.Contains(s, "ARR=Dois POS=0") {
		t.Fatalf("saída = %q", s)
	}
}

func TestPo3cOnDropForaDeContainer(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oBtn := PoButton():New(oPg, "B")
	oBtn:OnDrop({|| .T.})
Return
`)
	if err == nil || !strings.Contains(err.Error(), "OnDrop") {
		t.Fatalf("esperava erro OnDrop, veio %v", err)
	}
}
