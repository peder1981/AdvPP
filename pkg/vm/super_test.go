package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
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
	if out != "base|filho\n" {
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

func (testBaseNative) NativeCallMethod(v *VM, obj *advplrt.ObjectValue, method string, args []advplrt.Value) (advplrt.Value, error) {
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

// _Super com argumentos nomeados (`ident := valor`): o reorderNamedArgs de
// invokeMethodFrame precisa enxergar os namedArgs — callSuper não pode
// zerá-los antes, senão a ligação posicional troca os parâmetros.
func TestSuperComArgsNomeados(t *testing.T) {
	out := saidaDe(t, `
Class BaseN
	Method Conf(cA, nB)
EndClass
Method Conf(cA, nB) Class BaseN
	::cA := cA
	::nB := nB
	Return Self

Class FilhoN2 From BaseN
	Method Conf(cA, nB)
EndClass
Method Conf(cA, nB) Class FilhoN2
	_Super:Conf(nB := nB, cA := cA)
	Return Self

User Function Main()
	Local o := FilhoN2():Conf("a", 2)
	ConOut(o:cA + "|" + cValToChar(o:nB))
Return
`)
	if !strings.Contains(out, "a|2") {
		t.Fatalf("got %q", out)
	}
}

// _Super com @var: o argumento vai cru (RefValue) para o método do pai,
// que grava de volta na variável do chamador — mesmo comportamento de
// callMethod. Com Deref no callSuper a escrita se perderia (n fica 1).
func TestSuperComReferencia(t *testing.T) {
	out := saidaDe(t, `
Class BaseR
	Method Bump(nAlvo)
EndClass
Method Bump(nAlvo) Class BaseR
	nAlvo += 10
	Return .T.

Class FilhoR From BaseR
	Method Bump(nAlvo)
	Method New()
EndClass
Method Bump(nAlvo) Class FilhoR
	_Super:Bump(@nAlvo)
	Return Self
Method New() Class FilhoR
	Return Self

User Function Main()
	Local n := 1
	Local o := FilhoR():New()
	o:Bump(@n)
	ConOut("n=" + cValToChar(n))
Return
`)
	if !strings.Contains(out, "n=11") {
		t.Fatalf("got %q", out)
	}
}

// _Super dentro de codeblock/closure: o className do funcContext tem que
// ser propagado, senão o Str sai ":Quem" e a resolução cai no pai da classe
// CONCRETA do objeto — aqui NetoK, que reentraria em MeioK:Quem para
// sempre (stack overflow) em vez de achar BaseK:Quem.
func TestSuperDentroDeClosure(t *testing.T) {
	out := saidaDe(t, `
Class BaseK
	Method Quem()
EndClass
Method Quem() Class BaseK
	Return "K"

Class MeioK From BaseK
	Method Quem()
EndClass
Method Quem() Class MeioK
	Local b := {|| _Super:Quem()}
	Return Eval(b)

Class NetoK From MeioK
	Method New()
EndClass
Method New() Class NetoK
	Return Self

User Function Main()
	ConOut(NetoK():New():Quem())
Return
`)
	if !strings.Contains(out, "K") {
		t.Fatalf("got %q", out)
	}
}

// Raiz nativa só vale para classe com dispatch real: built-in de UI sem
// handler (FWPanel) NÃO é raiz — rootNativeClass devolve "" e o dispatch de
// método continua erro "unknown method" como antes da herança nativa.
// Já JsonObject (case real em callNativeMethodOn) e as registradas via
// registerNativeBase são raiz.
func TestRootNativeClassSoComHandlerReal(t *testing.T) {
	registerNativeBase("TESTBASE3", func() *advplrt.ObjectValue {
		return advplrt.NewObject("TESTBASE3", nil)
	})
	bc := compilarPrg(t, `
Class PainelX From FWPanel
EndClass
Class ObjX From JsonObject
EndClass
Class RegX From TestBase3
EndClass
`)
	v := NewVM(bc, false)
	if got := v.rootNativeClass("PainelX"); got != "" {
		t.Fatalf("FWPanel nao e raiz nativa real: rootNativeClass = %q", got)
	}
	if got := v.rootNativeClass("ObjX"); got != "JsonObject" {
		t.Fatalf("JsonObject deveria ser raiz: %q", got)
	}
	if got := v.rootNativeClass("RegX"); got != "TESTBASE3" {
		t.Fatalf("registerNativeBase deveria ser raiz: %q", got)
	}
}

// _Super numa classe RAIZ (sem pai) chamado sobre instância de subclasse:
// a busca NÃO pode cair no pai da classe concreta do objeto — isso
// reentraria no mesmo método para sempre (stack overflow). Tem que dar
// erro limpo de "super não encontrado".
func TestSuperEmClasseRaizDaErroLimpo(t *testing.T) {
	_, _, err := rodaPrg(t, `
Class BaseS
	Method New()
	Method Quem()
EndClass
Method New() Class BaseS
	Return Self
Method Quem() Class BaseS
	Return _Super:Quem()

Class FilhoS From BaseS
EndClass

User Function Main()
	Local o := FilhoS():New()
	ConOut(o:Quem())
Return
`)
	if err == nil || !strings.Contains(err.Error(), "_Super") {
		t.Fatalf("esperava erro limpo de _Super, got err=%v", err)
	}
}

// _Super dentro de codeblock avaliado por uma FUNÇÃO (não método) cujo
// primeiro parâmetro é um objeto: Locals[0] do frame da função é o objeto,
// mas não é `self` de classe — selfObjectOf não pode aceitá-lo, senão o
// _Super resolveria contra a classe errada. Tem que dar erro de self
// ausente.
func TestSuperEmFuncaoComObjetoNaoViraSelf(t *testing.T) {
	_, _, err := rodaPrg(t, `
Class BaseF
	Method Quem()
EndClass
Method Quem() Class BaseF
	Return "F"

Class FilhoF From BaseF
	Method New()
EndClass
Method New() Class FilhoF
	Return Self

Static Function Avalia(o, b)
	Return Eval(b)

User Function Main()
	Local o := FilhoF():New()
	Local b := {|| _Super:Quem()}
	ConOut(Avalia(o, b))
Return
`)
	if err == nil || !strings.Contains(err.Error(), "_Super") {
		t.Fatalf("esperava erro limpo de _Super fora de metodo, got err=%v", err)
	}
}

// Classe de usuário herdando de componente PoUI (PoApp): PoUI é estado por
// ClassName exato — sub classe de usuário NÃO é raiz nativa válida.
// rootNativeClass devolve "" e o método nativo dá erro limpo (sem criar
// estado poComp/poAppState errado).
func TestHerancaDeClassePoUIRestrita(t *testing.T) {
	bc := compilarPrg(t, `
Class MinhaApp From PoApp
EndClass
`)
	v := NewVM(bc, false)
	if got := v.rootNativeClass("MinhaApp"); got != "" {
		t.Fatalf("PoApp nao deveria ser raiz nativa: rootNativeClass = %q", got)
	}
}

// Codeblock no corpo principal do arquivo (fora de função/método):
// compileCodeBlock rodava com savedFunc == nil e dereferenciava ao propagar
// className — o pânico era em TEMPO DE COMPILAÇÃO. O corpo top-level é o
// MainOffset do bytecode, então o codeblock compila E executa.
func TestCodeblockNoCorpoPrincipalCompila(t *testing.T) {
	out := saidaDe(t, `
Local b := {|| "top"}
ConOut(Eval(b))

User Function Main()
Return
`)
	if !strings.Contains(out, "top") {
		t.Fatalf("got %q", out)
	}
}

// Arg nomeado em parâmetro POSTERIOR: reorderNamedArgs devolve uma posição
// por parâmetro, então o binding tem que iterar len(args) — com o argCount
// original o arg nomeado em posição >= argCount era descartado.
func TestMetodoNamedArgParametroPosterior(t *testing.T) {
	out := saidaDe(t, `
Class BaseP
	Method Conf(a, b, c)
EndClass
Method Conf(a, b, c) Class BaseP
	::r := cValToChar(a) + "|" + cValToChar(b) + "|" + cValToChar(c)
	Return Self

User Function Main()
	Local o := BaseP():Conf(1, c := 9)
	ConOut(o:r)
Return
`)
	if !strings.Contains(out, "1||9") {
		t.Fatalf("got %q", out)
	}
}

// Mesma regra via _Super: o método do pai recebe o arg nomeado em
// parâmetro posterior corretamente.
func TestSuperNamedArgParametroPosterior(t *testing.T) {
	out := saidaDe(t, `
Class BaseP2
	Method Conf(a, b, c)
EndClass
Method Conf(a, b, c) Class BaseP2
	::r := cValToChar(a) + "|" + cValToChar(b) + "|" + cValToChar(c)
	Return Self

Class FilhoP2 From BaseP2
	Method Conf(a, b, c)
EndClass
Method Conf(a, b, c) Class FilhoP2
	_Super:Conf(a, c := c)
	Return Self

User Function Main()
	Local o := FilhoP2():Conf(1, 2, 3)
	ConOut(o:r)
Return
`)
	if !strings.Contains(out, "1||3") {
		t.Fatalf("got %q", out)
	}
}

// Todo nome canônico de nativeHeritageNames precisa ter case real no
// switch de callNativeMethodOn: método inexistente tem que cair num
// handler próprio ("unknown method ... on <Classe>"), nunca no default
// ("unknown method ... on object ...").
func TestNativeHeritageNamesBatemComDispatch(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	for upper, canon := range nativeHeritageNames {
		obj := advplrt.NewObject(canon, nil)
		err := v.callNativeMethodOn(obj, canon, "METODO_INEXISTENTE_QQ", nil)
		if err == nil {
			t.Errorf("%s (%s): esperava erro para método inexistente", upper, canon)
			continue
		}
		if strings.Contains(err.Error(), "on object") {
			t.Errorf("%s -> %s: caiu no default de callNativeMethodOn (sem case real)", upper, canon)
		}
	}
}

// Classe de usuário herdando de built-in STATEFUL (FWMBrowse cria
// browseState no construtor, não no :New): a instância tem que receber
// obj.Native da fábrica built-in — sem isso o primeiro método nativo
// quebraria com tipo errado.
func TestHerancaBuiltinStatefulRecebeNative(t *testing.T) {
	bc := compilarPrg(t, `
Class MeuBrowse From FWMBrowse
EndClass
`)
	v := NewVM(bc, false)
	if got := v.rootNativeClass("MeuBrowse"); got != "FWMBrowse" {
		t.Fatalf("rootNativeClass = %q, esperava FWMBrowse", got)
	}
	if err := v.newInstance("MeuBrowse", nil); err != nil {
		t.Fatalf("newInstance: %v", err)
	}
	obj, ok := v.stack[len(v.stack)-1].(*advplrt.ObjectValue)
	if !ok {
		t.Fatalf("topo da pilha não é objeto: %T", v.stack[len(v.stack)-1])
	}
	if _, ok := obj.Native.(*browseState); !ok {
		t.Fatalf("obj.Native = %T, esperava *browseState", obj.Native)
	}
}

// registerNativeBase rejeita nome vazio (erro de programação, em init).
func TestRegisterNativeBaseNomeVazioPanica(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("registerNativeBase(\"\") deveria entrar em pânico")
		}
	}()
	registerNativeBase("", func() *advplrt.ObjectValue { return nil })
}

// _Super com defClass fora da hierarquia do objeto: uso indevido (ex.:
// código montado à mão ou self trocado) — erro limpo, não despacho errado.
func TestSuperDefClassForaDaHierarquia(t *testing.T) {
	v := NewVM(compilarPrg(t, `
Class A1
	Method Quem()
EndClass
Method Quem() Class A1
	Return "A1"
Class B1
	Method Quem()
EndClass
Method Quem() Class B1
	Return "B1"
`), false)
	// Simula OP_CALL_SUPER com Str "A1:Quem" executado num frame de método
	// de B1 (defClass A1 não é ancestral de B1).
	objB := advplrt.NewObject("B1", nil)
	frame := &CallFrame{
		FuncName:  "B1::Quem",
		Code:      v.bc.Code,
		IP:        0,
		Locals:    newLocals(4),
		StackBase: len(v.stack),
		Self:      objB,
		IsMethod:  true,
	}
	v.frames = append(v.frames, frame)
	v.current = frame
	err := v.callSuper("A1:Quem", 0)
	if err == nil || !strings.Contains(err.Error(), "nao pertence a hierarquia") {
		t.Fatalf("esperava erro de hierarquia, got %v", err)
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
