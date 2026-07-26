package p2p

import (
	"testing"
)

func TestGreedyForwarding(t *testing.T) {
	peer := &Peer{
		ID:       "p0",
		Location: 0.0,
	}

	neighbors := []*Peer{
		{ID: "p1", Location: 0.1},
		{ID: "p2", Location: 0.3},
		{ID: "p3", Location: 0.9},
	}
	peer.Neighbors = neighbors

	router := NewRouter(peer)

	nextHop := router.FindNextHop(0.25)

	if nextHop == nil {
		t.Fatal("FindNextHop returned nil")
	}
	if nextHop.ID != "p2" {
		t.Errorf("expected p2, got %s", nextHop.ID)
	}
}

func TestRouterPicksClosestNeighbor(t *testing.T) {
	peer := &Peer{
		ID:       "origin",
		Location: 0.5,
	}

	peer.Neighbors = []*Peer{
		{ID: "n1", Location: 0.6},
		{ID: "n2", Location: 0.4},
		{ID: "n3", Location: 0.7},
	}

	router := NewRouter(peer)
	nextHop := router.FindNextHop(0.35)

	if nextHop.ID != "n2" {
		t.Errorf("expected n2, got %s", nextHop.ID)
	}
}
