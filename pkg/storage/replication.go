package storage

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
)

type Peer interface {
	GetID() string
	GetLocation() float64
}

type Replicator struct {
	rng *rand.Rand
}

func NewReplicator() *Replicator {
	return &Replicator{
		rng: rand.New(rand.NewSource(42)),
	}
}

func (r *Replicator) SelectReplicaPeers(peers []Peer, k int) []Peer {
	if k > len(peers) {
		k = len(peers)
	}

	shuffled := make([]Peer, len(peers))
	copy(shuffled, peers)

	for i := len(shuffled) - 1; i > 0; i-- {
		j := r.rng.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled[:k]
}

func (r *Replicator) SelectReplicaPeersForKey(peers []Peer, contractKey string, k int) []Peer {
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
