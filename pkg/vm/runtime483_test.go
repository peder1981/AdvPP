package vm

import (
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
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

// "U_X" chama a User Function X (convenção do Protheus); uma User Function
// declarada como U_X só é alcançada por "U_U_X". Antes U_X() de uma User
// Function X dava "unknown function" e, com as duas declaradas, U_X caía na
// errada (inclusive em rota REST e StartJob, via RunFunction).
func TestResolucaoUserFunction(t *testing.T) {
	src := `
User Function Main()
	ConOut("r=" + U_XPTO("a") + "|" + U_U_XPTO("b") + "|" + u_xpto("c") + "|" + Outra() + "|" + cValToChar(FindFunction("U_XPTO")) + cValToChar(FindFunction("U_NADA")))
Return
User Function XPTO(c)
Return "N" + c
User Function U_XPTO(c)
Return "W" + c
Static Function Outra()
Return U_SO()
User Function SO()
Return "so"
`
	out := saidaDe(t, src)
	if !strings.Contains(out, "r=Na|Wb|Nc|so|.T..F.") {
		t.Errorf("resolução errada:\n%s", out)
	}
	v := NewVM(compilarPrg(t, src), false) // como StartJob/MCP: VM nova, sem Run()
	r, err := v.RunFunction("U_XPTO", []advplrt.Value{advplrt.NewString("z")})
	if err != nil || r.String() != "Nz" {
		t.Errorf("RunFunction(U_XPTO) = %v, %v; esperava a User Function XPTO", r, err)
	}
}
