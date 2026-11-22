package parser

import (
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/ast"
	"github.com/advpl/compiler/pkg/lexer"
)

func parseWSRestfulSrc(t *testing.T, src string) (*ast.Program, *Parser) {
	t.Helper()
	toks, err := lexer.Tokenize(src, "t.prw")
	if err != nil {
		t.Fatal(err)
	}
	p := NewParser(toks, "t.prw", nil)
	prog, err := p.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return prog, p
}

func anotacoesDe(c *ast.ClassDecl) map[string][]string {
	out := map[string][]string{}
	for _, a := range c.Annotations {
		out[a.Name] = append(out[a.Name], a.Value)
	}
	return out
}

func nomesDosMetodos(c *ast.ClassDecl) map[string]bool {
	out := map[string]bool{}
	for _, m := range c.Methods {
		out[m.Name] = true
	}
	return out
}

// O parser deve guardar, em ClassDecl.Annotations da classe WSRESTFUL:
//   - {WSRESTFUL, ""}
//   - {WSMETHOD, "<VERBO>|<NomeDoMetodo>|<PATH>"} por método — o PATH é o
//     literal string da cláusula PATH (vazio se ausente).
// Método sem nome passa a se chamar pelo verbo, na declaração e na
// implementação.
func TestWSRestfulGuardaVerboEPath(t *testing.T) {
	src := `WSRESTFUL Clientes DESCRIPTION "x"
	WSDATA page AS INTEGER OPTIONAL
	WSMETHOD GET DESCRIPTION "lista" PATH "/api/v1/clientes" PRODUCES APPLICATION_JSON
	WSMETHOD GET umCliente DESCRIPTION "um" PATH "/api/v1/clientes/{id}"
END WSRESTFUL

WSMETHOD GET WSRECEIVE page WSSERVICE Clientes
Return .T.

WSMETHOD GET umCliente WSRECEIVE id WSSERVICE Clientes
Return .T.
`
	prog, _ := parseWSRestfulSrc(t, src)
	var cls *ast.ClassDecl
	for _, c := range prog.Classes {
		if strings.EqualFold(c.Name, "Clientes") {
			cls = c
		}
	}
	if cls == nil {
		t.Fatal("classe Clientes não encontrada")
	}

	anns := anotacoesDe(cls)
	if len(anns["WSRESTFUL"]) != 1 || anns["WSRESTFUL"][0] != "" {
		t.Errorf("anotação WSRESTFUL ausente/errada: %+v", cls.Annotations)
	}
	got := anns["WSMETHOD"]
	want := []string{"GET|GET|/api/v1/clientes", "GET|umCliente|/api/v1/clientes/{id}"}
	if len(got) != len(want) {
		t.Fatalf("WSMETHOD annotations: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("WSMETHOD[%d]: got %q want %q", i, got[i], want[i])
		}
	}

	metodos := nomesDosMetodos(cls)
	if !metodos["GET"] || !metodos["umCliente"] {
		t.Errorf("declarações de método erradas: %v", metodos)
	}

	// WSDATA segue em Properties (nome -> tipo).
	var achouPage bool
	for _, pr := range cls.Properties {
		if strings.EqualFold(pr.Name, "page") {
			achouPage = true
		}
	}
	if !achouPage {
		t.Errorf("WSDATA page não virou Property: %+v", cls.Properties)
	}

	// As implementações: o método sem nome chama-se "GET".
	impls := map[string]string{}
	for _, m := range prog.Methods {
		impls[m.Name] = m.ClassName
	}
	if impls["GET"] != "Clientes" || impls["umCliente"] != "Clientes" {
		t.Errorf("implementações WSMETHOD erradas: %+v", impls)
	}
}

// Dois WSMETHOD sem nome (GET e POST) na mesma classe não podem colidir:
// cada um vira um método chamado pelo próprio verbo.
func TestWSRestfulDoisVerbosSemNomeNaoColidem(t *testing.T) {
	src := `WSRESTFUL Servico
	WSMETHOD GET PATH "/a"
	WSMETHOD POST PATH "/b"
END WSRESTFUL

WSMETHOD GET WSRECEIVE x WSSERVICE Servico
Return .T.

WSMETHOD POST WSRECEIVE x WSSERVICE Servico
Return .T.
`
	prog, _ := parseWSRestfulSrc(t, src)
	var cls *ast.ClassDecl
	for _, c := range prog.Classes {
		if strings.EqualFold(c.Name, "Servico") {
			cls = c
		}
	}
	if cls == nil {
		t.Fatal("classe Servico não encontrada")
	}
	metodos := nomesDosMetodos(cls)
	if !metodos["GET"] || !metodos["POST"] {
		t.Errorf("métodos declarados errados: %v", metodos)
	}
	impls := map[string]bool{}
	for _, m := range prog.Methods {
		impls[m.Name] = true
	}
	if !impls["GET"] || !impls["POST"] {
		t.Errorf("implementações erradas: %v", impls)
	}
	anns := anotacoesDe(cls)["WSMETHOD"]
	want := []string{"GET|GET|/a", "POST|POST|/b"}
	if len(anns) != 2 || anns[0] != want[0] || anns[1] != want[1] {
		t.Errorf("anotações WSMETHOD erradas: %v", anns)
	}
}

// Regressão: dentro de WSCLIENT/WSSTRUCT/WSSERVICE, GET/POST/PATH são
// nomes de método comuns — o parser não pode tratá-los como verbo HTTP
// nem como cláusula REST.
func TestWSClientMetodosComNomeDeVerboEOuClause(t *testing.T) {
	src := `WSCLIENT SoapService
	WSMETHOD GET
	WSMETHOD POST
	WSMETHOD PATH
	WSMETHOD DeleteItem
END WSCLIENT

WSMETHOD GET WSCLIENT SoapService
Return .T.

WSMETHOD POST WSCLIENT SoapService
Return .T.

WSMETHOD PATH WSCLIENT SoapService
Return .T.

WSMETHOD DeleteItem WSCLIENT SoapService
Return .T.
`
	prog, _ := parseWSRestfulSrc(t, src)
	var cls *ast.ClassDecl
	for _, c := range prog.Classes {
		if strings.EqualFold(c.Name, "SoapService") {
			cls = c
		}
	}
	if cls == nil {
		t.Fatal("classe SoapService não encontrada")
	}
	metodos := nomesDosMetodos(cls)
	for _, nome := range []string{"GET", "POST", "PATH", "DeleteItem"} {
		if !metodos[nome] {
			t.Errorf("método %s não declarado: %v", nome, metodos)
		}
	}
	// Nenhuma anotação REST em classe WSCLIENT.
	if anns := anotacoesDe(cls); len(anns["WSMETHOD"]) != 0 || len(anns["WSRESTFUL"]) != 0 {
		t.Errorf("WSCLIENT não devia ter anotações REST: %+v", cls.Annotations)
	}
	impls := map[string]bool{}
	for _, m := range prog.Methods {
		impls[m.Name] = true
	}
	for _, nome := range []string{"GET", "POST", "PATH", "DeleteItem"} {
		if !impls[nome] {
			t.Errorf("implementação %s não encontrada: %v", nome, impls)
		}
	}
}

// Regressão: `WSMETHOD GET(...) WSCLIENT Foo` é um método chamado GET com
// lista de parâmetros — o verbo não pode ser consumido antes de saber que
// a implementação não é REST.
func TestWSMethodImplVerboComoNomeComParams(t *testing.T) {
	src := `WSCLIENT Foo
	WSMETHOD Get
END WSCLIENT

WSMETHOD GET(cX, nY) WSCLIENT Foo
Return .T.
`
	prog, _ := parseWSRestfulSrc(t, src)
	var m *ast.MethodImpl
	for _, mm := range prog.Methods {
		if strings.EqualFold(mm.ClassName, "Foo") {
			m = mm
		}
	}
	if m == nil {
		t.Fatal("implementação não encontrada")
	}
	if !strings.EqualFold(m.Name, "GET") {
		t.Errorf("nome do método: got %q want GET", m.Name)
	}
	if len(m.Params) != 2 {
		t.Errorf("params: got %d want 2", len(m.Params))
	}
}

// Implementações REST continuam corretas: verbo+nome, verbo sem nome
// (chama-se pelo verbo) e linkage WSSERVICE/WSREST.
func TestWSRestfulImplVerboNomeESemNome(t *testing.T) {
	src := `WSRESTFUL Clientes
	WSMETHOD GET lista PATH "/a"
	WSMETHOD POST PATH "/b"
END WSRESTFUL

WSMETHOD GET lista WSRECEIVE x WSREST Clientes
Return .T.

WSMETHOD POST WSRECEIVE y WSSERVICE Clientes
Return .T.
`
	prog, _ := parseWSRestfulSrc(t, src)
	impls := map[string]bool{}
	for _, m := range prog.Methods {
		impls[m.Name] = true
	}
	if !impls["lista"] || !impls["POST"] {
		t.Errorf("implementações REST erradas: %v", impls)
	}
}

// `WSMETHOD GET` sem nome e sem cláusulas no fim do bloco: o verbo é o
// nome do método — o parser não pode engolir ENDWSRESTFUL/END WSRESTFUL,
// outro WSMETHOD ou WSDATA como nome.
func TestWSRestfulMetodoSemNomeNoFimDaClasse(t *testing.T) {
	src := `WSRESTFUL Servico
	WSMETHOD GET PATH "/a"
	WSMETHOD POST
	WSDATA page AS INTEGER OPTIONAL
END WSRESTFUL
`
	prog, _ := parseWSRestfulSrc(t, src)
	var cls *ast.ClassDecl
	for _, c := range prog.Classes {
		if strings.EqualFold(c.Name, "Servico") {
			cls = c
		}
	}
	if cls == nil {
		t.Fatal("classe Servico não encontrada")
	}
	metodos := nomesDosMetodos(cls)
	if !metodos["GET"] || !metodos["POST"] {
		t.Errorf("métodos declarados errados: %v", metodos)
	}
	anns := anotacoesDe(cls)["WSMETHOD"]
	want := []string{"GET|GET|/a", "POST|POST|"}
	if len(anns) != 2 || anns[0] != want[0] || anns[1] != want[1] {
		t.Errorf("anotações WSMETHOD erradas: %v", anns)
	}
	// WSDATA depois do método sem nome continua sendo propriedade.
	if len(cls.Properties) != 1 || !strings.EqualFold(cls.Properties[0].Name, "page") {
		t.Errorf("WSDATA page perdida: %+v", cls.Properties)
	}
}

// Mesma situação com o fechamento em duas palavras (`END WSRESTFUL`).
func TestWSRestfulMetodoSemNomeAntesDeEndEmDuasPalavras(t *testing.T) {
	src := `WSRESTFUL Servico
	WSMETHOD DELETE
END WSRESTFUL
`
	prog, _ := parseWSRestfulSrc(t, src)
	var cls *ast.ClassDecl
	for _, c := range prog.Classes {
		if strings.EqualFold(c.Name, "Servico") {
			cls = c
		}
	}
	if cls == nil {
		t.Fatal("classe Servico não encontrada")
	}
	if !nomesDosMetodos(cls)["DELETE"] {
		t.Errorf("método DELETE não declarado: %v", nomesDosMetodos(cls))
	}
}

// WSMETHOD em WSRESTFUL sem verbo HTTP gera aviso (não erro).
func TestWSRestfulSemVerboGeraAviso(t *testing.T) {
	src := `WSRESTFUL Servico
	WSMETHOD lista PATH "/a"
END WSRESTFUL
`
	_, p := parseWSRestfulSrc(t, src)
	if len(p.Avisos()) == 0 {
		t.Error("WSMETHOD sem verbo em WSRESTFUL devia gerar aviso")
	}
}

// PATH que não é literal string (expressão/identificador) é guardado
// vazio e gera aviso do parser — não derruba o parse.
func TestWSRestfulPathNaoLiteralVazioComAviso(t *testing.T) {
	src := `WSRESTFUL Servico
	WSMETHOD GET PATH cPathVar
END WSRESTFUL
`
	prog, p := parseWSRestfulSrc(t, src)
	var cls *ast.ClassDecl
	for _, c := range prog.Classes {
		if strings.EqualFold(c.Name, "Servico") {
			cls = c
		}
	}
	if cls == nil {
		t.Fatal("classe Servico não encontrada")
	}
	anns := anotacoesDe(cls)["WSMETHOD"]
	if len(anns) != 1 || anns[0] != "GET|GET|" {
		t.Errorf("PATH não-literal devia virar string vazia: %v", anns)
	}
	if len(p.Avisos()) == 0 {
		t.Error("PATH não-literal devia gerar aviso")
	}
}
