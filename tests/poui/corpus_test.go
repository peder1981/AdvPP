// Validação da ponte FwCallApp/JsToAdvpl contra o corpus Protheus
// 12.1.2510 (plano PO-UI onda 2, Task 2.8, Step 1).
//
// Sem ADVPP_CORPUS_2510 apontando para um checkout local do corpus, o
// teste faz SKIP — a compatibilidade NÃO é declarada com base nele.
// Nenhum fonte TOTVS é copiado para o repositório.
package poui

import (
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

func corpusDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("ADVPP_CORPUS_2510")
	if dir == "" {
		t.Skip("ADVPP_CORPUS_2510 não definida")
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Skipf("ADVPP_CORPUS_2510 inválida: %s", dir)
	}
	// Aceita a raiz do corpus ou a pasta fontes/: os caminhos dos testes
	// são relativos a fontes/.
	if st, err := os.Stat(filepath.Join(dir, "fontes")); err == nil && st.IsDir() {
		return filepath.Join(dir, "fontes")
	}
	return dir
}

func compilaFonte(path string) error {
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

func fontesCom(root, needle string) []string {
	var out []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Diretórios ocultos (.vscode/.advpl etc.) não são fontes do
		// corpus — são artefatos de IDE com sintaxe própria.
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
	sort.Strings(out)
	return out
}

// TestCorpusFwCallAppCheck compila todo fonte do corpus que usa FwCallApp
// ou JsToAdvpl — cada erro de check é um bug real da ponte.
func TestCorpusFwCallAppCheck(t *testing.T) {
	root := corpusDir(t)
	vistos := map[string]bool{}
	var fontes []string
	for _, needle := range []string{"FwCallApp", "JsToAdvpl"} {
		for _, f := range fontesCom(root, needle) {
			if !vistos[f] {
				vistos[f] = true
				fontes = append(fontes, f)
			}
		}
	}
	if len(fontes) == 0 {
		t.Skip("nenhum fonte com FwCallApp/JsToAdvpl no corpus")
	}
	var fails []string
	for _, f := range fontes {
		if err := compilaFonte(f); err != nil {
			fails = append(fails, filepath.Base(f)+": "+err.Error())
		}
	}
	for _, f := range fails {
		t.Errorf("%s", f)
	}
	t.Logf("%d fontes com FwCallApp/JsToAdvpl, %d falhas", len(fontes), len(fails))
}
