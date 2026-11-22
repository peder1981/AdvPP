package vm

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/ast"
	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
	"github.com/advpl/compiler/pkg/preprocessor"
)

// fakePouiAppUI é o PouiAppUI de mentira: fila de mensagens jsToAdvpl,
// registra o que AdvPLToJS enviou e as pastas de busca recebidas.
type fakePouiAppUI struct {
	fila     []pouiMsgFake
	enviados [][2]string
	dirs     []string
	aberto   bool
	erro     error
}

type pouiMsgFake struct{ tipo, conteudo string }

func (f *fakePouiAppUI) AbrePouiAppPorNome(nome string, dirs []string) (string, error) {
	if f.erro != nil {
		return "", f.erro
	}
	f.aberto = true
	f.dirs = append([]string{}, dirs...)
	return "/poui/" + nome + "/?sid=x&app=" + nome, nil
}
func (f *fakePouiAppUI) PouiNext(timeout time.Duration) (string, string, bool) {
	if len(f.fila) == 0 {
		return "", "", true
	}
	m := f.fila[0]
	f.fila = f.fila[1:]
	if m.tipo == "close" {
		return m.tipo, m.conteudo, true
	}
	return m.tipo, m.conteudo, false
}
func (f *fakePouiAppUI) PouiEnvia(tipo, conteudo string) {
	f.enviados = append(f.enviados, [2]string{tipo, conteudo})
}
func (f *fakePouiAppUI) FechaPouiApp() { f.aberto = false }

// UIProvider no-op (o FwCallApp não usa diálogos).
func (f *fakePouiAppUI) MsgInfo(string, string)               {}
func (f *fakePouiAppUI) MsgStop(string, string)               {}
func (f *fakePouiAppUI) MsgAlert(string, string)              {}
func (f *fakePouiAppUI) MsgYesNo(string, string) bool         { return false }
func (f *fakePouiAppUI) Menu([]string, string) int            { return 0 }
func (f *fakePouiAppUI) InputText(_, d string, _ bool) string { return d }

// compilarArq compila um fonte com nome de arquivo próprio (define o
// fileTag das Static Function: "STATIC:<ARQ>:<NOME>").
func compilarArq(t *testing.T, nome, src string) *ast.Program {
	t.Helper()
	pp := preprocessor.NewPreprocessor(nil)
	processed, err := pp.Process(src, nome)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := lexer.Tokenize(processed, nome)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.NewParser(tokens, nome, pp.GetDefines())
	prog, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	return prog
}

func rodaComFakePoui(t *testing.T, src string, ui *fakePouiAppUI) (*VM, string, error) {
	t.Helper()
	v := NewVM(compilarPrg(t, src), false)
	var out strings.Builder
	v.SetOutputWriter(&out)
	if ui != nil {
		v.SetUIProvider(ui)
	}
	_, err := v.Run()
	return v, out.String(), err
}

const fwCallAppSrc = `
User Function Main()
	FwCallApp("meu-app")
	Return Nil

Static Function JsToAdvpl(oWebChannel, cType, cContent)
	ConOut("js:" + cType + ":" + cContent)
	If cType == "ping"
		oWebChannel:AdvPLToJS("r-1", "pong")
	EndIf
	Return Nil
`

func TestFwCallAppForaDoModoWeb(t *testing.T) {
	_, _, err := rodaComFakePoui(t, fwCallAppSrc, nil)
	if err == nil || !strings.Contains(err.Error(), "FwCallApp: requer o modo web (advplc serve)") {
		t.Fatalf("erro = %v", err)
	}
}

func TestFwCallAppChamaJsToAdvplDoFonte(t *testing.T) {
	fake := &fakePouiAppUI{fila: []pouiMsgFake{{"preLoad", ""}, {"close", ""}}}
	_, out, err := rodaComFakePoui(t, fwCallAppSrc, fake)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "js:preLoad:") {
		t.Fatalf("JsToAdvpl não chamado; out=%q", out)
	}
	if fake.aberto {
		t.Fatal("FechaPouiApp não chamado ao sair")
	}
}

func TestFwCallAppAdvplToJS(t *testing.T) {
	fake := &fakePouiAppUI{fila: []pouiMsgFake{{"ping", ""}, {"close", ""}}}
	_, _, err := rodaComFakePoui(t, fwCallAppSrc, fake)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fake.enviados) != 1 || fake.enviados[0] != [2]string{"r-1", "pong"} {
		t.Fatalf("AdvPLToJS enviados = %v", fake.enviados)
	}
}

func TestFwCallAppErroNoCallbackNaoDerruba(t *testing.T) {
	src := `
User Function Main()
	FwCallApp("meu-app")
	ConOut("depois")
	Return Nil

Static Function JsToAdvpl(oWebChannel, cType, cContent)
	If cType == "boom"
		__Inexistente__()
	EndIf
	Return Nil
`
	fake := &fakePouiAppUI{fila: []pouiMsgFake{{"boom", ""}, {"close", ""}}}
	_, out, err := rodaComFakePoui(t, src, fake)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "depois") {
		t.Fatalf("programa não seguiu após erro no callback; out=%q", out)
	}
}

func TestFwCallAppAppInexistente(t *testing.T) {
	fake := &fakePouiAppUI{erro: errors.New("app \"x\" não encontrado")}
	_, _, err := rodaComFakePoui(t, `User Function Main()
	FwCallApp("x")
Return`, fake)
	if err == nil || !strings.Contains(err.Error(), "não encontrado") {
		t.Fatalf("erro = %v", err)
	}
}

func TestFwCallAppCSourceOutroArquivo(t *testing.T) {
	progA := compilarArq(t, "a.prw", `
User Function Main()
	ConOut("main-a")
	FwCallApp("meu-app", Nil, Nil, Nil, "host", "b")
	Return Nil
`)
	progB := compilarArq(t, "b.prw", `
Static Function JsToAdvpl(oWebChannel, cType, cContent)
	ConOut("veio-de-b:" + cType)
	Return Nil
`)
	bc, err := compiler.CompileProject([]*ast.Program{progA, progB})
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakePouiAppUI{fila: []pouiMsgFake{{"preLoad", ""}, {"close", ""}}}
	v := NewVM(bc, false)
	var out strings.Builder
	v.SetOutputWriter(&out)
	v.SetUIProvider(fake)
	// Sem Run() prévio: o main do projeto não é executado (só inicializadores
	// dos Static) — a entrada é RunFunction, como nos VMs-filho do REST.
	if _, err := v.RunFunction("Main", nil); err != nil {
		t.Fatalf("RunFunction(Main): %v", err)
	}
	if !strings.Contains(out.String(), "veio-de-b:preLoad") {
		t.Fatalf("JsToAdvpl de b.prw não chamado; out=%q", out.String())
	}
}

func TestFwCallAppDirsPassadosAoProvider(t *testing.T) {
	fake := &fakePouiAppUI{fila: []pouiMsgFake{{"close", ""}}}
	v := NewVM(compilarPrg(t, fwCallAppSrc), false)
	var outBuf strings.Builder
	v.SetOutputWriter(&outBuf)
	v.SetUIProvider(fake)
	v.SetSourceDir("/src/dir")
	if _, err := v.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fake.dirs) == 0 || fake.dirs[0] != "/src/dir" {
		t.Fatalf("dirs = %v", fake.dirs)
	}
}
