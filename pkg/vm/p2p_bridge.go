// pkg/vm/p2p_bridge.go
//
// VM<->Go P2P bridge (Task 4 of the freenet-chat-poc plan). Task 3 left
// examples/chat/chat.prw with structural AdvPL-only stand-ins for Peer,
// Transport, Router and PeerAPI (see the "TODO: real P2P binding" comments
// in that file) because the AdvPP VM had no way to reach the real Go P2P
// stack (pkg/p2p, pkg/contract, pkg/storage). This file is that bridge.
//
// Architecture decision (see task-4-brief.md "Key Architectural Question"):
// one real peer per OS process (Option 3, singleton), lazily built the
// first time any PeerAPI_*/Router_* native is called, from the same
// ADVPP_PEER_ID/ADVPP_LISTEN/ADVPP_BOOTSTRAP environment variables
// ChatMain() already reads in AdvPL (there is no PARAMCOUNT()/PARAMSTR()
// native yet to pass a peer handle explicitly — see that file's comment).
//
// Merge semantics (see brief "Note on merge semantics"): Option A,
// hardcoded for this PoC. chatContractMerge below implements Task 1's
// ChatContract rule — append-only, idempotent on (from, ts) — as a
// contract.MergeOp so it can go through the same PeerAPI.Update() path a
// real WASM contract would use.
package vm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/advpl/compiler/pkg/contract"
	"github.com/advpl/compiler/pkg/p2p"
	"github.com/advpl/compiler/pkg/storage"
)

// --- Merge function registry (Option A: hardcoded for this PoC) -----------

// chatMessage mirrors the JSON shape ChatSend()/ChatRead() in chat.prw
// build via JsonObject() (from/text/ts).
type chatMessage struct {
	From string `json:"from"`
	Text string `json:"text"`
	TS   string `json:"ts"`
}

// chatContractMerge implements Task 1's ChatContract merge rule: current
// state (a) is a JSON array of messages; an update (b) is either a single
// message object (the normal ChatSend() path) or a full array (a
// STATE_SYNC reply from a peer we just joined). Either way, the result is
// the union of both, deduplicated on (from, ts) — append-only, idempotent,
// commutative, per the plan's global constraints.
type chatContractMerge struct{}

func (chatContractMerge) Identity() []byte { return []byte("[]") }

func (chatContractMerge) Merge(a, b []byte) []byte {
	var list []chatMessage
	if len(a) > 0 {
		_ = json.Unmarshal(a, &list)
	}

	add := func(msg chatMessage) {
		if msg.From == "" && msg.Text == "" && msg.TS == "" {
			return
		}
		for _, existing := range list {
			if existing.From == msg.From && existing.TS == msg.TS {
				return
			}
		}
		list = append(list, msg)
	}

	var single chatMessage
	if err := json.Unmarshal(b, &single); err == nil && (single.From != "" || single.Text != "") {
		add(single)
	} else {
		var many []chatMessage
		if err := json.Unmarshal(b, &many); err == nil {
			for _, msg := range many {
				add(msg)
			}
		}
	}

	out, err := json.Marshal(list)
	if err != nil {
		return a
	}
	return out
}

// chatMergeOps is the merge-function registry PeerAPI_Update()'s
// mergeOpName argument resolves against. Only the one name chat.prw uses
// is registered — a general registry (Option B in the brief) is out of
// scope for this PoC.
var chatMergeOps = map[string]contract.MergeOp{
	"ChatContractMerge": chatContractMerge{},
}

// --- Bridge: real Go-side peer wiring for this process ---------------------

// chatBridge owns the real p2p.Peer/Transport/Router/PeerAPI for the
// process's one chat peer, plus the minimal gossip needed for the Task 4
// two-peer test: JOIN (announce + request current state) and UPDATE
// (broadcast a merged write to known neighbors).
type chatBridge struct {
	peer      *p2p.Peer
	transport *p2p.Transport
	router    *p2p.Router
	api       *p2p.PeerAPI
	store     *storage.Store

	mu        sync.RWMutex
	neighbors map[string]*net.UDPAddr // "host:port" -> resolved addr
}

var (
	globalChatBridge     *chatBridge
	globalChatBridgeOnce sync.Once
	globalChatBridgeErr  error
)

// getChatBridge returns the process-wide chat peer, building it on first
// use from ADVPP_PEER_ID/ADVPP_LISTEN/ADVPP_BOOTSTRAP.
func getChatBridge() (*chatBridge, error) {
	globalChatBridgeOnce.Do(func() {
		globalChatBridge, globalChatBridgeErr = newChatBridge()
	})
	return globalChatBridge, globalChatBridgeErr
}

func newChatBridge() (*chatBridge, error) {
	peerID := getEnvOrDefault("ADVPP_PEER_ID", "peer-1")
	listen := getEnvOrDefault("ADVPP_LISTEN", "127.0.0.1:9000")
	bootstrap := getEnvOrDefault("ADVPP_BOOTSTRAP", "")

	listenAddr, err := net.ResolveUDPAddr("udp", listen)
	if err != nil {
		return nil, fmt.Errorf("chat bridge: resolve ADVPP_LISTEN %q: %w", listen, err)
	}

	transport, err := p2p.NewTransport(listenAddr)
	if err != nil {
		return nil, fmt.Errorf("chat bridge: bind transport on %q: %w", listen, err)
	}

	peer := p2p.NewPeer(peerID, transport.LocalAddr())
	store := storage.NewStore()

	b := &chatBridge{
		peer:      peer,
		transport: transport,
		router:    p2p.NewRouter(peer),
		api:       p2p.NewPeerAPI(store),
		store:     store,
		neighbors: make(map[string]*net.UDPAddr),
	}

	go transport.Listen()
	go b.receiveLoop()

	if bootstrap != "" {
		if addr, err := net.ResolveUDPAddr("udp", bootstrap); err == nil {
			b.addNeighbor(addr)
			b.send(addr, &p2p.Message{Type: "JOIN", From: b.selfAddr()})
		} else {
			fmt.Fprintf(os.Stderr, "[chat bridge] bad ADVPP_BOOTSTRAP %q: %v\n", bootstrap, err)
		}
	}

	return b, nil
}

func (b *chatBridge) selfAddr() string {
	return b.transport.LocalAddr().String()
}

func (b *chatBridge) send(addr *net.UDPAddr, msg *p2p.Message) {
	if err := b.transport.Send(addr, msg); err != nil {
		fmt.Fprintf(os.Stderr, "[chat bridge] send to %s failed: %v\n", addr, err)
	}
}

func (b *chatBridge) addNeighbor(addr *net.UDPAddr) {
	key := addr.String()

	b.mu.Lock()
	_, known := b.neighbors[key]
	if !known {
		b.neighbors[key] = addr
	}
	b.mu.Unlock()

	if known {
		return
	}
	b.peer.Neighbors = append(b.peer.Neighbors, p2p.NewPeer(key, addr))
}

func (b *chatBridge) neighborAddrs() []*net.UDPAddr {
	b.mu.RLock()
	defer b.mu.RUnlock()
	addrs := make([]*net.UDPAddr, 0, len(b.neighbors))
	for _, a := range b.neighbors {
		addrs = append(addrs, a)
	}
	return addrs
}

// receiveLoop applies incoming JOIN/UPDATE/STATE_SYNC messages. Runs for
// the lifetime of the process (mirrors transport.Listen()'s own loop).
func (b *chatBridge) receiveLoop() {
	for msg := range b.transport.Received {
		switch msg.Type {
		case "JOIN":
			b.handleJoin(msg)
		case "UPDATE", "STATE_SYNC":
			b.handleUpdate(msg)
		}
	}
}

func (b *chatBridge) handleJoin(msg *p2p.Message) {
	if msg.From == "" {
		return
	}
	addr, err := net.ResolveUDPAddr("udp", msg.From)
	if err != nil {
		return
	}
	b.addNeighbor(addr)

	// Reply with whatever this peer already has so the joining peer
	// converges immediately instead of waiting for the next local write.
	for _, key := range b.store.Keys() {
		data, err := b.store.Get(key)
		if err != nil {
			continue
		}
		b.send(addr, &p2p.Message{Type: "STATE_SYNC", Key: key, Data: data, From: b.selfAddr()})
	}
}

func (b *chatBridge) handleUpdate(msg *p2p.Message) {
	if msg.Key == "" {
		return
	}
	mergeOp := chatMergeOps["ChatContractMerge"]
	if err := b.api.Update(msg.Key, msg.Data, mergeOp); err != nil {
		fmt.Fprintf(os.Stderr, "[chat bridge] merge on receive failed: %v\n", err)
	}
}

// get returns the current merged state for key (raw JSON bytes).
func (b *chatBridge) get(key string) ([]byte, error) {
	return b.api.Get(key)
}

// update merges data into key locally via mergeName, then broadcasts the
// resulting merged state to every known neighbor (best-effort UDP; the
// two-peer JOIN handshake is what guarantees eventual delivery even if a
// broadcast is lost, since a peer's JOIN always pulls current state).
func (b *chatBridge) update(key string, data []byte, mergeName string) error {
	mergeOp, ok := chatMergeOps[mergeName]
	if !ok {
		return fmt.Errorf("chat bridge: unknown merge op %q", mergeName)
	}
	if err := b.api.Update(key, data, mergeOp); err != nil {
		return err
	}
	merged, err := b.store.Get(key)
	if err != nil {
		return err
	}
	for _, addr := range b.neighborAddrs() {
		b.send(addr, &p2p.Message{Type: "UPDATE", Key: key, Data: merged, From: b.selfAddr()})
	}
	return nil
}

// findNextHop mirrors pkg/p2p/routing.go's greedy ring routing: hash
// targetID to a ring location (same SHA-256 -> [0,1) formula as
// pkg/contract/types.go and pkg/p2p/types.go) and ask the router for the
// closest neighbor strictly closer to it than this peer. Returns this
// peer's own ID when there's no better hop (single-peer / terminal case).
func (b *chatBridge) findNextHop(targetID string) string {
	target := chatTargetLocation(targetID)
	next := b.router.FindNextHop(target)
	if next == nil {
		return b.peer.ID
	}
	return next.ID
}

// subscribe marks interest in key and, if any neighbors are already known,
// asks them for its current value right away (rather than waiting for the
// next UPDATE broadcast) — the real-subscription-tree equivalent this PoC
// needs: eventual delivery without a long-lived push channel back into
// AdvPL, which the VM has no mechanism for yet.
func (b *chatBridge) subscribe(key string) {
	for _, addr := range b.neighborAddrs() {
		b.send(addr, &p2p.Message{Type: "JOIN", From: b.selfAddr()})
	}
	_ = key // key-scoped subscriptions are out of scope for this PoC; see brief.
}

func chatTargetLocation(id string) float64 {
	h := sha256.Sum256([]byte(id))
	sum := uint64(0)
	for i := 0; i < 8; i++ {
		sum = sum*256 + uint64(h[i])
	}
	return float64(sum) / float64(^uint64(0))
}
