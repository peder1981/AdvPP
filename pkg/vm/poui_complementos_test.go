package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

// Classes que completam o catálogo do llms.txt: PoDynamicForm, PoLogin e
// PoModalPasswordRecovery.

func TestPoComplementosMount(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oFm  := PoDynamicForm():New(oPg, {{"nome", "Nome", "string", .T.}, {"idade", "Idade", "number"}})
	Local oLg  := PoLogin():New(oPg, "Usuário", "ana")
	Local oRc  := PoModalPasswordRecovery():New(oPg, "email")
	oRc:SetContactEmail("suporte@exemplo.com")
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	var m struct{ Tree noMount }
	if err := json.Unmarshal(v.poApp.mountJSON(), &m); err != nil {
		t.Fatal(err)
	}
	k := m.Tree.Children
	if len(k) != 3 || k[0].Type != "dynamicform" || k[1].Type != "login" || k[2].Type != "passwordrecovery" {
		t.Fatalf("filhos = %+v", k)
	}
	fs, _ := k[0].Props["fields"].([]any)
	if len(fs) != 2 {
		t.Fatalf("fields = %v", k[0].Props["fields"])
	}
	if k[1].Props["label"] != "Usuário" || k[1].Props["value"] != "ana" {
		t.Fatalf("login = %v", k[1].Props)
	}
	if k[2].Props["type"] != "email" || k[2].Props["open"] != false || k[2].Props["contactEmail"] != "suporte@exemplo.com" {
		t.Fatalf("recovery = %v", k[2].Props)
	}
}

// fieldchange {property, value}: atualiza o valor (GetValue) e chama
// OnChange(oForm, cCampo, xValor).
func TestPoDynamicFormFieldChange(t *testing.T) {
	f := newFakeUI(
		fakeEv{2, "fieldchange", []byte(`{"property":"nome","value":"Bia"}`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oFm  := PoDynamicForm():New(oPg, {{"nome", "Nome", "string"}})
	oFm:OnChange({|o, cCampo, xValor| ConOut("C=" + cCampo + " V=" + xValor + " G=" + o:GetValue()["nome"]), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "C=nome V=Bia G=Bia") {
		t.Errorf("saída = %q", out.String())
	}
}

// PoModalPasswordRecovery: Open/Close mudam "open"; p-submit chega como
// recoverysubmit e vira OnSubmit(oModal, jDados).
func TestPoModalPasswordRecoverySubmit(t *testing.T) {
	f := newFakeUI(
		fakeEv{2, "recoverysubmit", []byte(`{"email":"ana@exemplo.com"}`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oRc  := PoModalPasswordRecovery():New(oPg, "email")
	oRc:Open()
	oRc:OnSubmit({|o, jDados| ConOut("E=" + jDados["email"]), o:Completed(), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "E=ana@exemplo.com") {
		t.Errorf("saída = %q", out.String())
	}
}

func TestPoModalPasswordRecoveryTipoInvalido(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oRc  := PoModalPasswordRecovery():New(oPg, "fax")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "fax") {
		t.Fatalf("erro = %v", err)
	}
}
