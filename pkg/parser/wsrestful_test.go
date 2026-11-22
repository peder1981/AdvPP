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
