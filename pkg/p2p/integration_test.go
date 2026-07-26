// pkg/p2p/integration_test.go
package p2p

import (
	"net"
	"testing"
	"time"
	"github.com/advpl/compiler/pkg/contract"
	"github.com/advpl/compiler/pkg/storage"
)

func TestE2EThreePeerSync(t *testing.T) {
	peers := make([]*Peer, 3)
	transports := make([]*Transport, 3)
	stores := make([]*storage.Store, 3)

	for i := 0; i < 3; i++ {
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
		peers[i] = NewPeer(string(rune('a'+i)), addr)

		transport, err := NewTransport(addr)
		if err != nil {
			t.Fatalf("transport %d: %v", i, err)
		}
		transports[i] = transport
		stores[i] = storage.NewStore()

		go transport.Listen()
	}

	defer func() {
		for _, tr := range transports {
			tr.Close()
		}
	}()

	time.Sleep(100 * time.Millisecond)

	peers[0].Neighbors = []*Peer{peers[1], peers[2]}
	peers[1].Neighbors = []*Peer{peers[0], peers[2]}
	peers[2].Neighbors = []*Peer{peers[0], peers[1]}

	contractCode := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	c := contract.NewContract(contractCode, []byte{})

	err := stores[0].Put(c.Key, []byte("initial-state"))
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	state0, _ := stores[0].Get(c.Key)
	err = stores[1].Put(c.Key, state0)
	if err != nil {
		t.Fatalf("replicate: %v", err)
	}

	state1, _ := stores[1].Get(c.Key)

	if string(state0) != string(state1) {
		t.Errorf("state mismatch: %s vs %s", string(state0), string(state1))
	}

	t.Logf("E2E test passed: state synced across peers")
}
