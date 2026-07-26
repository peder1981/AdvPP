package p2p

import (
	"math"
	"testing"
)

func TestRingDistance(t *testing.T) {
	tests := []struct {
		a, b     float64
		expected float64
	}{
		{0.1, 0.3, 0.2},
		{0.9, 0.1, 0.2},
		{0.0, 0.5, 0.5},
		{0.0, 0.0, 0.0},
		{0.99, 0.01, 0.02},
	}

	for _, tt := range tests {
		got := RingDistance(tt.a, tt.b)
		if math.Abs(got-tt.expected) > 1e-10 {
			t.Errorf("RingDistance(%f, %f) = %f, want %f", tt.a, tt.b, got, tt.expected)
		}
	}
}

func TestFindClosestPeers(t *testing.T) {
	peers := []*Peer{
		{Location: 0.1, ID: "p1"},
		{Location: 0.2, ID: "p2"},
		{Location: 0.5, ID: "p3"},
		{Location: 0.9, ID: "p4"},
	}

	closest := FindClosestPeers(0.0, peers, 2)

	if len(closest) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(closest))
	}

	ids := map[string]bool{closest[0].ID: true, closest[1].ID: true}
	if !ids["p1"] || !ids["p4"] {
		t.Errorf("expected p1 and p4, got %v", ids)
	}
}
