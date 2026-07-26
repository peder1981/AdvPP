// pkg/p2p/transport_test.go
package p2p

import (
	"net"
	"testing"
	"time"
)

func TestTransportSendReceive(t *testing.T) {
	addr1, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	addr2, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")

	t1, err := NewTransport(addr1)
	if err != nil {
		t.Fatalf("NewTransport failed: %v", err)
	}
	defer t1.Close()

	t2, err := NewTransport(addr2)
	if err != nil {
		t.Fatalf("NewTransport failed: %v", err)
	}
	defer t2.Close()

	go t1.Listen()
	go t2.Listen()

	time.Sleep(100 * time.Millisecond)

	msg := &Message{
		Type: "PING",
		Data: []byte("hello"),
	}

	err = t1.Send(t2.LocalAddr(), msg)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	select {
	case received := <-t2.Received:
		if received.Type != "PING" {
			t.Errorf("wrong message type: %s", received.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}
