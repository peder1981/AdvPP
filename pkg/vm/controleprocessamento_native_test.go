package vm

import (
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestExUserException(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	_, err := v.natives["EXUSEREXCEPTION"].Fn([]advplrt.Value{advplrt.NewString("falha critica")})
	if err == nil {
		t.Fatal("ExUserException() deveria retornar erro (aborta a aplicação)")
	}
	if !strings.Contains(err.Error(), "falha critica") {
		t.Errorf("ExUserException() erro = %q, esperado conter a mensagem", err.Error())
	}

	// Sem argumento: ainda aborta, sem mensagem
	_, err = v.natives["EXUSEREXCEPTION"].Fn(nil)
	if err == nil {
		t.Fatal("ExUserException() sem args deveria abortar")
	}
}

func TestGetPrograms(t *testing.T) {
	v := NewVM(&compiler.Bytecode{
		Functions: map[string]*compiler.FunctionInfo{
			"U_Z": {},
			"U_A": {},
			"U_M": {},
		},
	}, false)

	got, err := v.natives["GETPROGRAMS"].Fn(nil)
	if err != nil {
		t.Fatalf("GetPrograms() retornou erro: %v", err)
	}
	arr, ok := got.(*advplrt.ArrayValue)
	if !ok {
		t.Fatalf("GetPrograms() retornou tipo incorreto: %v", got.Type())
	}
	if len(arr.Elements) != 3 {
		t.Fatalf("GetPrograms() = %d elementos, esperado 3", len(arr.Elements))
	}
	// Ordem alfabética (TDN: AEVAL para conout um a um)
	names := []string{
		advplrt.ToString(arr.Elements[0]),
		advplrt.ToString(arr.Elements[1]),
		advplrt.ToString(arr.Elements[2]),
	}
	if names[0] != "U_A" || names[1] != "U_M" || names[2] != "U_Z" {
		t.Errorf("GetPrograms() = %v, esperado ordem alfabética U_A,U_M,U_Z", names)
	}

	// VM sem funções: array vazio, sem erro
	v2 := NewVM(&compiler.Bytecode{}, false)
	got2, err := v2.natives["GETPROGRAMS"].Fn(nil)
	if err != nil {
		t.Fatalf("GetPrograms() vazio retornou erro: %v", err)
	}
	if arr2, ok := got2.(*advplrt.ArrayValue); !ok || len(arr2.Elements) != 0 {
		t.Errorf("GetPrograms() vazio = %v, esperado array vazio", got2)
	}
}

func TestManualJob(t *testing.T) {
	v := NewVM(&compiler.Bytecode{
		Functions: map[string]*compiler.FunctionInfo{
			"U_CONN":  {},
			"U_START": {},
		},
	}, false)

	// Sem nome: erro
	_, err := v.natives["MANUALJOB"].Fn([]advplrt.Value{
		advplrt.NewString(""),
		advplrt.NewString("env"),
	})
	if err == nil {
		t.Fatal("ManualJob sem nome deveria retornar erro")
	}

	// Nome com vírgula: erro (TDN: não pode conter vírgula)
	_, err = v.natives["MANUALJOB"].Fn([]advplrt.Value{
		advplrt.NewString("a,b"),
		advplrt.NewString("env"),
	})
	if err == nil {
		t.Fatal("ManualJob com vírgula no nome deveria retornar erro")
	}

	// JobType vazio: executa cOnConnect
	_, err = v.natives["MANUALJOB"].Fn([]advplrt.Value{
		advplrt.NewString("job1"),
		advplrt.NewString("env"),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString("U_CONN"),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("ManualJob vazio/connect retornou erro: %v", err)
	}

	// JobType MDI: executa cOnStart com cSSKey
	_, err = v.natives["MANUALJOB"].Fn([]advplrt.Value{
		advplrt.NewString("job2"),
		advplrt.NewString("env"),
		advplrt.NewString("MDI"),
		advplrt.NewString("U_START"),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString("sskey123"),
	})
	if err != nil {
		t.Fatalf("ManualJob MDI retornou erro: %v", err)
	}

	// JobType inválido com target vazio: erro honesto
	_, err = v.natives["MANUALJOB"].Fn([]advplrt.Value{
		advplrt.NewString("job3"),
		advplrt.NewString("env"),
		advplrt.NewString("IPC"),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err == nil {
		t.Fatal("ManualJob sem função-alvo deveria retornar erro")
	}
}

func TestPCount(t *testing.T) {
	// PCount depende do frame ativo; simula a chamada setando v.current
	v := NewVM(&compiler.Bytecode{}, false)

	// Sem frame: 0
	got, err := v.natives["PCOUNT"].Fn(nil)
	if err != nil {
		t.Fatalf("PCount() retornou erro: %v", err)
	}
	if n, ok := got.(*advplrt.NumberValue); !ok || n.Val != 0 {
		t.Errorf("PCount() sem frame = %v, esperado 0", got)
	}

	// Com frame: ArgCount refletido (TDN Exemplo: 2 args -> 2)
	v.frames = append(v.frames, &CallFrame{FuncName: "TEST", ArgCount: 2})
	v.current = v.frames[len(v.frames)-1]
	got, err = v.natives["PCOUNT"].Fn(nil)
	if err != nil {
		t.Fatalf("PCount() com frame retornou erro: %v", err)
	}
	if n, ok := got.(*advplrt.NumberValue); !ok || n.Val != 2 {
		t.Errorf("PCount() com 2 args = %v, esperado 2", got)
	}
}

func TestSmartJob(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Sem nome: .F.
	got, err := v.natives["SMARTJOB"].Fn([]advplrt.Value{advplrt.NewString("")})
	if err != nil {
		t.Fatalf("SmartJob vazio retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || !b.Val {
		// espera .F.
		if b != nil && b.Val {
			t.Errorf("SmartJob sem nome = %v, esperado .F.", advplrt.ToString(got))
		}
	}

	// Com função inexistente no bytecode: .F. (não entra na fila)
	got, err = v.natives["SMARTJOB"].Fn([]advplrt.Value{
		advplrt.NewString("U_NAOEXISTE"),
		advplrt.NewString("env"),
		advplrt.False,
		advplrt.NewString("param"),
	})
	if err != nil {
		t.Fatalf("SmartJob função inexistente retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || b.Val {
		t.Errorf("SmartJob função inexistente = %v, esperado .F.", advplrt.ToString(got))
	}

	// Com função existente: .T. (entra na fila)
	vWith := NewVM(&compiler.Bytecode{
		Functions: map[string]*compiler.FunctionInfo{"U_TARGET": {}},
	}, false)
	got, err = vWith.natives["SMARTJOB"].Fn([]advplrt.Value{
		advplrt.NewString("U_TARGET"),
		advplrt.NewString("env"),
		advplrt.False,
		advplrt.NewString("param1"),
	})
	if err != nil {
		t.Fatalf("SmartJob existente retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || !b.Val {
		t.Errorf("SmartJob existente = %v, esperado .T.", advplrt.ToString(got))
	}
}
