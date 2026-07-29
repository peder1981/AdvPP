package storage

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
)

// Peer represents a core AdvPP type.
type Peer interface {
	GetID() string
	GetLocation() float64
}

// Replicator represents a core AdvPP type.
type Replicator struct{}

// NewReplicator performs a core operation.
func NewReplicator() *Replicator {
	return &Replicator{}
}

// SelectReplicaPeers deterministically picks k replica peers for a contract key.
// The shuffle is seeded from the key itself, so every peer independently derives
// the same replica set without shared mutable state.
func (r *Replicator) SelectReplicaPeers(peers []Peer, contractKey string, k int) []Peer {
	if k > len(peers) {
		k = len(peers)
	}

	h := sha256.Sum256([]byte(contractKey))
	seed := int64(binary.BigEndian.Uint64(h[:8]))

	rng := rand.New(rand.NewSource(seed))

	shuffled := make([]Peer, len(peers))
	copy(shuffled, peers)

	for i := len(shuffled) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled[:k]
}
