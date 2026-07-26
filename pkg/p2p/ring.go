package p2p

import (
	"math"
	"sort"
)

// RingDistance calculates wrap-around distance: d(x,y) = min(|x-y|, 1-|x-y|)
func RingDistance(a, b float64) float64 {
	delta := math.Abs(a - b)
	return math.Min(delta, 1.0-delta)
}

// FindClosestPeers returns k peers closest to target location.
func FindClosestPeers(target float64, peers []*Peer, k int) []*Peer {
	if k > len(peers) {
		k = len(peers)
	}
	if k <= 0 {
		return []*Peer{}
	}

	sorted := make([]*Peer, len(peers))
	copy(sorted, peers)
	sort.Slice(sorted, func(i, j int) bool {
		di := RingDistance(target, sorted[i].Location)
		dj := RingDistance(target, sorted[j].Location)
		return di < dj
	})

	return sorted[:k]
}
