package vm

import (
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerVerificacaodostiposdevariaveisNatives registers type-checking functions:
// TYPE, VALTYPE, and others for variable/value type inspection.
func (v *VM) registerVerificacaodostiposdevariaveisNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// VALTYPE(xValue) -> cTypeCode
	// Returns the type code of a value passed directly.
	// Type codes: A=Array, B=CodeBlock, C=Character, D=Date, L=Logical,
	// N=Numeric, F=Fixed Decimal, O=Object, U=Undefined (nil)
	natives["VALTYPE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(advplrt.ValType(getArg(args, 0))), nil
	}

	// TYPE(cExpr) -> cTypeCode
	// Returns the type code of an expression/variable by name.
	// Evaluates the string expression to get the actual variable/value.
	// Works with: PRIVATE/PUBLIC variables, field access, expressions.
	// Returns "U" for: Local variables, Static variables, invalid expressions, function calls.
	natives["TYPE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		expr := advplrt.ToString(getArg(args, 0))
		expr = strings.Trim(expr, " ")

		// Empty expression -> undefined
		if expr == "" {
			return advplrt.NewString("U"), nil
		}

		// Try to look up in dynEnv (PRIVATE/PUBLIC variables)
		if val, ok := v.dynEnv[expr]; ok {
			return advplrt.NewString(advplrt.ValType(val)), nil
		}

		// Try to look up as field from current alias (e.g., "A1_COD")
		if v.currentAlias != "" && v.dbEngine != nil {
			if fieldVal, err := v.dbEngine.FieldGet(expr); err == nil {
				return advplrt.NewString(advplrt.ValType(fieldVal)), nil
			}
		}

		// Variable/field not found -> undefined
		return advplrt.NewString("U"), nil
	}
}
