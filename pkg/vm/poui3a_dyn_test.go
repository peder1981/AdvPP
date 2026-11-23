package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

// Onda 3a.6 — templates dinâmicos: classes, SetFields/SetActions e o
// round-trip síncrono OnBeforeSave/OnBeforeRemove (beforeResult).

func TestPoDynMount(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oTb  := PoPageDynamicTable():New(oApp, "Armazéns", "/rest/api/est/v1/armazens")
	oTb:SetFields({{"A1_COD","Código","string",.T.,.T.,.T.},{"A1_NOME","Nome","string"}})
	oTb:SetQuickSearchField("A1_NOME")
	oTb:SetKeepFilters(.T.)
	oTb:SetActions({{"edit","Editar"},{"del","Excluir"}})
	Local oEd := PoPageDynamicEdit():New(oApp, "Editar", "/rest/api/est/v1/armazens")
	oEd:SetFields({{"A1_COD","Código","string",.T.}})
	Local oSe := PoPageDynamicSearch():New(oApp, "Busca")
	oSe:SetFilters({{"A1_COD","Código","string",.T.,.T.}})
	oSe:SetQuickFilter("A1_COD")
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
	if m.Tree.Type != "dyntable" {
		t.Fatalf("type = %s", m.Tree.Type)
	}
	if !strings.HasPrefix(m.Tree.Props["url"].(string), "/rest/") {
		t.Fatalf("url = %v", m.Tree.Props["url"])
	}
	fields, _ := m.Tree.Props["fields"].([]any)
	if len(fields) != 2 {
		t.Fatalf("fields = %v", m.Tree.Props["fields"])
	}
	f0, _ := fields[0].(map[string]any)
	if f0["property"] != "A1_COD" || f0["label"] != "Código" || f0["key"] != true {
		t.Fatalf("field0 = %v", f0)
	}
}

func TestPoDynBeforeSave(t *testing.T) {
	// dynedit id=1; beforesave chega com reqId+data; handler .F. -> ok=false
	f := newFakeUI(
		fakeEv{1, "beforesave", []byte(`{"reqId":7,"data":{"A1_COD":"X"}}`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oEd  := PoPageDynamicEdit():New(oApp, "E", "/rest/x")
	oEd:OnBeforeSave({|p, r| ConOut("B=" + r["A1_COD"]), oApp:Close(), .F.})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "B=X") {
		t.Errorf("handler não recebeu registro: %q", out.String())
	}
}

func TestPoDynTipoInvalido(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oTb  := PoPageDynamicTable():New(oApp, "T", "/x")
	oTb:SetFields({{"P","L","widget"}})
Return
`)
	if err == nil || !strings.Contains(err.Error(), "tipo inválido") {
		t.Errorf("erro = %v", err)
	}
}

func TestPoDynPaiInvalido(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	PoPageDynamicTable():New(oPg, "T", "/x")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "pai inválido") {
		t.Errorf("erro = %v", err)
	}
}

// OnCustomAction(oSelf, cAcao, oLinha): ações da tabela dinâmica real
// (po-page-dynamic-table) chegam como {acao, linha}.
func TestPoDynCustomActionTresArgs(t *testing.T) {
	f := newFakeUI(
		fakeEv{1, "customaction", []byte(`{"acao":"edit","linha":{"codigo":"00007"}}`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oTb  := PoPageDynamicTable():New(oApp, "A", "/rest/x")
	oTb:OnCustomAction({|o, cAcao, oLinha| ConOut("A=" + cAcao + " C=" + oLinha["codigo"]), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "A=edit C=00007") {
		t.Errorf("OnCustomAction = %q", out.String())
	}
}

// OnAdvancedSearch(oSelf, oFiltros) no PoPageDynamicSearch recebe os
// filtros do p-advanced-search (o PoPageList continua sem argumento).
func TestPoDynSearchAdvancedRecebeFiltros(t *testing.T) {
	f := newFakeUI(
		fakeEv{1, "advancedsearch", []byte(`{"codigo":"001"}`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oSe  := PoPageDynamicSearch():New(oApp, "Busca")
	oSe:OnAdvancedSearch({|o, oFil| ConOut("F=" + oFil["codigo"]), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "F=001") {
		t.Errorf("OnAdvancedSearch = %q", out.String())
	}
}

// SetId(cId): o registro que PoPageDynamicDetail/Edit carregam (o PO-UI lê
// o id da rota; o frontend entrega uma rota com este id).
func TestPoDynSetId(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oDt  := PoPageDynamicDetail():New(oApp, "D", "/rest/x")
	oDt:SetId("00007")
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	var m struct{ Tree noMount }
	if err := json.Unmarshal(v.poApp.mountJSON(), &m); err != nil {
		t.Fatal(err)
	}
	if m.Tree.Props["recordId"] != "00007" {
		t.Fatalf("recordId = %v", m.Tree.Props["recordId"])
	}
}
