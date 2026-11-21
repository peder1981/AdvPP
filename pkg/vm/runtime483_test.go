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

// Transform com picture do Protheus: separadores, @E, alinhamento, estouro,
// negativo e template de texto (@R).
func TestTransformPicture(t *testing.T) {
	casos := map[string]string{
		`Transform(1234567.5, "@E 999,999,999.99")`:            "  1.234.567,50",
		`Transform(1234567.5, "999,999,999.99")`:               "  1,234,567.50",
		`Transform(-1234.5, "@E 999,999.99")`:                  " -1.234,50",
		`Transform(0.5, "@E 999.99")`:                          "  0,50",
		`Transform(42, "999")`:                                 " 42",
		`Transform(12345, "999")`:                              "***",
		`Transform(3.14159, "@E")`:                             "3,14159",
		`Transform("12345678000195", "@R 99.999.999/9999-99")`: "12.345.678/0001-95",
		`Transform("abc", "@!")`:                               "ABC",
	}
	for expr, esp := range casos {
		out := saidaDe(t, "User Function Main()\n\tConOut(\"[\" + "+expr+" + \"]\")\nReturn\n")
		if !strings.Contains(out, "["+esp+"]") {
			t.Errorf("%s = %q, esperava [%s]", expr, strings.TrimSpace(out), esp)
		}
	}
}

// Passagem por referência (@var): a função chamada lê e grava a variável do
// chamador — local, Private, Static de arquivo, variável capturada por
// codeblock, repasse em cadeia, método e nativas (que recebem o valor).
func TestPassagemPorReferencia(t *testing.T) {
	out := saidaDe(t, `
Static nStat := 10

Class Conta
	Method New() Constructor
	Method Soma(nAlvo, nQt)
EndClass
Method New() Class Conta
Return Self
Method Soma(nAlvo, nQt) Class Conta
	nAlvo := nAlvo + nQt
Return .T.

User Function Main()
	Local n := 0
	Local c := "a"
	Local aL := {1}
	Local nVal := 5
	Local nCad := 0
	Local nBloco := 0
	Local bInc := {|| Inc(@nBloco)}
	Local oC := Conta():New()
	Private nPriv := 1

	Inc(@n)
	Soma(@n, 10)
	ConOut("local=" + cValToChar(n))
	Texto(@c)
	Troca(@aL)
	ConOut("texto=" + c + " array=" + cValToChar(Len(aL)) + "/" + cValToChar(aL[3]))
	PorValor(nVal)
	ConOut("valor=" + cValToChar(nVal))
	A(@nCad)
	ConOut("cadeia=" + cValToChar(nCad))
	Inc(@nPriv)
	Inc(@nStat)
	ConOut("priv=" + cValToChar(nPriv) + " stat=" + cValToChar(nStat))
	Eval(bInc)
	Eval(bInc)
	ConOut("bloco=" + cValToChar(nBloco))
	ConOut("closure=" + cValToChar(ViaBloco(@n)) + " n=" + cValToChar(n))
	oC:Soma(@n, 100)
	ConOut("metodo=" + cValToChar(n))
	ConOut("nativa=" + cValToChar(Len(@aL)) + Upper(@c) + ValType(@n))
	ConOut("retorno=" + cValToChar(Devolve(@n)))
Return

Static Function Inc(x)
	x++
Return .T.
Static Function Soma(x, y)
	x := x + y
Return .T.
Static Function Texto(c)
	c += "bc"
Return .T.
Static Function Troca(a)
	a := {7, 8, 9}
Return .T.
Static Function PorValor(x)
	x := 99
Return .T.
Static Function A(x)
	B(@x)
Return .T.
Static Function B(y)
	C(@y)
	y += 1
Return .T.
Static Function C(z)
	z := 40
Return .T.
Static Function ViaBloco(x)
	Local b := {|| x := x * 2}
	Eval(b)
Return x
Static Function Devolve(x)
Return x
`)
	for _, esp := range []string{
		"local=11", "texto=abc array=3/9", "valor=5", "cadeia=41",
		"priv=2 stat=11", "bloco=2", "closure=22 n=22", "metodo=122",
		"nativa=3ABCN", "retorno=122",
	} {
		if !strings.Contains(out, esp) {
			t.Errorf("esperava %q na saída:\n%s", esp, out)
		}
	}
}

// Nil é o nulo: convertido para texto vira "" (cValToChar, AllTrim, Upper,
// concatenação de campo ausente) — antes virava a palavra "Nil".
func TestNilComoNulo(t *testing.T) {
	out := saidaDe(t, `
User Function Main()
	Local o := JsonObject():New()
	o:FromJson('{"a":null}')
	ConOut("[" + cValToChar(Nil) + "][" + cValToChar(o["a"]) + "][" + cValToChar(o["inexistente"]) + "][" + AllTrim(Nil) + "][" + Upper(o["a"]) + "]")
	ConOut("tipos=" + ValType(o["a"]) + ValType(Nil) + cValToChar(Empty(cValToChar(Nil))))
Return
`)
	if !strings.Contains(out, "[][][][][]") || !strings.Contains(out, "tipos=UU.T.") {
		t.Errorf("Nil deveria virar texto vazio:\n%s", out)
	}
}

// PrivSignRSA devolve a mensagem de erro no parâmetro @cErrStr.
func TestPrivSignRSAErroPorReferencia(t *testing.T) {
	out := saidaDe(t, `
User Function Main()
	Local cErr := "inicial"
	Local xSig := PrivSignRSA("/nao/existe.pem", "abc", 2, "", @cErr)
	ConOut("sig=" + ValType(xSig) + " err=[" + cErr + "]")
Return
`)
	if !strings.Contains(out, "sig=U err=[chave privada nao encontrada ou ilegivel]") {
		t.Errorf("cErrStr nao preenchido:\n%s", out)
	}
}
