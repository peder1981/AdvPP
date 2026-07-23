package vm

import (
	"fmt"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// evalMacroString implementa a semântica real do operador `&` (macro
// substitution) do Clipper/AdvPL: o conteúdo em runtime da string é
// lexado, parseado e compilado como uma expressão isolada, e executado
// contra o MESMO estado de VM (dynEnv compartilhado, para que a macro
// enxergue variáveis Private/Public correntes — Locals não são visíveis,
// igual ao Clipper real, já que Locals não existem por nome em runtime).
//
// A expressão roda com seu próprio array de bytecode (não é injetada em
// v.bc), então uma chamada a uma função de usuário de dentro da macro não
// resolve (mesma limitação documentada de MacroExp para invocação
// dinâmica por nome) — cobre o caso comum: literais, operadores,
// composição de nome via `ident&macro` (`K2&cSuf` já chega aqui como a
// string concatenada, ex. "K2A", e uma referência a identificador puro
// vira um lookup de variável dinâmica normal).
func (v *VM) evalMacroString(src string) (advplrt.Value, error) {
	const fnName = "__AdvppMacroEval"
	wrapped := "Function " + fnName + "()\nReturn (" + src + ")\n"

	tokens, err := lexer.Tokenize(wrapped, "&macro")
	if err != nil {
		return advplrt.Nil, err
	}
	prog, err := parser.NewParser(tokens, "&macro", nil).Parse()
	if err != nil {
		return advplrt.Nil, err
	}
	macroBc, err := compiler.Compile(prog)
	if err != nil {
		return advplrt.Nil, err
	}
	info, ok := macroBc.Functions[fnName]
	if !ok {
		return advplrt.Nil, fmt.Errorf("macro: função de avaliação não gerada")
	}

	frame := &CallFrame{
		FuncName:  "&macro",
		Code:      macroBc.Code,
		IP:        info.Offset,
		Locals:    make([]advplrt.Value, info.NumLocals),
		StackBase: len(v.stack),
	}

	// runLoop() é reentrante em v.frames/v.current: ele roda até v.current
	// virar nil, o que só acontece quando TODOS os frames empilhados
	// retornam. Chamá-lo de novo por cima de frames já em execução (o
	// próprio programa que está avaliando esta macro) faria o loop
	// aninhado continuar executando o programa CHAMADOR inteiro assim que
	// o frame da macro retornasse, em vez de parar ali — bug real
	// encontrado testando esta função (saída duplicada/embaralhada).
	// Isola a macro numa pilha de frames própria (só ela), roda até
	// esvaziar, e restaura a pilha do chamador depois.
	savedFrames, savedCurrent := v.frames, v.current
	v.frames = []*CallFrame{frame}
	v.current = frame

	// OP_NUMBER/OP_STRING/OP_DATE resolvem constantes via v.bc.Constants
	// (não por frame) — sem trocar v.bc pro bytecode recém-compilado da
	// macro, os índices de constante caem por acidente nas constantes do
	// PROGRAMA PRINCIPAL, não nas da macro (bug real encontrado testando
	// esta função: `&"42"` avaliava para 0).
	prevBc := v.bc
	v.bc = macroBc

	result, err := v.runLoop()

	v.bc = prevBc
	v.frames, v.current = savedFrames, savedCurrent

	return result, err
}
