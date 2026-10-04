package webui

import (
	"testing"
	"time"
)

// Sessão abandonada (browser sumiu com um menu aberto): a goroutine do
// programa sai pelos defers em vez de ficar presa para sempre segurando
// a conexão de banco da sessão.
func TestAbandonReleasesBlockedDialog(t *testing.T) {
	s := newSession()
	p := &Provider{s}
	deferRan := make(chan struct{})
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		defer close(deferRan) // representa o defer v.CloseOwnedConnections()
		p.Menu([]string{"a", "b"}, "menu")
		t.Error("Menu nao deveria retornar numa sessao abandonada")
	}()
	<-s.events // o menu foi enviado e agora espera resposta
	s.abandon()
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine da sessao continuou presa apos abandon()")
	}
	select {
	case <-deferRan:
	default:
		t.Fatal("defers da sessao nao rodaram")
	}
}

// Saída de console numa sessão abandonada com o buffer cheio não trava.
func TestAbandonDropsOutput(t *testing.T) {
	s := newSession()
	w := &OutWriter{s}
	s.abandon()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			w.Write([]byte("linha\n"))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Write travou numa sessao abandonada")
	}
}
