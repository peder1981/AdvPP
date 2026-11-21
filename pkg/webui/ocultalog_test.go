package webui

import "testing"

// ADVPP_WEBUI_HIDE_LOG: com o log oculto, ConOut não vira evento para o browser.
func TestOcultaLog(t *testing.T) {
	s := newSession()
	w := &OutWriter{s}

	ocultaLog = true
	defer func() { ocultaLog = false }()
	w.Write([]byte("segredo\n"))
	if len(s.events) != 0 {
		t.Fatalf("com o log oculto saiu evento para o browser")
	}

	ocultaLog = false
	w.Write([]byte("visivel\n"))
	if ev := <-s.events; ev.Type != "output" || ev.Text != "visivel" {
		t.Fatalf("evento inesperado: %+v", ev)
	}
}
