package vm

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Onda 3b.4 — exibição: props iniciais, métodos e eventos dos
// componentes PoAvatar..PoFilterChip + PoApp:Notify estendido.

func TestPo3b4Mount(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P1")
	Local oAv  := PoAvatar():New(oPg, "u.png", "large")
	Local oBd  := PoBadge():New(oPg, 7, "caption-tag-01")
	Local oIm  := PoImage():New(oPg, "foto.png", "alt")
	Local oSk  := PoSkeleton():New(oPg, "text", "medium")
	Local oHp  := PoHelper():New(oPg, "Texto de ajuda", "Titulo")
	Local oDv  := PoDynamicView():New(oPg)
	Local oDg  := PoDisclaimerGroup():New(oPg, "Avisos", {"um", "dois"})
	Local oFc  := PoFilterChip():New(oPg, "Filtro")
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
	if len(kids) != 8 {
		t.Fatalf("children = %d", len(kids))
	}
	tipos := map[string]string{}
	for _, k := range kids {
		tipos[k.Type] = k.Type
	}
	for _, want := range []string{"avatar", "badge", "image", "skeleton", "helper", "dynamicview", "disclaimergroup", "filterchip"} {
		if tipos[want] == "" {
			t.Fatalf("falta %s em %v", want, tipos)
		}
	}
	if kids[0].Props["src"] != "u.png" || kids[0].Props["size"] != "large" {
		t.Fatalf("avatar props = %v", kids[0].Props)
	}
	if kids[1].Props["value"] != float64(7) {
		t.Fatalf("badge value = %v", kids[1].Props["value"])
	}
	if kids[6].Props["title"] != "Avisos" {
		ds, _ := kids[6].Props["disclaimers"].([]any)
		if len(ds) != 2 {
			t.Fatalf("disclaimers = %v", kids[6].Props)
		}
	}
}

func TestPo3b4Methods(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P1")
	Local oAv  := PoAvatar():New(oPg, "a.png")
	Local oBd  := PoBadge():New(oPg, 1)
	Local oIm  := PoImage():New(oPg, "i.png")
	Local oSk  := PoSkeleton():New(oPg)
	Local oHp  := PoHelper():New(oPg, "h")
	Local oCal := PoCalendar():New(oPg, CToD("01/02/2026"))
	Local oDv  := PoDynamicView():New(oPg)
	Local oDg  := PoDisclaimerGroup():New(oPg, "t", {})
	Local oFc  := PoFilterChip():New(oPg, "f")
	oAv:SetSrc("b.png")
	oBd:SetValue(9)
	oBd:SetColor("x")
	oBd:SetStatus("success")
	oIm:SetSrc("j.png")
	oIm:SetHeight(120)
	oSk:SetVariant("circle")
	oHp:SetType("warning")
	oCal:SetMode("range")
	oCal:SetValue(CToD("03/03/2026"))
	oDv:SetFields({{"nome", "Nome", "string"}})
	oDg:SetItems({"x"})
	oFc:SetSelected(.T.)
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	_ = v
}

func TestPo3b4NotifyEstendido(t *testing.T) {
	// Notify com ação: op carrega actionId e o clique dispara notifyact.
	f := newFakeUI(
		fakeEv{1, "notifyact", []byte(`null`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P1")
	oApp:Notify("oi", "success", 5000, "top", "Ver", {|| ConOut("ACAO=1"), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if s := out.String(); !strings.Contains(s, "ACAO=1") {
		t.Fatalf("out = %q", s)
	}
}

// Onda 3b.5 — páginas especiais: mount, login síncrono e higiene da senha.

func TestPo3b5Mount(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oLg  := PoPageLogin():New(oApp, "MeuERP")
	Local oCp  := PoPageChangePassword():New(oApp)
	Local oBu  := PoPageBlockedUser():New(oApp, "tentativas")
	Local oSl  := PoPageSlide():New(oApp, "Ajuda")
	Local oJs  := PoPageJobScheduler():New(oApp, "Jobs", "/api/jobs")
	oLg:SetRecovery(.T.)
	oBu:SetContactEmail("adm@x.com")
	oJs:SetParameters({"p1", "p2"})
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
	// root = pagelogin
	if m.Tree.Type != "pagelogin" || m.Tree.Props["product"] != "MeuERP" || m.Tree.Props["recovery"] != true {
		t.Fatalf("login = %+v", m.Tree.Props)
	}
}

func TestPo3b5LoginSync(t *testing.T) {
	// ids: pagelogin=1 — evento "login" {reqId,user,pass} -> beforeResult.
	f := newFakeUI(
		fakeEv{1, "login", []byte(`{"reqId":7,"user":"ana","pass":"s3cr3t"}`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oLg  := PoPageLogin():New(oApp, "ERP")
	oLg:OnLogin({|o, u, p| ConOut("LOGIN=" + u + ":" + p), .T., oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if s := out.String(); !strings.Contains(s, "LOGIN=ana:s3cr3t") {
		t.Fatalf("out = %q", s)
	}
}

func TestLoginSenhaNaoFicaNaProp(t *testing.T) {
	// Após o login, nenhuma prop de nenhum componente contém a senha.
	f := newFakeUI(
		fakeEv{1, "login", []byte(`{"reqId":1,"user":"u","pass":"SEGREDO123"}`)},
	)
	v := NewVM(compilarPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oLg  := PoPageLogin():New(oApp, "ERP")
	oLg:OnLogin({|o, u, p| ConOut("P=" + p), .T., oApp:Close()})
	oApp:Activate()
Return
`), true)
	v.SetUIProvider(f)
	out := &syncBuf{}
	v.SetOutputWriter(out)
	errc := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := v.Run()
		errc <- err
	}()
	esperaFim(t, done, errc)
	_ = out
	for _, c := range v.poApp.byID {
		for k, val := range c.props {
			if strings.Contains(fmt.Sprint(val), "SEGREDO123") {
				t.Fatalf("senha vazou na prop %s do comp %d", k, c.id)
			}
		}
	}
}

// O po-disclaimer-group descarta disclaimer sem "value" (PoDisclaimer):
// cada item precisa de value e label.
func TestPoDisclaimerGroupFormatoPoUI(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P1")
	Local oDg  := PoDisclaimerGroup():New(oPg, "Avisos", {"um", "dois"})
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	var m struct{ Tree noMount }
	if err := json.Unmarshal(v.poApp.mountJSON(), &m); err != nil {
		t.Fatal(err)
	}
	ds, _ := m.Tree.Children[0].Props["disclaimers"].([]any)
	if len(ds) != 2 {
		t.Fatalf("disclaimers = %v", m.Tree.Children[0].Props)
	}
	for i, want := range []string{"um", "dois"} {
		d, _ := ds[i].(map[string]any)
		if d["value"] != want || d["label"] != want {
			t.Fatalf("disclaimer %d = %v, want value/label %q", i, d, want)
		}
	}
}
