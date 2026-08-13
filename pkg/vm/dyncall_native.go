package vm

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// `go vet` (analisador `unsafeptr`) sinaliza "possible misuse of
// unsafe.Pointer" em várias linhas deste arquivo (conversões
// uintptr<->unsafe.Pointer em StrLen/StrCpy/MemCpy/GetVar/SetVar). É
// esperado e inevitável: essas funções deliberadamente leem/escrevem
// memória bruta fora do heap Go (endereços resolvidos por Dlsym em uma
// DLL/SO carregada externamente) — o analisador não sabe distinguir um
// endereço C legítimo de um inteiro arbitrário e sinaliza todo esse
// padrão por design, mesmo quando semanticamente correto (mesma
// categoria de FFI que purego, internamente, evita tocar por só
// precisar passar dados Go PARA C, nunca desreferenciar um endereço
// arbitrário QUE VEIO de C). CI roda `go vet -unsafeptr=false ./...`
// (ver Makefile e .github/workflows/test.yml) — os demais analisadores
// do vet continuam ativos normalmente. Não "corrigir" às cegas
// convertendo para outra forma só para silenciar o vet.

// tRunDll (TDN: "DynCall") — classe TLPP para chamar funções/métodos de
// DLL/SO dinamicamente, dada uma assinatura de tipos C/C++ em string
// (legenda: V,B,C,c,S,s,I,i,L,l,G,g,F,D,P,A,T — ver
// https://tdn.totvs.com/display/tec/DynCall). Implementada com
// github.com/ebitengine/purego: sem cgo, carrega a lib nativa via
// dlopen/LoadLibrary e monta a chamada real na ABI da plataforma
// (inclusive double/float em registrador correto), via reflect.FuncOf +
// purego.RegisterFunc.
//
// CallMethod (DLL C++) resolve o símbolo mangled Itanium (GCC/Clang —
// Linux, macOS, MinGW) a partir de "Classe::metodo(tipos)". Mangling
// MSVC (cl.exe, o compilador nativo mais comum em Windows) é um esquema
// proprietário não implementado — ver docs/tdn-known-limitations.md.
//
// Todo método de tRunDll, exceto New/NewPointer/NewObj/GetTimeout,
// documenta na própria TDN "retorno: lógico (.T. sucesso / .F. erro)" —
// o valor de fato (nRet/cRet/xRet) é sempre um parâmetro de SAÍDA POR
// REFERÊNCIA, nunca o retorno do método. Essa VM não tem mecanismo para
// mutar a variável do chamador passada nessa posição (mesma limitação de
// "Parâmetros por referência (@var) em natives", documentada acima) —
// então, seguindo a convenção já usada em todo o resto do projeto
// (GetGlbVars, IPCWaitEx, SFTP*, XmlC14N etc.), o retorno lógico
// documentado pela TDN é preservado tal como é, e o valor requisitado
// via xRet simplesmente não é populado. Não substituímos o retorno
// lógico pelo valor computado — isso divergiria do contrato real da TDN
// e do padrão já estabelecido no restante do compilador.
//
// Exceção real (não um substituto, uma mutação de verdade): quando o
// xRet passado for um objeto TRunDllPointer (obtido via NewPointer/
// NewObj/CallMethod anterior), seu ponteiro interno É atualizado
// in-place — objetos são tipo referência neste VM, mesmo mecanismo já
// usado por HMGet/VarGet para parâmetros de array.
//
// LIMITAÇÃO ARQUITETURAL SÉRIA — passagem de parâmetro por referência
// via '@' (TDN: DynCall - Passagem de parâmetros): a TDN reusa a MESMA
// letra de assinatura escalar (ex. 'I') tanto para "int x" (por valor)
// quanto para "int* x"/"int& x" (por referência) — a diferença de ABI é
// sinalizada só pelo '@' no lado TLPP (`callFunction("sucessor", "II",
// nRet, @nValue)`), nunca na cSignature. Confirmado em
// pkg/lexer/lexer.go (TOKEN_AT) e pkg/parser/expressions.go
// (`case lexer.TOKEN_AT: p.advance(); return p.parsePrimary()`): o
// compilador AdvPP DESCARTA o '@' na hora de compilar um argumento de
// chamada — `@nValue` compila exatamente igual a `nValue`. Não há
// nenhum lugar no VM/compilador (native, method call ou qualquer outro)
// onde essa informação sobreviva até aqui. Por isso, dynCallArgToReflect
// SEMPRE monta o parâmetro como valor puro para letras escalares — se a
// função/método C/C++ real espera um ponteiro nessa posição (`int*`,
// `int&`), a chamada feita por este VM está ABI-incorreta (passa 4/8
// bytes de valor onde o C espera um endereço de 8 bytes) e pode
// crashar o processo. Não há correção possível sem adicionar rastreio
// de '@' ao compilador inteiro — fora do escopo desta classe. Único
// caminho seguro hoje: usar a letra 'P' explicitamente na assinatura e
// passar um TRunDllPointer real (NewPointer/NewObj) como argumento —
// funciona porque 'P' já é inequivocamente um ponteiro, sem depender de
// '@'. Ver docs/tdn-known-limitations.md.
type dllState struct {
	handle  uintptr
	path    string
	lastErr string
	timeout int // segundos; aceito e devolvido, sem efeito real (chamada é síncrona in-process)
}

type dllPointerState struct {
	addr uintptr
	// owned é não-nil quando o endereço foi alocado pelo próprio Go
	// (NewObj(nBytes), caso 2 da TDN — "TLPP vai alocar um espaço de
	// memória e depois chamar o construtor"). Mantido vivo e fixado
	// (runtime.Pinner) para que o coletor de lixo do Go nunca invalide
	// um endereço que uma DLL C++ recebeu como "this". Nil quando o
	// endereço veio de dentro da DLL (factory/malloc — memória que este
	// VM não possui e não deve tentar liberar).
	owned  []byte
	pinner *runtime.Pinner
}

// dynCallDefaultTimeout é o default documentado pela TDN (DynCall -
// Configuração de timeout: "utilize o valor default configurado
// internamente (60 segundos)") quando SetTimeout nunca foi chamado.
const dynCallDefaultTimeout = 60

func newDllObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("TRunDll", nil)
	obj.Native = &dllState{timeout: dynCallDefaultTimeout}
	return obj
}

func newDllPointerObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("TRunDllPointer", nil)
	obj.Native = &dllPointerState{}
	return obj
}

// newDllOwnedObject implementa NewObj(nBytes) (TDN: caso 2 — "a
// aplicação TLPP vai fazer o new"): aloca nBytes de memória Go e a fixa
// com runtime.Pinner para que o endereço passado como "this" a um
// construtor de DLL C++ permaneça válido enquanto o objeto existir.
func newDllOwnedObject(nBytes int) *advplrt.ObjectValue {
	obj := advplrt.NewObject("TRunDllPointer", nil)
	buf := make([]byte, nBytes)
	p := &runtime.Pinner{}
	p.Pin(&buf[0])
	obj.Native = &dllPointerState{
		addr:   uintptr(unsafe.Pointer(&buf[0])),
		owned:  buf,
		pinner: p,
	}
	return obj
}

func (v *VM) callTRunDllMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	d, ok := obj.Native.(*dllState)
	if !ok {
		return fmt.Errorf("TRunDll: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		// New(cDllName) -> objeto tRunDll (TDN: DynCall - Construtor)
		path := advplrt.ToString(getArg(args, 0))
		h, err := dynCallDlopen(path)
		if err != nil {
			d.lastErr = err.Error()
		} else {
			d.handle = h
			d.path = path
		}
		v.push(obj)
		return nil

	case "FREE":
		// Free() -> lógico (TDN: DynCall - Free)
		if d.handle != 0 {
			_ = dynCallDlclose(d.handle)
			d.handle = 0
			v.push(advplrt.NewBool(true))
			return nil
		}
		v.push(advplrt.NewBool(false))
		return nil

	case "CALLFUNCTION":
		// CallFunction(cFuncName, cSignature, xRet, ...args) -> lógico
		// (TDN: DynCall - CallFunction). xRet É o retorno documentado, mas
		// como saída por referência que este VM não escreve (ver comentário
		// do tipo dllState) — exceto quando xRet é TRunDllPointer, mutado
		// in-place de verdade.
		if d.handle == 0 {
			d.lastErr = "DLL não carregada"
			v.push(advplrt.NewBool(false))
			return nil
		}
		name := advplrt.ToString(getArg(args, 0))
		sig := advplrt.ToString(getArg(args, 1))
		sym, err := dynCallDlsym(d.handle, name)
		if err != nil {
			d.lastErr = err.Error()
			v.push(advplrt.NewBool(false))
			return nil
		}
		result, callErr := dynCallInvoke(sym, sig, args)
		if callErr != nil {
			d.lastErr = callErr.Error()
			v.push(advplrt.NewBool(false))
			return nil
		}
		writeDynCallOutParam(getArg(args, 2), result)
		v.push(advplrt.NewBool(true))
		return nil

	case "CALLMETHOD":
		// CallMethod("Classe::metodo(tipos)", cSignature, xRet, ...args) -> lógico
		if d.handle == 0 {
			d.lastErr = "DLL não carregada"
			v.push(advplrt.NewBool(false))
			return nil
		}
		spec := advplrt.ToString(getArg(args, 0))
		sig := advplrt.ToString(getArg(args, 1))
		mangled, mangleErr := itaniumMangle(spec)
		if mangleErr != nil {
			d.lastErr = mangleErr.Error()
			v.push(advplrt.NewBool(false))
			return nil
		}
		sym, err := dynCallDlsym(d.handle, mangled)
		if err != nil {
			// Fallback C1->C2 / D1->D2 (ver itaniumMangleCtorDtorFallback):
			// clang (default no macOS) pode não emitir o construtor
			// "complete object" (C1) como símbolo próprio quando idêntico
			// ao "base object" (C2) para uma classe sem bases virtuais —
			// achado real via CI macOS. gcc (Linux/MinGW) emite os dois.
			if alt, ok := itaniumMangleCtorDtorFallback(mangled); ok {
				if altSym, altErr := dynCallDlsym(d.handle, alt); altErr == nil {
					sym, mangled, err = altSym, alt, nil
				}
			}
		}
		if err != nil {
			d.lastErr = fmt.Sprintf("símbolo C++ %q (mangled %q) não encontrado: %s", spec, mangled, err.Error())
			v.push(advplrt.NewBool(false))
			return nil
		}
		result, callErr := dynCallInvoke(sym, sig, args)
		if callErr != nil {
			d.lastErr = callErr.Error()
			v.push(advplrt.NewBool(false))
			return nil
		}
		writeDynCallOutParam(getArg(args, 2), result)
		v.push(advplrt.NewBool(true))
		return nil

	case "GETVAR":
		// GetVar(cVarName, cSignature, xRet) -> lógico (TDN: DynCall - GetVar)
		if d.handle == 0 {
			d.lastErr = "DLL não carregada"
			v.push(advplrt.NewBool(false))
			return nil
		}
		name := advplrt.ToString(getArg(args, 0))
		typ := advplrt.ToString(getArg(args, 1))
		sym, err := dynCallDlsym(d.handle, name)
		if err != nil {
			d.lastErr = err.Error()
			v.push(advplrt.NewBool(false))
			return nil
		}
		letter := byte('D')
		if len(typ) > 0 {
			letter = typ[0]
		}
		val, rerr := dynCallReadVar(sym, letter)
		if rerr != nil {
			d.lastErr = rerr.Error()
			v.push(advplrt.NewBool(false))
			return nil
		}
		writeDynCallOutParam(getArg(args, 2), val)
		v.push(advplrt.NewBool(true))
		return nil

	case "SETVAR":
		// SetVar(cVarName, cSignature, xValue) -> lógico (TDN: DynCall - SetVar)
		if d.handle == 0 {
			d.lastErr = "DLL não carregada"
			v.push(advplrt.NewBool(false))
			return nil
		}
		name := advplrt.ToString(getArg(args, 0))
		typ := advplrt.ToString(getArg(args, 1))
		sym, err := dynCallDlsym(d.handle, name)
		if err != nil {
			d.lastErr = err.Error()
			v.push(advplrt.NewBool(false))
			return nil
		}
		letter := byte('D')
		if len(typ) > 0 {
			letter = typ[0]
		}
		werr := dynCallWriteVar(sym, letter, getArg(args, 2))
		if werr != nil {
			d.lastErr = werr.Error()
			v.push(advplrt.NewBool(false))
			return nil
		}
		v.push(advplrt.NewBool(true))
		return nil

	case "NEWPOINTER":
		// NewPointer() -> objeto opaco (TDN: DynCall - NewPointer)
		v.push(newDllPointerObject())
		return nil

	case "NEWOBJ":
		// NewObj([nBytes]) -> objeto opaco (TDN: DynCall - NewObj). Sem
		// nBytes (ou 0): abstração vazia, preenchida por um factory/método
		// estático subsequente (caso 1 da TDN). Com nBytes: aloca memória
		// Go de verdade e fixa (runtime.Pinner) para servir de "this" a um
		// construtor de DLL C++ chamado em seguida via CallMethod (caso 2).
		nBytes := int(advplrt.ToFloat(getArg(args, 0)))
		if nBytes > 0 {
			v.push(newDllOwnedObject(nBytes))
			return nil
		}
		v.push(newDllPointerObject())
		return nil

	case "FREEOBJ":
		// FreeObj(oObj) -> lógico (TDN: DynCall - FreeObj). Libera a
		// alocação Go quando o objeto veio de NewObj(nBytes) (caso 2); um
		// objeto obtido de dentro da DLL (factory/malloc, caso 1) não é
		// liberado por este VM — não é memória que possuímos.
		p, ok := getArg(args, 0).(*advplrt.ObjectValue)
		if !ok {
			v.push(advplrt.NewBool(false))
			return nil
		}
		ps, ok := p.Native.(*dllPointerState)
		if !ok {
			v.push(advplrt.NewBool(false))
			return nil
		}
		if ps.pinner != nil {
			ps.pinner.Unpin()
			ps.pinner = nil
			ps.owned = nil
		}
		ps.addr = 0
		v.push(advplrt.NewBool(true))
		return nil

	case "STRLEN":
		// StrLen(nRet, oPointer) -> lógico (TDN: DynCall - StrLen). nRet é
		// saída por referência (não escrita, ver comentário do tipo
		// dllState); o sucesso indica apenas que o ponteiro é válido e a
		// varredura até o '\0' terminou.
		ptr := dllPointerArg(getArg(args, 1))
		if ptr == 0 {
			v.push(advplrt.NewBool(false))
			return nil
		}
		for n := 0; ; n++ {
			if *(*byte)(unsafe.Pointer(ptr + uintptr(n))) == 0 {
				break
			}
		}
		v.push(advplrt.NewBool(true))
		return nil

	case "STRCPY":
		// StrCpy(oPointer, nMaxSize) -> String (desvio deliberado do TDN, que
		// documenta "lógico" com cRet de saída por referência — inaplicável
		// aqui pois este VM não escreve @var. Lê até '\0' ou nMaxSize bytes,
		// o que vier primeiro, e RETORNA o conteúdo lido. Sem o cRet mudo do
		// TDN original: nenhum uso real deste método existia antes desta
		// correção (ver docs/tdn-known-limitations.md), então não há
		// contrato de posição a preservar.
		ptr := dllPointerArg(getArg(args, 0))
		n := int(advplrt.ToFloat(getArg(args, 1)))
		if ptr == 0 || n < 0 {
			d.lastErr = "StrCpy: ponteiro inválido ou tamanho negativo"
			v.push(advplrt.NewString(""))
			return nil
		}
		buf := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), n)
		end := 0
		for end < n && buf[end] != 0 {
			end++
		}
		v.push(advplrt.NewString(string(buf[:end])))
		return nil

	case "MEMCPY":
		// MemCpy(oPointer, nBytes) -> String (mesmo desvio do StrCpy: retorna
		// os nBytes brutos lidos, sem parar em '\0').
		ptr := dllPointerArg(getArg(args, 0))
		n := int(advplrt.ToFloat(getArg(args, 1)))
		if ptr == 0 || n < 0 {
			d.lastErr = "MemCpy: ponteiro inválido ou tamanho negativo"
			v.push(advplrt.NewString(""))
			return nil
		}
		buf := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), n)
		v.push(advplrt.NewString(string(buf)))
		return nil

	case "GETTIMEOUT":
		// GetTimeout() -> numérico, em segundos (TDN: DynCall - GetTimeout)
		v.push(advplrt.NewNumber(float64(d.timeout)))
		return nil

	case "SETTIMEOUT":
		// SetTimeout(nSeconds) -> lógico (TDN: DynCall - SetTimeout).
		// Aceito e armazenado; sem efeito real — a chamada FFI é síncrona
		// in-process (mesma goroutine da VM); não há mecanismo seguro para
		// abortar preemptivamente uma chamada C travada a meio caminho
		// sem arriscar corromper o processo inteiro (documentado).
		d.timeout = int(advplrt.ToFloat(getArg(args, 0)))
		v.push(advplrt.NewBool(true))
		return nil

	case "GETLASTERROR":
		if d.lastErr != "" {
			v.push(advplrt.NewNumber(1))
		} else {
			v.push(advplrt.NewNumber(0))
		}
		return nil

	case "GETERRORMSG":
		v.push(advplrt.NewString(d.lastErr))
		return nil

	default:
		return fmt.Errorf("unknown method %s on object TRunDll", method)
	}
}

func dllPointerArg(val advplrt.Value) uintptr {
	if p, ok := val.(*advplrt.ObjectValue); ok {
		if ps, ok := p.Native.(*dllPointerState); ok {
			return ps.addr
		}
	}
	// Aceita também um número bruto (endereço), para composição avançada.
	if n, ok := val.(*advplrt.NumberValue); ok {
		return uintptr(n.Val)
	}
	return 0
}

// writeDynCallOutParam aplica a exceção de mutação in-place: se xRet for
// um objeto TRunDllPointer, seu endereço interno é atualizado com o
// resultado (quando o resultado é um ponteiro). Números/strings/bool não
// são graváveis de volta (limitação de @var, ver comentário do arquivo).
func writeDynCallOutParam(xRet advplrt.Value, result advplrt.Value) {
	p, ok := xRet.(*advplrt.ObjectValue)
	if !ok {
		return
	}
	ps, ok := p.Native.(*dllPointerState)
	if !ok {
		return
	}
	switch r := result.(type) {
	case *advplrt.NumberValue:
		ps.addr = uintptr(r.Val)
	case *advplrt.ObjectValue:
		if rps, ok := r.Native.(*dllPointerState); ok {
			ps.addr = rps.addr
		}
	}
}

// dynCallLetterToType converte uma letra da legenda DynCall no reflect.Type
// Go equivalente, usado tanto para parâmetros quanto para o tipo de
// retorno (exceto 'V', tratado à parte por não gerar valor de saída).
func dynCallLetterToType(letter byte) (reflect.Type, error) {
	longIs32 := runtime.GOOS == "windows"
	switch letter {
	case 'B':
		return reflect.TypeOf(false), nil
	case 'C':
		return reflect.TypeOf(int8(0)), nil
	case 'c':
		return reflect.TypeOf(uint8(0)), nil
	case 'S':
		return reflect.TypeOf(int16(0)), nil
	case 's':
		return reflect.TypeOf(uint16(0)), nil
	case 'I':
		return reflect.TypeOf(int32(0)), nil
	case 'i':
		return reflect.TypeOf(uint32(0)), nil
	case 'L':
		if longIs32 {
			return reflect.TypeOf(int32(0)), nil
		}
		return reflect.TypeOf(int64(0)), nil
	case 'l':
		if longIs32 {
			return reflect.TypeOf(uint32(0)), nil
		}
		return reflect.TypeOf(uint64(0)), nil
	case 'G':
		return reflect.TypeOf(int64(0)), nil
	case 'g':
		return reflect.TypeOf(uint64(0)), nil
	case 'F':
		return reflect.TypeOf(float32(0)), nil
	case 'D':
		return reflect.TypeOf(float64(0)), nil
	case 'P':
		return reflect.TypeOf(unsafe.Pointer(nil)), nil
	case 'A':
		return reflect.TypeOf(""), nil
	case 'T':
		return reflect.TypeOf(uintptr(0)), nil
	default:
		return nil, fmt.Errorf("DynCall: letra de assinatura desconhecida %q", string(letter))
	}
}

// dynCallInvoke monta dinamicamente (via reflect.FuncOf + purego.RegisterFunc)
// o tipo de função Go que representa a assinatura C/C++ dada, registra o
// símbolo resolvido nessa função e a invoca com os argumentos reais
// (args[3:], alinhados a sig[1:]).
func dynCallInvoke(sym uintptr, sig string, args []advplrt.Value) (advplrt.Value, error) {
	if len(sig) == 0 {
		return nil, fmt.Errorf("DynCall: assinatura vazia")
	}
	retLetter := sig[0]
	paramLetters := sig[1:]

	inTypes := make([]reflect.Type, 0, len(paramLetters))
	for i := 0; i < len(paramLetters); i++ {
		t, err := dynCallLetterToType(paramLetters[i])
		if err != nil {
			return nil, err
		}
		inTypes = append(inTypes, t)
	}

	var outTypes []reflect.Type
	if retLetter != 'V' {
		t, err := dynCallLetterToType(retLetter)
		if err != nil {
			return nil, err
		}
		outTypes = []reflect.Type{t}
	}

	fnType := reflect.FuncOf(inTypes, outTypes, false)
	fnPtr := reflect.New(fnType)
	purego.RegisterFunc(fnPtr.Interface(), sym)
	fn := fnPtr.Elem()

	realArgs := args[3:]
	if len(realArgs) < len(inTypes) {
		return nil, fmt.Errorf("DynCall: assinatura pede %d parâmetro(s), %d recebido(s)", len(inTypes), len(realArgs))
	}

	inVals := make([]reflect.Value, len(inTypes))
	for i, t := range inTypes {
		rv, err := dynCallArgToReflect(realArgs[i], paramLetters[i], t)
		if err != nil {
			return nil, err
		}
		inVals[i] = rv
	}

	results := fn.Call(inVals)
	if len(results) == 0 {
		return advplrt.Nil, nil
	}
	return dynCallReflectToValue(results[0], retLetter), nil
}

func dynCallArgToReflect(val advplrt.Value, letter byte, t reflect.Type) (reflect.Value, error) {
	switch letter {
	case 'A':
		return reflect.ValueOf(advplrt.ToString(val)), nil
	case 'P':
		addr := dllPointerArg(val)
		return reflect.ValueOf(unsafe.Pointer(addr)), nil
	case 'B':
		return reflect.ValueOf(advplrt.ToBool(val)), nil
	default:
		f := advplrt.ToFloat(val)
		rv := reflect.New(t).Elem()
		switch t.Kind() {
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			rv.SetInt(int64(f))
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
			rv.SetUint(uint64(f))
		case reflect.Float32, reflect.Float64:
			rv.SetFloat(f)
		default:
			return reflect.Value{}, fmt.Errorf("DynCall: não sei converter parâmetro tipo %q", string(letter))
		}
		return rv, nil
	}
}

func dynCallReflectToValue(rv reflect.Value, letter byte) advplrt.Value {
	switch letter {
	case 'A':
		return advplrt.NewString(rv.String())
	case 'B':
		return advplrt.NewBool(rv.Bool())
	case 'P':
		addr := uintptr(rv.UnsafePointer())
		obj := newDllPointerObject()
		obj.Native.(*dllPointerState).addr = addr
		return obj
	default:
		switch rv.Kind() {
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			return advplrt.NewNumber(float64(rv.Int()))
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
			return advplrt.NewNumber(float64(rv.Uint()))
		case reflect.Float32, reflect.Float64:
			return advplrt.NewNumber(rv.Float())
		default:
			return advplrt.Nil
		}
	}
}

// dynCallReadVar/dynCallWriteVar leem/escrevem diretamente a memória de uma
// variável global exportada pela DLL (GetVar/SetVar), pelo endereço
// resolvido via Dlsym, respeitando o tamanho do tipo indicado pela letra.
func dynCallReadVar(addr uintptr, letter byte) (advplrt.Value, error) {
	switch letter {
	case 'B':
		return advplrt.NewBool(*(*byte)(unsafe.Pointer(addr)) != 0), nil
	case 'C':
		return advplrt.NewNumber(float64(*(*int8)(unsafe.Pointer(addr)))), nil
	case 'c':
		return advplrt.NewNumber(float64(*(*uint8)(unsafe.Pointer(addr)))), nil
	case 'S':
		return advplrt.NewNumber(float64(*(*int16)(unsafe.Pointer(addr)))), nil
	case 's':
		return advplrt.NewNumber(float64(*(*uint16)(unsafe.Pointer(addr)))), nil
	case 'I':
		return advplrt.NewNumber(float64(*(*int32)(unsafe.Pointer(addr)))), nil
	case 'i':
		return advplrt.NewNumber(float64(*(*uint32)(unsafe.Pointer(addr)))), nil
	case 'L', 'G':
		return advplrt.NewNumber(float64(*(*int64)(unsafe.Pointer(addr)))), nil
	case 'l', 'g', 'T':
		return advplrt.NewNumber(float64(*(*uint64)(unsafe.Pointer(addr)))), nil
	case 'F':
		return advplrt.NewNumber(float64(*(*float32)(unsafe.Pointer(addr)))), nil
	case 'D':
		return advplrt.NewNumber(*(*float64)(unsafe.Pointer(addr))), nil
	case 'P':
		obj := newDllPointerObject()
		obj.Native.(*dllPointerState).addr = *(*uintptr)(unsafe.Pointer(addr))
		return obj, nil
	default:
		return nil, fmt.Errorf("DynCall: letra de tipo desconhecida %q em GetVar", string(letter))
	}
}

func dynCallWriteVar(addr uintptr, letter byte, val advplrt.Value) error {
	switch letter {
	case 'B':
		var b byte
		if advplrt.ToBool(val) {
			b = 1
		}
		*(*byte)(unsafe.Pointer(addr)) = b
	case 'C':
		*(*int8)(unsafe.Pointer(addr)) = int8(advplrt.ToFloat(val))
	case 'c':
		*(*uint8)(unsafe.Pointer(addr)) = uint8(advplrt.ToFloat(val))
	case 'S':
		*(*int16)(unsafe.Pointer(addr)) = int16(advplrt.ToFloat(val))
	case 's':
		*(*uint16)(unsafe.Pointer(addr)) = uint16(advplrt.ToFloat(val))
	case 'I':
		*(*int32)(unsafe.Pointer(addr)) = int32(advplrt.ToFloat(val))
	case 'i':
		*(*uint32)(unsafe.Pointer(addr)) = uint32(advplrt.ToFloat(val))
	case 'L', 'G':
		*(*int64)(unsafe.Pointer(addr)) = int64(advplrt.ToFloat(val))
	case 'l', 'g', 'T':
		*(*uint64)(unsafe.Pointer(addr)) = uint64(advplrt.ToFloat(val))
	case 'F':
		*(*float32)(unsafe.Pointer(addr)) = float32(advplrt.ToFloat(val))
	case 'D':
		*(*float64)(unsafe.Pointer(addr)) = advplrt.ToFloat(val)
	case 'P':
		*(*uintptr)(unsafe.Pointer(addr)) = dllPointerArg(val)
	default:
		return fmt.Errorf("DynCall: letra de tipo desconhecida %q em SetVar", string(letter))
	}
	return nil
}

// itaniumMangleCtorDtorFallback devolve a variante alternativa de um
// símbolo mangled de construtor/destrutor Itanium ("C1"<->"C2",
// "D1"<->"D2") quando aplicável. Existe porque clang (compilador default
// do macOS) pode, para uma classe sem bases virtuais, não emitir o
// construtor/destrutor "complete object" (C1/D1) como símbolo próprio
// quando seu código é idêntico ao "base object" (C2/D2) — gcc (Linux,
// MinGW/Windows) emite os dois como símbolos distintos. Achado real via
// CI: Dlsym("_ZN6tArithC1Ev") falhava só no macOS; "_ZN6tArithC2Ev"
// (mesma classe, mesmo `nm -D` real) resolve. CallMethod tenta C1/D1
// primeiro (a escolha ABI-correta para construir/destruir o objeto mais
// derivado) e só cai para C2/D2 se o símbolo primário não existir.
func itaniumMangleCtorDtorFallback(mangled string) (string, bool) {
	switch {
	case strings.Contains(mangled, "C1"):
		return strings.Replace(mangled, "C1", "C2", 1), true
	case strings.Contains(mangled, "D1"):
		return strings.Replace(mangled, "D1", "D2", 1), true
	default:
		return "", false
	}
}

// itaniumMangle resolve "Classe::metodo(tipos)" (opcionalmente com mais de
// um nível de escopo, "Ns::Classe::metodo(tipos)") para o símbolo mangled
// Itanium C++ ABI (GCC/Clang) correspondente. Suporta apenas tipos
// primitivos e ponteiros para primitivos — cobre o caso documentado pela
// TDN (ex.: "tArith::Add(double,double)" -> "_ZN6tArith3AddEdd"). Nomes
// de escopo sem "::" (função livre) não são aceitos aqui: CallMethod
// pressupõe uma DLL C++, use CallFunction para símbolos C não mangled.
//
// Construtor/destrutor: a TDN documenta a chamada ao construtor como
// "Classe::Classe(tipos)" (ex.: "tArith::tArith()", NewObj) — Itanium
// não repete o nome da classe como componente comum, usa o marcador
// especial "C1" (complete object constructor); confirmado via
// `nm -D`/`c++filt` em uma libclasse.so real compilada com g++
// (`tArith::tArith()` -> `_ZN6tArithC1Ev`, nunca `_ZN6tArith6tArithEv`).
// Destrutor "~Classe()" usa o marcador "D1" pelo mesmo motivo, por
// simetria (não documentado explicitamente pela TDN, mas mesma regra
// Itanium).
func itaniumMangle(spec string) (string, error) {
	spec = strings.TrimSpace(spec)
	open := strings.IndexByte(spec, '(')
	if open < 0 || !strings.HasSuffix(spec, ")") {
		return "", fmt.Errorf("DynCall: CallMethod espera \"Classe::metodo(tipos)\", recebido %q", spec)
	}
	qualified := spec[:open]
	paramsStr := strings.TrimSpace(spec[open+1 : len(spec)-1])

	parts := strings.Split(qualified, "::")
	if len(parts) < 2 {
		return "", fmt.Errorf("DynCall: CallMethod espera nome qualificado \"Classe::metodo\", recebido %q", qualified)
	}
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			return "", fmt.Errorf("DynCall: nome qualificado inválido %q", qualified)
		}
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	className := parts[len(parts)-2]

	var nameEnc strings.Builder
	for i, p := range parts {
		if i == len(parts)-1 && p == className {
			nameEnc.WriteString("C1") // construtor: "Classe::Classe(...)"
			continue
		}
		if i == len(parts)-1 && p == "~"+className {
			nameEnc.WriteString("D1") // destrutor: "Classe::~Classe()"
			continue
		}
		nameEnc.WriteString(fmt.Sprintf("%d%s", len(p), p))
	}

	var paramEnc strings.Builder
	if paramsStr == "" {
		paramEnc.WriteString("v")
	} else {
		for _, raw := range strings.Split(paramsStr, ",") {
			enc, err := itaniumEncodeType(strings.TrimSpace(raw))
			if err != nil {
				return "", err
			}
			paramEnc.WriteString(enc)
		}
	}

	return "_ZN" + nameEnc.String() + "E" + paramEnc.String(), nil
}

// itaniumEncodeType mangled um único tipo de parâmetro C/C++ primitivo,
// com até um nível de ponteiro e qualificador const opcional
// ("const char *" -> "PKc"). Tipos compostos (referências, templates,
// classes por valor) não são suportados — retornam erro explícito em vez
// de mangling incorreto.
func itaniumEncodeType(t string) (string, error) {
	t = strings.TrimSpace(t)
	isConst := false
	if strings.HasPrefix(t, "const ") {
		isConst = true
		t = strings.TrimSpace(strings.TrimPrefix(t, "const "))
	}
	isPtr := false
	if strings.HasSuffix(t, "*") {
		isPtr = true
		t = strings.TrimSpace(strings.TrimSuffix(t, "*"))
	}

	base, ok := itaniumBuiltins[t]
	if !ok {
		return "", fmt.Errorf("DynCall: tipo de parâmetro C++ não suportado no mangling: %q", t)
	}

	if !isPtr {
		if isConst {
			return "", fmt.Errorf("DynCall: 'const %s' por valor não suportado no mangling", t)
		}
		return base, nil
	}
	if isConst {
		return "PK" + base, nil
	}
	return "P" + base, nil
}

var itaniumBuiltins = map[string]string{
	"void":                   "v",
	"bool":                   "b",
	"char":                   "c",
	"signed char":            "a",
	"unsigned char":          "h",
	"short":                  "s",
	"short int":              "s",
	"unsigned short":         "t",
	"unsigned short int":     "t",
	"int":                    "i",
	"unsigned int":           "j",
	"unsigned":               "j",
	"long":                   "l",
	"long int":               "l",
	"unsigned long":          "m",
	"unsigned long int":      "m",
	"long long":              "x",
	"long long int":          "x",
	"unsigned long long":     "y",
	"unsigned long long int": "y",
	"float":                  "f",
	"double":                 "d",
	"long double":            "e",
}
