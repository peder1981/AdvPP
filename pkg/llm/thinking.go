package llm

import (
	"strings"
)

// Supressão de pensamento (reasoning) em inferência crua.
//
// Técnicas adaptadas do PiG (ai/llama_cpp_classify.go, renderPrompt), que
// opera sem parâmetros de API — só manipulação de prompt/prefill:
//
//  1. Fechar o bloco no prefill: se o prompt termina com "<think>" aberto,
//     completa-lo de imediato produz um bloco de pensamento VAZIO — o mesmo
//     que templates com thinking desabilitado geram — e o próximo token já
//     é a resposta.
//  2. Pós-processar a saída: remover blocos "<think>...</think>", ficando
//     só a resposta (economia de tokens + saída limpa p/ agentes).
//
// Modelos não-pensantes (MiniCPM5, Falcon3) nunca emitem esses blocos: as
// duas funções são no-op para eles. Pensantes típicos: QwQ, DeepSeek-R1
// (e destilados), Qwen3 (híbrido), Granite-thinking.

const (
	thinkOpen  = "<think>"
	thinkClose = "</think>"
)

// CloseThinkingPrefill implementa a técnica 1: prompt terminado em
// "<think>" aberto ganha o fechamento imediato.
func CloseThinkingPrefill(prompt string) string {
	if strings.HasSuffix(prompt, thinkOpen) {
		return prompt + thinkClose
	}
	return prompt
}

// StripThinking implementa a técnica 2: remove todos os blocos
// "<think>...</think>" (não-guloso). Bloco aberto e nunca fechado trunca
// dali em diante (pensamento vazado não é resposta). Sem bloco, devolve o
// texto como está (trimado nas pontas).
func StripThinking(s string) string {
	var sb strings.Builder
	rest := s
	for {
		start := strings.Index(rest, thinkOpen)
		if start < 0 {
			sb.WriteString(rest)
			break
		}
		sb.WriteString(rest[:start])
		after := rest[start+len(thinkOpen):]
		if end := strings.Index(after, thinkClose); end >= 0 {
			rest = after[end+len(thinkClose):]
			continue
		}
		// Aberto sem fechar: descarta o resto.
		break
	}
	return strings.TrimSpace(sb.String())
}
