package vm

import (
	"fmt"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerManipulacaodoblocodecodigoNatives registra funções de manipulação de bloco de código:
// AEVal, DBEVal, Eval, GetCbSource.
func (v *VM) registerManipulacaodoblocodecodigoNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// AEval(aArray, bBlock, [nStart], [nCount]) -> aRet (array)
	// Executa um bloco de código para cada elemento de um array.
	// Retorna uma cópia do array após a operação.
	// O codeblock recebe: (nValor, nIndice) onde nValor é o elemento e nIndice é a posição (1-based).
	natives["AEVAL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		// Parâmetro 1: array
		arrayVal := getArg(args, 0)
		array, ok := arrayVal.(*advplrt.ArrayValue)
		if !ok {
			return advplrt.Nil, fmt.Errorf("AEval: primeiro argumento deve ser um array")
		}

		// Parâmetro 2: codeblock
		blockVal := getArg(args, 1)
		block, ok := blockVal.(*advplrt.CodeBlockValue)
		if !ok {
			return advplrt.Nil, fmt.Errorf("AEval: segundo argumento deve ser um bloco de código")
		}

		// Parâmetro 3: nStart (opcional, padrão = 1)
		nStart := 1
		if startVal := getArg(args, 2); startVal != nil && startVal != advplrt.Nil {
			nStart = int(advplrt.ToFloat(startVal))
			if nStart < 1 {
				nStart = 1
			}
		}

		// Parâmetro 4: nCount (opcional, padrão = todos os elementos a partir de nStart)
		nCount := len(array.Elements) - nStart + 1
		if countVal := getArg(args, 3); countVal != nil && countVal != advplrt.Nil {
			nCount = int(advplrt.ToFloat(countVal))
			if nCount < 0 {
				nCount = 0
			}
		}

		// Calcula o índice final (1-based)
		endIdx := nStart + nCount - 1
		if endIdx > len(array.Elements) {
			endIdx = len(array.Elements)
		}

		// Itera sobre os elementos e executa o codeblock
		// Índices em AdvPL são 1-based, mas o array em Go é 0-based
		for i := nStart - 1; i < endIdx; i++ {
			if i < 0 || i >= len(array.Elements) {
				break
			}
			// Executa o codeblock com (elemento, índice)
			// O índice passado para o codeblock é 1-based
			_, _ = v.evalBlock(block, array.Elements[i], advplrt.NewNumber(float64(i+1)))
		}

		// Retorna uma cópia do array
		return advplrt.NewArray(array.Elements), nil
	}

	// DBEval(bBlock, [bFirstCondition], [bSecondCondition], [nCount], [nRecno], [lRest]) -> nil
	// Avalia um bloco de código para cada registro que atenda um escopo definido.
	// Sintaxe TDN:
	//   bBlock: código a executar por registro
	//   bFirstCondition: bloco de código (condição de inclusão - primeira)
	//   bSecondCondition: bloco de código (condição de inclusão - segunda)
	//   nCount: número máximo de registros a processar
	//   nRecno: processa apenas um registro específico
	//   lRest: processa registros restantes (a partir da posição atual)
	natives["DBEVAL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		// Parâmetro 1: codeblock (obrigatório)
		blockVal := getArg(args, 0)
		block, ok := blockVal.(*advplrt.CodeBlockValue)
		if !ok {
			return advplrt.Nil, fmt.Errorf("DBEval: primeiro argumento deve ser um bloco de código")
		}

		// Se não há engine de banco, não há registros para processar
		// Caso legítimo: nenhuma área de banco aberta
		if v.dbEngine == nil {
			return advplrt.Nil, nil
		}

		// Parâmetro 2: bFirstCondition (opcional)
		bFirstCondition := getArg(args, 1)

		// Parâmetro 3: bSecondCondition (opcional)
		bSecondCondition := getArg(args, 2)

		// Parâmetro 4: nCount (opcional, máximo de registros)
		nCount := 0 // 0 significa ilimitado
		if countVal := getArg(args, 3); countVal != nil && countVal != advplrt.Nil {
			nCount = int(advplrt.ToFloat(countVal))
		}

		// Parâmetro 5: nRecno (opcional, um registro específico)
		nRecno := 0 // 0 significa não aplicável
		if recnoVal := getArg(args, 4); recnoVal != nil && recnoVal != advplrt.Nil {
			nRecno = int(advplrt.ToFloat(recnoVal))
		}

		// Parâmetro 6: lRest (opcional, processar registros restantes)
		lRest := false
		if restVal := getArg(args, 5); restVal != nil && restVal != advplrt.Nil {
			lRest = advplrt.ToBool(restVal)
		}

		// Se nRecno foi especificado, processa apenas esse registro
		if nRecno > 0 {
			v.dbEngine.GoTop()
			if v.dbEngine.RecNo() == nRecno && !v.dbEngine.EOF() {
				// Avalia primeira condição
				if bFirstCondition != nil && bFirstCondition != advplrt.Nil {
					if fb, ok := bFirstCondition.(*advplrt.CodeBlockValue); ok {
						result, _ := v.RunFunction(fb.FuncName, []advplrt.Value{fb})
						if !advplrt.ToBool(result) {
							return advplrt.Nil, nil
						}
					}
				}

				// Avalia segunda condição
				if bSecondCondition != nil && bSecondCondition != advplrt.Nil {
					if sb, ok := bSecondCondition.(*advplrt.CodeBlockValue); ok {
						result, _ := v.RunFunction(sb.FuncName, []advplrt.Value{sb})
						if !advplrt.ToBool(result) {
							return advplrt.Nil, nil
						}
					}
				}

				// Executa o bloco principal para este registro
				_, _ = v.RunFunction(block.FuncName, []advplrt.Value{block})
			}
			return advplrt.Nil, nil
		}

		// Se lRest for true, comece a partir do registro atual
		// Caso contrário, comece do topo
		if !lRest {
			v.dbEngine.GoTop()
		}

		// Itera sobre os registros (com limite de segurança para evitar loops infinitos)
		processed := 0
		maxSafeIterations := 100000 // proteção contra loops infinitos
		safeIterations := 0

		for !v.dbEngine.EOF() && safeIterations < maxSafeIterations {
			safeIterations++

			// Respeita nCount (limite máximo de registros)
			if nCount > 0 && processed >= nCount {
				break
			}

			// Avalia primeira condição
			if bFirstCondition != nil && bFirstCondition != advplrt.Nil {
				if fb, ok := bFirstCondition.(*advplrt.CodeBlockValue); ok {
					result, _ := v.RunFunction(fb.FuncName, []advplrt.Value{fb})
					if !advplrt.ToBool(result) {
						v.dbEngine.Skip(1)
						continue
					}
				}
			}

			// Avalia segunda condição
			if bSecondCondition != nil && bSecondCondition != advplrt.Nil {
				if sb, ok := bSecondCondition.(*advplrt.CodeBlockValue); ok {
					result, _ := v.RunFunction(sb.FuncName, []advplrt.Value{sb})
					if !advplrt.ToBool(result) {
						v.dbEngine.Skip(1)
						continue
					}
				}
			}

			// Executa o bloco principal para este registro
			_, _ = v.RunFunction(block.FuncName, []advplrt.Value{block})
			processed++

			// Move para o próximo registro
			v.dbEngine.Skip(1)
		}

		// Sempre retorna nil como especificado no TDN
		return advplrt.Nil, nil
	}

	// Eval(bBloco, [xVariavel]) -> xRetorno (qualquer)
	// Executa um bloco de código.
	// Retorna o valor da última expressão do bloco de código.
	natives["EVAL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		// Parâmetro 1: codeblock
		blockVal := getArg(args, 0)
		block, ok := blockVal.(*advplrt.CodeBlockValue)
		if !ok {
			return advplrt.Nil, fmt.Errorf("Eval: primeiro argumento deve ser um bloco de código")
		}

		// Parâmetro 2: xVariável (opcional)
		var arg advplrt.Value = advplrt.Nil
		if varVal := getArg(args, 1); varVal != nil && varVal != advplrt.Nil {
			arg = varVal
		}

		// Executa o codeblock com o argumento (se fornecido)
		result, err := v.evalBlock(block, arg)
		if err != nil {
			return advplrt.Nil, fmt.Errorf("Eval: erro ao executar bloco: %w", err)
		}
		return result, nil
	}

	// GetCbSource(bBlocoDeCodigo) -> cRet (caractere)
	// Recupera o código-fonte de um bloco de código.
	// NOTA: Esta implementação retorna o FuncName do codeblock bytecode.
	// No compilador AdvPL real, isso retornaria o source literal do bloco, mas
	// em bytecode, só temos acesso ao nome da função gerada.
	natives["GETCBSOURCE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		// Parâmetro 1: codeblock
		blockVal := getArg(args, 0)
		block, ok := blockVal.(*advplrt.CodeBlockValue)
		if !ok {
			return advplrt.Nil, fmt.Errorf("GetCbSource: argumento deve ser um bloco de código")
		}

		// Retorna o nome da função que implementa o codeblock
		// Em bytecode, não temos o source original, apenas o FuncName
		return advplrt.NewString(block.FuncName), nil
	}
}
