package vm

import (
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerManipulacaodevariaveisglobaisNatives registra funções de manipulação de variáveis globais:
// GetGlbValue, GetGlbVars, PutGlbValue, PutGlbVars.
func (v *VM) registerManipulacaodevariaveisglobaisNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// GetGlbValue(cGlbName) -> cValue — retorna o valor string de uma variável global
	// Retorna vazio se a variável não for encontrada
	natives["GETGLBVALUE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cGlbName := advplrt.ToString(getArg(args, 0))

		v.globalVarsSingleMu.Lock()
		defer v.globalVarsSingleMu.Unlock()

		if val, exists := v.globalVarsSingle[cGlbName]; exists {
			return advplrt.NewString(val), nil
		}
		return advplrt.NewString(""), nil
	}

	// PutGlbValue(cGlbName, cValue) -> Nil — armazena um valor string em uma variável global
	natives["PUTGLBVALUE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cGlbName := advplrt.ToString(getArg(args, 0))
		cValue := advplrt.ToString(getArg(args, 1))

		v.globalVarsSingleMu.Lock()
		defer v.globalVarsSingleMu.Unlock()

		v.globalVarsSingle[cGlbName] = cValue
		return advplrt.Nil, nil
	}

	// GetGlbVars(cGlbName, @xValue1...N) -> lRet — retorna múltiplos valores de uma variável global
	// Retorna .T. se encontrado, .F. caso contrário
	natives["GETGLBVARS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cGlbName := advplrt.ToString(getArg(args, 0))

		v.globalVarsMultiMu.Lock()
		defer v.globalVarsMultiMu.Unlock()

		values, exists := v.globalVarsMulti[cGlbName]
		if !exists {
			return advplrt.NewBool(false), nil
		}

		// LIMITAÇÃO: AdvPP não implementa suporte a argumentos por referência
		// para funções nativas. A TDN especifica que GetGlbVars deve permitir
		// recuperar até N argumentos por referência (via @xValue1...N) que seriam
		// mutados com os valores armazenados. Isso é uma limitação arquitetural
		// do VM que afeta potencialmente várias outras funções.
		//
		// Comportamento atual: a função retorna .T. corretamente (valores encontrados),
		// mas os dados armazenados não são refletidos nas variáveis do caller.
		// Para usar GetGlbVars corretamente nesta implementação, seria necessário
		// repensar o mecanismo de chamada de nativas ou usar workarounds
		// (ex.: armazenar valores em variáveis globais acessíveis, usar retorno estruturado).
		//
		_ = values // valores recuperados, mas não podem ser passados ao caller via @params

		return advplrt.NewBool(true), nil
	}

	// PutGlbVars(cGlbName, xValue1...N) -> Nil — armazena múltiplos valores em uma variável global
	// Valores do tipo codeblock ou objeto são convertidos para NIL
	natives["PUTGLBVARS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cGlbName := advplrt.ToString(getArg(args, 0))

		// Collect all variadic values (starting from index 1)
		values := []advplrt.Value{}
		for i := 1; i < len(args); i++ {
			val := getArg(args, i)
			// Convert unsupported types (codeblock and object) to NIL
			if isCodeBlockOrObject(val) {
				val = advplrt.Nil
			}
			values = append(values, val)
		}

		v.globalVarsMultiMu.Lock()
		defer v.globalVarsMultiMu.Unlock()

		v.globalVarsMulti[cGlbName] = values
		return advplrt.Nil, nil
	}
}

// isCodeBlockOrObject checks if a value is a codeblock or object
// These types are not supported in PutGlbVars and should be converted to NIL
func isCodeBlockOrObject(val advplrt.Value) bool {
	if val == nil || val == advplrt.Nil {
		return false
	}
	// Check for codeblock
	if _, ok := val.(*advplrt.CodeBlockValue); ok {
		return true
	}
	// Check for object (any other complex type not in the basic set)
	// In AdvPL, objects are stored as various types but we can check by exclusion
	switch val.(type) {
	case *advplrt.StringValue, *advplrt.NumberValue, *advplrt.BoolValue,
		*advplrt.DateValue, *advplrt.ArrayValue, *advplrt.NilValue, *advplrt.ErrorValue:
		return false
	default:
		// Unknown type might be an object - convert to NIL
		return true
	}
}
