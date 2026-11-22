// Validação da camada REST TOTVS contra o corpus Protheus 12.1.2510.
//
// Sem ADVPP_CORPUS_2510 apontando para um checkout local do corpus, todos
// os testes fazem SKIP — a compatibilidade NÃO é declarada com base neles.
// Nenhum fonte TOTVS é copiado para o repositório: os testes apenas leem o
// corpus no caminho da variável de ambiente.
package resttotvs

import (
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
	"github.com/advpl/compiler/pkg/preprocessor"
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
		if err != nil || info.IsDir() {
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

// TestCorpusCincoFontesHTTP escolhe 5 fontes reais do corpus (módulos
// diferentes; ≥1 oRest, ≥1 WSRESTFUL clássico, ≥1 FWAdapterBaseV2), carrega
// com CompileProject + fixtures SQL e valida o GET documentado.
//
// PENDENTE: a escolha dos fontes e a escrita dos fixtures SQL exigem o
// corpus presente — sem ADVPP_CORPUS_2510 este teste é SKIP e, com a
// variável, permanece SKIP até a seleção ser feita (registrar os caminhos
// relativos ao corpus aqui quando implementado).
func TestCorpusCincoFontesHTTP(t *testing.T) {
	corpusDir(t)
	t.Skip("seleção dos 5 fontes e fixtures SQL pendentes — requer corpus presente (ver plano Task 10 Step 3)")
}
