package main

import (
	"strings"
	"testing"
)

// Fixture mínima no formato de pig --mode json (docs json.md).
const agentStreamOK = `{"type":"session","version":3,"id":"s1","timestamp":"2026-01-15T14:00:00Z","cwd":"/tmp"}
{"type":"tool_execution_start","toolCallId":"call-1","toolName":"bash","args":{"command":"echo hi"}}
{"type":"tool_execution_end","toolCallId":"call-1","toolName":"bash","result":{"content":[{"type":"text","text":"hi"}]},"isError":false}
{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"A saída foi hi."}],"stopReason":"stop"}}
`

const agentStreamTurnFallback = `{"type":"session","version":3,"id":"s2","timestamp":"2026-01-15T14:00:00Z","cwd":"/tmp"}
{"type":"turn_end","message":{"role":"assistant","content":[{"type":"text","text":"Resposta do turno."}]}}
`

func TestExtractFinalAnswerPrefersMessageEnd(t *testing.T) {
	got, err := extractFinalAnswer([]byte(agentStreamOK))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got != "A saída foi hi." {
		t.Fatalf("resposta errada: %q", got)
	}
}

func TestExtractFinalAnswerTurnFallback(t *testing.T) {
	got, err := extractFinalAnswer([]byte(agentStreamTurnFallback))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got != "Resposta do turno." {
		t.Fatalf("resposta errada: %q", got)
	}
}

func TestExtractFinalAnswerEmptyErrors(t *testing.T) {
	if _, err := extractFinalAnswer(nil); err == nil {
		t.Fatal("stream vazio deveria gerar erro, não resposta inventada")
	}
}

func TestBuildAgentPromptTruncates(t *testing.T) {
	big := strings.Repeat("x", agentMaxContextChars+100)
	p := buildAgentPrompt("explique", "ORCAMENTO.prw", big)
	if !strings.Contains(p, "ORCAMENTO.prw") || !strings.Contains(p, "explique") {
		t.Fatal("prompt perdeu pergunta ou nome do arquivo")
	}
	if !strings.Contains(p, "truncado") {
		t.Fatal("prompt deveria marcar truncamento")
	}
	if len(p) > agentMaxContextChars+2000 {
		t.Fatalf("prompt grande demais: %d chars", len(p))
	}
}
