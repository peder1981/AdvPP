package vm

import (
	"runtime"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// newMiscNatives cria um VM novo e registra as natives misc num mapa próprio
// (padrão dos testes de arquivos nativos que não alteram natives.go).
func newMiscNatives() (*VM, map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	v := NewVM(&compiler.Bytecode{}, false)
	natives := map[string]func(args []advplrt.Value) (advplrt.Value, error){}
	v.registerMiscNatives(natives)
	return v, natives
}

// TestCopyClipboard verifica o roundtrip COPYTOCLIPBOARD/PASTEFROMCLIPBOARD
// no buffer do processo (o clipboard real do SO não é usado nesta VM).
func TestCopyClipboard(t *testing.T) {
	_, natives := newMiscNatives()

	res, err := natives["COPYTOCLIPBOARD"]([]advplrt.Value{advplrt.NewString("HELLO WORLD")})
	if err != nil {
		t.Fatalf("COPYTOCLIPBOARD retornou erro: %v", err)
	}
	if res != advplrt.Nil {
		t.Fatalf("COPYTOCLIPBOARD retornou %v, esperado Nil", res.Type())
	}

	got, err := natives["PASTEFROMCLIPBOARD"](nil)
	if err != nil {
		t.Fatalf("PASTEFROMCLIPBOARD retornou erro: %v", err)
	}
	if advplrt.ToString(got) != "HELLO WORLD" {
		t.Errorf("PASTEFROMCLIPBOARD = %q, esperado %q", advplrt.ToString(got), "HELLO WORLD")
	}
}

// TestGetClientDir verifica que GETCLIENTDIR devolve o diretório de trabalho.
func TestGetClientDir(t *testing.T) {
	_, natives := newMiscNatives()
	got, err := natives["GETCLIENTDIR"](nil)
	if err != nil {
		t.Fatalf("GETCLIENTDIR retornou erro: %v", err)
	}
	if advplrt.ToString(got) == "" {
		t.Error("GETCLIENTDIR retornou string vazia")
	}
}

// TestTone verifica que TONE devolve .T. (o bell foi emitido).
func TestTone(t *testing.T) {
	_, natives := newMiscNatives()
	got, err := natives["TONE"](nil)
	if err != nil {
		t.Fatalf("TONE retornou erro: %v", err)
	}
	if !advplrt.ToBool(got) {
		t.Errorf("TONE retornou %v, esperado .T.", got)
	}
}

// TestErrorBlock verifica o roundtrip define/consulta do handler de erro:
// ao definir um bloco novo, ERRORBLOCK devolve o anterior.
func TestErrorBlock(t *testing.T) {
	_, natives := newMiscNatives()

	b1 := advplrt.NewString("handler1")
	if _, err := natives["ERRORBLOCK"]([]advplrt.Value{b1}); err != nil {
		t.Fatalf("ERRORBLOCK(b1) retornou erro: %v", err)
	}
	cur, err := natives["ERRORBLOCK"](nil)
	if err != nil {
		t.Fatalf("ERRORBLOCK() retornou erro: %v", err)
	}
	if cur != b1 {
		t.Errorf("ERRORBLOCK() = %v, esperado o handler definido", cur)
	}

	b2 := advplrt.NewString("handler2")
	prev, err := natives["ERRORBLOCK"]([]advplrt.Value{b2})
	if err != nil {
		t.Fatalf("ERRORBLOCK(b2) retornou erro: %v", err)
	}
	if prev != b1 {
		t.Errorf("ERRORBLOCK(b2) retornou %v, esperado o handler anterior", prev)
	}
	cur2, _ := natives["ERRORBLOCK"](nil)
	if cur2 != b2 {
		t.Errorf("ERRORBLOCK() após b2 = %v, esperado b2", cur2)
	}
}

// TestDeleteRMT verifica que __DELETERMT remove a lista identificada do
// armazenamento remoto (mesmo store da __DeleteRmt pré-existente) e retorna Nil.
func TestDeleteRMT(t *testing.T) {
	v, natives := newMiscNatives()
	v.remoteMemory["myId1"] = []advplrt.Value{advplrt.NewString("var1"), advplrt.NewNumber(2)}
	v.remoteMemory["myId2"] = []advplrt.Value{advplrt.NewString("var1")}

	res, err := natives["__DELETERMT"]([]advplrt.Value{advplrt.NewString("myId1")})
	if err != nil {
		t.Fatalf("__DELETERMT retornou erro: %v", err)
	}
	if res != advplrt.Nil {
		t.Errorf("__DELETERMT retornou %v, esperado Nil", res.Type())
	}
	if _, exists := v.remoteMemory["myId1"]; exists {
		t.Error("__DELETERMT não removeu 'myId1'")
	}
	if _, exists := v.remoteMemory["myId2"]; !exists {
		t.Error("__DELETERMT removeu 'myId2' por engano")
	}

	// Identificador inexistente: sem erro, retorna Nil.
	res2, err := natives["__DELETERMT"]([]advplrt.Value{advplrt.NewString("inexistente")})
	if err != nil {
		t.Fatalf("__DELETERMT(inexistente) retornou erro: %v", err)
	}
	if res2 != advplrt.Nil {
		t.Errorf("__DELETERMT(inexistente) retornou %v, esperado Nil", res2.Type())
	}
}

// TestPing verifica a forma TCP (host/porta) e a forma TDN (numérica):
// porta sem listener -> .F.; latência numérica -> 0 (processo único).
func TestPing(t *testing.T) {
	_, natives := newMiscNatives()

	got, err := natives["PING"]([]advplrt.Value{advplrt.NewString("localhost"), advplrt.NewNumber(0)})
	if err != nil {
		t.Fatalf("PING(host,port) retornou erro: %v", err)
	}
	if advplrt.ToBool(got) {
		t.Error("PING(localhost,0) deveria ser .F. (porta sem listener)")
	}

	got2, err := natives["PING"]([]advplrt.Value{advplrt.NewNumber(3)})
	if err != nil {
		t.Fatalf("PING(n) retornou erro: %v", err)
	}
	if _, ok := got2.(*advplrt.NumberValue); !ok {
		t.Errorf("PING(n) retornou %v, esperado numérico", got2.Type())
	}
}

// TestWinExec verifica WINEXEC: 0 em sucesso (comando que existe em qualquer
// SO) e código != 0 quando o executável não existe.
func TestWinExec(t *testing.T) {
	_, natives := newMiscNatives()

	lsCmd := "/bin/ls"
	if runtime.GOOS == "windows" {
		lsCmd = "cmd.exe"
	}
	got, err := natives["WINEXEC"]([]advplrt.Value{advplrt.NewString(lsCmd)})
	if err != nil {
		t.Fatalf("WINEXEC(%s) retornou erro: %v", lsCmd, err)
	}
	if advplrt.ToFloat(got) != 0 {
		t.Errorf("WINEXEC(%s) = %v, esperado 0 (sucesso)", lsCmd, advplrt.ToFloat(got))
	}

	got2, err := natives["WINEXEC"]([]advplrt.Value{advplrt.NewString("/bin/comando-inexistente-advpp")})
	if err != nil {
		t.Fatalf("WINEXEC(inexistente) retornou erro: %v", err)
	}
	if advplrt.ToFloat(got2) == 0 {
		t.Error("WINEXEC(inexistente) deveria retornar código de erro != 0")
	}
}

// TestShell verifica SHELLEXECUTE: > 32 em sucesso (comando que existe em
// qualquer SO) e código de erro ShellExecute (2..32) quando o arquivo não
// existe.
func TestShell(t *testing.T) {
	_, natives := newMiscNatives()

	lsCmd := "/bin/ls"
	if runtime.GOOS == "windows" {
		lsCmd = "cmd.exe"
	}
	got, err := natives["SHELLEXECUTE"]([]advplrt.Value{
		advplrt.NewString("open"),
		advplrt.NewString(lsCmd),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("SHELLEXECUTE(open,%s) retornou erro: %v", lsCmd, err)
	}
	if advplrt.ToFloat(got) <= 32 {
		t.Errorf("SHELLEXECUTE(open,%s) = %v, esperado > 32 (sucesso)", lsCmd, advplrt.ToFloat(got))
	}

	got2, err := natives["SHELLEXECUTE"]([]advplrt.Value{
		advplrt.NewString("open"),
		advplrt.NewString("/caminho/inexistente/advpp.txt"),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("SHELLEXECUTE(inexistente) retornou erro: %v", err)
	}
	c := advplrt.ToFloat(got2)
	if c == 0 || c > 32 {
		t.Errorf("SHELLEXECUTE(inexistente) = %v, esperado código de erro 2..32", c)
	}
}

// TestExecInDLL verifica o ciclo de vida simulado das DLLs: open (idempotente),
// run/run2/run3 (saída vazia/0), close (handle deixa de valer) e handle
// inválido -> Nil.
func TestExecInDLL(t *testing.T) {
	_, natives := newMiscNatives()

	h, err := natives["EXECINDLLOPEN"]([]advplrt.Value{advplrt.NewString("TSTDLL.DLL")})
	if err != nil {
		t.Fatalf("EXECINDLLOPEN retornou erro: %v", err)
	}
	hval := advplrt.ToFloat(h)
	if hval <= 0 {
		t.Fatalf("EXECINDLLOPEN = %v, esperado handle > 0", hval)
	}

	// Reabrir o mesmo DLL (case-insensitive) devolve o mesmo handle.
	h2, _ := natives["EXECINDLLOPEN"]([]advplrt.Value{advplrt.NewString("tstdll.dll")})
	if advplrt.ToFloat(h2) != hval {
		t.Errorf("EXECINDLLOPEN repetido = %v, esperado %v", advplrt.ToFloat(h2), hval)
	}

	// Run: buffer vazio (DLL simulada não produz retorno).
	r, err := natives["EXECINDLLRUN"]([]advplrt.Value{h, advplrt.NewNumber(1), advplrt.NewString("in")})
	if err != nil {
		t.Fatalf("EXECINDLLRUN retornou erro: %v", err)
	}
	if advplrt.ToString(r) != "" {
		t.Errorf("EXECINDLLRUN = %q, esperado buffer vazio", advplrt.ToString(r))
	}

	// Run2/Run3: retorno numérico 0.
	r2, _ := natives["EXEDLLRUN2"]([]advplrt.Value{h, advplrt.NewNumber(1), advplrt.NewString("in")})
	if advplrt.ToFloat(r2) != 0 {
		t.Errorf("EXEDLLRUN2 = %v, esperado 0", advplrt.ToFloat(r2))
	}
	r3, _ := natives["EXEDLLRUN3"]([]advplrt.Value{h, advplrt.NewNumber(1), advplrt.NewString("in")})
	if advplrt.ToFloat(r3) != 0 {
		t.Errorf("EXEDLLRUN3 = %v, esperado 0", advplrt.ToFloat(r3))
	}

	// Handle inválido -> Nil.
	bad, _ := natives["EXECINDLLRUN"]([]advplrt.Value{advplrt.NewNumber(999), advplrt.NewNumber(1), advplrt.NewString("in")})
	if bad != advplrt.Nil {
		t.Errorf("EXECINDLLRUN(handle inválido) = %v, esperado Nil", bad.Type())
	}

	// Close remove o handle; chamadas subsequentes devolvem Nil.
	if _, err := natives["EXECINDLLCLOSE"]([]advplrt.Value{h}); err != nil {
		t.Fatalf("EXECINDLLCLOSE retornou erro: %v", err)
	}
	bad2, _ := natives["EXEDLLRUN2"]([]advplrt.Value{h, advplrt.NewNumber(1), advplrt.NewString("in")})
	if bad2 != advplrt.Nil {
		t.Errorf("EXEDLLRUN2 após close = %v, esperado Nil", bad2.Type())
	}
}

// TestGetChildCt verifica GETCHILDCT: janela válida -> 0 filhos (sem GUI);
// parâmetro inválido (nil) -> -1.
func TestGetChildCt(t *testing.T) {
	_, natives := newMiscNatives()

	got, err := natives["GETCHILDCT"]([]advplrt.Value{advplrt.NewObject("TWindow", nil)})
	if err != nil {
		t.Fatalf("GETCHILDCT(oWindow) retornou erro: %v", err)
	}
	if advplrt.ToFloat(got) != 0 {
		t.Errorf("GETCHILDCT(oWindow) = %v, esperado 0 (sem GUI)", advplrt.ToFloat(got))
	}

	got2, err := natives["GETCHILDCT"](nil)
	if err != nil {
		t.Fatalf("GETCHILDCT(nil) retornou erro: %v", err)
	}
	if advplrt.ToFloat(got2) != -1 {
		t.Errorf("GETCHILDCT(nil) = %v, esperado -1", advplrt.ToFloat(got2))
	}
}

// TestGetResArray verifica GETRESARRAY: array (vazio — sem repositório de
// resources nesta VM), com e sem o parâmetro nRPO.
func TestGetResArray(t *testing.T) {
	_, natives := newMiscNatives()

	got, err := natives["GETRESARRAY"]([]advplrt.Value{advplrt.NewString("*.png")})
	if err != nil {
		t.Fatalf("GETRESARRAY retornou erro: %v", err)
	}
	arr, ok := got.(*advplrt.ArrayValue)
	if !ok {
		t.Fatalf("GETRESARRAY retornou %v, esperado array", got.Type())
	}
	if len(arr.Elements) != 0 {
		t.Errorf("GETRESARRAY retornou %d resources, esperado 0 (sem repositório)", len(arr.Elements))
	}

	got2, err := natives["GETRESARRAY"]([]advplrt.Value{advplrt.NewString("*.*"), advplrt.NewNumber(3)})
	if err != nil {
		t.Fatalf("GETRESARRAY(cMask,nRPO) retornou erro: %v", err)
	}
	if _, ok := got2.(*advplrt.ArrayValue); !ok {
		t.Errorf("GETRESARRAY(cMask,nRPO) retornou %v, esperado array", got2.Type())
	}
}

// TestGetPort verifica GETPORT: tipo fora de 1..4 -> -1; sem servidor, porta
// desabilitada -> -1, exceto o tipo 1 com env ADVPP_PORT.
func TestGetPort(t *testing.T) {
	_, natives := newMiscNatives()

	for _, n := range []float64{0, 5, -1} {
		got, err := natives["GETPORT"]([]advplrt.Value{advplrt.NewNumber(n)})
		if err != nil {
			t.Fatalf("GETPORT(%v) retornou erro: %v", n, err)
		}
		if advplrt.ToFloat(got) != -1 {
			t.Errorf("GETPORT(%v) = %v, esperado -1", n, advplrt.ToFloat(got))
		}
	}

	t.Setenv("ADVPP_PORT", "")
	got, _ := natives["GETPORT"]([]advplrt.Value{advplrt.NewNumber(1)})
	if advplrt.ToFloat(got) != -1 {
		t.Errorf("GETPORT(1) sem ADVPP_PORT = %v, esperado -1", advplrt.ToFloat(got))
	}

	t.Setenv("ADVPP_PORT", "443")
	got, _ = natives["GETPORT"]([]advplrt.Value{advplrt.NewNumber(1)})
	if advplrt.ToFloat(got) != 443 {
		t.Errorf("GETPORT(1) com ADVPP_PORT=443 = %v, esperado 443", advplrt.ToFloat(got))
	}

	// License (tipo 2) continua -1 mesmo com ADVPP_PORT definido.
	got, _ = natives["GETPORT"]([]advplrt.Value{advplrt.NewNumber(2)})
	if advplrt.ToFloat(got) != -1 {
		t.Errorf("GETPORT(2) = %v, esperado -1", advplrt.ToFloat(got))
	}
}

// TestMiscNativesDiscoverability garante que as 17 natives da categoria
// estão registradas no mapa de registro.
func TestMiscNativesDiscoverability(t *testing.T) {
	_, natives := newMiscNatives()
	expected := []string{
		"COPYTOCLIPBOARD", "PASTEFROMCLIPBOARD", "SHELLEXECUTE", "WINEXEC",
		"TONE", "EXECINDLLOPEN", "EXECINDLLCLOSE", "EXECINDLLRUN",
		"EXEDLLRUN2", "EXEDLLRUN3", "GETCHILDCT", "GETCLIENTDIR",
		"GETRESARRAY", "GETPORT", "PING", "ERRORBLOCK", "__DELETERMT",
	}
	for _, name := range expected {
		if _, ok := natives[name]; !ok {
			t.Errorf("native %q não registrada", name)
		}
	}
}
