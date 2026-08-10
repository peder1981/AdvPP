package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// TestGetPortActive verifica a única função com spec real da categoria
// Controle-de-impressao: GetPortActive(lDirect) retorna array de portas
// disponíveis; sem portas, array vazio (comportamento das builds >7.00.111010P).
func TestGetPortActive(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Retorna sempre um array (portas reais do SO ou vazio).
	got, err := v.natives["GETPORTACTIVE"].Fn([]advplrt.Value{advplrt.True})
	if err != nil {
		t.Fatalf("GetPortActive(.T.) retornou erro: %v", err)
	}
	arr, ok := got.(*advplrt.ArrayValue)
	if !ok {
		t.Fatalf("GetPortActive(.T.) retornou %v, esperado *ArrayValue", got.Type())
	}
	// Cada elemento (se houver) deve ser string.
	for _, el := range arr.Elements {
		if el.Type() != "C" {
			t.Errorf("GetPortActive() elemento = %v, esperado string", el.Type())
		}
	}

	// lDirect=.F. (portas do Smart Client) — mesma semântica no runtime
	// embutido (sem AppServer/SmartClient distintos): array, possivelmente vazio.
	got2, err := v.natives["GETPORTACTIVE"].Fn([]advplrt.Value{advplrt.False})
	if err != nil {
		t.Fatalf("GetPortActive(.F.) retornou erro: %v", err)
	}
	if _, ok := got2.(*advplrt.ArrayValue); !ok {
		t.Fatalf("GetPortActive(.F.) retornou %v, esperado *ArrayValue", got2.Type())
	}

	// Sem argumento: default .T. (servidor) — deve retornar array também.
	got3, err := v.natives["GETPORTACTIVE"].Fn(nil)
	if err != nil {
		t.Fatalf("GetPortActive() sem args retornou erro: %v", err)
	}
	if _, ok := got3.(*advplrt.ArrayValue); !ok {
		t.Fatalf("GetPortActive() sem args retornou %v, esperado *ArrayValue", got3.Type())
	}
}
