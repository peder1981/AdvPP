package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

// Onda 3a.2 — data e hora. Fronteira: Date AdvPL <-> ISO AAAA-MM-DD,
// range -> {dIni,dFim}, datetime -> ISO 8601, hora -> "HH:MM[:SS]".

const telaDatas = `
User Function Main()
	Local oApp := PoApp():New("D")
	Local oPg  := PoPage():New(oApp, "Pg")
	Local oDt  := PoDatepicker():New(oPg, "Emissão", CToD("10/05/2026"))
	Local oRg  := PoDatepickerRange():New(oPg, "Período", CToD("01/01/2026"), CToD("31/12/2026"))
	Local oDtH := PoDatetimepicker():New(oPg, "Timestamp", "2026-05-10T14:30:00")
	Local oHr  := PoTimepicker():New(oPg, "Hora", "08:30")
	oDt:SetMinDate(CToD("01/01/2026"))
	oDt:SetMaxDate(CToD("31/12/2026"))
	oDt:SetFormat("dd/mm/yyyy")
Return
`

func TestPoDataMount(t *testing.T) {
	v, _, err := rodaPrg(t, telaDatas)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Tree noMount
	}
	if err := json.Unmarshal(v.poApp.mountJSON(), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Tree.Children) != 4 {
		t.Fatalf("filhos = %d", len(m.Tree.Children))
	}
	dt := m.Tree.Children[0]
	if dt.Type != "datepicker" || dt.Props["value"] != "2026-05-10" {
		t.Fatalf("datepicker = %+v", dt)
	}
	if dt.Props["minDate"] != "2026-01-01" || dt.Props["maxDate"] != "2026-12-31" || dt.Props["format"] != "dd/mm/yyyy" {
		t.Errorf("datepicker props = %v", dt.Props)
	}
	rg := m.Tree.Children[1]
	if rg.Type != "daterange" {
		t.Fatalf("daterange type = %s", rg.Type)
	}
	val, _ := json.Marshal(rg.Props["value"])
	if string(val) != `{"end":"2026-12-31","start":"2026-01-01"}` {
		t.Errorf("daterange value = %s", val)
	}
	dth := m.Tree.Children[2]
	if dth.Type != "datetimepicker" || dth.Props["value"] != "2026-05-10T14:30:00" {
		t.Fatalf("datetimepicker = %+v", dth)
	}
	hr := m.Tree.Children[3]
	if hr.Type != "timepicker" || hr.Props["value"] != "08:30" {
		t.Fatalf("timepicker = %+v", hr)
	}
}

func TestPoDataSetValueGetValue(t *testing.T) {
	_, out, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oDt  := PoDatepicker():New(oPg, "D", CToD("01/01/2020"))
	Local oRg  := PoDatepickerRange():New(oPg, "R", CToD("01/01/2020"), CToD("02/01/2020"))
	Local oDtH := PoDatetimepicker():New(oPg, "H", "")
	Local oHr  := PoTimepicker():New(oPg, "T", "")
	oDt:SetValue(CToD("15/06/2026"))
	oRg:SetValue(CToD("10/02/2026"), CToD("20/02/2026"))
	oDtH:SetValue("2026-06-15T10:00:00")
	oHr:SetValue("17:45")
	ConOut("D=" + DTOC(oDt:GetValue()) + "|H=" + cValToChar(oDtH:GetValue()) + "|T=" + cValToChar(oHr:GetValue()))
	Local oV := oRg:GetValue()
	ConOut("|R=" + DTOC(oV["dIni"]) + "/" + DTOC(oV["dFim"]))
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "D=15/06/2026|H=2026-06-15T10:00:00|T=17:45") {
		t.Errorf("saída = %q", out)
	}
	if !strings.Contains(out, "R=10/02/2026/20/02/2026") {
		t.Errorf("range = %q", out)
	}
}

func TestPoDataEventos(t *testing.T) {
	// ids: página 1, datepicker 2, daterange 3, timepicker 4
	f := newFakeUI(
		fakeEv{2, "change", []byte(`"2026-03-10"`)},
		fakeEv{3, "change", []byte(`{"start":"2026-01-05","end":"2026-01-20"}`)},
		fakeEv{4, "change", []byte(`"14:30"`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("D")
	Local oPg  := PoPage():New(oApp, "P")
	Local oDt  := PoDatepicker():New(oPg, "D")
	Local oRg  := PoDatepickerRange():New(oPg, "R")
	Local oHr  := PoTimepicker():New(oPg, "T")
	oDt:OnChange({|o, d| ConOut("D=" + ValType(d) + DTOC(d))})
	oRg:OnChange({|o, r| ConOut("R=" + ValType(r["dIni"]) + DTOC(r["dIni"]) + "/" + DTOC(r["dFim"]))})
	oHr:OnChange({|o, t| ConOut("T=" + ValType(t) + cValToChar(t)), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	s := out.String()
	if !strings.Contains(s, "D=D10/03/2026") {
		t.Errorf("datepicker change = %q", s)
	}
	if !strings.Contains(s, "R=D05/01/2026/20/01/2026") {
		t.Errorf("daterange change = %q", s)
	}
	if !strings.Contains(s, "T=C14:30") {
		t.Errorf("timepicker change = %q", s)
	}
}

func TestPoDataPaiInvalido(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oDt  := PoDatepicker():New(oApp, "x")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "pai inválido") {
		t.Errorf("erro = %v, esperado 'pai inválido'", err)
	}
}
