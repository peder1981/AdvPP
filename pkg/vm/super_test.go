package vm

import (
	"fmt"
	"strings"
	"testing"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// _Super: chama o método de mesma assinatura na superclasse, sobre o mesmo
// self — idioma do Protheus (`_Super:New()` na primeira linha do construtor
// da subclasse). O opcode carrega "Classe:Metodo" da classe que define o
// método; a VM resolve o pai em tempo de execução.

const srcSuperDoisNiveis = `
Class Base
	Method New()
EndClass
Method New() Class Base
	::cX := "base"
	Return Self

Class Filho From Base
	Method New()
EndClass
Method New() Class Filho
	_Super:New()
	::cY := "filho"
	Return Self

User Function Main()
	Local o := Filho():New()
	ConOut(o:cX + "|" + o:cY)
Return
`

func TestSuperChamaMetodoDoPai(t *testing.T) {
	out := saidaDe(t, srcSuperDoisNiveis)
	if !strings.Contains(out, "base|filho") {
		t.Fatalf("got %q", out)
	}
}

// _Super com argumentos: chegam ao método do pai na ordem declarada.
func TestSuperComArgs(t *testing.T) {
	out := saidaDe(t, `
Class BaseA
	Method New(cA, nB)
EndClass
Method New(cA, nB) Class BaseA
	::cA := cA
	::nB := nB
	Return Self

Class FilhoA From BaseA
	Method New(cA, nB)
EndClass
Method New(cA, nB) Class FilhoA
	_Super:New(cA, nB)
	::cY := "filho"
	Return Self

User Function Main()
	Local o := FilhoA():New("a", 2)
	ConOut(o:cA + "|" + cValToChar(o:nB) + "|" + o:cY)
Return
`)
	if !strings.Contains(out, "a|2|filho") {
		t.Fatalf("got %q", out)
	}
}

// Herança em cadeia: cada nível resolve o _Super relativo à classe que
// define o método, não à classe concreta do objeto.
func TestSuperTresNiveis(t *testing.T) {
	out := saidaDe(t, `
Class A
	Method Quem()
EndClass
Method Quem() Class A
	Return "A"

Class B From A
	Method Quem()
EndClass
Method Quem() Class B
	Return _Super:Quem() + "+B"

Class C From B
	Method New()
	Method Quem()
EndClass
Method New() Class C
	Return Self
Method Quem() Class C
	Return _Super:Quem() + "+C"

User Function Main()
	Local o := C():New()
	ConOut(o:Quem())
Return
`)
	if !strings.Contains(out, "A+B+C") {
		t.Fatalf("got %q", out)
	}
}

// Estado nativo de classe-base registrada: a instância da classe de usuário
// recebe obj.Native da fábrica e os métodos que o bytecode não define caem
// no estado nativo da raiz.
type testBaseNative struct{}

func (testBaseNative) NativeCallMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) (advplrt.Value, error) {
	switch method {
	case "NEW":
		return obj, nil
	case "ECO":
		if len(args) > 0 {
			return args[0], nil
		}
		return advplrt.Nil, nil
	default:
		return advplrt.Nil, fmt.Errorf("testbase: metodo desconhecido %s", method)
	}
}

func TestSuperNativoRegistrado(t *testing.T) {
	registerNativeBase("TESTBASE", func() *advplrt.ObjectValue {
		o := advplrt.NewObject("TESTBASE", nil)
		o.Native = testBaseNative{}
		return o
	})

	out := saidaDe(t, `
Class FilhoN From TestBase
	Method New()
EndClass
Method New() Class FilhoN
	_Super:New()
	Return Self

User Function Main()
	Local o := FilhoN():New()
	ConOut(o:Eco("oi"))
Return
`)
	if !strings.Contains(out, "oi") {
		t.Fatalf("got %q", out)
	}
}

// Instância de classe de usuário com raiz nativa: método que não existe no
// bytecode de nenhum ancestral é atendido pelo estado nativo da raiz,
// mesmo sem _Super no caminho.
func TestMetodoNativoDaRaizSemSuper(t *testing.T) {
	registerNativeBase("TESTBASE2", func() *advplrt.ObjectValue {
		o := advplrt.NewObject("TESTBASE2", nil)
		o.Native = testBaseNative{}
		return o
	})

	out := saidaDe(t, `
Class FilhoM From TestBase2
EndClass

User Function Main()
	Local o := FilhoM():New()
	ConOut(o:Eco("direto"))
Return
`)
	if !strings.Contains(out, "direto") {
		t.Fatalf("got %q", out)
	}
}
