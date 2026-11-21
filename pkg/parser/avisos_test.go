package parser

import (
	"testing"

	"github.com/advpl/compiler/pkg/lexer"
)

// Terminador de bloco órfão (EndIf/Next/End/EndDo sem bloco aberto)
// continua compilando, como no Protheus, mas vira aviso com a posição.
func TestTerminadorOrfaoViraAviso(t *testing.T) {
	src := "User Function A()\n\tConOut(\"a\")\n\tEndIf\n\tNext\nReturn\n"
	toks, err := lexer.Tokenize(src, "t.prw")
	if err != nil {
		t.Fatal(err)
	}
	p := NewParser(toks, "t.prw", nil)
	if _, err := p.Parse(); err != nil {
		t.Fatalf("órfão deveria compilar (compatível com o Protheus): %v", err)
	}
	av := p.Avisos()
	if len(av) != 2 {
		t.Fatalf("esperava 2 avisos, veio %d: %+v", len(av), av)
	}
	if av[0].Line != 3 || av[0].Col != 2 || av[0].Message != "EndIf sem bloco aberto (ignorado)" {
		t.Errorf("aviso 1 errado: %+v", av[0])
	}
	if av[1].Line != 4 || av[1].Message != "Next sem bloco aberto (ignorado)" {
		t.Errorf("aviso 2 errado: %+v", av[1])
	}
}

func TestBlocoFechadoNaoGeraAviso(t *testing.T) {
	src := "User Function A()\n\tLocal n := 1\n\tIf n == 1\n\t\tConOut(\"a\")\n\tEndIf\n\tFor n := 1 To 2\n\tNext\nReturn\n"
	toks, _ := lexer.Tokenize(src, "t.prw")
	p := NewParser(toks, "t.prw", nil)
	if _, err := p.Parse(); err != nil {
		t.Fatal(err)
	}
	if len(p.Avisos()) != 0 {
		t.Errorf("sem órfãos, sem avisos: %+v", p.Avisos())
	}
}
