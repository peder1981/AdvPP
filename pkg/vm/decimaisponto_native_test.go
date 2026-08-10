package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// decCall é um atalho de teste para chamar uma native DEC_* e já extrair o
// *decState do resultado, falhando o teste se não for um decimal válido.
func decCall(t *testing.T, v *VM, name string, args []advplrt.Value) *decState {
	t.Helper()
	got, err := v.natives[name].Fn(args)
	if err != nil {
		t.Fatalf("%s retornou erro inesperado: %v", name, err)
	}
	state, ok := decGetState(got)
	if !ok {
		t.Fatalf("%s(%v) = %T, quer decimal de ponto fixo", name, args, got)
	}
	return state
}

func TestDecCreate(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Exemplo TDN: DEC_CREATE( "5.7591111111111119", 21, 20 )
	d1 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("5.7591111111111119"), advplrt.NewNumber(21), advplrt.NewNumber(20)})
	if d1.prec != 21 || d1.scale != 20 {
		t.Errorf("DEC_CREATE precisão/escala = %d/%d, quer 21/20", d1.prec, d1.scale)
	}
	if got := decFormat(d1); got != "5.75911111111111190000" {
		t.Errorf("DEC_CREATE(string) valor = %q, quer %q", got, "5.75911111111111190000")
	}

	// Exemplo TDN, forma numérica (menos exata, ainda deve criar sem erro)
	d2 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewNumber(25.15), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	if got := decFormat(d2); got != "25.15" {
		t.Errorf("DEC_CREATE(25.15,10,2) = %q, quer %q", got, "25.15")
	}

	// Edge case: string inválida como decimal -> valor inicial 0 (documentado)
	d3 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("abc"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	if got := decFormat(d3); got != "0.00" {
		t.Errorf("DEC_CREATE(\"abc\",10,2) = %q, quer %q (valor inválido -> 0)", got, "0.00")
	}

	// Edge case: tipo inválido (não caractere nem numérico) -> exceção
	_, err := v.natives["DEC_CREATE"].Fn([]advplrt.Value{advplrt.True, advplrt.NewNumber(10), advplrt.NewNumber(2)})
	if err == nil {
		t.Errorf("DEC_CREATE(.T.,10,2) deveria retornar erro (tipo inválido)")
	}
}

func TestDecAdd(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dec1 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("25.15"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	dec2 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("14.789"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	dec3 := decCall(t, v, "DEC_ADD", []advplrt.Value{decNewObject(dec1), decNewObject(dec2)})
	if got := decFormat(dec3); got != "39.94" {
		t.Errorf("DEC_ADD(25.15,14.789) = %q, quer %q", got, "39.94")
	}

	// Edge case: parâmetro não-decimal -> exceção
	_, err := v.natives["DEC_ADD"].Fn([]advplrt.Value{decNewObject(dec1), advplrt.NewNumber(1)})
	if err == nil {
		t.Errorf("DEC_ADD com dRight não-decimal deveria retornar erro")
	}
}

func TestDecSub(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dec1 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("25.15"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	dec2 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("14.789"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	dec3 := decCall(t, v, "DEC_SUB", []advplrt.Value{decNewObject(dec1), decNewObject(dec2)})
	if got := decFormat(dec3); got != "10.36" {
		t.Errorf("DEC_SUB(25.15,14.789) = %q, quer %q", got, "10.36")
	}

	// Edge case: resultado negativo
	dec4 := decCall(t, v, "DEC_SUB", []advplrt.Value{decNewObject(dec2), decNewObject(dec1)})
	if got := decFormat(dec4); got != "-10.36" {
		t.Errorf("DEC_SUB(14.789,25.15) = %q, quer %q", got, "-10.36")
	}
}

func TestDecMul(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dec1 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("10"), advplrt.NewNumber(15), advplrt.NewNumber(2)})
	dec2 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("2"), advplrt.NewNumber(14), advplrt.NewNumber(3)})
	dec3 := decCall(t, v, "DEC_MUL", []advplrt.Value{decNewObject(dec1), decNewObject(dec2)})
	if got := decFormat(dec3); got != "20.00000" {
		t.Errorf("DEC_MUL(10,2) = %q, quer %q", got, "20.00000")
	}

	// Edge case: multiplicação por zero
	dec0 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("0"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	dec4 := decCall(t, v, "DEC_MUL", []advplrt.Value{decNewObject(dec1), decNewObject(dec0)})
	if got := decFormat(dec4); got != "0.0000" {
		t.Errorf("DEC_MUL(10,0) = %q, quer %q", got, "0.0000")
	}
}

func TestDecDiv(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dec1 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("10"), advplrt.NewNumber(15), advplrt.NewNumber(2)})
	dec2 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("20"), advplrt.NewNumber(14), advplrt.NewNumber(3)})
	dec3 := decCall(t, v, "DEC_DIV", []advplrt.Value{decNewObject(dec1), decNewObject(dec2)})
	// Escala do resultado (convenção própria, ver decDivPrecScale):
	// scale = max(6, s1 + p2 + 1) = max(6, 2+14+1) = 17.
	want := "0.50000000000000000" // "5" seguido de 16 zeros = 17 casas decimais
	if got := decFormat(dec3); got != want {
		t.Errorf("DEC_DIV(10,20) = %q, quer %q", got, want)
	}

	// Edge case: divisão por zero -> exceção
	dec0 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("0"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	_, err := v.natives["DEC_DIV"].Fn([]advplrt.Value{decNewObject(dec1), decNewObject(dec0)})
	if err == nil {
		t.Errorf("DEC_DIV por zero deveria retornar erro")
	}
}

func TestDecMod(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dec1 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("9"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	dec2 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("4"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	dec3 := decCall(t, v, "DEC_MOD", []advplrt.Value{decNewObject(dec1), decNewObject(dec2)})
	if got := decFormat(dec3); got != "1.00" {
		t.Errorf("DEC_MOD(9,4) = %q, quer %q", got, "1.00")
	}

	// Edge case: dividendo exatamente divisível -> resto 0
	dec4 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("8"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	dec5 := decCall(t, v, "DEC_MOD", []advplrt.Value{decNewObject(dec4), decNewObject(dec2)})
	if got := decFormat(dec5); got != "0.00" {
		t.Errorf("DEC_MOD(8,4) = %q, quer %q", got, "0.00")
	}
}

func TestDecPow(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dec1 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("3"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	dec2 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("2"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	dec3 := decCall(t, v, "DEC_POW", []advplrt.Value{decNewObject(dec1), decNewObject(dec2)})
	if got := decFormat(dec3); got != "9.00" {
		t.Errorf("DEC_POW(3,2) = %q, quer %q", got, "9.00")
	}

	// Edge case: expoente zero -> 1
	dec0 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("0"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	dec4 := decCall(t, v, "DEC_POW", []advplrt.Value{decNewObject(dec1), decNewObject(dec0)})
	if got := decFormat(dec4); got != "1.00" {
		t.Errorf("DEC_POW(3,0) = %q, quer %q", got, "1.00")
	}
}

func TestDecRescale(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Exemplo TDN: DEC_CREATE( 5.7591111111111119, 21, 20 ) = 5.75911111111111200000
	dec1 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("5.7591111111111119"), advplrt.NewNumber(21), advplrt.NewNumber(20)})
	dec2 := decCall(t, v, "DEC_RESCALE", []advplrt.Value{decNewObject(dec1), advplrt.NewNumber(5)})
	if got := decFormat(dec2); got != "5.75911" {
		t.Errorf("DEC_RESCALE(dec1,5) = %q, quer %q", got, "5.75911")
	}
	if dec2.prec != 21 {
		t.Errorf("DEC_RESCALE não deveria alterar a precisão, ficou %d", dec2.prec)
	}

	// Edge case: nRound=2 (truncate) vs nRound=0 (padrão, arredonda 5 p/ cima)
	dec3 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("1.25"), advplrt.NewNumber(10), advplrt.NewNumber(2)})
	dRoundUp := decCall(t, v, "DEC_RESCALE", []advplrt.Value{decNewObject(dec3), advplrt.NewNumber(1)})
	if got := decFormat(dRoundUp); got != "1.3" {
		t.Errorf("DEC_RESCALE(1.25,1) modo padrão = %q, quer %q", got, "1.3")
	}
	dTrunc := decCall(t, v, "DEC_RESCALE", []advplrt.Value{decNewObject(dec3), advplrt.NewNumber(1), advplrt.NewNumber(2)})
	if got := decFormat(dTrunc); got != "1.2" {
		t.Errorf("DEC_RESCALE(1.25,1,2) truncate = %q, quer %q", got, "1.2")
	}

	// Edge case (regressão do code review): DEC_RESCALE.md documenta
	// literalmente exceção quando nScale < 0 ou nScale >= precisão de
	// dNum — mesmo contrato de DEC_RESIZE. dec1 tem precisão 21, então
	// nScale=21 (== precisão) e nScale=-1 devem lançar erro, não clampar
	// silenciosamente.
	_, err := v.natives["DEC_RESCALE"].Fn([]advplrt.Value{decNewObject(dec1), advplrt.NewNumber(21)})
	if err == nil {
		t.Errorf("DEC_RESCALE com nScale == precisão de dNum deveria retornar erro")
	}
	_, err = v.natives["DEC_RESCALE"].Fn([]advplrt.Value{decNewObject(dec1), advplrt.NewNumber(-1)})
	if err == nil {
		t.Errorf("DEC_RESCALE com nScale negativo deveria retornar erro")
	}
}

func TestDecResize(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Exemplo TDN: dec1 := DEC_CREATE( 5.759, 4, 3 ); dec2 := DEC_RESIZE( dec1, 3, 2 ) = 5.75
	// (nRound padrão = 2, truncate; a página TDN usa "nvar1" não declarado
	// no exemplo — preservamos verbatim no fonte, mas o teste usa dec1)
	dec1 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("5.759"), advplrt.NewNumber(4), advplrt.NewNumber(3)})
	dec2 := decCall(t, v, "DEC_RESIZE", []advplrt.Value{decNewObject(dec1), advplrt.NewNumber(3), advplrt.NewNumber(2)})
	if got := decFormat(dec2); got != "5.75" {
		t.Errorf("DEC_RESIZE(5.759,3,2) = %q, quer %q", got, "5.75")
	}
	if dec2.prec != 3 || dec2.scale != 2 {
		t.Errorf("DEC_RESIZE precisão/escala = %d/%d, quer 3/2", dec2.prec, dec2.scale)
	}

	// Edge case: nPrecision fora da faixa documentada (>=64) -> exceção
	_, err := v.natives["DEC_RESIZE"].Fn([]advplrt.Value{decNewObject(dec1), advplrt.NewNumber(64), advplrt.NewNumber(2)})
	if err == nil {
		t.Errorf("DEC_RESIZE com nPrecision=64 deveria retornar erro")
	}

	// Edge case: nScale >= nPrecision -> exceção
	_, err = v.natives["DEC_RESIZE"].Fn([]advplrt.Value{decNewObject(dec1), advplrt.NewNumber(3), advplrt.NewNumber(3)})
	if err == nil {
		t.Errorf("DEC_RESIZE com nScale >= nPrecision deveria retornar erro")
	}
}

func TestDecRound(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Exemplo TDN: dec1 := DEC_CREATE( 5.759, 4, 3 ); dec2 := DEC_ROUND( dec1, 1 ) = 5.800
	dec1 := decCall(t, v, "DEC_CREATE", []advplrt.Value{advplrt.NewString("5.759"), advplrt.NewNumber(4), advplrt.NewNumber(3)})
	dec2 := decCall(t, v, "DEC_ROUND", []advplrt.Value{decNewObject(dec1), advplrt.NewNumber(1)})
	if got := decFormat(dec2); got != "5.800" {
		t.Errorf("DEC_ROUND(5.759,1) = %q, quer %q", got, "5.800")
	}
	if dec2.prec != 4 || dec2.scale != 3 {
		t.Errorf("DEC_ROUND não deveria alterar precisão/escala, ficou %d/%d", dec2.prec, dec2.scale)
	}

	// Edge case: nRound >= escala de dNum -> exceção
	_, err := v.natives["DEC_ROUND"].Fn([]advplrt.Value{decNewObject(dec1), advplrt.NewNumber(3)})
	if err == nil {
		t.Errorf("DEC_ROUND com nRound >= escala deveria retornar erro")
	}

	// Edge case: nRound negativo -> exceção
	_, err = v.natives["DEC_ROUND"].Fn([]advplrt.Value{decNewObject(dec1), advplrt.NewNumber(-1)})
	if err == nil {
		t.Errorf("DEC_ROUND com nRound negativo deveria retornar erro")
	}
}
