# AdvPP Freenet Distributed Chat PoC

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Demonstrate that AdvPP v2.0 P2P infrastructure works end-to-end with a real application: a distributed chat room where multiple peers sync messages across a Freenet-style network.

**Architecture:** AdvPL application deployed as standalone executables (compiled via `advplc build`). Each peer runs the same AdvPL code, joins a local network (ring topology on localhost), subscribes to a shared chat contract, and displays synced messages in real-time.

**Tech Stack:** AdvPL/TLPP (application), AdvPP v2.0 (compiler + P2P runtime), SQLite (per-peer storage), JSON (contract state), UDP (transport).

## Global Constraints

- **3 peers minimum:** Chat must demonstrate multi-node sync (not a single-peer database)
- **WASM contracts:** Chat logic lives in a WASM contract deployed to the network, not in the AdvPL app
- **Eventual consistency:** Messages must converge across all peers within 1 second
- **No centralized coordination:** Peers discover each other via ring topology, not a bootstrap server
- **Merge semantics:** Message log uses append-only merge (idempotent + commutative) — duplicates ignored
- **Persisted state:** Messages survive peer restart (loaded from local store on startup)
- **Language:** All source code in AdvPL/TLPP; compilation via `advplc`

---

## Design

### Chat Contract (WASM)

**State:** JSON array of messages.
```json
[
  {"from": "peer-a", "text": "hello", "ts": 1234567890},
  {"from": "peer-b", "text": "hi there", "ts": 1234567891}
]
```

**Merge:** Append-only with last-write-wins on timestamp. Receiving a message from another peer triggers merge:
- If message is new (not in local list), append it
- If message exists (same `from` + `ts`), ignore (idempotent)
- Final state is union of all messages, sorted by timestamp

**Contract Interface:**
- `GetMessages()` → JSON array (read chat history)
- `AddMessage(cFrom, cText)` → boolean (append new message with current timestamp, triggers merge)

### AdvPL Application (chat.prw)

**Startup:**
1. Parse command-line arguments: `--peer-id p1 --listen 127.0.0.1:9000 --bootstrap 127.0.0.1:9001`
2. Initialize P2P peer (join ring, discover neighbors)
3. Deploy or subscribe to chat contract (key: `contract:chat-main`)
4. Enter UI loop

**UI Loop:**
```
Main Menu:
  1. Read messages (fetch from contract)
  2. Send message (type text, add to contract)
  3. Peer info (show local state, neighbors, subscribers)
  4. Exit
```

**Key Functions:**
- `ChatInit()` — peer setup, P2P initialization
- `ChatRead()` — fetch messages from contract via GET, display formatted
- `ChatSend()` — prompt for text, call contract's AddMessage, wait for sync confirmation
- `ChatSubscribe()` — register peer as subscriber to chat contract
- `ChatDisplay()` — format and print message list (from, text, timestamp)

### Network Setup (Test Harness)

**3 Standalone Binaries:**
- `chat-peer-1`: `advplc build chat.prw -o chat-peer-1 -X "--peer-id p1 --listen 127.0.0.1:9000"`
- `chat-peer-2`: `advplc build chat.prw -o chat-peer-2 -X "--peer-id p2 --listen 127.0.0.1:9001"`
- `chat-peer-3`: `advplc build chat.prw -o chat-peer-3 -X "--peer-id p3 --listen 127.0.0.1:9002"`

**Topology:** Each peer knows 1-2 neighbors (bootstrapped manually via CLI args).

**Execution Flow:**
1. Start peer-1 on port 9000 (bootstrap)
2. Start peer-2 on port 9001, bootstrap to 9000
3. Start peer-3 on port 9002, bootstrap to 9001
4. Type message on peer-1 → peers 2 & 3 see it within 1 second
5. Type message on peer-2 → peers 1 & 3 see it
6. Restart peer-1 → it reads stored messages and syncs any new ones

---

## Success Criteria

| Criterion | Validation |
|-----------|-----------|
| **Network formation** | All 3 peers start, establish neighbors, no errors |
| **Message sync** | Message typed on peer A appears on peers B & C within 1s |
| **Convergence** | After all messages settled, all 3 peers show identical chat history |
| **Persistence** | Kill peer-1, restart it, it shows all previous messages |
| **No duplicates** | No message appears twice (merge idempotence) |
| **Merge order-independence** | Message order same on all peers regardless of send order |

---

## Out of Scope

- **User authentication:** All peers trusted (no signatures yet)
- **Encryption:** Messages plaintext (oblivious storage deferred to future)
- **Web UI:** CLI only
- **Scalability:** 3 peers on localhost only (not 100+ WAN peers)
- **Advanced chat features:** No threading, reactions, user profiles, etc.

---

## Files to Create

- `examples/chat/chat.prw` — Main AdvPL application
- `examples/chat/chat-contract.prw` — Contract logic (compiled to WASM)
- `examples/chat/BUILD.sh` — Shell script to compile 3 peers
- `examples/chat/README.md` — How to run the PoC
- `examples/chat/test-scenario.md` — Step-by-step test plan

---

## Acceptance Tests

1. **Boot test:** All 3 binaries start without errors
2. **Send test:** Type message on peer-1, confirm on peers 2 & 3 within 1s
3. **Sync test:** Type messages simultaneously on all 3 peers, verify convergence
4. **Restart test:** Kill peer-1, restart it, verify it recovers all messages
5. **Chaos test:** Kill peer-2 mid-message, resume, verify state consistency

---

## Notes

- This PoC validates the **infrastructure** (routing, sync, merge) more than the **application** (chat is trivial)
- Success proves: WASM contracts work, P2P routing works, merge algebra works, replication works
- Failure modes will pinpoint which layer (network, contract, storage, sync) needs debug
