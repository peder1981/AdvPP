// Validação da camada REST TOTVS contra o corpus Protheus 12.1.2510.
//
// Sem ADVPP_CORPUS_2510 apontando para um checkout local do corpus, todos
// os testes fazem SKIP — a compatibilidade NÃO é declarada com base neles.
// Nenhum fonte TOTVS é copiado para o repositório: os testes apenas leem o
// corpus no caminho da variável de ambiente.
package resttotvs

import (
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/ast"
	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/db"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
	"github.com/advpl/compiler/pkg/preprocessor"
	"github.com/advpl/compiler/pkg/rest"
	"github.com/advpl/compiler/pkg/vm"
)

// corpusDir devolve a raiz do corpus ou faz SKIP do teste.
func corpusDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("ADVPP_CORPUS_2510")
	if dir == "" {
		t.Skip("ADVPP_CORPUS_2510 não definida")
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Skipf("ADVPP_CORPUS_2510 inválida: %s", dir)
	}
	return dir
}

// compilaCorpusFonte reproduz o pipeline check do advplc (preprocessa,
// lexa, parseia, compila) para UM fonte do corpus — erro de compilação é
// falha real a ser corrigida na task de origem.
func compilaCorpusFonte(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	pp := preprocessor.NewPreprocessor([]string{filepath.Dir(path)})
	processed, err := pp.Process(preprocessor.SourceToUTF8(src), path)
	if err != nil {
		return err
	}
	tokens, err := lexer.Tokenize(processed, path)
	if err != nil {
		return err
	}
	p := parser.NewParser(tokens, path, pp.GetDefines())
	prog, err := p.Parse()
	if err != nil {
		return err
	}
	_, err = compiler.Compile(prog)
	return err
}

// fontesCorpus devolve os fontes .prw/.prx/.tlpp do corpus cujo conteúdo
// contém needle (case-insensitive).
func fontesCorpus(t *testing.T, root, needle string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Diretórios ocultos (.vscode/.advpl etc.) não são fontes do
		// corpus — são artefatos de IDE.
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".prw", ".prx", ".tlpp":
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(strings.ToUpper(string(data)), strings.ToUpper(needle)) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	sort.Strings(out)
	return out
}

// TestCorpusAdaptersCheck compila todo fonte do corpus que menciona
// FWAdapterBaseV2 — cada erro é um bug real da camada de compatibilidade.
func TestCorpusAdaptersCheck(t *testing.T) {
	root := corpusDir(t)
	fontes := fontesCorpus(t, root, "FWAdapterBaseV2")
	if len(fontes) == 0 {
		t.Skip("nenhum fonte com FWAdapterBaseV2 no corpus")
	}
	var fails []string
	for _, f := range fontes {
		if err := compilaCorpusFonte(f); err != nil {
			fails = append(fails, filepath.Base(f)+": "+err.Error())
		}
	}
	for _, f := range fails {
		t.Errorf("%s", f)
	}
	t.Logf("%d fontes com FWAdapterBaseV2, %d falhas", len(fontes), len(fails))
}

// TestCorpusWSRestfulAmostra compila uma amostra determinística (semente
// 2510, ordenação alfabética) de até 500 fontes com WSRESTFUL.
func TestCorpusWSRestfulAmostra(t *testing.T) {
	root := corpusDir(t)
	fontes := fontesCorpus(t, root, "WSRESTFUL")
	if len(fontes) == 0 {
		t.Skip("nenhum fonte com WSRESTFUL no corpus")
	}
	if len(fontes) > 500 {
		r := rand.New(rand.NewSource(2510))
		r.Shuffle(len(fontes), func(i, j int) { fontes[i], fontes[j] = fontes[j], fontes[i] })
		fontes = fontes[:500]
		sort.Strings(fontes)
	}
	var fails []string
	for _, f := range fontes {
		if err := compilaCorpusFonte(f); err != nil {
			fails = append(fails, filepath.Base(f)+": "+err.Error())
		}
	}
	for _, f := range fails {
		t.Errorf("%s", f)
	}
	t.Logf("%d fontes WSRESTFUL na amostra, %d falhas", len(fontes), len(fails))
}

// compilaFontesCorpus processa (preprocessa→lex→parse) cada fonte listado
// e devolve o Bytecode via CompileProject — o mesmo pipeline do advplc,
// com todos os programas do conjunto registrados antes de compilar (como
// um RPO). Fontes únicas também passam por CompileProject: classes e
// métodos são globais da mesma forma.
func compilaFontesCorpus(t *testing.T, paths []string) *compiler.Bytecode {
	t.Helper()
	progs := make([]*ast.Program, 0, len(paths))
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ler %s: %v", path, err)
		}
		pp := preprocessor.NewPreprocessor([]string{filepath.Dir(path)})
		processed, err := pp.Process(preprocessor.SourceToUTF8(src), path)
		if err != nil {
			t.Fatalf("preprocess %s: %v", filepath.Base(path), err)
		}
		tokens, err := lexer.Tokenize(processed, path)
		if err != nil {
			t.Fatalf("lex %s: %v", filepath.Base(path), err)
		}
		p := parser.NewParser(tokens, path, pp.GetDefines())
		prog, err := p.Parse()
		if err != nil {
			t.Fatalf("parse %s: %v", filepath.Base(path), err)
		}
		progs = append(progs, prog)
	}
	bc, err := compiler.CompileProject(progs)
	if err != nil {
		t.Fatalf("CompileProject: %v", err)
	}
	return bc
}

// corpusQLGFixture cria o SQLite com a tabela QLG (filas do monitor de
// saldos) e as entradas SX3 que TamSx3 consulta — dados mínimos para o
// adapter FWAdapterBaseV2 do corpus produzir um items não-vazio.
func corpusQLGFixture(t *testing.T) *db.SQLiteEngine {
	t.Helper()
	eng, err := db.NewSQLiteEngine(t.TempDir() + "/qlg.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	t.Cleanup(func() { eng.Close() })
	stmts := []string{
		`CREATE TABLE QLG (
			R_E_C_N_O_ INTEGER PRIMARY KEY,
			D_E_L_E_T_ TEXT DEFAULT ' ',
			QLG_FILIAL TEXT, QLG_ROTINA TEXT, QLG_FILDE TEXT, QLG_FILATE TEXT,
			QLG_DATADE TEXT, QLG_DATATE TEXT, QLG_DATAP TEXT, QLG_HORA TEXT,
			QLG_USER TEXT)`,
		`CREATE TABLE SX3 (X3_CAMPO TEXT, X3_TITULO TEXT, X3_TIPO TEXT,
			X3_TAMANHO TEXT, X3_DECIMAL TEXT, X3_ARQUIVO TEXT, X3_ORDEM TEXT)`,
		`INSERT INTO QLG (QLG_FILIAL, QLG_ROTINA, QLG_FILDE, QLG_DATAP, QLG_HORA, QLG_USER)
			VALUES ('01', 'CTBA961', '01', '20260101', '120000', 'ADMIN')`,
		`INSERT INTO QLG (QLG_FILIAL, QLG_ROTINA, QLG_FILDE, QLG_DATAP, QLG_HORA, QLG_USER)
			VALUES ('01', 'CTBA961', '02', '20260102', '130000', 'ADMIN')`,
	}
	for _, s := range stmts {
		if err := eng.Exec(s); err != nil {
			t.Fatalf("fixture qlg: %v\n%s", err, s)
		}
	}
	return eng
}

// TestCorpusCincoFontesHTTP executa de verdade 5 fontes reais do corpus
// Protheus 12.1.2510, de módulos e estilos diferentes, e valida o GET
// documentado de cada um:
//
//	1. WSRESTFUL clássico   — totvspcp/api/PCPCommAPI.prw (GET whois)
//	2. WSRESTFUL clássico   — taf/TAFA553.prw (GET "online")
//	3. oRest (anotada)      — portal do cliente systemCurrency.tlpp
//	4. Controlador TLPP     — tgv/typesofOperation (class + @Get em método)
//	5. FWAdapterBaseV2      — monitor de saldos queueusers (controller→
//	                          service→data adapter→SQLite QLG+SX3)
//
// Cada caso compila com o pipeline real (preprocess→lex→parse→
// CompileProject), registra as rotas via VM.RegisterRestRoutes (anotadas
// + WSRESTFUL) e dispara o GET por HTTP.
func TestCorpusCincoFontesHTTP(t *testing.T) {
	root := corpusDir(t)

	type caso struct {
		nome    string
		fontes  []string // caminhos relativos ao corpus
		comDB   bool     // precisa do fixture QLG/SX3
		method  string
		path    string
		valida  func(t *testing.T, code int, body string)
	}

	casos := []caso{
		{
			nome:   "WSRESTFUL totvspcp whois",
			fontes: []string{"totvspcp/api/PCPCommAPI.prw"},
			method: "GET",
			path:   "/api/pcp/v1/pcpcommapi/whois",
			valida: func(t *testing.T, code int, body string) {
				if code != 200 {
					t.Fatalf("status %d body %q", code, body)
				}
			},
		},
		{
			nome:   "WSRESTFUL taf online",
			fontes: []string{"taf/integração/api/TAFA553.prw"},
			method: "GET",
			path:   "/api/taf/general/v1/TafValidRestOnline",
			valida: func(t *testing.T, code int, body string) {
				if code != 200 || !strings.Contains(body, `"online"`) {
					t.Fatalf("status %d body %q", code, body)
				}
			},
		},
		{
			nome:   "oRest systemCurrency",
			fontes: []string{"adm/po-ui/portal do cliente/util/services/systemCurrency.tlpp"},
			method: "GET",
			path:   "/api/pc/v1/systemCurrency",
			valida: func(t *testing.T, code int, body string) {
				if code != 200 {
					t.Fatalf("status %d body %q", code, body)
				}
				var m map[string]any
				if err := json.Unmarshal([]byte(body), &m); err != nil {
					t.Fatalf("resposta não-JSON %q: %v", body, err)
				}
				if _, ok := m["result"]; !ok {
					t.Fatalf("contrato systemCurrency sem 'result': %v", m)
				}
			},
		},
		{
			nome: "controlador TLPP typesOfOperation",
			fontes: []string{
				"materiais/po-ui/tgv/api/typesofoperation/tgv.typesOfOperation.controller.tlpp",
				"materiais/po-ui/tgv/api/typesofoperation/tgv.typesOfOperation.data.factory.tlpp",
				"materiais/po-ui/tgv/api/typesofoperation/tgv.typesOfOperation.data.protheus.tlpp",
				"materiais/po-ui/tgv/api/typesofoperation/tgv.typesOfOperation.service.tlpp",
			},
			method: "GET",
			path:   "/api/tgv/typesofoperation/",
			valida: func(t *testing.T, code int, body string) {
				if code != 200 {
					t.Fatalf("status %d body %q", code, body)
				}
			},
		},
		{
			nome:   "FWAdapterBaseV2 queueusers",
			fontes: []string{
				"adm/po-ui-control/monitor de saldos/queueusers/backoffice.queuemonitor.queueusers.controller.tlpp",
				"adm/po-ui-control/monitor de saldos/queueusers/backoffice.queuemonitor.queueusers.data.protheus.tlpp",
				"adm/po-ui-control/monitor de saldos/queueusers/backoffice.queuemonitor.queueusers.service.tlpp",
			},
			comDB:  true,
			method: "GET",
			path:   "/api/qm/queueusers/",
			valida: func(t *testing.T, code int, body string) {
				if code != 200 {
					t.Fatalf("status %d body %q", code, body)
				}
				var m map[string]any
				if err := json.Unmarshal([]byte(body), &m); err != nil {
					t.Fatalf("resposta não-JSON %q: %v", body, err)
				}
				items, ok := m["items"].([]any)
				if !ok || len(items) == 0 {
					t.Fatalf("esperava items não-vazio da tabela QLG: %v", m)
				}
			},
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			paths := make([]string, 0, len(c.fontes))
			for _, rel := range c.fontes {
				paths = append(paths, filepath.Join(root, rel))
			}
			bc := compilaFontesCorpus(t, paths)

			v := vm.NewVM(bc, false)
			if c.comDB {
				eng := corpusQLGFixture(t)
				v.SetDBEngine(eng)
				v.SetDBFactory(func() vm.DBEngine { return eng })
			}

			s := rest.NewServer("corpus", "1.0")
			if errs := v.RegisterRestRoutes(s); len(errs) > 0 {
				t.Fatalf("RegisterRestRoutes: %v", errs)
			}
			srv := httptest.NewServer(s.Handler())
			t.Cleanup(srv.Close)

			req, err := http.NewRequest(c.method, srv.URL+c.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			c.valida(t, resp.StatusCode, string(body))
		})
	}
}
