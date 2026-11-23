package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/ast"
	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
	"github.com/advpl/compiler/pkg/preprocessor"
)

// compProjStat compila um fonte pelo pipeline de projeto (CompileProject) —
// necessário para exercitar os Static de arquivo, que só existem no "main"
// do projeto.
func compProjStat(t *testing.T, src string) *compiler.Bytecode {
	t.Helper()
	pp := preprocessor.NewPreprocessor(nil)
	processed, err := pp.Process(src, "t.tlpp")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := lexer.Tokenize(processed, "t.tlpp")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.NewParser(tokens, "t.tlpp", pp.GetDefines())
	prog, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	bc, err := compiler.CompileProject([]*ast.Program{prog})
	if err != nil {
		t.Fatal(err)
	}
	return bc
}

// TestStaticsDeArquivoEmRunFunction cobre o caso das VMs-filhas de REST:
// RunFunction nunca executa o "main" do projeto, mas os Static de arquivo
// precisam estar inicializados e compartilhados — o padrão singleton do
// corpus (`static __instance` + getInstance) depende disso.
func TestStaticsDeArquivoEmRunFunction(t *testing.T) {
	src := `
namespace t.t

static __instance as Object
static nVal := 41

User Function Probe()
	ConOut("N=" + Str(nVal))
	__instance := "x"
	ConOut("I=" + ValType(__instance) + ":" + __instance)
Return
`
	bc := compProjStat(t, src)
	v := NewVM(bc, false)
	if _, err := v.RunFunction("Probe", nil); err != nil {
		t.Fatal(err)
	}
}

// TestDefaultNaoCorrompePilhaDoCaller cobre o bug em que `Default param := v`
// (forma simples, via ExprStmt) emitia um POP extra que consumia um slot da
// pilha do frame CHAMADOR — qualquer método com Default de parâmetro chamado
// no meio de uma expressão corrompia a pilha (foi o que quebrava
// QueueUsersProtheusData():new() no corpus real).
func TestDefaultNaoCorrompePilhaDoCaller(t *testing.T) {
	src := `
namespace t.t

class C
	public method new()
	public method m(a, b)
endclass

method new() class C
return

method m(a, b) class C
	Default a := 10
	Default b := 20
return a + b

User Function Soma()
	Local o := C():new()
	// 100 fica na pilha do caller durante a chamada de m(); se o Default
	// de m() emitir POP espúrio, o 100 é comido e o resultado corrompe.
	Return 100 + o:m() + 1

User Function SomaArgs()
	Local o := C():new()
	// Default é atribuição incondicional nesta semântica (fixture
	// docase_default_test.prw: Local nVal := 0; Default nVal := 42
	// resulta 42) — args explícitos são sobrescritos, m(1,2) dá 30.
	Return o:m(1, 2)
`
	bc := compProjStat(t, src)
	v := NewVM(bc, false)
	ret, err := v.RunFunction("Soma", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := ret.String(); got != "131" {
		t.Fatalf("esperava 131, obtive %s", got)
	}
	ret2, err := v.RunFunction("SomaArgs", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := ret2.String(); got != "30" {
		t.Fatalf("semântica Default incondicional: esperava 30, obtive %s", got)
	}
}
