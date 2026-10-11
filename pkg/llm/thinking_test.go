package llm

import (
	"testing"
)

func TestCloseThinkingPrefill(t *testing.T) {
	if got := CloseThinkingPrefill("Responda:<think>"); got != "Responda:<think></think>" {
		t.Fatalf("deveria fechar o bloco: %q", got)
	}
	if got := CloseThinkingPrefill("Responda direto."); got != "Responda direto." {
		t.Fatalf("sem bloco aberto, no-op: %q", got)
	}
	if got := CloseThinkingPrefill(""); got != "" {
		t.Fatalf("vazio, no-op: %q", got)
	}
}

func TestStripThinking(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Paris.", "Paris."},
		{"", ""},
		{"<think>hmm, França...</think>Paris.", "Paris."},
		{"A:<think>x</think>B<think>y</think>C", "A:BC"},
		{"Resposta<think>vazou sem fechar", "Resposta"},
		{"<think>só pensamento</think>", ""},
		{"  <think>t</think>  Com espaços.  ", "Com espaços."},
	}
	for _, c := range cases {
		if got := StripThinking(c.in); got != c.want {
			t.Errorf("StripThinking(%q) = %q, queria %q", c.in, got, c.want)
		}
	}
}
