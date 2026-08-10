package vm

import (
	"math/rand"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerManipulacaodevariaveisnumericasNatives registra funções de manipulação de
// variáveis numéricas: NAnd, NOr, NXor, Randomize.
func (v *VM) registerManipulacaodevariaveisnumericasNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// NAnd(nNum1, nNum2, [nNumN]...) -> nRet — operação binária E (AND)
	natives["NAND"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if len(args) == 0 {
			return advplrt.NewNumber(0), nil
		}
		result := int32(advplrt.ToFloat(getArg(args, 0)))
		for _, arg := range args[1:] {
			result = result & int32(advplrt.ToFloat(arg))
		}
		return advplrt.NewNumber(float64(result)), nil
	}

	// NOr(nNum1, nNum2, [nNumN]...) -> nRet — operação binária OU (OR)
	natives["NOR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if len(args) == 0 {
			return advplrt.NewNumber(0), nil
		}
		result := int32(advplrt.ToFloat(getArg(args, 0)))
		for _, arg := range args[1:] {
			result = result | int32(advplrt.ToFloat(arg))
		}
		return advplrt.NewNumber(float64(result)), nil
	}

	// NXor(nNum1, nNum2, [nNumN]...) -> nRet — operação binária XOR
	natives["NXOR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if len(args) == 0 {
			return advplrt.NewNumber(0), nil
		}
		result := int32(advplrt.ToFloat(getArg(args, 0)))
		for _, arg := range args[1:] {
			result = result ^ int32(advplrt.ToFloat(arg))
		}
		return advplrt.NewNumber(float64(result)), nil
	}

	// Randomize(nMinimo, nMaximo) -> nRet — gera número inteiro aleatório em [nMinimo, nMaximo)
	// O intervalo interno é limitado a 32767 números
	natives["RANDOMIZE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nMinimo := int(advplrt.ToFloat(getArg(args, 0)))
		nMaximo := int(advplrt.ToFloat(getArg(args, 1)))

		// O intervalo total é limitado a 32767 números
		// Se nMaximo - nMinimo > 32767, o intervalo efetivo é nMinimo + 32767
		intervalo := nMaximo - nMinimo
		if intervalo > 32767 {
			intervalo = 32767
		}
		if intervalo <= 0 {
			return advplrt.NewNumber(float64(nMinimo)), nil
		}

		// Gera número aleatório e retorna nMinimo + aleatorio(0, intervalo)
		randomValue := rand.Intn(intervalo)
		return advplrt.NewNumber(float64(nMinimo + randomValue)), nil
	}
}
