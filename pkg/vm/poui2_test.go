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

func TestPoTreeItensRecursivos(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oTr  := PoTree():New(oPg)
	oTr:SetItems({{"Raiz", "r", {{"Filho 1", "f1"}, {"Filho 2", "f2"}}}, {"Solto", "s"}})
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	s := string(v.poApp.mountJSON())
	want := `"items":[{"label":"Raiz","subItems":[{"label":"Filho 1","value":"f1"},{"label":"Filho 2","value":"f2"}],"value":"r"},{"label":"Solto","value":"s"}]`
	if !strings.Contains(s, want) {
		t.Errorf("mount = %s", s)
	}
}

func TestPoChartAddPointJanela(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oCh  := PoChart():New(oPg, "CPU", "line")
	Local nI
	oCh:SetSeries({{"cpu", {}}, {"mem", {}}})
	oCh:SetMaxPoints(3)
	For nI := 1 To 5
		oCh:AddPoint("t" + cValToChar(nI), {nI, nI * 10})
	Next nI
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	s := string(v.poApp.mountJSON())
	for _, frag := range []string{`"categories":["t3","t4","t5"]`, `{"data":[3,4,5],"label":"cpu"}`, `{"data":[30,40,50],"label":"mem"}`, `"type":"line"`, `"title":"CPU"`} {
		if !strings.Contains(s, frag) {
			t.Errorf("mount sem %s: %s", frag, s)
		}
	}
}

func TestPoChartTipoInvalido(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oCh  := PoChart():New(oPg, "X", "pizza")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "tipo inválido") {
		t.Errorf("erro = %v", err)
	}
}

func TestPoProgressLimites(t *testing.T) {
	_, out, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oPr  := PoProgress():New(oPg, "x")
	Local oGa  := PoGauge():New(oPg, "g")
	oPr:SetValue(150)
	oGa:SetValue(-5)
	ConOut("p=" + cValToChar(oPr:GetValue()) + " g=" + cValToChar(oGa:GetValue()))
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "p=100 g=0") {
		t.Errorf("saída = %q", out)
	}
}

func TestPoEntradasTipadasNoMount(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oA := PoPassword():New(oPg, "Senha")
	Local oB := PoTextarea():New(oPg, "Obs", "linha")
	Local oC := PoNumber():New(oPg, "Qtd", 3)
	Local oD := PoSelect():New(oPg, "Cor", {"azul", {"Verde", "g"}}, "g")
	Local oE := PoCheckbox():New(oPg, "Aceito", .T.)
	Local oF := PoSwitch():New(oPg, "Wi-Fi")
	Local oG := PoInfo():New(oPg, "Kernel", "6.12")
	Local oH := PoList():New(oPg)
	oH:SetItems({{"Um", "det 1"}, {"Dois"}})
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	s := string(v.poApp.mountJSON())
	for _, frag := range []string{
		`"type":"password"`, `"value":"linha"`, `"type":"number"`, `"value":3`,
		`"options":[{"label":"azul","value":"azul"},{"label":"Verde","value":"g"}]`, `"value":"g"`,
		`"type":"checkbox"`, `"value":true`, `"type":"switch"`, `"value":false`,
		`"label":"Kernel"`, `"value":"6.12"`,
		`"items":[{"detail":"det 1","title":"Um"},{"title":"Dois"}]`,
	} {
		if !strings.Contains(s, frag) {
			t.Errorf("mount sem %s", frag)
		}
	}
}

func TestPoChartAddPointPatch(t *testing.T) {
	// ids: página 1, chart 2, botão 3
	f := newFakeUI(fakeEv{3, "click", nil})
	_, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oCh  := PoChart():New(oPg, "C", "column")
	Local oB   := PoButton():New(oPg, "x", {|| oCh:AddPoint("a", {1}), oApp:Close()})
	oCh:SetSeries({{"s", {}}})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if len(f.sent) < 2 || strings.Count(string(f.sent[1]), `"op":"patch"`) != 1 {
		t.Fatalf("esperado um patch: %q", f.sent)
	}
	if !strings.Contains(string(f.sent[1]), `"categories":["a"]`) || !strings.Contains(string(f.sent[1]), `"series":[{"data":[1],"label":"s"}]`) {
		t.Errorf("patch = %s", f.sent[1])
	}
}

func TestChangeTipado(t *testing.T) {
	// ids: página 1, number 2, switch 3, select 4
	f := newFakeUI(fakeEv{2, "change", []byte(`"7.5"`)}, fakeEv{3, "change", []byte(`true`)}, fakeEv{4, "change", []byte(`"b"`)})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oN   := PoNumber():New(oPg, "N")
	Local oS   := PoSwitch():New(oPg, "S")
	Local oSel := PoSelect():New(oPg, "C", {"a", "b"})
	oN:OnChange({|o, n| ConOut("n=" + ValType(n) + cValToChar(n * 2) + ValType(o:GetValue()))})
	oS:OnChange({|o, l| ConOut("s=" + ValType(l) + cValToChar(l))})
	oSel:OnChange({|o, c| ConOut("c=" + c), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	for _, frag := range []string{"n=N15N", "s=L.T.", "c=b"} {
		if !strings.Contains(out.String(), frag) {
			t.Errorf("saída sem %s: %q", frag, out.String())
		}
	}
}

func TestSelectTreeEList(t *testing.T) {
	// ids: página 1, tree 2, list 3
	f := newFakeUI(fakeEv{2, "select", []byte(`"x2"`)}, fakeEv{3, "select", []byte(`1`)})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oTr  := PoTree():New(oPg)
	Local oLs  := PoList():New(oPg)
	oTr:SetItems({{"X1", "x1"}, {"X2", "x2"}})
	oLs:SetItems({{"Um"}, {"Dois", "d"}})
	oTr:OnSelect({|o, c| ConOut("tree=" + c)})
	oLs:OnSelect({|o, a| ConOut("list=" + a[1]), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "tree=x2") || !strings.Contains(out.String(), "list=Dois") {
		t.Errorf("saída = %q", out.String())
	}
}

// Métodos de componente não podem deixar valores sobrando na pilha da VM
// (um gráfico atualizado por timer faria a pilha crescer sem limite).
func TestPoMetodosNaoVazamPilha(t *testing.T) {
	var evs []fakeEv
	for i := 0; i < 60; i++ {
		evs = append(evs, fakeEv{3, "click", nil}) // ids: página 1, chart 2, botão 3
	}
	evs = append(evs, fakeEv{4, "click", nil})
	f := newFakeUI(evs...)
	v := NewVM(compilarPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oCh  := PoChart():New(oPg, "C", "line")
	Local oB   := PoButton():New(oPg, "x", {|| oCh:AddPoint("t", {1}), oCh:SetCategories({"a"}), oCh:SetMaxPoints(5)})
	Local oS   := PoButton():New(oPg, "sair", {|| oApp:Close()})
	Local nI
	oCh:SetSeries({{"s", {}}})
	For nI := 1 To 200 // laço longo na mesma função: o vazamento não some no retorno
		oCh:AddPoint("t", {nI})
		oCh:SetMaxPoints(5)
	Next nI
	oApp:Activate()
Return
`), true)
	v.SetUIProvider(f)
	v.SetOutputWriter(&syncBuf{})
	var maxPilha int
	f.onNext = func() {
		if n := len(v.stack); n > maxPilha {
			maxPilha = n
		}
	}
	if _, err := v.Run(); err != nil {
		t.Fatal(err)
	}
	if maxPilha > 20 {
		t.Errorf("pilha chegou a %d itens após 180 chamadas de método do gráfico", maxPilha)
	}
}

func TestConfirmRespostaUnica(t *testing.T) {
	// ids: página 1, botão confirma 2, botão sair 3, confirm 4
	f := newFakeUI(fakeEv{2, "click", nil}, fakeEv{4, "confirm", []byte(`true`)}, fakeEv{4, "confirm", []byte(`true`)},
		fakeEv{999, "confirm", []byte(`true`)}, fakeEv{3, "click", nil})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oB   := PoButton():New(oPg, "x", {|| oApp:Confirm("Apagar?", {|| ConOut("sim")}, {|| ConOut("nao")}, "Atenção")})
	Local oS   := PoButton():New(oPg, "sair", {|| oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if n := strings.Count(out.String(), "sim"); n != 1 || strings.Contains(out.String(), "nao") {
		t.Errorf("saída = %q", out.String())
	}
	if string(f.sent[1]) != `{"op":"batch","ops":[{"id":4,"op":"confirm","text":"Apagar?","title":"Atenção"}]}` {
		t.Errorf("batch = %s", f.sent[1])
	}
}

func TestConfirmNao(t *testing.T) {
	f := newFakeUI(fakeEv{2, "click", nil}, fakeEv{3, "confirm", []byte(`false`)})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oB   := PoButton():New(oPg, "x", {|| oApp:Confirm("?", {|| ConOut("sim")}, {|| ConOut("nao"), oApp:Close()})})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "nao") || strings.Contains(out.String(), "sim") {
		t.Errorf("saída = %q", out.String())
	}
}

func TestLoadingOp(t *testing.T) {
	f := newFakeUI(fakeEv{2, "click", nil})
	_, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oB   := PoButton():New(oPg, "x", {|| oApp:Close()})
	oApp:Loading(.T., "aguarde")
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if string(f.sent[1]) != `{"op":"batch","ops":[{"on":true,"op":"loading","text":"aguarde"}]}` {
		t.Errorf("batch = %s", f.sent[1])
	}
}
