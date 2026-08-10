package vm

import (
	"os"
	"os/user"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestComputerName(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	result, err := v.natives["COMPUTERNAME"].Fn([]advplrt.Value{})
	if err != nil {
		t.Fatalf("ComputerName retornou erro: %v", err)
	}

	// Deve retornar uma string com o hostname
	str, ok := result.(*advplrt.StringValue)
	if !ok {
		t.Errorf("ComputerName retornou tipo %T, esperado *StringValue", result)
	}
	if str.Val == "" {
		t.Errorf("ComputerName retornou string vazia, esperado hostname")
	}

	// Validar que é um hostname válido (obtém do Go)
	hostname, _ := os.Hostname()
	if str.Val != hostname {
		t.Errorf("ComputerName = %q, esperado %q", str.Val, hostname)
	}
}

func TestLogUserName(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	result, err := v.natives["LOGUSERNAME"].Fn([]advplrt.Value{})
	if err != nil {
		t.Fatalf("LogUserName retornou erro: %v", err)
	}

	// Deve retornar uma string com o nome do usuário
	str, ok := result.(*advplrt.StringValue)
	if !ok {
		t.Errorf("LogUserName retornou tipo %T, esperado *StringValue", result)
	}
	if str.Val == "" {
		t.Errorf("LogUserName retornou string vazia, esperado username")
	}

	// Validar que é o usuário atual
	currentUser, _ := user.Current()
	if currentUser != nil && str.Val != currentUser.Username {
		t.Errorf("LogUserName = %q, esperado %q", str.Val, currentUser.Username)
	}
}

func TestGetAuthArgs(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	result, err := v.natives["GETAUTHARGS"].Fn([]advplrt.Value{})
	if err != nil {
		t.Fatalf("GetAuthArgs retornou erro: %v", err)
	}

	// Deve retornar um objeto (ObjectValue)
	obj, ok := result.(*advplrt.ObjectValue)
	if !ok {
		t.Errorf("GetAuthArgs retornou tipo %T, esperado *ObjectValue", result)
	}

	// Validar que é uma instância de THASMAP
	if obj.ClassName != "THASMAP" {
		t.Errorf("GetAuthArgs ClassName = %q, esperado THASMAP", obj.ClassName)
	}

	// Deve ter Props (possivelmente vazio em contexto de teste)
	if obj.Props == nil {
		t.Errorf("GetAuthArgs Props é nil, esperado map")
	}
}

func TestADUserValidArgumentValidation(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Testa com 3 argumentos (deve validar)
	result, err := v.natives["ADUSERSVALID"].Fn([]advplrt.Value{
		advplrt.NewString("DOMAIN"),
		advplrt.NewString("user"),
		advplrt.NewString("password"),
	})
	if err != nil {
		t.Fatalf("ADUserValid retornou erro: %v", err)
	}

	// Deve retornar um booleano (false em contexto sem AD real)
	b, ok := result.(*advplrt.BoolValue)
	if !ok {
		t.Errorf("ADUserValid retornou tipo %T, esperado *BoolValue", result)
	}
	if b.Val {
		t.Errorf("ADUserValid = true, esperado false (AD não disponível em teste)")
	}
}

func TestADUserValidWithEmptyArgs(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Testa com strings vazias
	result, err := v.natives["ADUSERSVALID"].Fn([]advplrt.Value{
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("ADUserValid com args vazios retornou erro: %v", err)
	}

	b, ok := result.(*advplrt.BoolValue)
	if !ok {
		t.Errorf("ADUserValid retornou tipo %T, esperado *BoolValue", result)
	}
	if b.Val {
		t.Errorf("ADUserValid com args vazios = true, esperado false")
	}
}
