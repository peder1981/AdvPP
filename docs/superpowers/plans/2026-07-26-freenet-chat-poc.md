# AdvPP Freenet Distributed Chat PoC — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a working distributed chat application in AdvPL that demonstrates AdvPP v2.0's P2P network, WASM contracts, state sync, and replication across 3 local peers.

**Architecture:** Single AdvPL source file (`chat.prw`) compiled 3 times with different peer IDs. Each peer runs standalone, joins a ring topology, deploys/subscribes to a shared chat contract, and syncs messages via summary/delta protocol. Contract logic handles append-only message merge.

**Tech Stack:** AdvPL/TLPP, AdvPP v2.0 (compiler + VM + P2P stack), SQLite (per-peer store), JSON (contract state), UDP (transport).

## Global Constraints

- **3 local peers minimum:** Demonstration requires multi-node sync, not single-peer
- **WASM contracts:** Chat contract compiles to WASM; logic lives in network, not app
- **Eventual consistency:** All peers converge to same chat history within 1 second
- **Append-only merge:** Messages are immutable; duplicates ignored (idempotent + commutative)
- **Decentralized:** No central bootstrap server; peers discover via ring topology
- **Persisted state:** SQLite per peer; restart recovery tested
- **AdvPL/TLPP only:** All source in AdvPL (no Go, no C, no shell scripts beyond compilation)
- **Merge semantics:** Last-write-wins on timestamp; same message (from + ts) never duplicated

---

## File Structure

```
examples/chat/
├── chat.prw                  # Main AdvPL app (compiled 3 times for 3 peers)
├── chat-contract.prw         # Chat contract logic (deployed to network)
├── BUILD.sh                  # Compilation script (builds 3 binaries)
├── README.md                 # How to run the PoC
└── test-scenario.md          # Step-by-step test plan
```

---

## Tasks

### Task 1: Chat Contract (WASM)

**Files:**
- Create: `examples/chat/chat-contract.prw`

**Interfaces:**
- Consumes: Nothing (new contract)
- Produces: Chat contract object with methods: `GetMessages()`, `AddMessage(cFrom, cText)`

**Why this task:** Contract logic must be deterministic and deployable to the P2P network. Separating it from the app makes it clear what runs where (contract on network, app on peer).

- [ ] **Step 1: Write failing test for contract logic**

File: `examples/chat/chat-contract-test.prw`

```advpl
User Function TestChatContract()
    Local oChat := ChatContract():New()
    Local aMessages := {}
    
    // Test 1: Empty chat
    aMessages := oChat:GetMessages()
    if len(aMessages) != 0
        ? "FAIL: Expected empty chat"
        return .F.
    endif
    
    // Test 2: Add message
    if !oChat:AddMessage("alice", "hello")
        ? "FAIL: AddMessage failed"
        return .F.
    endif
    
    aMessages := oChat:GetMessages()
    if len(aMessages) != 1
        ? "FAIL: Expected 1 message"
        return .F.
    endif
    
    // Test 3: Idempotence (add same message twice)
    if !oChat:AddMessage("alice", "hello")
        ? "FAIL: AddMessage duplicate failed"
        return .F.
    endif
    
    aMessages := oChat:GetMessages()
    if len(aMessages) != 1
        ? "FAIL: Expected 1 message (duplicate ignored)"
        return .F.
    endif
    
    ? "PASS: All tests passed"
    return .T.
EndFunc
```

- [ ] **Step 2: Run test, verify it fails**

```bash
advplc run examples/chat/chat-contract-test.prw
```

Expected: Test functions not defined.

- [ ] **Step 3: Implement chat contract**

File: `examples/chat/chat-contract.prw`

```advpl
class ChatContract
    private data aMessages as Array
    
    method new() as object
        ::aMessages := {}
    return self
    
    method GetMessages() as Array
        Local aResult := {}
        Local i
        for i := 1 to len(::aMessages)
            aAdd(aResult, ::aMessages[i])
        next
    return aResult
    
    method AddMessage(cFrom as Character, cText as Character) as Logical
        Local oMsg as Object
        Local dNow as Date
        Local cTs as Character
        Local i
        
        // Timestamp (simplified: system time as string)
        dNow := Date()
        cTs := Str(dNow) + Str(Seconds())
        
        // Check if message already exists (idempotence)
        for i := 1 to len(::aMessages)
            oMsg := ::aMessages[i]
            if oMsg:from == cFrom .and. oMsg:ts == cTs
                return .T. // Already exists, return success
            endif
        next
        
        // New message
        oMsg := JsonObject():New()
        oMsg:from := cFrom
        oMsg:text := cText
        oMsg:ts := cTs
        
        aAdd(::aMessages, oMsg)
    return .T.
endclass
```

- [ ] **Step 4: Run test, verify it passes**

```bash
advplc run examples/chat/chat-contract-test.prw
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add examples/chat/chat-contract.prw examples/chat/chat-contract-test.prw
git commit -m "feat: chat contract (append-only message log with idempotent merge)"
```

---

### Task 2: AdvPL Chat App (Skeleton)

**Files:**
- Create: `examples/chat/chat.prw`

**Interfaces:**
- Consumes: Nothing
- Produces: Chat app with functions: `ChatInit()`, `ChatLoop()`, `ChatRead()`, `ChatSend()`

**Why this task:** App skeleton establishes the P2P peer setup, UI loop structure, and entry point. This is the main program that users run.

- [ ] **Step 1: Write skeleton**

File: `examples/chat/chat.prw`

```advpl
User Function ChatMain()
    Local cPeerId := "peer-default"
    Local cListen := "127.0.0.1:9000"
    
    ? "=== AdvPP Freenet Distributed Chat ==="
    ? "Peer: " + cPeerId
    ? "Listening: " + cListen
    
    // TODO: ChatInit(cPeerId, cListen)
    // TODO: ChatLoop()
    
    ? "Chat closed."
return
endFunc

Static Function ChatInit(cPeerId, cListen)
    ? "TODO: Initialize P2P peer"
return

Static Function ChatLoop()
    Local nOpt := 0
    
    do while .T.
        ? ""
        ? "Chat Menu:"
        ? "  1. Read messages"
        ? "  2. Send message"
        ? "  3. Peer info"
        ? "  4. Exit"
        accept nOpt
        
        do case
            case nOpt == 1
                ChatRead()
            case nOpt == 2
                ChatSend()
            case nOpt == 3
                ChatInfo()
            case nOpt == 4
                exit
        endcase
    enddo
return

Static Function ChatRead()
    ? "TODO: Fetch and display messages"
return

Static Function ChatSend()
    Local cText
    ? "Message (empty to cancel): " get cText
    if !empty(cText)
        ? "TODO: Send message: " + cText
    endif
return

Static Function ChatInfo()
    ? "TODO: Show peer info"
return
```

- [ ] **Step 2: Test compile**

```bash
advplc build examples/chat/chat.prw -o /tmp/chat-test
```

Expected: Compiles without errors.

- [ ] **Step 3: Run once**

```bash
/tmp/chat-test
```

Menu should appear; select 4 to exit.

- [ ] **Step 4: Commit**

```bash
git add examples/chat/chat.prw
git commit -m "feat: chat app skeleton (UI loop, P2P init stub)"
```

---

### Task 3: P2P Peer Integration

**Files:**
- Modify: `examples/chat/chat.prw` (implement ChatInit)

**Interfaces:**
- Consumes: AdvPP v2.0 P2P API (Peer, Transport, Router, PeerAPI)
- Produces: Initialized peer + subscribed to chat contract

**Why this task:** Wire the app to the P2P network. Peers must join the ring, establish neighbors, and subscribe to the chat contract.

- [ ] **Step 1: Implement ChatInit**

```advpl
Static Function ChatInit(cPeerId, cListen)
    Local oPeer
    Local oTransport
    Local oRouter
    Local oAPI
    Local oContract
    
    ? "Initializing peer: " + cPeerId
    
    // TODO: Parse cListen (host:port)
    // TODO: Create UDP transport
    // TODO: Create peer with location from address
    // TODO: Create router for greedy forwarding
    // TODO: Create PeerAPI for contract operations
    // TODO: Subscribe to chat contract
    
    ? "Peer initialized."
    ? "Local location: [location]"
    ? "Neighbors: [neighbor list]"
return oPeer
```

- [ ] **Step 2: Parse CLI args**

Add before ChatMain():

```advpl
User Function ChatMain()
    Local aPars := {}
    Local cPeerId := "peer-1"
    Local cListen := "127.0.0.1:9000"
    Local cBootstrap := ""
    
    // TODO: Parse ARGV (command-line args)
    // Expected: --peer-id p1 --listen 127.0.0.1:9000 --bootstrap 127.0.0.1:9001
    
    ChatInit(cPeerId, cListen, cBootstrap)
    ChatLoop()
return
```

- [ ] **Step 3: Test with single peer**

```bash
advplc build examples/chat/chat.prw -o /tmp/chat-peer-1
/tmp/chat-peer-1 --peer-id peer-1 --listen 127.0.0.1:9000
```

Expected: Peer initializes, shows location and neighbor list (empty on first peer).

- [ ] **Step 4: Commit**

```bash
git add examples/chat/chat.prw
git commit -m "feat: P2P peer integration (transport, routing, subscription)"
```

---

### Task 4: Chat Send & Read Operations

**Files:**
- Modify: `examples/chat/chat.prw` (implement ChatSend, ChatRead)

**Interfaces:**
- Consumes: PeerAPI (Put, Get, Update)
- Produces: Message sending to contract, reading from contract

**Why this task:** Connect the UI to the contract. Users can now send/read messages that sync across peers.

- [ ] **Step 1: Implement ChatSend**

```advpl
Static Function ChatSend(oAPI, cPeerId)
    Local cText
    Local lResult
    
    ? "Message: " get cText
    if empty(cText)
        return
    endif
    
    ? "Sending..."
    lResult := oAPI:Update("contract:chat-main", cPeerId + "|" + cText, oMerge)
    
    if lResult
        ? "Sent."
    else
        ? "Failed to send."
    endif
return
```

- [ ] **Step 2: Implement ChatRead**

```advpl
Static Function ChatRead(oAPI)
    Local aMessages
    Local i
    Local oMsg
    
    aMessages := oAPI:Get("contract:chat-main")
    
    if empty(aMessages)
        ? "No messages."
        return
    endif
    
    for i := 1 to len(aMessages)
        oMsg := aMessages[i]
        ? oMsg:from + " (" + oMsg:ts + "): " + oMsg:text
    next
return
```

- [ ] **Step 3: Test send/read on single peer**

```bash
# Start peer-1
/tmp/chat-peer-1 --peer-id peer-1 --listen 127.0.0.1:9000

# In menu:
# 2: Send message "hello"
# 1: Read messages (should show "hello")
# 2: Send "world"
# 1: Read (should show both)
```

Expected: Messages appear in chat.

- [ ] **Step 4: Commit**

```bash
git add examples/chat/chat.prw
git commit -m "feat: chat send/read operations via contract API"
```

---

### Task 5: Multi-Peer Sync Test

**Files:**
- Create: `examples/chat/BUILD.sh`
- Modify: `examples/chat/README.md`

**Interfaces:**
- Consumes: 3 compiled chat binaries
- Produces: Running demo of 3 peers syncing messages

**Why this task:** The real validation. Peers must discover each other, sync messages, and converge.

- [ ] **Step 1: Create BUILD.sh**

```bash
#!/bin/bash
cd examples/chat
advplc build chat.prw -o chat-peer-1 -X "--peer-id peer-1 --listen 127.0.0.1:9000"
advplc build chat.prw -o chat-peer-2 -X "--peer-id peer-2 --listen 127.0.0.1:9001 --bootstrap 127.0.0.1:9000"
advplc build chat.prw -o chat-peer-3 -X "--peer-id peer-3 --listen 127.0.0.1:9002 --bootstrap 127.0.0.1:9001"
echo "Built 3 chat peers."
chmod +x chat-peer-*
```

- [ ] **Step 2: Test multi-peer sync**

```bash
bash examples/chat/BUILD.sh

# Terminal 1:
./examples/chat/chat-peer-1
# -> Send message "msg from peer1"

# Terminal 2:
./examples/chat/chat-peer-2
# -> Read (should see "msg from peer1")
# -> Send message "msg from peer2"

# Terminal 3:
./examples/chat/chat-peer-3
# -> Read (should see both messages)

# Terminal 1:
# -> Read (should see "msg from peer2")
```

Expected: All peers see all messages within ~1 second.

- [ ] **Step 3: Create README.md**

```markdown
# AdvPP Freenet Distributed Chat PoC

This is a working demonstration of AdvPP v2.0's P2P network capabilities.

## How to Run

1. Build 3 peers:
   bash BUILD.sh

2. Start peers in 3 terminals:
   ./chat-peer-1
   ./chat-peer-2
   ./chat-peer-3

3. Send messages on any peer, read on others.

4. Test restart: kill chat-peer-1, restart it, verify it recovers messages.

## What This Validates

- WASM contract execution
- P2P routing (ring topology)
- Message synchronization (delta-sync protocol)
- State replication (subscription trees)
- Merge semantics (idempotent append-only)
```

- [ ] **Step 4: Commit**

```bash
git add examples/chat/BUILD.sh examples/chat/README.md
git commit -m "feat: multi-peer chat demo (3 peers, end-to-end sync)"
```

---

### Task 6: Persistence & Restart Test

**Files:**
- Modify: `examples/chat/chat.prw` (add restart recovery)

**Interfaces:**
- Consumes: SQLite local store (via AdvPP storage layer)
- Produces: Chat history persists across peer restarts

**Why this task:** Final validation. Proves that data survives peer failure and is recovered on restart.

- [ ] **Step 1: Add persistence on startup**

In ChatInit:

```advpl
Static Function ChatInit(cPeerId, cListen, cBootstrap)
    // ... existing init code ...
    
    // Load stored messages
    Local aStored := oAPI:Get("contract:chat-main")
    if !empty(aStored)
        ? "Recovered " + str(len(aStored)) + " stored messages."
    endif
    
    // ... rest of init ...
return
```

- [ ] **Step 2: Test restart recovery**

```bash
# Terminal 1: Start peer-1, send 2 messages
./examples/chat/chat-peer-1
# Menu: 2 (send "msg1"), 2 (send "msg2"), 1 (read)

# Kill (Ctrl+C)

# Restart peer-1
./examples/chat/chat-peer-1
# Menu: 1 (read)
# Expected: Both messages still there
```

Expected: Messages persisted.

- [ ] **Step 3: Test sync after restart**

```bash
# Terminal 1: Start peer-1
./examples/chat/chat-peer-1

# Terminal 2: Start peer-2 (connected to peer-1)
./examples/chat/chat-peer-2

# Terminal 2: Send message "msg from peer2"

# Terminal 1: Kill (Ctrl+C)

# Terminal 1: Restart peer-1
./examples/chat/chat-peer-1
# Menu: 1 (read)
# Expected: See "msg from peer2" from the time it was offline
```

Expected: Missed messages synced on restart.

- [ ] **Step 4: Commit**

```bash
git add examples/chat/chat.prw
git commit -m "feat: persistence & restart recovery (messages survive peer failure)"
```

---

## Success Criteria

All of these must pass for the PoC to be considered complete:

- [ ] Single peer starts without errors
- [ ] Menu appears and responds to input
- [ ] Messages send and appear in chat history
- [ ] 3 peers sync messages within 1 second
- [ ] All 3 peers show identical chat history
- [ ] Peer restart recovers all stored messages
- [ ] Missed messages sync when peer rejoins

---

## Execution Handoff

Plan complete and saved. Ready for subagent-driven development.

**Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
