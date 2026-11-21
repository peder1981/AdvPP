package parser

import (
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/lexer"
)

func parseFonte(src string) error {
	toks, err := lexer.Tokenize(src, "t.prw")
	if err != nil {
		return err
	}
	_, err = NewParser(toks, "t.prw", nil).Parse()
	return err
}

// Argumentos sem vírgula ou sem o ")" de fechamento são erro (como no
// Protheus). Antes, `ConOut("a" + x` seguido de outra linha virava uma
// chamada com os argumentos colados e o programa rodava.
func TestArgumentosExigemVirgulaEParenteses(t *testing.T) {
	ruins := map[string]string{
		"parêntese aberto": "User Function A()\n\tConOut(\"oi\" + \"x\"\n\tConOut(\"dois\")\nReturn\n",
		"sem vírgula":      "User Function A()\n\tConOut(\"a\" \"b\")\nReturn\n",
		"fim do arquivo":   "User Function A()\n\tConOut(\"a\"",
	}
	for nome, src := range ruins {
		if err := parseFonte(src); err == nil {
			t.Errorf("%s: deveria ser erro de compilação", nome)
		} else if !strings.Contains(err.Error(), "t.prw:") {
			t.Errorf("%s: erro sem posição: %v", nome, err)
		}
	}
	bons := []string{
		"User Function A()\n\tConOut(\"a\", \"b\")\nReturn\n",
		"User Function A()\n\tF(, x)\n\tF(x,, y)\n\tF(x,)\n\tF()\nReturn\n",
		"User Function A()\n\tConOut(\"a\", ;\n\t\t\"b\")\nReturn\n",
		"User Function A()\n\tF(@x, n := 1)\nReturn\n",
	}
	for _, src := range bons {
		if err := parseFonte(src); err != nil {
			t.Errorf("fonte válido recusado: %v\n%s", err, src)
		}
	}
}
