package vm

import (
	"strings"
	"testing"
)

// TestTWebChannelConnectDisconnect: TWebChannel():New() + connect() liga
// lConnected quando há sessão web (fake PouiAppUI); disconnect() desliga.
// Fidelidade ao dump binário do corpus: data lconnected/nport/bjstoadvpl.
func TestTWebChannelConnectDisconnect(t *testing.T) {
	src := `
User Function Main()
	Local oCh := TWebChannel():New()
	oCh:connect()
	If oCh:lConnected
		ConOut("CONECTADO")
	Else
		ConOut("OFFLINE")
	EndIf
	oCh:nPort := 8080
	ConOut("PORTA=" + cValToChar(oCh:nPort))
	oCh:advpltojs("evt", "dados")
	oCh:disconnect()
	If !oCh:lConnected
		ConOut("DESCONECTADO")
	EndIf
Return
`
	fake := &fakePouiAppUI{}
	_, out, err := rodaComFakePoui(t, src, fake)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, want := range []string{"CONECTADO", "PORTA=8080", "DESCONECTADO"} {
		if !strings.Contains(out, want) {
			t.Fatalf("out sem %q: %q", want, out)
		}
	}
	if len(fake.enviados) != 1 || fake.enviados[0] != [2]string{"evt", "dados"} {
		t.Fatalf("advpltojs não chegou ao canal: %v", fake.enviados)
	}
}

// TestTWebEngineNewENavigate: TWebEngine():New(ownd,row,col,w,h,curl,port)
// guarda CURL/NPORT; navigate atualiza CURL; métodos de navegação são
// no-op fiéis que devolvem o objeto.
func TestTWebEngineNewENavigate(t *testing.T) {
	src := `
User Function Main()
	Local oE := TWebEngine():New(Nil, 0, 0, 100, 100, "https://app.exemplo", 9090)
	ConOut("URL=" + oE:curl)
	ConOut("PORTA=" + cValToChar(oE:nPort))
	oE:navigate("https://outro.exemplo")
	ConOut("URL2=" + oE:curl)
	oE:runjavascript()
	oE:goback()
	oE:goforward()
	oE:setasmain()
	oE:sethtml("<b>x</b>", "https://base")
	oE:clearcache()
	oE:reload()
Return
`
	_, out, err := rodaComFakePoui(t, src, &fakePouiAppUI{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, want := range []string{"URL=https://app.exemplo", "PORTA=9090", "URL2=https://outro.exemplo"} {
		if !strings.Contains(out, want) {
			t.Fatalf("out sem %q: %q", want, out)
		}
	}
}

// TestTWebEngineNewSemUrl: corpus chama TWebEngine():New(oDlg,0,0,100,100)
// sem curl/nport — CURL fica "" e NPORT 0.
func TestTWebEngineNewSemUrl(t *testing.T) {
	src := `
User Function Main()
	Local oE := TWebEngine():New(Nil, 0, 0, 100, 100)
	ConOut("Vazio=[" + oE:curl + "]")
Return
`
	_, out, err := rodaComFakePoui(t, src, &fakePouiAppUI{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "Vazio=[]") {
		t.Fatalf("out = %q", out)
	}
}

// TestFwCallAppOBjsToAdvplPeloCanal: uma vez setado oWebChannel:bJsToAdvpl
// no JsToAdvpl (padrão do corpus: {|self,codeType,codeContent| ...}), os
// eventos jsToAdvpl seguintes vão para o codeblock — não para a função.
func TestFwCallAppOBjsToAdvplPeloCanal(t *testing.T) {
	src := `
User Function Main()
	FwCallApp("meu-app")
Return

Static Function JsToAdvpl(oWebChannel, cType, cContent)
	If cType == "preLoad"
		oWebChannel:bJsToAdvpl := {|self, t, c| ConOut("CB:" + t + ":" + c) }
	Else
		ConOut("FN:" + cType)
	EndIf
Return
`
	fake := &fakePouiAppUI{fila: []pouiMsgFake{{"preLoad", ""}, {"ping", "x"}, {"close", ""}}}
	_, out, err := rodaComFakePoui(t, src, fake)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "CB:ping:x") {
		t.Fatalf("bJsToAdvpl não despachou ping: %q", out)
	}
	if strings.Contains(out, "FN:ping") {
		t.Fatalf("JsToAdvpl foi chamado apesar do bJsToAdvpl: %q", out)
	}
}

// TestFwCallAppORetornaCanalDoCaller: FwCallApp(cApp, owner, engine,
// oChannel, host, source) — o TWebChannel passado no 4º parâmetro é o
// objeto entregue ao JsToAdvpl (propriedades configuradas antes da
// chamada são visíveis lá).
func TestFwCallAppORetornaCanalDoCaller(t *testing.T) {
	src := `
User Function Main()
	Local oCh := TWebChannel():New()
	oCh:nPort := 4321
	FwCallApp("meu-app", Nil, Nil, oCh, "host")
Return

Static Function JsToAdvpl(oWebChannel, cType, cContent)
	If cType == "preLoad"
		ConOut("NP=" + cValToChar(oWebChannel:nPort))
	EndIf
Return
`
	fake := &fakePouiAppUI{fila: []pouiMsgFake{{"preLoad", ""}, {"close", ""}}}
	_, out, err := rodaComFakePoui(t, src, fake)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "NP=4321") {
		t.Fatalf("oChannel do caller não foi reutilizado: %q", out)
	}
}
