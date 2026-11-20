package parser

import (
	"testing"

	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/preprocessor"
)

// Fonte arbitrária: erro sim, pânico/travamento não.
func FuzzPipeline(f *testing.F) {
	f.Add("User Function T()\nLocal a := {1,'x'}\nIf a[1] > 0\n ConOut(a[2])\nEndIf\nReturn\n")
	f.Add("#define X(a) a+1\n#xcommand FOO <x> => ConOut(<x>)\nFOO X(2)\n")
	f.Add("Class A from B\n method new() constructor\nEndClass\nWhile .T.\nEnd\nBeginSql alias 'Q'\nSELECT 1\nEndSql\n")
	f.Fuzz(func(t *testing.T, src string) {
		out, err := preprocessor.NewPreprocessor(nil).Process(src, "f.prw")
		if err != nil {
			return
		}
		toks, err := lexer.NewLexer(out, "f.prw").Tokenize()
		if err != nil {
			return
		}
		NewParser(toks, "f.prw", nil).Parse()
	})
}
