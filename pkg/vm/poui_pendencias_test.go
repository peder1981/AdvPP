package vm

import (
	"strings"
	"testing"
)

func TestPoMenuIconeInvalido(t *testing.T) {
	DefinirIconesValidos(map[string]bool{"house": true, "gauge": true})
	defer DefinirIconesValidos(nil)
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp  := PoApp():New("T")
	Local oMenu := PoMenu():New(oApp)
	oMenu:AddItem("Ok", "an an-house", {|| .T.})
	oMenu:AddItem("Curto", "an-gauge", {|| .T.})
	oMenu:AddItem("Sem", "", {|| .T.})
	oMenu:AddItem("Ruim", "an an-nao-existe", {|| .T.})
Return
`)
	if err == nil || !strings.Contains(err.Error(), `ícone inválido "an an-nao-existe"`) {
		t.Fatalf("erro = %v", err)
	}
}

func TestPoMenuIconeNormalizado(t *testing.T) {
	DefinirIconesValidos(map[string]bool{"gauge": true})
	defer DefinirIconesValidos(nil)
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp  := PoApp():New("T")
	Local oMenu := PoMenu():New(oApp)
	oMenu:AddItem("Curto", "an-gauge", {|| .T.})
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(v.poApp.mountJSON()), `"icon":"an an-gauge"`) {
		t.Errorf("ícone não normalizado: %s", v.poApp.mountJSON())
	}
}

func TestPoActivateAninhadoRecusado(t *testing.T) {
	// ids: página 1, botão aninha 2, botão sair 3
	f := newFakeUI(fakeEv{2, "click", nil}, fakeEv{3, "click", nil})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oB   := PoButton():New(oPg, "x", {|| oApp:Activate()})
	Local oS   := PoButton():New(oPg, "sair", {|| oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "PoApp:Activate: a tela já está ativa") {
		t.Errorf("saída = %q", out.String())
	}
	if got := strings.Join(f.ops(), ","); got != "mount,close" {
		t.Errorf("ops = %s (o Activate aninhado não pode remontar)", got)
	}
}

func TestPoGetEstruturado(t *testing.T) {
	_, out, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oTab := PoTable():New(oPg)
	Local oTr  := PoTree():New(oPg)
	Local aCol, aIt, aArv
	oTab:SetColumns({{"a", "Coluna A"}})
	oTab:SetItems({{"x"}, {"y"}})
	oTr:SetItems({{"Raiz", "r", {{"Filho", "f"}}}})
	aCol := oTab:Get("columns")
	aIt  := oTab:Get("items")
	aArv := oTr:Get("items")
	ConOut("col=" + aCol[1]["label"] + " n=" + cValToChar(Len(aIt)) + " it2=" + aIt[2]["a"] + " filho=" + aArv[1]["subItems"][1]["value"])
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "col=Coluna A n=2 it2=y filho=f") {
		t.Errorf("saída = %q", out)
	}
}

// Static de arquivo (comum no Protheus) não pode impedir a chamada da função
// de entrada: antes, qualquer declaração no nível do arquivo virava "corpo"
// e o Main nunca rodava.
func TestStaticDeArquivoNaoImpedeMain(t *testing.T) {
	for _, src := range []string{
		"Static nCont := 0\nStatic cNome := \"x\"\n\nUser Function Main()\n\tnCont++\n\tConOut(\"n=\" + cValToChar(nCont) + cNome)\nReturn\n",
		"Static nCont\nStatic cNome := \"x\"\n\nUser Function Main()\n\tnCont := 0\n\tFor nCont := 1 To 1\n\tNext nCont\n\tConOut(\"n=\" + cValToChar(nCont - 1) + cNome)\n\tSoma()\nReturn\n\nStatic Function Soma()\n\tnCont += 10\n\tConOut(\"soma=\" + cValToChar(nCont))\nReturn\n",
	} {
		_, out, err := rodaPrg(t, src)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "n=1x") || (strings.Contains(src, "Soma()") && !strings.Contains(out, "soma=12")) {
			t.Errorf("saída = %q para:\n%s", out, src)
		}
	}
}

// Compatibilidade: corpo de nível superior com código de verdade continua
// sendo o programa (sem chamar função automaticamente).
func TestCorpoComCodigoNaoChamaMain(t *testing.T) {
	_, out, err := rodaPrg(t, "Static nX := 1\nConOut(\"corpo\")\n\nUser Function Main()\n\tConOut(\"main\")\nReturn\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "corpo") || strings.Contains(out, "main") {
		t.Errorf("saída = %q", out)
	}
}

// VM que roda só uma função (sem Run, ex.: StartJob): Static de arquivo vale
// NIL — nunca o slot de uma variável local de mesmo índice.
func TestStaticDeArquivoSemRunNaoLeLocal(t *testing.T) {
	bc := compilarPrg(t, "Static nCont := 7\n\nUser Function Main()\nReturn\n\nUser Function Job()\n\tLocal cLocal := \"nao-e-static\"\nReturn nCont\n")
	v := NewVM(bc, false)
	r, err := v.RunFunction("Job", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Type() != "U" {
		t.Errorf("Job() = %v (%s), esperado NIL", r, r.Type())
	}
}
