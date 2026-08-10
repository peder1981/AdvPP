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

	// Randomize(nMinimo, nMaximo) -> nRet — gera número inteiro aleatório em [nMinimo, nMinimo+32766]
	// TDN: "A função Randomize() trabalha com um intervalo interno de 32767 números, a partir do número inicial informado"
	// NOTA (TDN inconsistência): Example 2 prose ("entre 1 e 32766") conflita com regra geral + Example 3.
	// Ex1: Randomize(10,1000) → [10,999] ✓ (intervalo=990<32767)
	// Ex2: Randomize(1,34000) → TDN diz "entre 1 e 32766" mas regra geral implica "entre 1 e 32767"
	// Ex3: Randomize(-20000,25000) → "entre -20000 e 12766" ✓ (-20000+32766=12766, confirma regra geral)
	// Implementação segue a regra geral (32767 números sempre) — internamente consistente com Ex3 e abstrato TDN.
	natives["RANDOMIZE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nMinimo := int(advplrt.ToFloat(getArg(args, 0)))
		nMaximo := int(advplrt.ToFloat(getArg(args, 1)))

		// Intervalo clamped a 32767 números
		intervalo := nMaximo - nMinimo
		if intervalo > 32767 {
			intervalo = 32767
		}
		if intervalo <= 0 {
			return advplrt.NewNumber(float64(nMinimo)), nil
		}

		// Retorna nMinimo + aleatorio em [0, intervalo) => resultado em [nMinimo, nMinimo+intervalo)
		randomValue := rand.Intn(intervalo)
		return advplrt.NewNumber(float64(nMinimo + randomValue)), nil
	}
}
