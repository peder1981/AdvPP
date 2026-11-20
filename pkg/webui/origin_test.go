package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Página de outro site (CSRF) ou DNS rebinding não pode alcançar o servidor.
func TestSameOrigin(t *testing.T) {
	h := sameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	cases := []struct {
		host, origin string
		want         int
	}{
		{"localhost:8080", "", 200},
		{"127.0.0.1:8080", "http://127.0.0.1:8080", 200},
		{"[::1]:8080", "", 200},
		{"localhost:8080", "http://evil.example", 403},         // CSRF
		{"evil.example:8080", "http://evil.example:8080", 403}, // DNS rebinding
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/events?s=1", nil)
		r.Host = c.host
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != c.want {
			t.Errorf("host=%s origin=%s: código %d, esperava %d", c.host, c.origin, rec.Code, c.want)
		}
	}
}
