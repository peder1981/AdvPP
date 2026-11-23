package vm

import (
	"testing"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// URL que já traz o RestPrefix não ganha o prefixo de novo (antes:
// "/rest/rest/api/..." e 404 em todo template dinâmico dos fixtures).
func TestPoFilterURLNaoDuplicaPrefixo(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // RestPrefix padrão "/rest"
	casos := map[string]string{
		"/rest/api/est/v1/armazens": "/rest/api/est/v1/armazens",
		"/api/est/v1/armazens":      "/rest/api/est/v1/armazens",
		"api/x":                     "/rest/api/x",
		"/restaurantes":             "/rest/restaurantes",
		"/rest":                     "/rest",
		"https://h/x":               "https://h/x",
		"":                          "",
	}
	for in, want := range casos {
		if got := poFilterURL(advplrt.NewString(in)); got != want {
			t.Errorf("poFilterURL(%q) = %q, want %q", in, got, want)
		}
		if got := poRestPrefixURL(advplrt.NewString(in)); got != want {
			t.Errorf("poRestPrefixURL(%q) = %q, want %q", in, got, want)
		}
	}
}
