package compiler_test

import (
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/ast"
	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
	advplrt "github.com/advpl/compiler/pkg/runtime"
	"github.com/advpl/compiler/pkg/vm"
)

// parseProjeto parseia um fonte solto como se fosse um arquivo do projeto
// (CompileProject). O fileName vira prog.FileName — base do prefixo
// "STATIC:<ARQUIVO>:" das Static Function.
func parseProjeto(t *testing.T, file, src string) *ast.Program {
	t.Helper()
	tokens, err := lexer.Tokenize(src, file)
	if err != nil {
		t.Fatalf("lexer %s: %v", file, err)
	}
	p := parser.NewParser(tokens, file, nil)
	prog, err := p.Parse()
	if err != nil {
		t.Fatalf("parser %s: %v", file, err)
	}
	return prog
}

func rodaFuncao(t *testing.T, bc *compiler.Bytecode, nome string) string {
	t.Helper()
	v := vm.NewVM(bc, false)
	val, err := v.RunFunction(nome, nil)
	if err != nil {
		t.Fatalf("RunFunction(%s): %v", nome, err)
	}
	return advplrt.ToString(val)
}

// Duas Static Function com o mesmo nome em arquivos diferentes não podem
// colidir: cada uma fica presa ao seu arquivo ("STATIC:<ARQUIVO>:<NOME>") e
// a chamada dentro do arquivo resolve para a sua.
func TestProjetoStaticPorArquivo(t *testing.T) {
	a := parseProjeto(t, "a.prw", `
Static Function JsToAdvpl()
	Return "A"

User Function FA()
	Return JsToAdvpl()
`)
	b := parseProjeto(t, "b.prw", `
Static Function JsToAdvpl()
	Return "B"

User Function FB()
	Return JsToAdvpl()
`)

	bc, err := compiler.CompileProject([]*ast.Program{a, b})
	if err != nil {
		t.Fatalf("CompileProject: %v", err)
	}

	if _, ok := bc.Functions["STATIC:A.PRW:JSTOADVPL"]; !ok {
		t.Fatal("static de a.prw não registrada como STATIC:A.PRW:JSTOADVPL")
	}
	if _, ok := bc.Functions["STATIC:B.PRW:JSTOADVPL"]; !ok {
		t.Fatal("static de b.prw não registrada como STATIC:B.PRW:JSTOADVPL")
	}
	if _, ok := bc.Functions["JSTOADVPL"]; ok {
		t.Fatal("static vazou para o nome global JSTOADVPL")
	}
	if got := rodaFuncao(t, bc, "FA"); got != "A" {
		t.Fatalf("FA() = %q, esperava %q", got, "A")
	}
	if got := rodaFuncao(t, bc, "FB"); got != "B" {
		t.Fatalf("FB() = %q, esperava %q", got, "B")
	}
}

// Herança entre arquivos: a classe base pode estar em qualquer posição da
// lista (o registro de todos os programas acontece antes da compilação),
// e _Super: resolve o pai pelo bytecode único.
func TestProjetoHerancaEntreArquivos(t *testing.T) {
	base := parseProjeto(t, "base.tlpp", `
Class Base
	Method New()
EndClass
Method New() Class Base
	::cBase := "base"
	Return Self
`)
	repo := parseProjeto(t, "repo.tlpp", `
Class Repo From Base
	Method New()
EndClass
Method New() Class Repo
	_Super:New()
	::cRepo := "repo"
	Return Self

User Function FazRepo()
	Local o := Repo():New()
	Return o:cBase + "|" + o:cRepo
`)

	// repo.tlpp primeiro de propósito: a ordem da lista não pode importar.
	bc, err := compiler.CompileProject([]*ast.Program{repo, base})
	if err != nil {
		t.Fatalf("CompileProject: %v", err)
	}
	if bc.Classes["Repo"] == nil || bc.Classes["Repo"].Parent != "Base" {
		t.Fatalf("Classes[Repo] = %+v, esperava Parent Base", bc.Classes["Repo"])
	}
	if got := rodaFuncao(t, bc, "FazRepo"); got != "base|repo" {
		t.Fatalf("FazRepo() = %q, esperava %q", got, "base|repo")
	}
}

// User Function é global no projeto: o mesmo nome em dois arquivos é erro
// citando os dois arquivos.
func TestProjetoUserFunctionDuplicadaErro(t *testing.T) {
	a := parseProjeto(t, "a.prw", "User Function UX()\n\tReturn 1\n")
	b := parseProjeto(t, "b.prw", "User Function UX()\n\tReturn 2\n")

	_, err := compiler.CompileProject([]*ast.Program{a, b})
	if err == nil {
		t.Fatal("esperava erro de User Function duplicada")
	}
	if !strings.Contains(err.Error(), "a.prw") || !strings.Contains(err.Error(), "b.prw") {
		t.Fatalf("erro %q não cita os dois arquivos", err)
	}
}

// Classe também é global: mesmo nome em dois arquivos é erro.
func TestProjetoClasseDuplicadaErro(t *testing.T) {
	a := parseProjeto(t, "a.prw", "Class Dup\nEndClass\n")
	b := parseProjeto(t, "b.prw", "Class Dup\nEndClass\n")

	_, err := compiler.CompileProject([]*ast.Program{a, b})
	if err == nil {
		t.Fatal("esperava erro de classe duplicada")
	}
	if !strings.Contains(err.Error(), "a.prw") || !strings.Contains(err.Error(), "b.prw") {
		t.Fatalf("erro %q não cita os dois arquivos", err)
	}
}

// O corpo principal de cada programa é ignorado: MainOffset aponta para um
// OP_HALT (nada de auto-executar User Function de entrada por arquivo).
func TestProjetoMainOffsetApontaParaHalt(t *testing.T) {
	a := parseProjeto(t, "a.prw", `
User Function FA()
	Return 1
`)
	bc, err := compiler.CompileProject([]*ast.Program{a})
	if err != nil {
		t.Fatalf("CompileProject: %v", err)
	}
	if bc.MainOffset < 0 || bc.MainOffset >= len(bc.Code) {
		t.Fatalf("MainOffset %d fora do código", bc.MainOffset)
	}
	if bc.Code[bc.MainOffset].Op != compiler.OP_HALT {
		t.Fatalf("Code[MainOffset].Op = %v, esperava OP_HALT", bc.Code[bc.MainOffset].Op)
	}
}

// Static Function de um arquivo não pode ser chamada de outro: não existe
// vínculo global — vira chamada nativa (erro em runtime), como no Protheus.
func TestProjetoStaticNaoVazaParaOutroArquivo(t *testing.T) {
	a := parseProjeto(t, "a.prw", `
Static Function Segredo()
	Return "A"

User Function FA()
	Return Segredo()
`)
	b := parseProjeto(t, "b.prw", `
User Function FB()
	Return Segredo()
`)
	bc, err := compiler.CompileProject([]*ast.Program{a, b})
	if err != nil {
		t.Fatalf("CompileProject: %v", err)
	}
	if got := rodaFuncao(t, bc, "FA"); got != "A" {
		t.Fatalf("FA() = %q, esperava %q", got, "A")
	}
	// FB chama Segredo() que só existe em a.prw: ResolveFunc não acha o
	// nome puro e a chamada cai em OP_CALL_NATIVE -> erro de runtime.
	v := vm.NewVM(bc, false)
	if _, err := v.RunFunction("FB", nil); err == nil {
		t.Fatal("FB() deveria falhar: Segredo() é static de a.prw")
	}
}
