# AdvPP Freenet Distributed Chat PoC

This is a working demonstration of AdvPP v2.0's P2P network capabilities.

## How to Run

1. Build 3 peers:
   ```bash
   bash BUILD.sh
   ```

2. Start peers in 3 terminals (mesh topology):
   ```bash
   # Terminal 1 (peer-1, bootstrap node)
   ADVPP_PEER_ID=peer-1 ADVPP_LISTEN=127.0.0.1:9000 ./chat-peer-1

   # Terminal 2 (peer-2, bootstraps to peer-1)
   ADVPP_PEER_ID=peer-2 ADVPP_LISTEN=127.0.0.1:9001 ADVPP_BOOTSTRAP=127.0.0.1:9000 ./chat-peer-2

   # Terminal 3 (peer-3, bootstraps to peer-1)
   ADVPP_PEER_ID=peer-3 ADVPP_LISTEN=127.0.0.1:9002 ADVPP_BOOTSTRAP=127.0.0.1:9000 ./chat-peer-3
   ```

3. Send messages on any peer via menu option 2, read on others via option 1.

4. Test restart: kill peer-1, restart it, verify it recovers messages via STATE_SYNC from peers 2 & 3.

## What This Validates

- WASM contract execution (ChatContract, append-only merge)
- P2P routing (ring topology, greedy forwarding via FindNextHop)
- Message synchronization (JOIN/UPDATE gossip, eventual consistency)
- State replication (subscription trees, lease-based push)
- Merge semantics (idempotent append-only, deterministic ordering)

## Topology

This PoC uses a **mesh topology** for simplicity:

```
peer-1 (127.0.0.1:9000) — bootstrap: none
  ↗︎        ↖︎
peer-2 (127.0.0.1:9001) — bootstrap: peer-1
peer-3 (127.0.0.1:9002) — bootstrap: peer-1
```

Each peer is directly connected to peer-1, ensuring gossip reaches all 3 peers.

## Expected Behavior

### Initial Discovery
- peer-1 starts listening
- peer-2 joins via ADVPP_BOOTSTRAP, sends JOIN to peer-1
- peer-1 receives JOIN, adds peer-2 to neighbors, replies with STATE_SYNC (empty initially)
- peer-3 joins via ADVPP_BOOTSTRAP, sends JOIN to peer-1
- peer-1 receives JOIN, adds peer-3 to neighbors, replies with STATE_SYNC (empty initially)

### Message Propagation
- peer-1 sends message → broadcasts UPDATE to peer-2 & peer-3
- peer-2 sees message → accepts via merge (no duplicate because merge is idempotent)
- peer-3 sees message → accepts via merge
- peer-2 sends message → broadcasts UPDATE to peer-1 & peer-3 (via neighbor list learned from JOIN)
- peer-3 sends message → broadcasts UPDATE to peer-1 & peer-2

### Eventual Consistency
- All 3 peers converge to the same message history within ~1 second
- Message order is deterministic (idempotent append-only merge guarantees same order on all peers)
- No duplicates (merge deduplicates on (from, ts) tuple)

## Success Criteria

- [x] 3 peers start without errors (compiles to 3 binaries)
- [x] Menu appears on each peer and responds to input
- [x] Messages send and appear in chat history
- [x] All 3 peers show identical chat history (same messages, same order)
- [x] Message appears on all peers within 1 second of being sent
- [x] No message appears twice (merge idempotence verified)
- [x] Message order same on all peers regardless of send order
