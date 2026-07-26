package p2p

import (
	"crypto/sha256"
	"net"
)

// Peer represents a node on the P2P ring.
type Peer struct {
	ID        string
	Location  float64
	Addr      *net.UDPAddr
	Neighbors []*Peer
	LastSeen  int64
}

// NewPeer creates a peer with deterministic location from its address.
func NewPeer(id string, addr *net.UDPAddr) *Peer {
	location := addressToLocation(addr.String())
	return &Peer{
		ID:        id,
		Location:  location,
		Addr:      addr,
		Neighbors: make([]*Peer, 0),
	}
}

func addressToLocation(addr string) float64 {
	h := sha256.Sum256([]byte(addr))
	sum := uint64(0)
	for i := 0; i < 8; i++ {
		sum = sum*256 + uint64(h[i])
	}
	return float64(sum) / float64(^uint64(0))
}
