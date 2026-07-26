package storage

import (
	"testing"
	"github.com/advpl/compiler/pkg/p2p"
)

func TestSelectReplicaPeers(t *testing.T) {
	peers := []*p2p.Peer{
		{ID: "p1", Location: 0.1},
		{ID: "p2", Location: 0.2},
		{ID: "p3", Location: 0.3},
		{ID: "p4", Location: 0.4},
	}

	r := NewReplicator()
	selected := r.SelectReplicaPeers(peers, 2)

	if len(selected) != 2 {
		t.Errorf("expected 2 replicas, got %d", len(selected))
	}

	peerMap := make(map[string]bool)
	for _, p := range peers {
		peerMap[p.ID] = true
	}

	for _, s := range selected {
		if !peerMap[s.ID] {
			t.Errorf("replica %s not in peer list", s.ID)
		}
	}
}

func TestReplicationDeterministic(t *testing.T) {
	peers := []*p2p.Peer{
		{ID: "p1", Location: 0.1},
		{ID: "p2", Location: 0.2},
		{ID: "p3", Location: 0.3},
	}

	contractKey := "deterministic-key"

	r := NewReplicator()

	s1 := r.SelectReplicaPeersForKey(peers, contractKey, 2)
	s2 := r.SelectReplicaPeersForKey(peers, contractKey, 2)

	if len(s1) != len(s2) {
		t.Fatalf("different counts: %d vs %d", len(s1), len(s2))
	}

	for i := range s1 {
		if s1[i].ID != s2[i].ID {
			t.Errorf("determinism failed at index %d", i)
		}
	}
}
