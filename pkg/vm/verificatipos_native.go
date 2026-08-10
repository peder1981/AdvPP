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
	// Works with: PRIVATE/PUBLIC variables, field access from current alias.
	// Returns "U" for: Local variables, Static variables, invalid expressions, function calls.
	//
	// LIMITATION: Not implemented per TDN spec:
	//   - Alias-qualified field access (e.g., Type("SA1->A1_COD")) — requires macro expansion
	//   - Arbitrary AdvPL expression evaluation (e.g., Type("1+2") → "N") — requires macro expansion
	//   - Function-call detection (should return "UI" per spec, currently returns "U") — no access to parser
	// These require a macro/expression evaluator not available in this native context.
	natives["TYPE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		expr := advplrt.ToString(getArg(args, 0))
		expr = strings.Trim(expr, " ")

		// Empty expression -> undefined
		if expr == "" {
			return advplrt.NewString("U"), nil
		}

		// Lookup precedence: field SHADOWS memvar (matches real AdvPL/Clipper semantics).
		// Check field from current alias FIRST (e.g., "A1_COD").
		// In real AdvPL, fields take priority over same-named PRIVATE/PUBLIC variables
		// unless prefixed with M->. This implementation maintains that priority.
		if v.currentAlias != "" && v.dbEngine != nil {
			if fieldVal, err := v.dbEngine.FieldGet(expr); err == nil {
				return advplrt.NewString(advplrt.ValType(fieldVal)), nil
			}
		}

		// Fall back to lookup in dynEnv (PRIVATE/PUBLIC variables)
		if val, ok := v.dynEnv[expr]; ok {
			return advplrt.NewString(advplrt.ValType(val)), nil
		}

		// Variable/field not found -> undefined
		return advplrt.NewString("U"), nil
	}
}
