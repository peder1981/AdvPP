# Task 5 Multi-Peer Sync Test — Manual Testing Guide

This guide describes how to manually test the 3-peer message convergence.

## Prerequisites

- Run `bash BUILD.sh` to compile 3 binaries
- Open 3 terminal windows/tabs
- Each peer needs its own terminal for interactive menu operation

## Test Plan

### Phase 1: Network Topology Setup

**Terminal 1: Start peer-1 (bootstrap node)**
```bash
ADVPP_PEER_ID=peer-1 ADVPP_LISTEN=127.0.0.1:9000 ./examples/chat/chat-peer-1
```
Expected output:
```
=== AdvPP Freenet Distributed Chat ===
Peer: peer-1
Listening: 127.0.0.1:9000
Initializing peer: peer-1
Transport bound (stub): 127.0.0.1:9000
Subscribed to: contract:chat-main
Peer initialized.
Local location: 0.xxx
Neighbors: (none)

Chat Menu:
  1. Read messages
  2. Send message
  3. Peer info
  4. Exit
```

**Terminal 2: Start peer-2 (bootstraps to peer-1)**
```bash
ADVPP_PEER_ID=peer-2 ADVPP_LISTEN=127.0.0.1:9001 ADVPP_BOOTSTRAP=127.0.0.1:9000 ./examples/chat/chat-peer-2
```
Expected output:
- Same initialization, but with ADVPP_BOOTSTRAP set
- peer-2 should connect to peer-1 via JOIN message
- peer-1 should add peer-2 to its neighbors and reply with STATE_SYNC

**Terminal 3: Start peer-3 (bootstraps to peer-1)**
```bash
ADVPP_PEER_ID=peer-3 ADVPP_LISTEN=127.0.0.1:9002 ADVPP_BOOTSTRAP=127.0.0.1:9000 ./examples/chat/chat-peer-3
```
Expected output:
- Same as peer-2, but listening on 9002
- peer-3 should connect to peer-1 via JOIN message
- peer-1 should add peer-3 to its neighbors and reply with STATE_SYNC

### Phase 2: Message Propagation (Simple Case)

**Step 2.1: peer-1 sends first message**

Terminal 1 menu:
```
2
```
Input message:
```
Hello from peer-1
```

Expected:
- peer-1: "Sent."
- peer-1: "Sending..." appears before menu
- peer-2: Should see "Hello from peer-1" if it reads (menu option 1)
- peer-3: Should see "Hello from peer-1" if it reads (menu option 1)

**Step 2.2: peer-2 reads messages**

Terminal 2 menu:
```
1
```

Expected output:
```
peer-1 (date_time): Hello from peer-1
```

**Step 2.3: peer-2 sends a message**

Terminal 2 menu:
```
2
```
Input message:
```
Response from peer-2
```

Expected:
- peer-2: "Sent."
- peer-1: Should see "Response from peer-2" if it reads (menu option 1)
- peer-3: Should see "Response from peer-2" if it reads (menu option 1)

**Step 2.4: peer-3 reads both messages**

Terminal 3 menu:
```
1
```

Expected output:
```
peer-1 (date_time): Hello from peer-1
peer-2 (date_time): Response from peer-2
```

### Phase 3: Convergence Verification (Critical)

**Step 3.1: All peers read and compare**

Terminal 1:
```
1
```

Terminal 2:
```
1
```

Terminal 3:
```
1
```

**Verification:**
- All 3 terminals should display **identical** message histories
- Same order (peer-1's message first, then peer-2's)
- No duplicates (each message appears exactly once)
- Format: `from (timestamp): text`

### Phase 4: Message Order Independence Test

This test verifies that messages sent asynchronously are ordered consistently.

**Step 4.1: All 3 peers send simultaneously**

Terminal 1:
```
2
msg-A from peer-1
```

Terminal 2:
```
2
msg-B from peer-2
```

Terminal 3:
```
2
msg-C from peer-3
```

**Step 4.2: All peers read**

Terminal 1:
```
1
```

Terminal 2:
```
1
```

Terminal 3:
```
1
```

**Verification:**
- All 3 terminals should show the same message order
- Idempotent merge ensures no duplicates
- Even though sent asynchronously, all peers agree on ordering

### Phase 5: Peer Restart & Recovery Test

**Step 5.1: Kill peer-1**

Terminal 1:
```
Ctrl+C
```

**Step 5.2: Peers 2 & 3 continue running**

Terminal 2 & 3 should still respond to menu.

**Step 5.3: Restart peer-1**

Terminal 1:
```bash
ADVPP_PEER_ID=peer-1 ADVPP_LISTEN=127.0.0.1:9000 ./examples/chat/chat-peer-1
```

**Step 5.4: peer-1 reads immediately after restart**

Terminal 1 menu:
```
1
```

Expected output:
- peer-1 should recover **all** messages sent while it was offline
- It should see both msg-A, msg-B, msg-C (or at least the ones sent before restart)
- This proves the STATE_SYNC reply to JOIN works correctly

## Success Criteria Checklist

- [ ] 3 peers compile without errors
- [ ] Menu appears and responds to input (options 1-4 work)
- [ ] Messages send successfully (option 2)
- [ ] Messages appear in read output (option 1)
- [ ] All 3 peers show identical message histories
- [ ] No message duplicates (each appears exactly once)
- [ ] Message order is consistent across all peers
- [ ] Message appears on all peers within ~1 second
- [ ] Restart recovery works (peer rejoins and gets all messages)

## Troubleshooting

### "Unable to resolve ADVPP_BOOTSTRAP"
- Check that peer-1 is running on 127.0.0.1:9000
- Verify ADVPP_LISTEN addresses don't conflict

### Messages not appearing on other peers
- Ensure peer-1 is still running (it's the hub)
- Check that bootstrap addresses are correct
- Verify firewall/network doesn't block UDP 9000-9002

### Duplicate messages
- This indicates a merge bug
- Check p2p_bridge.go's chatContractMerge function
- Merge should deduplicate on (from, ts) tuple

### Messages in wrong order on different peers
- This indicates a non-idempotent merge
- Verify append-only semantics in merge function
- All peers must apply same merge operations in same order
