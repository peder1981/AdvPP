package vm

import (
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerManipulacaodememoriaNatives registra funções de manipulação de memória remota:
// __DeleteRmt.
func (v *VM) registerManipulacaodememoriaNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// __DeleteRmt(cIdentificador) -> Nil — exclui lista com identificador de conteúdo de variáveis
	natives["__DELETRMT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cIdentificador := getArg(args, 0)
		identifier := ""
		if str, ok := cIdentificador.(*advplrt.StringValue); ok {
			identifier = str.Val
		} else {
			// Convert to string if needed
			identifier = advplrt.ToString(cIdentificador)
		}

		// Remove from remote memory storage
		delete(v.remoteMemory, identifier)

		return advplrt.Nil, nil
	}
}
