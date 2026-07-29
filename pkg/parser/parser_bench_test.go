package parser

import (
	"testing"

	"github.com/advpl/compiler/pkg/lexer"
)

func BenchmarkParseSimpleFunction(b *testing.B) {
	src := `Function Hello()
    ConOut("Hello, World!")
Return`

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l := lexer.NewLexer(src, "bench.prw")
		tokens, _ := l.Tokenize()
		p := NewParser(tokens, "bench.prw", map[string]string{})
		_, _ = p.Parse()
	}
}

func BenchmarkParseComplexProgram(b *testing.B) {
	src := `Function ProcessData()
    Local nTotal := 0
    Local aRecs := {}
    Local oModel

    DbSelectArea("SA1")
    DbGoTop()

    Do While !Eof()
        If SA1->A1_VALOR > 100
            nTotal += SA1->A1_VALOR
            aAdd(aRecs, {"cod": SA1->A1_COD, "valor": SA1->A1_VALOR})
        EndIf
        DbSkip()
    EndDo

    oModel := FWFormModel():New("SA1")
    oModel:AddFields("SA1", {"A1_COD", "A1_NOME", "A1_VALOR"})

Return nTotal
`

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l := lexer.NewLexer(src, "bench.prw")
		tokens, _ := l.Tokenize()
		p := NewParser(tokens, "bench.prw", map[string]string{})
		_, _ = p.Parse()
	}
}

func BenchmarkParseClassDefinition(b *testing.B) {
	src := `Class MyClass
    Data nValue as Numeric
    Data cName as Character

    Method New(nVal as Numeric, cName as Character) as Object
    Method Calculate() as Numeric
    Method GetName() as Character
EndClass

Method New(nVal as Numeric, cName as Character) as Object class MyClass
    ::nValue := nVal
    ::cName := cName
Return Self

Method Calculate() as Numeric class MyClass
Return ::nValue * 2

Method GetName() as Character class MyClass
Return ::cName
`

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l := lexer.NewLexer(src, "bench.prw")
		tokens, _ := l.Tokenize()
		p := NewParser(tokens, "bench.prw", map[string]string{})
		_, _ = p.Parse()
	}
}

func BenchmarkParseNestedExpressions(b *testing.B) {
	src := `Function CalcNested()
    Local nResult := (((1 + 2) * (3 + 4)) / (5 - 1)) + (((10 * 11) - 12) / 13)
Return nResult
`

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l := lexer.NewLexer(src, "bench.prw")
		tokens, _ := l.Tokenize()
		p := NewParser(tokens, "bench.prw", map[string]string{})
		_, _ = p.Parse()
	}
}
