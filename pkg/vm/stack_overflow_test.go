package vm

import (
	"strings"
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
)

// TestStackOverflowIsVisible garante que um estouro da pilha de operandos
// aborta o runLoop com erro em vez de travar em silêncio.
//
// O bytecode empilha um Nil e salta de volta para si mesmo — um push sem
// fim. Antes do endurecimento, push retornava erro mas todos os chamadores
// o ignoravam, então a VM girava para sempre sobre a pilha cheia. Agora
// push marca v.fault e o runLoop devolve o erro.
func TestStackOverflowIsVisible(t *testing.T) {
	bc := &compiler.Bytecode{
		Constants:  []compiler.Constant{},
		Functions:  map[string]*compiler.FunctionInfo{},
		Classes:    map[string]*compiler.ClassInfo{},
		Code:       []compiler.Instruction{{Op: compiler.OP_NIL}, {Op: compiler.OP_JUMP, Arg: 0}},
		MainOffset: 0,
	}
	v := NewVM(bc, false)

	done := make(chan error, 1)
	go func() { _, err := v.Run(); done <- err }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("esperava erro de stack overflow, veio nil")
		}
		if !strings.Contains(err.Error(), "stack overflow") {
			t.Fatalf("erro inesperado: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("VM travou em vez de abortar por stack overflow")
	}
}
