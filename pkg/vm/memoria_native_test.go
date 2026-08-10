package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// TestDeleteRmt verifica que __DeleteRmt remove uma entrada do armazenamento remoto
func TestDeleteRmt(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Simulamos um armazenamento prévio (como se __SaveRmt tivesse sido chamado)
	v.remoteMemory["myId1"] = []advplrt.Value{
		advplrt.NewString("var1"),
		advplrt.NewNumber(2),
		advplrt.NewBool(true),
	}
	v.remoteMemory["myId2"] = []advplrt.Value{
		advplrt.NewString("var1"),
		advplrt.NewNumber(2),
		advplrt.NewBool(true),
	}

	// Deletar myId1
	result, err := v.natives["__DELETRMT"].Fn([]advplrt.Value{advplrt.NewString("myId1")})
	if err != nil {
		t.Fatalf("__DeleteRmt retornou erro: %v", err)
	}

	// Verificar que retorna Nil
	if result != advplrt.Nil {
		t.Errorf("__DeleteRmt retornou %v, quer advplrt.Nil", result)
	}

	// Verificar que myId1 foi removido
	if _, exists := v.remoteMemory["myId1"]; exists {
		t.Errorf("__DeleteRmt não removeu 'myId1' do armazenamento remoto")
	}

	// Verificar que myId2 ainda existe
	if _, exists := v.remoteMemory["myId2"]; !exists {
		t.Errorf("__DeleteRmt removeu 'myId2' por engano")
	}
}

// TestDeleteRmtNonexistent verifica que __DeleteRmt funciona mesmo com identifier inexistente
func TestDeleteRmtNonexistent(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Tentar deletar algo que não existe
	result, err := v.natives["__DELETRMT"].Fn([]advplrt.Value{advplrt.NewString("nonexistent")})
	if err != nil {
		t.Fatalf("__DeleteRmt retornou erro: %v", err)
	}

	// Verificar que retorna Nil (sem erro)
	if result != advplrt.Nil {
		t.Errorf("__DeleteRmt retornou %v, quer advplrt.Nil", result)
	}
}
