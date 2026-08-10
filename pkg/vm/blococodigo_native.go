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
	// NOTA: Esta implementação é limitada por falta de acesso direto ao banco de dados.
	// O comportamento real requer integração com o motor de banco de dados do Protheus,
	// que não está disponível neste runtime headless.
	natives["DBEVAL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		// Parâmetro 1: codeblock
		blockVal := getArg(args, 0)
		_, ok := blockVal.(*advplrt.CodeBlockValue)
		if !ok {
			return advplrt.Nil, fmt.Errorf("DBEval: primeiro argumento deve ser um bloco de código")
		}

		// Os demais parâmetros são condições de filtro e controle:
		// [bFirstCondition], [bSecondCondition], [nCount], [nRecno], [lRest]
		// Sem acesso ao banco, apenas validamos o argumento principal

		// TODO: Implementação futura requer integração com banco de dados
		// Por enquanto, retorna nil como documentado (sempre retorna nulo)
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
