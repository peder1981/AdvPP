package vm

import (
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerValidacaoNatives registra funções de validação de valores:
// AllwaysFalse, AllwaysTrue, Empty.
func (v *VM) registerValidacaoNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// AllwaysFalse() -> lFalse
	natives["ALLWAYSFALSE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewBool(false), nil
	}

	// AllwaysTrue() -> lTrue
	natives["ALLWAYSTRUE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewBool(true), nil
	}

	// Empty(xVal) -> lEmpty — verdadeiro se xVal é o valor "vazio" do seu tipo
	// (string em branco, 0, .F., data em branco, array/objeto sem elementos, nil).
	natives["EMPTY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		val := getArg(args, 0)
		return advplrt.NewBool(isEmptyValue(val)), nil
	}
}

func isEmptyValue(val advplrt.Value) bool {
	if val == nil || val == advplrt.Nil {
		return true
	}
	switch t := val.(type) {
	case *advplrt.StringValue:
		return len(strings.TrimSpace(t.Val)) == 0
	case *advplrt.NumberValue:
		return t.Val == 0
	case *advplrt.BoolValue:
		return !t.Val
	case *advplrt.DateValue:
		return t.Val.IsZero()
	case *advplrt.ArrayValue:
		return len(t.Elements) == 0
	case *advplrt.CodeBlockValue:
		return false
	default:
		return false
	}
}
