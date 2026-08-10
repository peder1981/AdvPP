package vm

import (
	"math"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerMatematicaNatives registra funções matemáticas de escalar:
// Ceiling, Exp, Log, Log10, Mod, Sqrt, ACos, ASin, ATan, Atn2, Cos, Sin, Tan.
func (v *VM) registerMatematicaNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// Ceiling(nValor) -> nRet — calcula o arredondamento (para cima) do valor de ponto flutuante
	// Retorna o menor inteiro que é maior ou igual ao valor
	natives["CEILING"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Ceil(advplrt.ToFloat(getArg(args, 0)))), nil
	}

	// Exp(nValor) -> nRet — calcula o valor de e elevado à potência nValor
	// Retorna a exponencial do número
	natives["EXP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Exp(advplrt.ToFloat(getArg(args, 0)))), nil
	}

	// Log(nValor) -> nRet — calcula o logaritmo natural (base e) do valor
	// Retorna o logaritmo natural
	natives["LOG"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Log(advplrt.ToFloat(getArg(args, 0)))), nil
	}

	// Log10(nValor) -> nRet — calcula o logaritmo de base 10 do valor
	// Retorna o logaritmo de base 10
	natives["LOG10"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Log10(advplrt.ToFloat(getArg(args, 0)))), nil
	}

	// Mod(nDividendo, nDivisor) -> nRet — calcula o resto da divisão
	// Retorna o resto da divisão de nDividendo por nDivisor
	natives["MOD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		a := advplrt.ToFloat(getArg(args, 0))
		b := advplrt.ToFloat(getArg(args, 1))
		if b == 0 {
			return advplrt.NewNumber(0), nil
		}
		return advplrt.NewNumber(math.Mod(a, b)), nil
	}

	// Sqrt(nValor) -> nRet — calcula a raiz quadrada do valor
	// Retorna a raiz quadrada positiva
	natives["SQRT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Sqrt(advplrt.ToFloat(getArg(args, 0)))), nil
	}

	// ACos(nValor) -> nRet — calcula o arcocosseno do valor (em radianos)
	// Retorna um valor entre 0 e PI radianos
	natives["ACOS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Acos(advplrt.ToFloat(getArg(args, 0)))), nil
	}

	// ASin(nValor) -> nRet — calcula o arcosseno do valor (em radianos)
	// Retorna um valor entre -PI/2 e PI/2 radianos
	natives["ASIN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Asin(advplrt.ToFloat(getArg(args, 0)))), nil
	}

	// ATan(nValor) -> nRet — calcula o arcotangente do valor (em radianos)
	// Retorna um valor entre -PI/2 e PI/2 radianos
	natives["ATAN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Atan(advplrt.ToFloat(getArg(args, 0)))), nil
	}

	// Atn2(nSin, nCos) -> nRet — calcula o ângulo cujos seno e cosseno são dados (em radianos)
	// Retorna um valor entre 0 e PI radianos
	natives["ATN2"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Atan2(advplrt.ToFloat(getArg(args, 0)), advplrt.ToFloat(getArg(args, 1)))), nil
	}

	// Cos(nAngulo) -> nRet — calcula o cosseno do ângulo (em radianos)
	// Retorna um valor entre -1 e 1
	natives["COS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Cos(advplrt.ToFloat(getArg(args, 0)))), nil
	}

	// Sin(nAngulo) -> nRet — calcula o seno do ângulo (em radianos)
	// Retorna um valor entre -1 e 1
	natives["SIN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Sin(advplrt.ToFloat(getArg(args, 0)))), nil
	}

	// Tan(nAngulo) -> nRet — calcula a tangente do ângulo (em radianos)
	// Retorna um valor numérico
	natives["TAN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Tan(advplrt.ToFloat(getArg(args, 0)))), nil
	}
}
