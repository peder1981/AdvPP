package vm

import (
	"testing"
)

func TestLlmTimeoutSeconds(t *testing.T) {
	t.Setenv("ADVPP_LLM_TIMEOUT_SECS", "")
	if got := llmTimeoutSeconds(); got != 30*60 {
		t.Fatalf("default moderno deveria ser 1800s, veio %d", got)
	}
	t.Setenv("ADVPP_LLM_TIMEOUT_SECS", "3600")
	if got := llmTimeoutSeconds(); got != 3600 {
		t.Fatalf("env 3600 deveria valer 3600, veio %d", got)
	}
	t.Setenv("ADVPP_LLM_TIMEOUT_SECS", "0")
	if got := llmTimeoutSeconds(); got != 0 {
		t.Fatalf("env 0 deveria desligar o teto, veio %d", got)
	}
	t.Setenv("ADVPP_LLM_TIMEOUT_SECS", "lixo")
	if got := llmTimeoutSeconds(); got != 30*60 {
		t.Fatalf("env inválido deveria cair no default, veio %d", got)
	}
	t.Setenv("ADVPP_LLM_TIMEOUT_SECS", "-5")
	if got := llmTimeoutSeconds(); got != 30*60 {
		t.Fatalf("env negativo deveria cair no default, veio %d", got)
	}
}
