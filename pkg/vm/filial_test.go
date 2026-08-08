package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestRpcSetEnvGravaFilialAtiva(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	_, err := v.natives["RPCSETENV"].Fn([]advplrt.Value{advplrt.NewString("010101")})
	if err != nil {
		t.Fatalf("RpcSetEnv retornou erro: %v", err)
	}
	if v.filialAtiva != "010101" {
		t.Errorf("filialAtiva = %q, quer %q", v.filialAtiva, "010101")
	}
}
