package storage

import (
	"crypto/sha256"
	"encoding/binary"
	"github.com/advpl/compiler/pkg/p2p"
	"math/rand"
)

type Replicator struct {
	rng *rand.Rand
}

func NewReplicator() *Replicator {
	return &Replicator{
		rng: rand.New(rand.NewSource(42)),
	}
}

func (r *Replicator) SelectReplicaPeers(peers []*p2p.Peer, k int) []*p2p.Peer {
	if k > len(peers) {
		k = len(peers)
	}

	shuffled := make([]*p2p.Peer, len(peers))
	copy(shuffled, peers)

	for i := len(shuffled) - 1; i > 0; i-- {
		j := r.rng.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled[:k]
}

func (r *Replicator) SelectReplicaPeersForKey(peers []*p2p.Peer, contractKey string, k int) []*p2p.Peer {
	if k > len(peers) {
		k = len(peers)
	}

	h := sha256.Sum256([]byte(contractKey))
	seed := int64(binary.BigEndian.Uint64(h[:8]))

	rng := rand.New(rand.NewSource(seed))

	shuffled := make([]*p2p.Peer, len(peers))
	copy(shuffled, peers)

	for i := len(shuffled) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled[:k]
}
