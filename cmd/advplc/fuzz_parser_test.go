package main

import (
	"testing"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
)

func FuzzParser(f *testing.F) {
	f.Add([]byte("Local x := 1"))
	f.Add([]byte("For i := 1 To 10 ... Next"))
	f.Add([]byte("If .T. ... EndIf"))
	f.Add([]byte("Class Test ... EndClass"))
	f.Add([]byte(""))

	f.Fuzz(func(t *testing.T, data []byte) {
		source := string(data)
		tokens, err := lexer.Tokenize(source, "fuzz.prw")
		if err != nil {
			return
		}
		p := parser.NewParser(tokens, "fuzz.prw", nil)
		_, err = p.Parse()
		_ = err
	})
}
