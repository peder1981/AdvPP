package storage

import (
	"testing"
)

type testPeer struct {
	id       string
	location float64
}

func (p *testPeer) GetID() string {
	return p.id
}

func (p *testPeer) GetLocation() float64 {
	return p.location
}

func TestSelectReplicaPeers(t *testing.T) {
	peers := []Peer{
		&testPeer{id: "p1", location: 0.1},
		&testPeer{id: "p2", location: 0.2},
		&testPeer{id: "p3", location: 0.3},
		&testPeer{id: "p4", location: 0.4},
	}

	r := NewReplicator()
	selected := r.SelectReplicaPeers(peers, "some-contract-key", 2)

	if len(selected) != 2 {
		t.Errorf("expected 2 replicas, got %d", len(selected))
	}

	peerMap := make(map[string]bool)
	for _, p := range peers {
		peerMap[p.GetID()] = true
	}

	for _, s := range selected {
		if !peerMap[s.GetID()] {
			t.Errorf("replica %s not in peer list", s.GetID())
		}
	}
}

func TestReplicationDeterministic(t *testing.T) {
	peers := []Peer{
		&testPeer{id: "p1", location: 0.1},
		&testPeer{id: "p2", location: 0.2},
		&testPeer{id: "p3", location: 0.3},
	}

	contractKey := "deterministic-key"

	r := NewReplicator()

	s1 := r.SelectReplicaPeers(peers, contractKey, 2)
	s2 := r.SelectReplicaPeers(peers, contractKey, 2)

	if len(s1) != len(s2) {
		t.Fatalf("different counts: %d vs %d", len(s1), len(s2))
	}

	for i := range s1 {
		if s1[i].GetID() != s2[i].GetID() {
			t.Errorf("determinism failed at index %d", i)
		}
	}
}
