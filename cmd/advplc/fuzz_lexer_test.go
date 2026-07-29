package main

import (
	"testing"
	"github.com/advpl/compiler/pkg/lexer"
)

func FuzzLexer(f *testing.F) {
	f.Add([]byte("Local x := 1"))
	f.Add([]byte("Function Test() ... EndFunction"))
	f.Add([]byte("If .T. ... EndIf"))
	f.Add([]byte(""))
	f.Add([]byte("Class Test ... EndClass"))
	f.Add([]byte("For i := 1 To 10 ... Next"))

	f.Fuzz(func(t *testing.T, data []byte) {
		source := string(data)
		tokens, err := lexer.Tokenize(source, "fuzz.prw")
		_ = tokens
		_ = err
	})
}
