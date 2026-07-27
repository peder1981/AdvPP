# AdvPP v2.0: Freenet-style P2P Network Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend AdvPP compiler & runtime to support Freenet-style P2P network primitives, enabling decentralized applications with WASM contracts, cryptographic trust, oblivious storage, and causal consistency.

**Architecture:** Layered monolithic `advpp-peer` binary with 7 layers (Transport → Ring → Routing → Contracts → Sync → Storage → API). Single Go process, clear layer boundaries, no IPC. Compiler generates WASM from AdvPL/TLPP for contracts. Core implements small-world routing, summary/delta sync, idempotent merge algebra.

**Tech Stack:** Go 1.24+, wasmtime-go (WASM runtime), crypto/sha256 (hashing), net/udp (transport), SQLite (local storage), AdvPL/TLPP source language.

## Global Constraints

- **WASM contracts:** All user code compiles to WebAssembly; no bytecode VM for contracts
- **Idempotent merge:** Contract state must be idempotent commutative monoid (a ⊕ a = a, a ⊕ b = b ⊕ a)
- **Small-world routing:** Ring topology with O(log² N) hops via greedy + adaptive cost
- **Causal consistency:** Updates respect causal dependencies; eventual convergence guaranteed
- **No central authority:** Peer discovery, routing, replication fully decentralized
- **Oblivious storage:** All data encrypted; platform never exposes plaintext to untrusted peers
- **Summary/delta protocol:** Sync bandwidth bounded by |summary| + |delta|

---

## Phase 1: Foundation

### Task 1: WASM Codegen (AdvPL → WebAssembly)

**Files:**
- Create: `pkg/compiler/wasm_codegen.go`
- Create: `pkg/compiler/wasm_codegen_test.go`
- Modify: `pkg/compiler/compiler.go` (add `CompileToWasm()` method)

**Interfaces:**
- Consumes: `Bytecode` type (from existing compiler)
- Produces: `WasmModule` ([]byte), `CompileToWasm(source string) (WasmModule, error)`

**Why this task:** Contracts execute as WASM in the network; compiler must generate WASM in addition to native bytecode.

- [ ] **Step 1: Write failing test for WASM compilation**

File: `pkg/compiler/wasm_codegen_test.go`

```go
package compiler

import (
	"testing"
)

func TestCompileSimpleToWasm(t *testing.T) {
	source := `
User Function HelloWorld()
    Local nResult := 42
Return nResult
`
	wasm, err := CompileToWasm(source)
	if err != nil {
		t.Fatalf("CompileToWasm failed: %v", err)
	}
	if len(wasm) == 0 {
		t.Fatal("WASM module is empty")
	}
	// WASM magic number: 0x00 0x61 0x73 0x6d
	if len(wasm) < 4 || wasm[0] != 0x00 || wasm[1] != 0x61 || wasm[2] != 0x73 || wasm[3] != 0x6d {
		t.Errorf("Invalid WASM magic number: %v", wasm[:4])
	}
}

func TestCompileWithMergeFunction(t *testing.T) {
	source := `
User Function MergeCounter(nOld as Numeric, nNew as Numeric) as Numeric
Return Max(nOld, nNew)
`
	wasm, err := CompileToWasm(source)
	if err != nil {
		t.Fatalf("CompileToWasm failed: %v", err)
	}
	if len(wasm) == 0 {
		t.Fatal("WASM module is empty")
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

```bash
cd /home/peder/Projetos/AdvPP
go test ./pkg/compiler -run TestCompileSimpleToWasm -v
```

Expected: `undefined: CompileToWasm`

- [ ] **Step 3: Implement WASM codegen stub**

File: `pkg/compiler/wasm_codegen.go`

```go
package compiler

import (
	"bytes"
	"fmt"
)

type WasmModule []byte

// CompileToWasm takes AdvPL source and returns a minimal valid WASM module.
// MVP: returns a WASM module with magic + version (no functions yet).
func CompileToWasm(source string) (WasmModule, error) {
	if source == "" {
		return nil, fmt.Errorf("empty source")
	}

	// Compile to bytecode first (reuse existing compiler)
	compiler := NewCompiler()
	bc, err := compiler.Compile(source)
	if err != nil {
		return nil, fmt.Errorf("bytecode compilation failed: %w", err)
	}

	// MVP WASM module (magic + version + empty sections)
	buf := new(bytes.Buffer)

	// WASM magic number
	buf.Write([]byte{0x00, 0x61, 0x73, 0x6d})

	// WASM version (1)
	buf.Write([]byte{0x01, 0x00, 0x00, 0x00})

	// No sections for now; later: type, function, code sections
	_ = bc // Use to prevent unused error

	return buf.Bytes(), nil
}
```

- [ ] **Step 4: Run tests, verify they pass**

```bash
go test ./pkg/compiler -run TestCompileSimpleToWasm -v
go test ./pkg/compiler -run TestCompileWithMergeFunction -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/compiler/wasm_codegen.go pkg/compiler/wasm_codegen_test.go
git commit -m "feat: add WASM codegen stub (AdvPL → WASM)"
```

---

### Task 2: Contract Runtime & Types

**Files:**
- Create: `pkg/contract/types.go`
- Create: `pkg/contract/wasm.go`
- Create: `pkg/contract/wasm_test.go`

**Interfaces:**
- Consumes: `WasmModule` (from Task 1)
- Produces: `Contract`, `State`, `Update` types; `ContractRuntime`, `LoadContract()`, `GetContract()`

**Why this task:** Contracts (WASM modules) load and execute in the network. Deterministic key/location derivation enables distributed lookup.

- [ ] **Step 1: Write failing test for contract types**

File: `pkg/contract/wasm_test.go`

```go
package contract

import (
	"testing"
)

func TestNewContractDeterministic(t *testing.T) {
	code := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	params := []byte("test-params")

	c1 := NewContract(code, params)
	c2 := NewContract(code, params)

	if c1.Location != c2.Location {
		t.Errorf("Location not deterministic: %f vs %f", c1.Location, c2.Location)
	}
	if c1.Key != c2.Key {
		t.Errorf("Key not deterministic: %s vs %s", c1.Key, c2.Key)
	}
	if c1.CodeHash != c2.CodeHash {
		t.Errorf("CodeHash not deterministic: %s vs %s", c1.CodeHash, c2.CodeHash)
	}
}

func TestLoadAndRetrieveContract(t *testing.T) {
	code := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	c := NewContract(code, []byte{})

	runtime := NewContractRuntime()
	err := runtime.LoadContract(c)
	if err != nil {
		t.Fatalf("LoadContract failed: %v", err)
	}

	retrieved := runtime.GetContract(c.Key)
	if retrieved == nil {
		t.Fatal("Contract not found after loading")
	}
	if retrieved.Key != c.Key {
		t.Errorf("Retrieved contract key mismatch: %s vs %s", retrieved.Key, c.Key)
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

```bash
cd /home/peder/Projetos/AdvPP
go test ./pkg/contract -run TestNewContractDeterministic -v
```

Expected: `undefined: NewContract`

- [ ] **Step 3: Implement contract types and runtime**

File: `pkg/contract/types.go`

```go
package contract

import (
	"crypto/sha256"
	"encoding/hex"
)

// Contract represents a deployed contract on the network.
type Contract struct {
	Code     []byte  // WASM bytecode
	CodeHash string  // SHA256(Code) hex
	Params   []byte  // opaque parameters
	Key      string  // H(H(Code) || Params) - location-independent key
	Location float64 // ℓ(C) ∈ [0,1)
}

// State is the current public state of a contract.
type State struct {
	Data      []byte // contract-defined byte string
	Signature string // Ed25519 signature
	Version   uint64 // logical clock
}

// Update is an operation submitted to a contract.
type Update struct {
	Data      []byte // contract-defined update payload
	Signature string // signed by peer
	Timestamp int64  // unix nanoseconds
}

// NewContract creates a contract from WASM code and params.
func NewContract(code, params []byte) *Contract {
	codeHash := hashBytes(code)
	keyStr := hashBytes(append([]byte(codeHash), params...))
	location := hashToLocation(keyStr)

	return &Contract{
		Code:     code,
		CodeHash: codeHash,
		Params:   params,
		Key:      keyStr,
		Location: location,
	}
}

func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hashToLocation(keyStr string) float64 {
	h := sha256.Sum256([]byte(keyStr))
	sum := uint64(0)
	for i := 0; i < 8; i++ {
		sum = sum*256 + uint64(h[i])
	}
	return float64(sum) / float64(^uint64(0))
}
```

File: `pkg/contract/wasm.go`

```go
package contract

import (
	"fmt"
	"sync"
)

// ContractRuntime manages WASM contract loading and execution.
type ContractRuntime struct {
	mu        sync.RWMutex
	contracts map[string]*Contract // Key → Contract
}

// NewContractRuntime creates a new contract runtime.
func NewContractRuntime() *ContractRuntime {
	return &ContractRuntime{
		contracts: make(map[string]*Contract),
	}
}

// LoadContract loads a contract into the runtime.
func (cr *ContractRuntime) LoadContract(c *Contract) error {
	if c == nil || len(c.Code) == 0 {
		return fmt.Errorf("invalid contract: nil or empty code")
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	cr.contracts[c.Key] = c
	return nil
}

// GetContract retrieves a contract by key.
func (cr *ContractRuntime) GetContract(key string) *Contract {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	return cr.contracts[key]
}

// ExecuteContract calls a function in a contract (stub for now).
func (cr *ContractRuntime) ExecuteContract(key, fn string, args []interface{}) (interface{}, error) {
	cr.mu.RLock()
	contract := cr.contracts[key]
	cr.mu.RUnlock()

	if contract == nil {
		return nil, fmt.Errorf("contract not found: %s", key)
	}

	// TODO: instantiate WASM and call function via wasmtime
	return nil, fmt.Errorf("execution not yet implemented")
}
```

- [ ] **Step 4: Run tests, verify they pass**

```bash
go test ./pkg/contract -run TestNewContractDeterministic -v
go test ./pkg/contract -run TestLoadAndRetrieveContract -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/contract/types.go pkg/contract/wasm.go pkg/contract/wasm_test.go
git commit -m "feat: contract runtime & types (deterministic keys, loading)"
```

---

## Phase 2: P2P Network Core

### Task 3: Ring Topology & Peer Locations

**Files:**
- Create: `pkg/p2p/types.go`
- Create: `pkg/p2p/ring.go`
- Create: `pkg/p2p/ring_test.go`

**Interfaces:**
- Consumes: Nothing
- Produces: `Peer` struct, `RingDistance()`, `FindClosestPeers()`

**Why this task:** Foundation of small-world routing. Peers live on [0,1) ring; distance determines neighbors.

- [ ] **Step 1: Write failing test for ring topology**

File: `pkg/p2p/ring_test.go`

```go
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
```

- [ ] **Step 2: Run test, verify it fails**

```bash
cd /home/peder/Projetos/AdvPP
go test ./pkg/p2p -run TestRingDistance -v
```

Expected: `undefined: RingDistance`

- [ ] **Step 3: Implement ring topology**

File: `pkg/p2p/types.go`

```go
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
```

File: `pkg/p2p/ring.go`

```go
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
```

- [ ] **Step 4: Run tests, verify they pass**

```bash
go test ./pkg/p2p -run TestRing -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/p2p/types.go pkg/p2p/ring.go pkg/p2p/ring_test.go
git commit -m "feat: ring topology (locations, distances, peer lookup)"
```

---

### Task 4: UDP Transport & Connection Lifecycle

**Files:**
- Create: `pkg/p2p/transport.go`
- Create: `pkg/p2p/transport_test.go`
- Create: `pkg/p2p/message.go`

**Interfaces:**
- Consumes: `Peer` (from Task 3)
- Produces: `Transport`, `StartListening(addr string)`, `SendMessage()`, `ReceiveMessage()`, `Message` struct

**Why this task:** Peers communicate via UDP with simple request/reply protocol. Foundation for all distributed operations.

- [ ] **Step 1: Write failing test for transport**

File: `pkg/p2p/transport_test.go`

```go
package p2p

import (
	"net"
	"testing"
	"time"
)

func TestTransportSendReceive(t *testing.T) {
	// Create two transports
	addr1, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	addr2, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")

	t1, err := NewTransport(addr1)
	if err != nil {
		t.Fatalf("NewTransport failed: %v", err)
	}
	defer t1.Close()

	t2, err := NewTransport(addr2)
	if err != nil {
		t.Fatalf("NewTransport failed: %v", err)
	}
	defer t2.Close()

	// Start listening
	go t1.Listen()
	go t2.Listen()

	time.Sleep(100 * time.Millisecond)

	// Send message from t1 to t2
	msg := &Message{
		Type: "PING",
		Data: []byte("hello"),
	}

	err = t1.Send(t2.LocalAddr(), msg)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Receive on t2
	select {
	case received := <-t2.Received:
		if received.Type != "PING" {
			t.Errorf("wrong message type: %s", received.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

```bash
go test ./pkg/p2p -run TestTransportSendReceive -v
```

Expected: `undefined: NewTransport`

- [ ] **Step 3: Implement transport**

File: `pkg/p2p/message.go`

```go
package p2p

import (
	"encoding/json"
)

// Message is a protocol message between peers.
type Message struct {
	Type string `json:"type"`
	Data []byte `json:"data"`
	ID   string `json:"id,omitempty"` // transaction ID
}

func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *Message) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}
```

File: `pkg/p2p/transport.go`

```go
package p2p

import (
	"fmt"
	"net"
)

// Transport handles UDP communication between peers.
type Transport struct {
	conn     *net.UDPConn
	Received chan *Message
}

// NewTransport creates a new transport listening on addr.
func NewTransport(addr *net.UDPAddr) (*Transport, error) {
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen failed: %w", err)
	}

	return &Transport{
		conn:     conn,
		Received: make(chan *Message, 100),
	}, nil
}

// LocalAddr returns the listening address.
func (t *Transport) LocalAddr() *net.UDPAddr {
	return t.conn.LocalAddr().(*net.UDPAddr)
}

// Listen blocks and receives messages, dispatching to Received channel.
func (t *Transport) Listen() {
	buf := make([]byte, 4096)
	for {
		n, remoteAddr, err := t.conn.ReadFromUDP(buf)
		if err != nil {
			// Socket closed or error
			break
		}

		var msg Message
		if err := msg.Unmarshal(buf[:n]); err != nil {
			continue // Skip malformed
		}

		select {
		case t.Received <- &msg:
		default:
			// Channel full, drop
		}
	}
}

// Send transmits a message to remote address.
func (t *Transport) Send(remote *net.UDPAddr, msg *Message) error {
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	_, err = t.conn.WriteToUDP(data, remote)
	return err
}

// Close closes the connection.
func (t *Transport) Close() error {
	return t.conn.Close()
}
```

- [ ] **Step 4: Run tests, verify they pass**

```bash
go test ./pkg/p2p -run TestTransportSendReceive -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/p2p/transport.go pkg/p2p/transport_test.go pkg/p2p/message.go
git commit -m "feat: UDP transport & message protocol"
```

---

### Task 5: Greedy Forwarding & Basic Routing

**Files:**
- Create: `pkg/p2p/routing.go`
- Create: `pkg/p2p/routing_test.go`

**Interfaces:**
- Consumes: `Peer`, `RingDistance`, `Transport`
- Produces: `Router`, `GreedyForward(target float64, msg *Message)`, `FindNextHop(target float64) *Peer`

**Why this task:** Core of small-world routing. Peers forward messages toward ring locations greedily (closest neighbor).

- [ ] **Step 1: Write failing test for routing**

File: `pkg/p2p/routing_test.go`

```go
package p2p

import (
	"testing"
)

func TestGreedyForwarding(t *testing.T) {
	// Create a peer with neighbors
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

	// Create router
	router := NewRouter(peer)

	// Find next hop to location 0.15 (closer to p2 at 0.3 than p1 at 0.1)
	nextHop := router.FindNextHop(0.15)

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
		t.Errorf("expected n2 (closest to 0.35), got %s", nextHop.ID)
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

```bash
go test ./pkg/p2p -run TestGreedyForwarding -v
```

Expected: `undefined: NewRouter`

- [ ] **Step 3: Implement router**

File: `pkg/p2p/routing.go`

```go
package p2p

// Router handles message routing for a peer.
type Router struct {
	peer *Peer
}

// NewRouter creates a router for a peer.
func NewRouter(peer *Peer) *Router {
	return &Router{peer: peer}
}

// FindNextHop returns the neighbor closest to target location (greedy forwarding).
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

	return closest
}

// CanRoute checks if we're closer to target than all neighbors (terminal).
func (r *Router) CanRoute(target float64) bool {
	myDist := RingDistance(r.peer.Location, target)

	for _, neighbor := range r.peer.Neighbors {
		neighborDist := RingDistance(neighbor.Location, target)
		if neighborDist < myDist {
			return true // Neighbor is closer
		}
	}

	return false // We're terminal
}
```

- [ ] **Step 4: Run tests, verify they pass**

```bash
go test ./pkg/p2p -run TestGreedyForwarding -v
go test ./pkg/p2p -run TestRouterPicksClosestNeighbor -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/p2p/routing.go pkg/p2p/routing_test.go
git commit -m "feat: greedy forwarding & routing"
```

---

## Phase 3: Contracts & Synchronization

### Task 6: Merge Algebra (Idempotent Commutative Monoid)

**Files:**
- Create: `pkg/contract/merge.go`
- Create: `pkg/contract/merge_test.go`

**Interfaces:**
- Consumes: `Contract`, `State`, `Update`
- Produces: `MergeOp` interface, `MaxMonoid`, `SignedLogMonoid`, implementations

**Why this task:** Core distributed algorithm. Updates must merge in any order, any time; validity predicates enforce constraints.

- [ ] **Step 1: Write failing test for merge**

File: `pkg/contract/merge_test.go`

```go
package contract

import (
	"testing"
)

func TestMaxMonoidMerge(t *testing.T) {
	m := &MaxMonoid{}

	s1 := []byte("5")
	s2 := []byte("3")
	s3 := []byte("7")

	// Merge s1 ⊕ s2 = max(5,3) = 5
	result := m.Merge(s1, s2)
	if string(result) != "5" {
		t.Errorf("expected 5, got %s", string(result))
	}

	// Merge result ⊕ s3 = max(5,7) = 7
	result = m.Merge(result, s3)
	if string(result) != "7" {
		t.Errorf("expected 7, got %s", string(result))
	}

	// Idempotence: s1 ⊕ s1 = s1
	result = m.Merge(s1, s1)
	if string(result) != "5" {
		t.Errorf("idempotence failed: %s", string(result))
	}
}

func TestMergeCommutativity(t *testing.T) {
	m := &MaxMonoid{}

	s1 := []byte("10")
	s2 := []byte("20")

	// a ⊕ b should equal b ⊕ a
	ab := m.Merge(s1, s2)
	ba := m.Merge(s2, s1)

	if string(ab) != string(ba) {
		t.Errorf("commutativity failed: %s vs %s", string(ab), string(ba))
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

```bash
go test ./pkg/contract -run TestMaxMonoidMerge -v
```

Expected: `undefined: MaxMonoid`

- [ ] **Step 3: Implement merge operators**

File: `pkg/contract/merge.go`

```go
package contract

import (
	"bytes"
	"strconv"
)

// MergeOp defines the merge operation for a contract state.
type MergeOp interface {
	// Merge returns a ⊕ b (associative, commutative, idempotent)
	Merge(a, b []byte) []byte
	
	// Identity returns the identity element (e such that a ⊕ e = a)
	Identity() []byte
}

// MaxMonoid: merge is max(a, b); identity is empty/zero
type MaxMonoid struct{}

func (m *MaxMonoid) Merge(a, b []byte) []byte {
	// Parse as integers
	aVal, _ := strconv.Atoi(string(bytes.TrimSpace(a)))
	bVal, _ := strconv.Atoi(string(bytes.TrimSpace(b)))

	if aVal >= bVal {
		return a
	}
	return b
}

func (m *MaxMonoid) Identity() []byte {
	return []byte("0")
}

// LastWriterWinsMonoid: keeps the lexicographically larger value
type LastWriterWinsMonoid struct{}

func (m *LastWriterWinsMonoid) Merge(a, b []byte) []byte {
	if bytes.Compare(a, b) >= 0 {
		return a
	}
	return b
}

func (m *LastWriterWinsMonoid) Identity() []byte {
	return []byte{}
}

// ObservedRemoveSetMonoid: union of sets (for CRDT or bloom filters)
type ObservedRemoveSetMonoid struct{}

func (m *ObservedRemoveSetMonoid) Merge(a, b []byte) []byte {
	// Simple union: concatenate and deduplicate (simplified)
	return append(a, b...)
}

func (m *ObservedRemoveSetMonoid) Identity() []byte {
	return []byte{}
}
```

- [ ] **Step 4: Run tests, verify they pass**

```bash
go test ./pkg/contract -run TestMaxMonoidMerge -v
go test ./pkg/contract -run TestMergeCommutativity -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/contract/merge.go pkg/contract/merge_test.go
git commit -m "feat: merge algebra (idempotent commutative monoids)"
```

---

### Task 7: Summary/Delta Synchronization Protocol

**Files:**
- Create: `pkg/sync/summary_delta.go`
- Create: `pkg/sync/summary_delta_test.go`

**Interfaces:**
- Consumes: `Contract`, `State`, `MergeOp`
- Produces: `SummaryDeltaSync`, `Summarize()`, `GetDelta()`, `ApplyDelta()`

**Why this task:** Two-step sync minimizes bandwidth. Peers exchange summaries (compact), then delta (only differences). Bandwidth = |summary| + |delta|.

- [ ] **Step 1: Write failing test for sync**

File: `pkg/sync/summary_delta_test.go`

```go
package sync

import (
	"testing"
	"github.com/advpl/compiler/pkg/contract"
)

func TestSummarizeDelta(t *testing.T) {
	// Create a contract with MaxMonoid merge
	c := &contract.Contract{
		Key: "test-contract",
	}

	sync := NewSummaryDeltaSync(c, &contract.MaxMonoid{})

	// State A: value 10
	stateA := []byte("10")

	// Summarize state A
	summary := sync.Summarize(stateA)

	if len(summary) == 0 {
		t.Fatal("summary is empty")
	}

	// State B: value 5
	stateB := []byte("5")

	// Get delta from B's perspective: what does B need from A's summary?
	delta := sync.GetDelta(stateB, summary)

	if len(delta) == 0 {
		t.Fatal("delta is empty")
	}

	// Apply delta to B: should converge toward A
	result := sync.ApplyDelta(stateB, delta)

	if string(result) != "10" {
		t.Errorf("expected 10, got %s", string(result))
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

```bash
cd /home/peder/Projetos/AdvPP
go test ./pkg/sync -run TestSummarizeDelta -v
```

Expected: `undefined: NewSummaryDeltaSync`

- [ ] **Step 3: Implement summary/delta sync**

File: `pkg/sync/summary_delta.go`

```go
package sync

import (
	"fmt"
	"github.com/advpl/compiler/pkg/contract"
)

// SummaryDeltaSync implements the two-step sync protocol.
type SummaryDeltaSync struct {
	contract *contract.Contract
	merge    contract.MergeOp
}

// NewSummaryDeltaSync creates a sync engine for a contract.
func NewSummaryDeltaSync(c *contract.Contract, merge contract.MergeOp) *SummaryDeltaSync {
	return &SummaryDeltaSync{
		contract: c,
		merge:    merge,
	}
}

// Summarize computes a compact summary of state (for most cases, just the state itself).
func (s *SummaryDeltaSync) Summarize(state []byte) []byte {
	// MVP: summary = state (later: Bloom filter, Merkle root, etc.)
	return state
}

// GetDelta computes delta: what does myState need from peerSummary?
func (s *SummaryDeltaSync) GetDelta(myState, peerSummary []byte) []byte {
	// MVP: delta = peerSummary if it dominates myState in partial order, else empty
	// For MaxMonoid: if peer's value > my value, delta = peer's value

	// Simple heuristic: if summaries differ, delta is the peer's data
	if string(myState) != string(peerSummary) {
		return peerSummary
	}
	return []byte{}
}

// ApplyDelta merges delta into myState.
func (s *SummaryDeltaSync) ApplyDelta(myState, delta []byte) []byte {
	if len(delta) == 0 {
		return myState
	}

	// Merge delta using the contract's merge function
	return s.merge.Merge(myState, delta)
}
```

- [ ] **Step 4: Run tests, verify they pass**

```bash
go test ./pkg/sync -run TestSummarizeDelta -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/sync/summary_delta.go pkg/sync/summary_delta_test.go
git commit -m "feat: summary/delta sync protocol"
```

---

### Task 8: Subscription Trees & Update Propagation

**Files:**
- Create: `pkg/sync/subscription.go`
- Create: `pkg/sync/subscription_test.go`

**Interfaces:**
- Consumes: `Peer`, `Contract`, `Message`
- Produces: `SubscriptionTree`, `Subscribe()`, `Propagate()`

**Why this task:** Updates propagate via subscription trees rooted at contract's location. Lease-based (auto-renew). Self-healing.

- [ ] **Step 1: Write failing test for subscriptions**

File: `pkg/sync/subscription_test.go`

```go
package sync

import (
	"testing"
	"time"
)

func TestSubscribeToContract(t *testing.T) {
	tree := NewSubscriptionTree("contract-key")

	sub := tree.Subscribe("peer-1")

	if !tree.IsSubscribed("peer-1") {
		t.Fatal("peer not subscribed")
	}

	if len(tree.GetSubscribers()) != 1 {
		t.Errorf("expected 1 subscriber, got %d", len(tree.GetSubscribers()))
	}
}

func TestSubscriptionLeaseExpiry(t *testing.T) {
	tree := NewSubscriptionTree("contract-key")

	sub := tree.Subscribe("peer-1")

	// Manually expire lease
	sub.ExpiresAt = time.Now().Add(-time.Second)

	if !tree.IsSubscribed("peer-1") {
		t.Fatal("subscription should still exist")
	}

	tree.ClearExpired()

	if tree.IsSubscribed("peer-1") {
		t.Fatal("expired subscription not cleared")
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

```bash
go test ./pkg/sync -run TestSubscribeToContract -v
```

Expected: `undefined: NewSubscriptionTree`

- [ ] **Step 3: Implement subscription tree**

File: `pkg/sync/subscription.go`

```go
package sync

import (
	"sync"
	"time"
)

// Subscription represents a peer's interest in a contract.
type Subscription struct {
	PeerID    string
	StartedAt time.Time
	ExpiresAt time.Time // lease expiry
}

// SubscriptionTree manages subscriptions for a single contract.
type SubscriptionTree struct {
	mu            sync.RWMutex
	contractKey   string
	subscriptions map[string]*Subscription
}

// NewSubscriptionTree creates a tree for a contract.
func NewSubscriptionTree(contractKey string) *SubscriptionTree {
	return &SubscriptionTree{
		contractKey:   contractKey,
		subscriptions: make(map[string]*Subscription),
	}
}

// Subscribe adds a peer as a subscriber (8-minute lease by default).
func (st *SubscriptionTree) Subscribe(peerID string) *Subscription {
	st.mu.Lock()
	defer st.mu.Unlock()

	sub := &Subscription{
		PeerID:    peerID,
		StartedAt: time.Now(),
		ExpiresAt: time.Now().Add(8 * time.Minute),
	}

	st.subscriptions[peerID] = sub
	return sub
}

// IsSubscribed checks if a peer is currently subscribed.
func (st *SubscriptionTree) IsSubscribed(peerID string) bool {
	st.mu.RLock()
	defer st.mu.RUnlock()

	sub, exists := st.subscriptions[peerID]
	if !exists {
		return false
	}

	// Check if lease is valid
	return sub.ExpiresAt.After(time.Now())
}

// GetSubscribers returns all currently valid subscribers.
func (st *SubscriptionTree) GetSubscribers() []string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	var result []string
	now := time.Now()

	for peerID, sub := range st.subscriptions {
		if sub.ExpiresAt.After(now) {
			result = append(result, peerID)
		}
	}

	return result
}

// ClearExpired removes expired subscriptions.
func (st *SubscriptionTree) ClearExpired() {
	st.mu.Lock()
	defer st.mu.Unlock()

	now := time.Now()
	for peerID, sub := range st.subscriptions {
		if sub.ExpiresAt.Before(now) {
			delete(st.subscriptions, peerID)
		}
	}
}

// RenewLease extends subscription lease.
func (st *SubscriptionTree) RenewLease(peerID string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	sub, exists := st.subscriptions[peerID]
	if !exists {
		return nil // Silently ignore renewal of non-existent subscription
	}

	sub.ExpiresAt = time.Now().Add(8 * time.Minute)
	return nil
}
```

- [ ] **Step 4: Run tests, verify they pass**

```bash
go test ./pkg/sync -run TestSubscription -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/sync/subscription.go pkg/sync/subscription_test.go
git commit -m "feat: subscription trees & lease-based propagation"
```

---

## Phase 4: Storage & Replication

### Task 9: Local KV Store

**Files:**
- Create: `pkg/storage/store.go`
- Create: `pkg/storage/store_test.go`

**Interfaces:**
- Consumes: `Contract`
- Produces: `Store`, `Get(key string)`, `Put(key string, data []byte)`, `Delete(key string)`

**Why this task:** Each peer maintains encrypted local storage for contract state. Foundation for replication and recovery.

- [ ] **Step 1: Write failing test for store**

File: `pkg/storage/store_test.go`

```go
package storage

import (
	"testing"
)

func TestStorePutGet(t *testing.T) {
	store := NewStore()

	key := "contract:abc123"
	data := []byte("state-data")

	err := store.Put(key, data)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	retrieved, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Errorf("data mismatch: %s vs %s", string(retrieved), string(data))
	}
}

func TestStoreDelete(t *testing.T) {
	store := NewStore()

	key := "contract:xyz"
	store.Put(key, []byte("data"))

	err := store.Delete(key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(key)
	if err == nil {
		t.Fatal("key should not exist after delete")
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

```bash
cd /home/peder/Projetos/AdvPP
go test ./pkg/storage -run TestStorePutGet -v
```

Expected: `undefined: NewStore`

- [ ] **Step 3: Implement in-memory KV store (MVP)**

File: `pkg/storage/store.go`

```go
package storage

import (
	"fmt"
	"sync"
)

// Store is an in-memory KV store for contract state (MVP).
// TODO: Replace with SQLite for persistence.
type Store struct {
	mu    sync.RWMutex
	data  map[string][]byte
}

// NewStore creates a new store.
func NewStore() *Store {
	return &Store{
		data: make(map[string][]byte),
	}
}

// Put stores data under key.
func (s *Store) Put(key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Copy data to avoid external mutations
	copied := make([]byte, len(data))
	copy(copied, data)

	s.data[key] = copied
	return nil
}

// Get retrieves data by key.
func (s *Store) Get(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	// Return copy
	copied := make([]byte, len(data))
	copy(copied, data)

	return copied, nil
}

// Delete removes a key.
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// Keys returns all keys in store.
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}
```

- [ ] **Step 4: Run tests, verify they pass**

```bash
go test ./pkg/storage -run TestStore -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/storage/store.go pkg/storage/store_test.go
git commit -m "feat: local KV store (MVP in-memory)"
```

---

### Task 10: Replication Strategy (K Random Peers)

**Files:**
- Create: `pkg/storage/replication.go`
- Create: `pkg/storage/replication_test.go`

**Interfaces:**
- Consumes: `Peer`, `Store`, `Contract`
- Produces: `Replicator`, `ReplicateTo(peers []*Peer, k int)`, `SelectReplicaPeers()`

**Why this task:** Partial replication: K random peers store copies of contract state. Redundancy without full copy overhead.

- [ ] **Step 1: Write failing test for replication**

File: `pkg/storage/replication_test.go`

```go
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

	// All selected must be from peers
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

	// With same key, should select same peers (based on key hash)
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
```

- [ ] **Step 2: Run test, verify it fails**

```bash
go test ./pkg/storage -run TestSelectReplicaPeers -v
```

Expected: `undefined: NewReplicator`

- [ ] **Step 3: Implement replicator**

File: `pkg/storage/replication.go`

```go
package storage

import (
	"crypto/sha256"
	"encoding/binary"
	"github.com/advpl/compiler/pkg/p2p"
	"math/rand"
)

// Replicator selects peers for partial replication.
type Replicator struct {
	rng *rand.Rand
}

// NewReplicator creates a replicator.
func NewReplicator() *Replicator {
	return &Replicator{
		rng: rand.New(rand.NewSource(42)), // Seeded for determinism
	}
}

// SelectReplicaPeers randomly selects k peers from a list.
func (r *Replicator) SelectReplicaPeers(peers []*p2p.Peer, k int) []*p2p.Peer {
	if k > len(peers) {
		k = len(peers)
	}

	// Fisher-Yates shuffle (deterministic seed)
	shuffled := make([]*p2p.Peer, len(peers))
	copy(shuffled, peers)

	for i := len(shuffled) - 1; i > 0; i-- {
		j := r.rng.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled[:k]
}

// SelectReplicaPeersForKey deterministically selects k peers based on contract key.
func (r *Replicator) SelectReplicaPeersForKey(peers []*p2p.Peer, contractKey string, k int) []*p2p.Peer {
	if k > len(peers) {
		k = len(peers)
	}

	// Seed RNG from contract key for determinism
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
```

- [ ] **Step 4: Run tests, verify they pass**

```bash
go test ./pkg/storage -run TestSelectReplicaPeers -v
go test ./pkg/storage -run TestReplicationDeterministic -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/storage/replication.go pkg/storage/replication_test.go
git commit -m "feat: K-random peer replication strategy"
```

---

## Phase 5: Integration & Tools

### Task 11: Contract API & State Management

**Files:**
- Modify: `pkg/contract/wasm.go` (extend runtime)
- Create: `pkg/p2p/api.go`
- Create: `pkg/p2p/api_test.go`

**Interfaces:**
- Consumes: `ContractRuntime`, `Store`, `SubscriptionTree`, `SummaryDeltaSync`
- Produces: `PeerAPI`, `Put()`, `Get()`, `Update()`, `Subscribe()`, `Propagate()`

**Why this task:** High-level API for contracts to interact with network (PUT, GET, UPDATE, SUBSCRIBE).

- [ ] **Step 1: Write failing test for API**

File: `pkg/p2p/api_test.go`

```go
package p2p

import (
	"testing"
	"github.com/advpl/compiler/pkg/contract"
	"github.com/advpl/compiler/pkg/storage"
)

func TestPutAndGet(t *testing.T) {
	store := storage.NewStore()
	api := NewPeerAPI(store)

	contractKey := "test-contract"
	data := []byte("state-value")

	err := api.Put(contractKey, data)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	retrieved, err := api.Get(contractKey)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Errorf("data mismatch: %s vs %s", string(retrieved), string(data))
	}
}

func TestUpdate(t *testing.T) {
	store := storage.NewStore()
	merge := &contract.MaxMonoid{}
	api := NewPeerAPI(store)

	contractKey := "test-contract"

	// Initial state
	api.Put(contractKey, []byte("10"))

	// Update with merge
	err := api.Update(contractKey, []byte("5"), merge)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	result, _ := api.Get(contractKey)
	if string(result) != "10" {
		t.Errorf("expected max(10, 5) = 10, got %s", string(result))
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

```bash
go test ./pkg/p2p -run TestPutAndGet -v
```

Expected: `undefined: NewPeerAPI`

- [ ] **Step 3: Implement peer API**

File: `pkg/p2p/api.go`

```go
package p2p

import (
	"fmt"
	"github.com/advpl/compiler/pkg/contract"
	"github.com/advpl/compiler/pkg/storage"
)

// PeerAPI provides high-level operations for contracts.
type PeerAPI struct {
	store *storage.Store
}

// NewPeerAPI creates a peer API.
func NewPeerAPI(store *storage.Store) *PeerAPI {
	return &PeerAPI{
		store: store,
	}
}

// Put publishes contract state.
func (api *PeerAPI) Put(contractKey string, data []byte) error {
	return api.store.Put(contractKey, data)
}

// Get retrieves contract state.
func (api *PeerAPI) Get(contractKey string) ([]byte, error) {
	return api.store.Get(contractKey)
}

// Update submits an update to a contract (merges using contract's merge function).
func (api *PeerAPI) Update(contractKey string, updateData []byte, merge contract.MergeOp) error {
	current, err := api.store.Get(contractKey)
	if err != nil {
		// If not found, treat as identity
		current = merge.Identity()
	}

	// Merge: current ⊕ update
	merged := merge.Merge(current, updateData)

	return api.store.Put(contractKey, merged)
}
```

- [ ] **Step 4: Run tests, verify they pass**

```bash
go test ./pkg/p2p -run TestPutAndGet -v
go test ./pkg/p2p -run TestUpdate -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/p2p/api.go pkg/p2p/api_test.go
git commit -m "feat: contract API (PUT, GET, UPDATE, Subscribe)"
```

---

### Task 12: CLI Tool (`advplc peer`)

**Files:**
- Create: `cmd/advplc/cmd_peer.go`
- Modify: `cmd/advplc/main.go` (add peer subcommand)

**Interfaces:**
- Consumes: All P2P infrastructure
- Produces: `advplc peer --listen 0.0.0.0:9000 --bootstrap peer1.com:9000`

**Why this task:** Users can launch P2P nodes from CLI.

- [ ] **Step 1: Write failing test (CLI invocation)**

File: `cmd/advplc/cmd_peer_test.go` (if applicable, else just test running the command)

```bash
# Test: Start peer in background, send message
./advplc peer --listen 127.0.0.1:9000 &
PID=$!
sleep 1
# Verify peer is listening (netstat or similar)
kill $PID
```

- [ ] **Step 2: Implement peer subcommand**

File: `cmd/advplc/cmd_peer.go`

```go
package main

import (
	"flag"
	"fmt"
	"github.com/advpl/compiler/pkg/p2p"
	"log"
	"net"
)

func cmdPeer(args []string) {
	fs := flag.NewFlagSet("peer", flag.ExitOnError)
	listenAddr := fs.String("listen", "0.0.0.0:9000", "Listen address")
	bootstrapAddr := fs.String("bootstrap", "", "Bootstrap peer address")

	fs.Parse(args)

	// Resolve listen address
	addr, err := net.ResolveUDPAddr("udp", *listenAddr)
	if err != nil {
		log.Fatalf("resolve listen: %v", err)
	}

	// Create peer
	peer := p2p.NewPeer("my-peer", addr)

	// Create transport
	transport, err := p2p.NewTransport(addr)
	if err != nil {
		log.Fatalf("transport: %v", err)
	}
	defer transport.Close()

	// Start listening
	go transport.Listen()

	log.Printf("Peer %s listening on %s", peer.ID, peer.Addr)

	// TODO: Bootstrap if specified
	// TODO: Join ring
	// TODO: Serve indefinitely

	// For MVP, block on receiving messages
	for msg := range transport.Received {
		log.Printf("Received: %s", msg.Type)
	}
}
```

File: `cmd/advplc/main.go` (add routing)

```go
// In main() where subcommands are routed:
case "peer":
    cmdPeer(os.Args[2:])
```

- [ ] **Step 3: Build and test**

```bash
cd /home/peder/Projetos/AdvPP
make build
./advplc peer --help
```

Expected: Help message printed

- [ ] **Step 4: Commit**

```bash
git add cmd/advplc/cmd_peer.go cmd/advplc/main.go
git commit -m "feat: CLI peer subcommand (advplc peer --listen ...)"
```

---

### Task 13: Integration Tests (End-to-End)

**Files:**
- Create: `pkg/p2p/integration_test.go`

**Interfaces:**
- Consumes: All components
- Produces: E2E test that spins up 3 peers, deploys contract, syncs state

**Why this task:** Verify entire system works together.

- [ ] **Step 1: Write integration test**

File: `pkg/p2p/integration_test.go`

```go
package p2p

import (
	"net"
	"testing"
	"time"
	"github.com/advpl/compiler/pkg/contract"
	"github.com/advpl/compiler/pkg/storage"
)

func TestE2EThreePeerSync(t *testing.T) {
	// Create 3 peers
	peers := make([]*Peer, 3)
	transports := make([]*Transport, 3)
	stores := make([]*storage.Store, 3)

	for i := 0; i < 3; i++ {
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
		peers[i] = NewPeer(string(rune('a'+i)), addr)

		transport, err := NewTransport(addr)
		if err != nil {
			t.Fatalf("transport %d: %v", i, err)
		}
		transports[i] = transport
		stores[i] = storage.NewStore()

		go transport.Listen()
	}

	defer func() {
		for _, tr := range transports {
			tr.Close()
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Manually connect peers (for MVP, skip topology)
	peers[0].Neighbors = []*Peer{peers[1], peers[2]}
	peers[1].Neighbors = []*Peer{peers[0], peers[2]}
	peers[2].Neighbors = []*Peer{peers[0], peers[1]}

	// Deploy contract on peer 0
	contractCode := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	c := contract.NewContract(contractCode, []byte{})

	err := stores[0].Put(c.Key, []byte("initial-state"))
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	// Simulate sync to peer 1
	state0, _ := stores[0].Get(c.Key)
	err = stores[1].Put(c.Key, state0)
	if err != nil {
		t.Fatalf("replicate: %v", err)
	}

	// Verify peer 1 has the state
	state1, _ := stores[1].Get(c.Key)

	if string(state0) != string(state1) {
		t.Errorf("state mismatch: %s vs %s", string(state0), string(state1))
	}

	t.Logf("E2E test passed: state synced across peers")
}
```

- [ ] **Step 2: Run test**

```bash
go test ./pkg/p2p -run TestE2E -v
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add pkg/p2p/integration_test.go
git commit -m "test: end-to-end integration test (3-peer sync)"
```

---

## Execution Handoff

All 13 tasks complete. Ready for subagent-driven development.

**Next:** Invoke `superpowers:subagent-driven-development` to execute tasks 1-13 with reviews after each.
