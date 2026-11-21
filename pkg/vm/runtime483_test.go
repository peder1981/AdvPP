package vm

import (
	"strings"
	"testing"
)

// saidaDe roda o programa e devolve a saída de console; falha o teste em erro.
func saidaDe(t *testing.T, src string) string {
	t.Helper()
	_, out, err := rodaPrg(t, src)
	if err != nil {
		t.Fatalf("erro de execução: %v\nsaída: %s", err, out)
	}
	return out
}

// .And./.Or. com curto-circuito (Protheus): o lado direito não é avaliado
// quando o esquerdo já decide; o resultado é sempre lógico.
func TestCurtoCircuito(t *testing.T) {
	out := saidaDe(t, `
User Function Main()
	Local x := Nil
	Private nChamadas := 0
	If x != Nil .And. x:Foo() == 1
		ConOut("nao devia")
	EndIf
	If .T. .Or. Conta()
		ConOut("or=" + cValToChar(nChamadas))
	EndIf
	If .F. .Or. Conta()
		ConOut("or2=" + cValToChar(nChamadas))
	EndIf
	If .F. .And. Conta()
		ConOut("nao devia")
	EndIf
	ConOut("and=" + cValToChar(nChamadas))
	ConOut("tipos=" + ValType(.T. .And. 5) + ValType(.F. .And. 5) + ValType(Nil .Or. "a"))
	ConOut("valores=" + cValToChar(.T. .And. "") + cValToChar(1 .Or. .F.) + cValToChar(.F. .And. .T. .Or. .T.))
Return
Static Function Conta()
	nChamadas++
Return .T.
`)
	for _, esp := range []string{"or=0", "or2=1", "and=1", "tipos=LLL", "valores=.F..T..T."} {
		if !strings.Contains(out, esp) {
			t.Errorf("esperava %q na saída:\n%s", esp, out)
		}
	}
	if strings.Contains(out, "nao devia") {
		t.Errorf("ramo indevido executado:\n%s", out)
	}
}

// Aritmética e comparação de datas: data ± n = data, data - data = dias,
// comparação pela data (antes toda data valia 0 e as comparações erravam).
func TestAritmeticaDeData(t *testing.T) {
	out := saidaDe(t, `
User Function Main()
	Local d := SToD("20280201")
	ConOut("soma=" + DToS(d + 90) + " " + ValType(d + 90))
	ConOut("soma2=" + DToS(1 + d))
	ConOut("sub=" + DToS(d - 1))
	ConOut("dif=" + cValToChar(SToD("20260301") - SToD("20260201")))
	ConOut("cmp=" + cValToChar(SToD("20260101") < SToD("20260102")) + cValToChar(SToD("20260102") > SToD("20260101")) + cValToChar(SToD("20260101") <= SToD("20260101")))
	ConOut("vazia=" + cValToChar(CToD("") < SToD("20000101")) + "[" + DToS(CToD("") + 1) + "]")
Return
`)
	for _, esp := range []string{"soma=20280501 D", "soma2=20280202", "sub=20280131", "dif=28", "cmp=.T..T..T.", "vazia=.T.[]"} {
		if !strings.Contains(out, esp) {
			t.Errorf("esperava %q na saída:\n%s", esp, out)
		}
	}
}
