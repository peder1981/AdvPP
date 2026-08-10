package vm

import (
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerManipulacaodematrizNatives registra funções de manipulação de matrizes/arrays:
// ACopy, AScanX, ATail.
func (v *VM) registerManipulacaodematrizNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// ACopy(aOrigem, aDestino, [nInicio], [nCont], [nPosDestino]): copia elementos de um array para outro.
	// Retorna referência a aDestino.
	natives["ACOPY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		source, ok := getArg(args, 0).(*advplrt.ArrayValue)
		if !ok {
			return advplrt.Nil, nil
		}

		dest, ok := getArg(args, 1).(*advplrt.ArrayValue)
		if !ok {
			return advplrt.Nil, nil
		}

		// Get optional parameters
		// nInicio: starting position in source (1-based, default 1)
		nInicio := 1
		if n, ok := getArg(args, 2).(*advplrt.NumberValue); ok && int(n.Val) >= 1 {
			nInicio = int(n.Val)
		}

		// nCont: number of elements to copy (default: all from nInicio to end)
		nCont := len(source.Elements) - nInicio + 1
		if nCont < 0 {
			nCont = 0
		}
		if n, ok := getArg(args, 3).(*advplrt.NumberValue); ok && int(n.Val) >= 0 {
			if int(n.Val) < nCont {
				nCont = int(n.Val)
			}
		}

		// nPosDestino: destination position (1-based, default 1)
		nPosDestino := 1
		if n, ok := getArg(args, 4).(*advplrt.NumberValue); ok && int(n.Val) >= 1 {
			nPosDestino = int(n.Val)
		}

		// Ensure destination array has enough capacity
		requiredLen := nPosDestino - 1 + nCont
		for len(dest.Elements) < requiredLen {
			dest.Elements = append(dest.Elements, advplrt.Nil)
		}

		// Copy elements
		for i := 0; i < nCont; i++ {
			srcIdx := nInicio - 1 + i
			dstIdx := nPosDestino - 1 + i

			if srcIdx >= 0 && srcIdx < len(source.Elements) {
				dest.Elements[dstIdx] = source.Elements[srcIdx]
			}
		}

		return dest, nil
	}

	// AScanX(aDest, bSearch, [nStart], [nCount]): searches array using codeblock.
	// The codeblock receives (element, index) and should return .T. for a match.
	// Returns position of first match, or 0 if not found.
	natives["ASCANX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		a, ok := getArg(args, 0).(*advplrt.ArrayValue)
		if !ok {
			return advplrt.NewNumber(0), nil
		}

		n := len(a.Elements)
		start, count := subRange(args, 2, 3, n)

		// AScanX requires a codeblock as the second parameter
		cb, ok := getArg(args, 1).(*advplrt.CodeBlockValue)
		if !ok {
			return advplrt.NewNumber(0), nil
		}

		// Scan the array using the codeblock
		// The codeblock is called with (element, index)
		for i := start; i <= start+count-1; i++ {
			r, err := v.callBlockSync(cb, a.Elements[i-1], advplrt.NewNumber(float64(i)))
			if err != nil {
				return advplrt.Nil, err
			}
			if r.IsTruthy() {
				return advplrt.NewNumber(float64(i)), nil
			}
		}

		return advplrt.NewNumber(0), nil
	}

	// ATail(aArray): returns the last element of an array.
	// Returns Nil if array is empty or argument is not an array.
	natives["ATAIL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		a, ok := getArg(args, 0).(*advplrt.ArrayValue)
		if !ok {
			return advplrt.Nil, nil
		}

		if len(a.Elements) == 0 {
			return advplrt.Nil, nil
		}

		return a.Elements[len(a.Elements)-1], nil
	}
}
