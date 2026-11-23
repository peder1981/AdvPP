package vm

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/db"
	"github.com/advpl/compiler/pkg/rest"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// adapterFixture sobe um SQLite com a fixture SQL + 25 armazéns e devolve
// o engine pronto.
func adapterFixture(t *testing.T) *db.SQLiteEngine {
	t.Helper()
	eng, err := db.NewSQLiteEngine(t.TempDir() + "/armazens.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	t.Cleanup(func() { eng.Close() })

	sqlBytes, err := os.ReadFile("../../tests/rest_totvs/armazens_fixture.sql")
	if err != nil {
		t.Fatalf("ler fixture sql: %v", err)
	}
	for _, stmt := range strings.Split(string(sqlBytes), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		// Remove linhas de comentário antes de executar.
		var lines []string
		for _, ln := range strings.Split(stmt, "\n") {
			ln = strings.TrimSpace(ln)
			if ln == "" || strings.HasPrefix(ln, "--") {
				continue
			}
			lines = append(lines, ln)
		}
		stmt = strings.Join(lines, "\n")
		if stmt == "" {
			continue
		}
		if err := eng.Exec(stmt); err != nil {
			t.Fatalf("fixture sql %q: %v", stmt[:min(40, len(stmt))], err)
		}
	}
	for i := 1; i <= 25; i++ {
		descri := fmt.Sprintf("Armazem %02d", i)
		if i <= 5 {
			descri = "Central " + fmt.Sprintf("%02d", i)
		}
		if err := eng.Exec(fmt.Sprintf(
			"INSERT INTO NNR (NNR_FILIAL, NNR_CODIGO, NNR_DESCRI, NNR_ESTOQUE) VALUES ('01', '%05d', '%s', %.2f)",
			i, descri, float64(i)*1.5)); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	return eng
}

// adapterServer compila repo+api, sobe VM com o engine e registra as rotas
// anotadas.
func adapterServer(t *testing.T, eng *db.SQLiteEngine) *httptest.Server {
	t.Helper()
	repo, err := os.ReadFile("../../tests/rest_totvs/armazens_repo.tlpp")
	if err != nil {
		t.Fatalf("ler repo: %v", err)
	}
	api, err := os.ReadFile("../../tests/rest_totvs/armazens_api.tlpp")
	if err != nil {
		t.Fatalf("ler api: %v", err)
	}
	bc := compilarPrg(t, string(repo)+"\n"+string(api))
	v := NewVM(bc, false)
	v.SetDBEngine(eng)
	// newChildVM só propaga engine local via dbFactory — sem ela a VM-filha
	// da requisição ficaria sem banco.
	v.SetDBFactory(func() DBEngine { return eng })
	s := rest.NewServer("armazens", "1.0")
	v.autoRegisterAnnotatedRoutes(s)
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func adapterGet(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	code, _, body := orestDo(t, "GET", url, "", nil)
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("resposta não-JSON (%d %q): %v", code, body, err)
	}
	return code, m
}

func TestAdapterListaPaginada(t *testing.T) {
	eng := adapterFixture(t)
	srv := adapterServer(t, eng)

	code, m := adapterGet(t, srv.URL+"/api/est/v1/armazens?page=3&pageSize=10")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	items, _ := m["items"].([]any)
	if len(items) != 5 {
		t.Fatalf("page=3&pageSize=10: %d items, quer 5", len(items))
	}
	if m["hasNext"] != false {
		t.Fatalf("hasNext = %v, quer false", m["hasNext"])
	}

	code, m = adapterGet(t, srv.URL+"/api/est/v1/armazens?page=1&pageSize=10")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	items, _ = m["items"].([]any)
	if len(items) != 10 {
		t.Fatalf("page=1&pageSize=10: %d items, quer 10", len(items))
	}
	if m["hasNext"] != true {
		t.Fatalf("hasNext = %v, quer true", m["hasNext"])
	}
}

func TestAdapterFiltroEOrdemPelaURL(t *testing.T) {
	eng := adapterFixture(t)
	srv := adapterServer(t, eng)

	// 5 armazéns "Central 01..05"; filtro OData + order=-codigo da URL.
	code, m := adapterGet(t, srv.URL+"/api/est/v1/armazens?filter="+
		"descri%20eq%20'Central%2001'&order=-codigo")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	items, _ := m["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %d, quer 1", len(items))
	}
	it := items[0].(map[string]any)
	if it["codigo"] != "00001" || it["descri"] != "Central 01" {
		t.Fatalf("item = %v", it)
	}
	if _, ok := it["estoque"]; !ok {
		t.Fatalf("item sem estoque: %v", it)
	}

	// order=-codigo sobre os 5 "Central*": descri contém "Central".
	code, m = adapterGet(t, srv.URL+"/api/est/v1/armazens?filter="+
		"contains(descri,'Central')&order=-codigo&pageSize=50")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	items, _ = m["items"].([]any)
	if len(items) != 5 {
		t.Fatalf("items = %d, quer 5", len(items))
	}
	prev := "99999"
	for _, e := range items {
		cod := e.(map[string]any)["codigo"].(string)
		if cod >= prev {
			t.Fatalf("ordem não decrescente: %s >= %s", cod, prev)
		}
		prev = cod
	}
}

func TestAdapterCampoNaoMapeado400(t *testing.T) {
	eng := adapterFixture(t)
	srv := adapterServer(t, eng)

	code, m := adapterGet(t, srv.URL+"/api/est/v1/armazens?order=senha")
	if code != 400 {
		t.Fatalf("status = %d, quer 400", code)
	}
	msg, _ := m["message"].(string)
	if !strings.Contains(msg, "senha") {
		t.Fatalf("message = %q, quer menção a senha", msg)
	}
}

// TamSx3 do Protheus devolve o array {nTamanho, nDecimais, cTipo}; o
// corpus usa TamSx3("X")[1] e aTam[3] (ex.: aStruct do FWAdapterBaseV2).
func TestTamSx3(t *testing.T) {
	eng := adapterFixture(t)
	bc := compilarPrg(t, `
User Function TamSx3Tst()
	Local aCod := TamSx3("NNR_CODIGO")
	Local aNada := TamSx3("XXX_NADA")
	Return {aCod[1], aCod[2], aCod[3], Len(aCod), aNada[1], aNada[3]}
`)
	v := NewVM(bc, false)
	v.SetDBEngine(eng)
	res, err := v.RunFunction("U_TAMSX3TST", nil)
	if err != nil {
		t.Fatalf("RunFunction: %v", err)
	}
	arr, ok := res.(*advplrt.ArrayValue)
	if !ok || len(arr.Elements) != 6 {
		t.Fatalf("retorno = %#v", res)
	}
	e := arr.Elements
	if advplrt.ToFloat(e[0]) != 10 || advplrt.ToFloat(e[1]) != 0 || advplrt.ToString(e[2]) != "C" || advplrt.ToFloat(e[3]) != 3 {
		t.Fatalf("TamSx3(NNR_CODIGO) = {%v, %v, %q} len %v, quer {10, 0, \"C\"} len 3",
			advplrt.ToFloat(e[0]), advplrt.ToFloat(e[1]), advplrt.ToString(e[2]), advplrt.ToFloat(e[3]))
	}
	if advplrt.ToFloat(e[4]) != 0 || advplrt.ToString(e[5]) != "" {
		t.Fatalf("TamSx3(XXX_NADA) = {%v, ..., %q}, quer {0, 0, \"\"}", advplrt.ToFloat(e[4]), advplrt.ToString(e[5]))
	}
}
