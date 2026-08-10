package vm

import (
	"fmt"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerIntegracaoExcelNatives registra funções de integração com Excel:
// SIGA, MsGetArray.
func (v *VM) registerIntegracaoExcelNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// SIGA(cFuncao, [cParam1, cParam2, ...]) -> xRet
	// Função implementada na integração com o Excel que permite a chamada de funções do Sistema ERP TOTVS.
	// Sintaxe: SIGA("NomeFuncao"; [param1]; [param2]; [...]; [paramN])
	// Retorna o resultado da função invocada.
	//
	// Funcionalidade:
	// - Suporta chamadas a funções nativas (built-in) registradas em v.natives
	// - Suporta chamadas a funções AdvPL de usuário compiladas em v.bc.Functions
	// - Tenta lookup em nativas primeiro, depois em funções compiladas (via RunFunction)
	// - A integração completa com Excel (renderização de valores em células) requer
	//   uma extensão do Excel (add-in) e um servidor Protheus real
	natives["SIGA"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if len(args) == 0 {
			return advplrt.Nil, fmt.Errorf("SIGA: função não informada")
		}

		funcName := advplrt.ToString(getArg(args, 0))

		// Prepara os argumentos para passar à função (excluindo o primeiro que é o nome)
		var fnArgs []advplrt.Value
		if len(args) > 1 {
			fnArgs = args[1:]
		}

		// Tenta encontrar a função em nativas (built-in) primeiro
		funcNameUpper := strings.ToUpper(funcName)
		if nativeFn, exists := v.natives[funcNameUpper]; exists {
			return nativeFn.Fn(fnArgs)
		}

		// Tenta encontrar a função em funções compiladas de usuário
		// RunFunction faz lookup case-insensitive e trata o prefixo U_
		result, err := v.RunFunction(funcName, fnArgs)
		if err != nil {
			return advplrt.Nil, fmt.Errorf("SIGA: função '%s' não encontrada ou erro na execução: %w", funcName, err)
		}

		return result, nil
	}

	// MsGetArray(oCell, xExecuteFunction) -> xRet
	// Obtém os dados de retorno de uma outra função, quando esta retornar um array de dados.
	// Sintaxe: MsGetArray(<Cell>, <ExecuteFunction>)
	// Retorna o mesmo valor do segundo parâmetro (ExecuteFunction).
	//
	// LIMITAÇÃO: AdvPP não suporta integração com Excel. Esta implementação:
	// - Retorna o valor do segundo parâmetro diretamente
	// - O parâmetro Cell é ignorado (é específico do Excel, usado para posicionar
	//   a exibição do array na planilha)
	// - Não há suporte a Range/Cell objects do Excel; o parâmetro é aceito mas não
	//   utilizado na posição dos dados (não há conceito de "célula" em AdvPP)
	natives["MSGETARRAY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if len(args) < 2 {
			return advplrt.Nil, fmt.Errorf("MsGetArray: parâmetros insuficientes (Cell e ExecuteFunction são obrigatórios)")
		}

		// Obtém o segundo parâmetro (o resultado da função)
		result := getArg(args, 1)

		// Retorna o resultado da função
		// Nota: o parâmetro Cell (args[0]) é ignorado em AdvPP pois não há integração com Excel
		return result, nil
	}
}
