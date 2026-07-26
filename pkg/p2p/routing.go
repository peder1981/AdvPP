package p2p

type Router struct {
	peer *Peer
}

func NewRouter(peer *Peer) *Router {
	return &Router{peer: peer}
}

func (r *Router) FindNextHop(target float64) *Peer {
	if len(r.peer.Neighbors) == 0 {
		return nil
	}

	var closest *Peer
	var minDist float64

	for i, neighbor := range r.peer.Neighbors {
		dist := RingDistance(target, neighbor.Location)

		if i == 0 || dist < minDist {
			minDist = dist
			closest = neighbor
		}
	}

	// Only forward to neighbor if it's closer than us
	myDist := RingDistance(r.peer.Location, target)
	if minDist >= myDist && closest != nil {
		return nil // We're terminal; don't forward
	}

	return closest
}

func (r *Router) CanRoute(target float64) bool {
	myDist := RingDistance(r.peer.Location, target)

	for _, neighbor := range r.peer.Neighbors {
		neighborDist := RingDistance(neighbor.Location, target)
		if neighborDist < myDist {
			return true
		}
	}

	return false
}
