package vm

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// puregoDlopenForTest usa os mesmos wrappers dynCallDlopen/dynCallDlsym
// de dyncall_native.go (não purego.Dlopen/Dlsym diretamente) — purego não
// implementa essas duas em Windows (ver dyncall_dlopen_windows.go);
// chamar purego.Dlopen aqui quebraria a COMPILAÇÃO deste arquivo de
// teste no CI Windows, mesmo com os testes pulados por falta de gcc/g++.
func puregoDlopenForTest(so string) (uintptr, error) {
	return dynCallDlopen(so)
}

func dlsymOrFatal(t *testing.T, handle uintptr, name string) uintptr {
	t.Helper()
	sym, err := dynCallDlsym(handle, name)
	if err != nil {
		t.Fatalf("Dlsym(%s): %v", name, err)
	}
	return sym
}

// buildDynCallFixture compila (gcc/g++, se disponíveis) as bibliotecas
// dinâmicas reais de teste a partir de pkg/vm/testdata/dyncall/*.c(pp),
// pulando o teste (não falhando) quando não houver toolchain C/C++ no
// ambiente — mesma decisão de portabilidade já usada em outros testes
// deste pacote que dependem de ferramentas externas.
func buildDynCallFixture(t *testing.T, compilerBin, src, out string) string {
	t.Helper()
	if _, err := exec.LookPath(compilerBin); err != nil {
		t.Skipf("%s não encontrado no ambiente, pulando teste de DynCall", compilerBin)
	}
	dir := t.TempDir()
	outPath := filepath.Join(dir, out)
	cmd := exec.Command(compilerBin, "-shared", "-fPIC", "-o", outPath, filepath.Join("testdata", "dyncall", src))
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("falha ao compilar fixture %s: %v\n%s", src, err, outBytes)
	}
	return outPath
}

func newDllTestVM() *VM {
	return NewVM(&compiler.Bytecode{}, false)
}

func openTestDll(t *testing.T, so string) (*VM, *advplrt.ObjectValue) {
	t.Helper()
	v := newDllTestVM()
	if err := v.newInstance("TRUNDLL", nil); err != nil {
		t.Fatalf("newInstance: %v", err)
	}
	obj := v.pop().(*advplrt.ObjectValue)
	if err := v.callTRunDllMethod(obj, "NEW", []advplrt.Value{advplrt.NewString(so)}); err != nil {
		t.Fatalf("New: %v", err)
	}
	v.pop()
	// Fecha o handle do SO antes do t.TempDir() apagar o diretório do
	// fixture: no Windows, LoadLibrary trava o arquivo em uso — sem este
	// Free, a limpeza automática do TempDir falha com "Access is denied"
	// (achado real via CI Windows, não hipotético).
	t.Cleanup(func() { v.callTRunDllMethod(obj, "FREE", nil); v.pop() })
	return v, obj
}

func dlsymOf(t *testing.T, so, name string) uintptr {
	t.Helper()
	h, err := puregoDlopenForTest(so)
	if err != nil {
		t.Fatalf("Dlopen(%s): %v", so, err)
	}
	t.Cleanup(func() { dynCallDlclose(h) }) // ver comentário em openTestDll
	return dlsymOrFatal(t, h, name)
}

// --- Camada baixa: dynCallInvoke, testada diretamente contra símbolos
// reais resolvidos por Dlsym, para provar que a matemática/ABI real está
// correta (double/float em registrador certo etc.) — a parte de risco
// real desta feature. A camada de método (callTRunDllMethod) só precisa
// ser testada quanto ao contrato .T./.F. documentado pela TDN, já que ela
// não escreve o valor de volta no xRet (limitação de engine, ver
// pkg/vm/dyncall_native.go).

func TestDynCallInvokeAddDouble(t *testing.T) {
	// Exemplo real do TDN (DynCall - Assinatura da chamada):
	// oDll1:CallFunction("Add", "DDD", nRet, nX, nY)
	so := buildDynCallFixture(t, "gcc", "dllc.c", "dllc.so")
	sym := dlsymOf(t, so, "Add")

	result, err := dynCallInvoke(sym, "DDD", []advplrt.Value{
		advplrt.Nil, advplrt.Nil, advplrt.Nil, // name, sig, xRet (ignorados por dynCallInvoke)
		advplrt.NewNumber(2.5), advplrt.NewNumber(4.25),
	})
	if err != nil {
		t.Fatalf("dynCallInvoke(Add): %v", err)
	}
	n, ok := result.(*advplrt.NumberValue)
	if !ok || n.Val != 6.75 {
		t.Errorf("Add(2.5, 4.25) = %v, quer 6.75", result)
	}
}

func TestDynCallInvokeAddInt(t *testing.T) {
	// Exemplo real do TDN (DynCall - CallFunction): add(4, 8) == 12
	so := buildDynCallFixture(t, "gcc", "dllc.c", "dllc.so")
	sym := dlsymOf(t, so, "add")

	result, err := dynCallInvoke(sym, "III", []advplrt.Value{
		advplrt.Nil, advplrt.Nil, advplrt.Nil,
		advplrt.NewNumber(4), advplrt.NewNumber(8),
	})
	if err != nil {
		t.Fatalf("dynCallInvoke(add): %v", err)
	}
	n, ok := result.(*advplrt.NumberValue)
	if !ok || n.Val != 12 {
		t.Errorf("add(4, 8) = %v, quer 12", result)
	}
}

func TestDynCallInvokeCppFactoryAndAdd(t *testing.T) {
	// Exemplo real do TDN (DynCall - CallMethod): tArith::factory() +
	// tArith::add(int,int), esperado 25 para add(11, 14).
	so := buildDynCallFixture(t, "g++", "dllcpp2.cpp", "dllcpp2.so")
	h, err := puregoDlopenForTest(so)
	if err != nil {
		t.Fatalf("Dlopen: %v", err)
	}
	t.Cleanup(func() { dynCallDlclose(h) }) // ver comentário em openTestDll

	factoryMangled, err := itaniumMangle("tArith::factory()")
	if err != nil {
		t.Fatalf("itaniumMangle(factory): %v", err)
	}
	addMangled, err := itaniumMangle("tArith::add(int, int)")
	if err != nil {
		t.Fatalf("itaniumMangle(add): %v", err)
	}

	factorySym := dlsymOrFatal(t, h, factoryMangled)
	addSym := dlsymOrFatal(t, h, addMangled)

	ptrResult, err := dynCallInvoke(factorySym, "P", []advplrt.Value{advplrt.Nil, advplrt.Nil, advplrt.Nil})
	if err != nil {
		t.Fatalf("dynCallInvoke(factory): %v", err)
	}
	thisObj, ok := ptrResult.(*advplrt.ObjectValue)
	if !ok || thisObj.Native.(*dllPointerState).addr == 0 {
		t.Fatalf("factory() não retornou ponteiro válido: %v", ptrResult)
	}

	addResult, err := dynCallInvoke(addSym, "IPII", []advplrt.Value{
		advplrt.Nil, advplrt.Nil, advplrt.Nil,
		thisObj, advplrt.NewNumber(11), advplrt.NewNumber(14),
	})
	if err != nil {
		t.Fatalf("dynCallInvoke(add): %v", err)
	}
	n, ok := addResult.(*advplrt.NumberValue)
	if !ok || n.Val != 25 {
		t.Errorf("tArith::add(11, 14) = %v, quer 25", addResult)
	}
}

func TestDynCallReadWriteVarDouble(t *testing.T) {
	so := buildDynCallFixture(t, "gcc", "dllc.c", "dllc.so")
	sym := dlsymOf(t, so, "gGlobal")

	got, err := dynCallReadVar(sym, 'D')
	if err != nil {
		t.Fatalf("dynCallReadVar: %v", err)
	}
	if got.(*advplrt.NumberValue).Val != 3.5 {
		t.Errorf("gGlobal inicial = %v, quer 3.5", got)
	}

	if err := dynCallWriteVar(sym, 'D', advplrt.NewNumber(9.75)); err != nil {
		t.Fatalf("dynCallWriteVar: %v", err)
	}
	got2, _ := dynCallReadVar(sym, 'D')
	if got2.(*advplrt.NumberValue).Val != 9.75 {
		t.Errorf("gGlobal após write = %v, quer 9.75", got2)
	}
}

// --- Camada de método: contrato .T./.F. documentado pela TDN para cada
// operação de tRunDll, mais a exceção real de mutação in-place quando
// xRet é um TRunDllPointer.

func TestTRunDllNewAndFree(t *testing.T) {
	so := buildDynCallFixture(t, "gcc", "dllc.c", "dllc.so")
	v, obj := openTestDll(t, so)

	if err := v.callTRunDllMethod(obj, "FREE", nil); err != nil {
		t.Fatalf("Free: %v", err)
	}
	if !v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("Free() com DLL carregada deveria devolver .T.")
	}

	// Segundo Free (já sem handle) deve devolver .F., não travar.
	if err := v.callTRunDllMethod(obj, "FREE", nil); err != nil {
		t.Fatalf("Free (2a chamada): %v", err)
	}
	if v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("Free() sem DLL carregada deveria devolver .F.")
	}
}

func TestTRunDllCallFunctionRetornoLogico(t *testing.T) {
	// TDN: CallFunction retorna lógico de sucesso — não o valor (xRet é
	// saída por referência, que este VM não escreve; ver
	// dyncall_native.go). Testado aqui apenas o contrato de retorno; a
	// matemática real é coberta por TestDynCallInvokeAddDouble.
	so := buildDynCallFixture(t, "gcc", "dllc.c", "dllc.so")
	v, obj := openTestDll(t, so)

	args := []advplrt.Value{
		advplrt.NewString("Add"), advplrt.NewString("DDD"), advplrt.Nil,
		advplrt.NewNumber(2.5), advplrt.NewNumber(4.25),
	}
	if err := v.callTRunDllMethod(obj, "CALLFUNCTION", args); err != nil {
		t.Fatalf("CallFunction: %v", err)
	}
	if !v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("CallFunction(Add) com args válidos deveria devolver .T.")
	}
}

func TestTRunDllCallFunctionSimboloInexistente(t *testing.T) {
	so := buildDynCallFixture(t, "gcc", "dllc.c", "dllc.so")
	v, obj := openTestDll(t, so)

	args := []advplrt.Value{
		advplrt.NewString("blablabla"), advplrt.NewString("V"), advplrt.Nil,
	}
	if err := v.callTRunDllMethod(obj, "CALLFUNCTION", args); err != nil {
		t.Fatalf("CallFunction não deve devolver erro Go: %v", err)
	}
	if v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("CallFunction de símbolo inexistente deveria devolver .F.")
	}

	v.callTRunDllMethod(obj, "GETLASTERROR", nil)
	if v.pop().(*advplrt.NumberValue).Val == 0 {
		t.Errorf("GetLastError deveria ser != 0 após falha de Dlsym")
	}
	v.callTRunDllMethod(obj, "GETERRORMSG", nil)
	if v.pop().(*advplrt.StringValue).Val == "" {
		t.Errorf("GetErrorMsg deveria ser não-vazio após falha de Dlsym")
	}
}

func TestTRunDllCallFunctionDllNaoCarregada(t *testing.T) {
	v, obj := openTestDll(t, "/caminho/inexistente.so")

	if err := v.callTRunDllMethod(obj, "CALLFUNCTION", []advplrt.Value{
		advplrt.NewString("Add"), advplrt.NewString("DDD"), advplrt.Nil,
		advplrt.NewNumber(1), advplrt.NewNumber(2),
	}); err != nil {
		t.Fatalf("CallFunction não deve retornar erro Go: %v", err)
	}
	if v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("CallFunction com DLL não carregada deveria devolver .F.")
	}
}

func TestTRunDllGetSetVarContratoLogico(t *testing.T) {
	so := buildDynCallFixture(t, "gcc", "dllc.c", "dllc.so")
	v, obj := openTestDll(t, so)

	if err := v.callTRunDllMethod(obj, "GETVAR", []advplrt.Value{
		advplrt.NewString("gGlobal"), advplrt.NewString("D"), advplrt.Nil,
	}); err != nil {
		t.Fatalf("GetVar: %v", err)
	}
	if !v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("GetVar(gGlobal) deveria devolver .T.")
	}

	if err := v.callTRunDllMethod(obj, "SETVAR", []advplrt.Value{
		advplrt.NewString("gGlobal"), advplrt.NewString("D"), advplrt.NewNumber(9.75),
	}); err != nil {
		t.Fatalf("SetVar: %v", err)
	}
	if !v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("SetVar(gGlobal) deveria devolver .T.")
	}

	got, err := dynCallReadVar(dlsymOf(t, so, "gGlobal"), 'D')
	if err != nil {
		t.Fatalf("dynCallReadVar pós-SetVar: %v", err)
	}
	if got.(*advplrt.NumberValue).Val != 9.75 {
		t.Errorf("gGlobal após SetVar via método = %v, quer 9.75 (prova real de escrita)", got)
	}

	if err := v.callTRunDllMethod(obj, "GETVAR", []advplrt.Value{
		advplrt.NewString("nome_inexistente_xyz"), advplrt.NewString("D"), advplrt.Nil,
	}); err != nil {
		t.Fatalf("GetVar: %v", err)
	}
	if v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("GetVar de variável inexistente deveria devolver .F.")
	}
}

func TestTRunDllCallMethodConstrutorEFactory(t *testing.T) {
	// Exercita a mutação in-place do xRet quando é um TRunDllPointer —
	// a única forma real de obter um ponteiro de volta via a camada de
	// método (o retorno do método em si é sempre .T./.F.).
	so := buildDynCallFixture(t, "g++", "dllcpp.cpp", "dllcpp.so")
	v, obj := openTestDll(t, so)

	if err := v.callTRunDllMethod(obj, "NEWPOINTER", nil); err != nil {
		t.Fatalf("NewPointer: %v", err)
	}
	oPointer := v.pop().(*advplrt.ObjectValue)
	if oPointer.Native.(*dllPointerState).addr != 0 {
		t.Fatalf("NewPointer deveria começar com endereço 0")
	}

	if err := v.callTRunDllMethod(obj, "CALLMETHOD", []advplrt.Value{
		advplrt.NewString("tArith::factory()"),
		advplrt.NewString("P"),
		oPointer,
	}); err != nil {
		t.Fatalf("CallMethod(factory): %v", err)
	}
	if !v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("CallMethod(factory) deveria devolver .T.")
	}
	if oPointer.Native.(*dllPointerState).addr == 0 {
		t.Errorf("oPointer não foi mutado in-place por CallMethod(factory) — xRet=TRunDllPointer deveria receber o endereço real")
	}

	if err := v.callTRunDllMethod(obj, "CALLMETHOD", []advplrt.Value{
		advplrt.NewString("tArith::Add(double,double)"),
		advplrt.NewString("DPDD"),
		advplrt.Nil,
		oPointer,
		advplrt.NewNumber(1.5),
		advplrt.NewNumber(2.25),
	}); err != nil {
		t.Fatalf("CallMethod(Add): %v", err)
	}
	if !v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("CallMethod(Add) com this válido deveria devolver .T.")
	}
}

func TestTRunDllNewObjComTamanhoAlocaEChamaConstrutor(t *testing.T) {
	// Caso 2 da TDN (DynCall - NewObj): "a aplicação TLPP vai fazer o
	// new tArith()" — aloca nBytes e chama o construtor da DLL sobre esse
	// endereço (placement new real, "tArith::tArith()" -> mangling "C1").
	so := buildDynCallFixture(t, "g++", "dllcpp.cpp", "dllcpp.so")
	v, obj := openTestDll(t, so)

	if err := v.callTRunDllMethod(obj, "NEWOBJ", []advplrt.Value{advplrt.NewNumber(64)}); err != nil {
		t.Fatalf("NewObj(64): %v", err)
	}
	oObj := v.pop().(*advplrt.ObjectValue)
	ps := oObj.Native.(*dllPointerState)
	if ps.addr == 0 || ps.owned == nil {
		t.Fatalf("NewObj(64) deveria alocar memória Go real e apontar para ela")
	}

	if err := v.callTRunDllMethod(obj, "CALLMETHOD", []advplrt.Value{
		advplrt.NewString("tArith::tArith()"),
		advplrt.NewString("VP"),
		advplrt.Nil,
		oObj,
	}); err != nil {
		t.Fatalf("CallMethod(construtor): %v", err)
	}
	if !v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("CallMethod(tArith::tArith()) deveria devolver .T. (construtor real chamado sobre memória alocada)")
	}

	if err := v.callTRunDllMethod(obj, "FREEOBJ", []advplrt.Value{oObj}); err != nil {
		t.Fatalf("FreeObj: %v", err)
	}
	if !v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("FreeObj deveria devolver .T.")
	}
	if ps.owned != nil || ps.addr != 0 {
		t.Errorf("FreeObj deveria liberar a alocação Go de NewObj(nBytes)")
	}
}

func TestTRunDllStrLenStrCpyMemCpyContratoLogico(t *testing.T) {
	so := buildDynCallFixture(t, "gcc", "dllc.c", "dllc.so")
	v, obj := openTestDll(t, so)

	if err := v.callTRunDllMethod(obj, "NEWPOINTER", nil); err != nil {
		t.Fatalf("NewPointer: %v", err)
	}
	oPtr := v.pop().(*advplrt.ObjectValue)

	if err := v.callTRunDllMethod(obj, "CALLFUNCTION", []advplrt.Value{
		advplrt.NewString("getPtr"), advplrt.NewString("P"), oPtr,
	}); err != nil {
		t.Fatalf("CallFunction(getPtr): %v", err)
	}
	if !v.pop().(*advplrt.BoolValue).Val {
		t.Fatalf("CallFunction(getPtr) deveria devolver .T.")
	}
	if oPtr.Native.(*dllPointerState).addr == 0 {
		t.Fatalf("oPtr não foi mutado in-place por CallFunction(getPtr)")
	}

	// StrLen(nRet, oPointer) -> lógico; nRet (posição 0) é saída por
	// referência, não escrita — só o contrato .T./.F. é verificável aqui.
	if err := v.callTRunDllMethod(obj, "STRLEN", []advplrt.Value{advplrt.Nil, oPtr}); err != nil {
		t.Fatalf("StrLen: %v", err)
	}
	if !v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("StrLen sobre ponteiro válido deveria devolver .T.")
	}

	if err := v.callTRunDllMethod(obj, "STRCPY", []advplrt.Value{advplrt.Nil, oPtr, advplrt.NewNumber(12)}); err != nil {
		t.Fatalf("StrCpy: %v", err)
	}
	if !v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("StrCpy sobre ponteiro válido deveria devolver .T.")
	}

	if err := v.callTRunDllMethod(obj, "MEMCPY", []advplrt.Value{advplrt.Nil, oPtr, advplrt.NewNumber(12)}); err != nil {
		t.Fatalf("MemCpy: %v", err)
	}
	if !v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("MemCpy sobre ponteiro válido deveria devolver .T.")
	}

	// Ponteiro nulo (NewPointer ainda não amarrado) deve devolver .F. em
	// todas as três, sem panic.
	if err := v.callTRunDllMethod(obj, "NEWPOINTER", nil); err != nil {
		t.Fatalf("NewPointer: %v", err)
	}
	nullPtr := v.pop().(*advplrt.ObjectValue)
	v.callTRunDllMethod(obj, "STRLEN", []advplrt.Value{advplrt.Nil, nullPtr})
	if v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("StrLen sobre ponteiro nulo deveria devolver .F.")
	}
}

func TestTRunDllGetSetTimeout(t *testing.T) {
	so := buildDynCallFixture(t, "gcc", "dllc.c", "dllc.so")
	v, obj := openTestDll(t, so)

	// TDN: DynCall - Configuração de timeout — "utilize o valor default
	// configurado internamente (60 segundos)".
	v.callTRunDllMethod(obj, "GETTIMEOUT", nil)
	if v.pop().(*advplrt.NumberValue).Val != 60 {
		t.Errorf("GetTimeout inicial deveria ser 60 (default documentado pela TDN)")
	}

	if err := v.callTRunDllMethod(obj, "SETTIMEOUT", []advplrt.Value{advplrt.NewNumber(10)}); err != nil {
		t.Fatalf("SetTimeout: %v", err)
	}
	if !v.pop().(*advplrt.BoolValue).Val {
		t.Errorf("SetTimeout deveria devolver .T.")
	}

	v.callTRunDllMethod(obj, "GETTIMEOUT", nil)
	if v.pop().(*advplrt.NumberValue).Val != 10 {
		t.Errorf("GetTimeout após SetTimeout(10) deveria ser 10")
	}
}

func TestItaniumMangleAddDouble(t *testing.T) {
	got, err := itaniumMangle("tArith::Add(double,double)")
	if err != nil {
		t.Fatalf("itaniumMangle: %v", err)
	}
	want := "_ZN6tArith3AddEdd" // confirmado via `nm -D dllcpp.so` (g++ real)
	if got != want {
		t.Errorf("itaniumMangle(Add) = %q, quer %q", got, want)
	}
}

func TestItaniumMangleFactoryStatic(t *testing.T) {
	got, err := itaniumMangle("tArith::factory()")
	if err != nil {
		t.Fatalf("itaniumMangle: %v", err)
	}
	want := "_ZN6tArith7factoryEv" // confirmado via `nm -D dllcpp.so` (g++ real)
	if got != want {
		t.Errorf("itaniumMangle(factory) = %q, quer %q", got, want)
	}
}

func TestItaniumMangleConstrutor(t *testing.T) {
	// tArith::tArith() (TDN: NewObj, caso 2) -> marcador C1, não "6tArith"
	// repetido. Confirmado via `nm -D dllcpp.so` real: _ZN6tArithC1Ev.
	got, err := itaniumMangle("tArith::tArith()")
	if err != nil {
		t.Fatalf("itaniumMangle: %v", err)
	}
	want := "_ZN6tArithC1Ev"
	if got != want {
		t.Errorf("itaniumMangle(construtor) = %q, quer %q", got, want)
	}
}

func TestItaniumMangleRejeitaFuncaoLivre(t *testing.T) {
	if _, err := itaniumMangle("Add(double,double)"); err == nil {
		t.Errorf("itaniumMangle deveria rejeitar nome sem escopo de classe")
	}
}
