package vm

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/db"
	"github.com/advpl/compiler/pkg/rest"
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
	m := advplValueToJSON(res).(map[string]any)
	if m["code"] != float64(400) || m["message"] != "x" {
		t.Fatalf("jsonError = %v", m)
	}
}

func TestApiManagerToExecAutoErroClaro(t *testing.T) {
	bc := compilarPrg(t, `
User Function ApiExec()
	Local oApi := FwApiManager():New("est", "v1")
	Return oApi:ToExecAuto()
`)
	v := NewVM(bc, false)
	_, err := v.RunFunction("U_APIEXEC", nil)
	if err == nil || !strings.Contains(err.Error(), "não suportado") {
		t.Fatalf("erro = %v, quer 'não suportado'", err)
	}
}
