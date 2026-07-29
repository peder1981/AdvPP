package lexer

import (
	"testing"
)

func BenchmarkTokenizationSmall(b *testing.B) {
	src := `Function Hello()
    ConOut("Hello, World!")
Return`

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l := NewLexer(src, "bench.prw")
		_, _ = l.Tokenize()
	}
}

func BenchmarkTokenizationMedium(b *testing.B) {
	src := `Function ProcessRecords()
    Local nTotal := 0
    Local aRecs := {}
    DbSelectArea("SA1")
    DbGoTop()
    Do While !Eof()
        nTotal += SA1->A1_VALOR
        aAdd(aRecs, SA1->A1_COD)
        DbSkip()
    EndDo
Return nTotal`

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l := NewLexer(src, "bench.prw")
		_, _ = l.Tokenize()
	}
}

func BenchmarkTokenizationLarge(b *testing.B) {
	// Simulate a large program
	src := ""
	for i := 0; i < 100; i++ {
		src += `Function TestFunc_` + string(rune(i)) + `()
    Local nVar := ` + string(rune(i)) + `
    If nVar > 0
        ConOut("Value: " + Str(nVar))
    EndIf
Return

`
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l := NewLexer(src, "bench.prw")
		_, _ = l.Tokenize()
	}
}

func BenchmarkKeywordRecognition(b *testing.B) {
	keywords := []string{
		"Function", "Local", "If", "Else", "EndIf", "Do", "While",
		"For", "To", "Step", "Next", "Return", "Break", "Continue",
		"Class", "Method", "EndClass", "Data", "Try", "Catch", "End",
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, kw := range keywords {
			l := NewLexer(kw, "bench.prw")
			_, _ = l.Tokenize()
		}
	}
}

func BenchmarkStringLiteral(b *testing.B) {
	src := `"This is a string literal with some content that spans multiple tokens in a program"`

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l := NewLexer(src, "bench.prw")
		_, _ = l.Tokenize()
	}
}
