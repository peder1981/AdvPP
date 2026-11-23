package vm

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/db"
	"github.com/advpl/compiler/pkg/rest"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// apiManagerServer compila a fixture apimanager_api.prw e registra a rota.
func apiManagerServer(t *testing.T, eng *db.SQLiteEngine) *httptest.Server {
	t.Helper()
	src, err := os.ReadFile("../../tests/rest_totvs/apimanager_api.prw")
	if err != nil {
		t.Fatalf("ler fixture: %v", err)
	}
	bc := compilarPrg(t, string(src))
	v := NewVM(bc, false)
	v.SetDBEngine(eng)
	v.SetDBFactory(func() DBEngine { return eng })
	s := rest.NewServer("apimgr", "1.0")
	v.autoRegisterAnnotatedRoutes(s)
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func TestApiManagerListaComFiltro(t *testing.T) {
	eng := adapterFixture(t)
	srv := apiManagerServer(t, eng)

	code, body := func() (int, string) {
		c, _, b := orestDo(t, "GET",
			srv.URL+"/api/est/v1/apimgr?filter="+
				"codigo%20eq%20'00001'&pageSize=50", "", nil)
		return c, b
	}()
	if code != 200 {
		t.Fatalf("status = %d (body %q)", code, body)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("resposta não-JSON: %v (%q)", err, body)
	}
	items, _ := m["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %d, quer 1", len(items))
	}
	it := items[0].(map[string]any)
	if it["codigo"] != "00001" {
		t.Fatalf("codigo = %v, quer 00001", it["codigo"])
	}
	if _, ok := m["hasNext"]; !ok {
		t.Fatalf("resposta sem hasNext: %v", m)
	}
}

func TestApiManagerErroJson(t *testing.T) {
	bc := compilarPrg(t, `
User Function ApiErr()
	Local oApi := FwApiManager():New("est", "v1")
	oApi:SetJsonError(400, "x")
	Return oApi:GetJsonError()
`)
	v := NewVM(bc, false)
	res, err := v.RunFunction("U_APIERR", nil)
	if err != nil {
		t.Fatalf("RunFunction: %v", err)
	}
	// GetJsonError retorna o JSON serializado (EncodeUtf8), como no
	// FWAPIManager real — não o objeto.
	var m map[string]any
	if err := json.Unmarshal([]byte(advplrt.ToString(res)), &m); err != nil {
		t.Fatalf("GetJsonError não é JSON: %v (%q)", err, advplrt.ToString(res))
	}
	if m["code"] != "400" || m["message"] != "x" {
		t.Fatalf("jsonError = %v", m)
	}
}

// TestApiManagerFormatoRealCorpus exercita o aApiMap real do corpus
// 12.1.2510 (ApiMap() de CRMS040): structs{alias,tipo,nome,nick,fields}
// + SetApiRelation aninhando o filho sob a chave JSON do struct.
func TestApiManagerFormatoRealCorpus(t *testing.T) {
	eng := adapterFixture(t)
	if err := eng.Exec(`CREATE TABLE FIL (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		D_E_L_E_T_ TEXT DEFAULT ' ',
		FIL_CODIGO TEXT,
		FIL_DESCRI TEXT
	)`); err != nil {
		t.Fatalf("create FIL: %v", err)
	}
	if err := eng.Exec(`INSERT INTO FIL (FIL_CODIGO, FIL_DESCRI) VALUES
		('00001','Filial A'),('00001','Filial B'),('00002','Filial C')`); err != nil {
		t.Fatalf("seed FIL: %v", err)
	}

	bc := compilarPrg(t, `
User Function ApiReal()
	Local aFields   := {}
	Local aChildFld := {}
	Local aStruct   := {}
	Local aQS       := {}
	Local oApi      := FwApiManager():New("tst","1.0")

	aFields   := {{"Codigo","NNR_CODIGO"},{"Descricao","NNR_DESCRI"}}
	aChildFld := {{"FilCodigo","FIL_CODIGO"},{"FilDescri","FIL_DESCRI"}}
	aStruct   := {{"NNR","field","items","items", aFields},;
				 {"FIL","Item","filiais","filiais", aChildFld}}

	oApi:SetApiAlias({"NNR","items","items"})
	oApi:SetApiMap({"tst","items","1.0","tst", aStruct, "items"})
	oApi:SetApiRelation({"FIL","filiais","filiais"}, {"NNR","items","items"},;
		{{"NNR_CODIGO","FIL_CODIGO"}}, "FIL_CODIGO")
	aQS := {{"page","1"},{"pageSize","20"}}
	oApi:SetApiQstring(aQS)
	If !oApi:Activate()
		Return "ACTIVATE-FAIL"
	EndIf
	If !oApi:SetJsonObject()
		Return "JSONOBJ-FAIL"
	EndIf
	Return oApi:GetJsonSerialize()
`)
	v := NewVM(bc, false)
	v.SetDBEngine(eng)
	res, err := v.RunFunction("U_APIREAL", nil)
	if err != nil {
		t.Fatalf("RunFunction: %v", err)
	}
	out := advplrt.ToString(res)
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("resposta não-JSON: %v (%q)", err, out)
	}
	items, _ := m["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("items vazio: %s", out)
	}
	first := items[0].(map[string]any)
	if first["Codigo"] != "00001" {
		t.Fatalf("Codigo = %v", first["Codigo"])
	}
	filiais, ok := first["filiais"].([]any)
	if !ok || len(filiais) != 2 {
		t.Fatalf("filiais aninhadas = %v (out %s)", first["filiais"], out)
	}
	if filiais[0].(map[string]any)["FilDescri"] != "Filial A" {
		t.Fatalf("filho = %v", filiais[0])
	}
	if _, ok := m["hasNext"]; !ok {
		t.Fatalf("sem hasNext: %v", m)
	}
}

// TestApiManagerToExecAuto: fiel ao FWAPIManager.prw do corpus 12.1.2510 —
// nTipo 1 anexa {campo, valor, Nil} por linha de aDados em aRet; demais
// tipos acumulam as mesmas triplas num aItem único anexado a aRet.
func TestApiManagerToExecAuto(t *testing.T) {
	bc := compilarPrg(t, `
User Function ApiExec()
	Local oApi := FwApiManager():New("est", "v1")
	Local aDados := {{"A1_COD", "0001"}, {"A1_NOME", "Cli"}}
	Local aTipo1 := oApi:ToExecAuto(1, aDados)
	Local aTipo2 := oApi:ToExecAuto(2, aDados)
	ConOut("T1=" + cValToChar(Len(aTipo1)) + ":" + aTipo1[1][1] + ":" + aTipo1[2][2])
	ConOut("T2=" + cValToChar(Len(aTipo2)) + ":" + cValToChar(Len(aTipo2[1])) + ":" + aTipo2[1][1][2])
	ConOut("NIL=" + cValToChar(aTipo1[1][3] == Nil))
Return
`)
	v := NewVM(bc, false)
	var outBuf bytes.Buffer
	v.SetOutputWriter(&outBuf)
	_, err := v.RunFunction("U_APIEXEC", nil)
	if err != nil {
		t.Fatalf("RunFunction: %v", err)
	}
	out := outBuf.String()
	for _, want := range []string{"T1=2:A1_COD:Cli", "T2=1:2:0001", "NIL=.T."} {
		if !strings.Contains(out, want) {
			t.Fatalf("out sem %q: %q", want, out)
		}
	}
}
