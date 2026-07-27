#!/bin/bash
# Automated peer-restart recovery test harness for Task 6.
# Starts a 3-peer mesh, sends a message from peer-1, kills peer-1, restarts
# it, and verifies it recovered the message from its SQLite database
# (chat_peer_peer-1.db) instead of coming back with an empty chat history.
#
# NOTE on FIFO usage: each peer's stdin FIFO is fed by exactly ONE writer
# subshell per peer lifetime (send-then-idle, or read-then-idle), which
# stays open with `sleep` between writes. Opening a second, separate writer
# subshell against the same FIFO after the first one has closed is racy —
# the reading process can observe EOF on stdin in the gap and exit its menu
# loop before the second writer's bytes ever arrive (this is a pre-existing
# property of examples/chat/test-sync.sh from Task 5 too, reproducible on
# this tree independent of Task 6's changes: its final "all peers read
# messages" step silently no-ops because each peer already exited after its
# own single earlier write). Task 6's own restart-recovery assertions do not
# depend on this: they compare SQLite disk state and peer-1's own recovered
# read, not live gossip observed by peer-2/peer-3 racing multiple writers.

set -e

SCRIPTDIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPTDIR"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}=== Task 6: Restart Recovery Test ===${NC}"
echo ""

if [ ! -f "chat-peer-1" ] || [ ! -f "chat-peer-2" ] || [ ! -f "chat-peer-3" ]; then
    echo -e "${RED}ERROR: Binaries not found. Run 'bash BUILD.sh' first.${NC}"
    exit 1
fi

# Start clean: a leftover DB from a previous run would make recovery look
# like it's working even if this run's persistence code were broken.
rm -f chat_peer_*.db

PEER1_FIFO="/tmp/chat-restart-peer-1-$$-input"
PEER2_FIFO="/tmp/chat-restart-peer-2-$$-input"
PEER3_FIFO="/tmp/chat-restart-peer-3-$$-input"

PEER1_OUT="/tmp/chat-restart-peer-1-$$-output.txt"
PEER2_OUT="/tmp/chat-restart-peer-2-$$-output.txt"
PEER3_OUT="/tmp/chat-restart-peer-3-$$-output.txt"
PEER1_OUT2="/tmp/chat-restart-peer-1-$$-restarted-output.txt"

FAIL=0

cleanup() {
    jobs -p | xargs -r kill 2>/dev/null || true
    rm -f "$PEER1_FIFO" "$PEER2_FIFO" "$PEER3_FIFO"
    rm -f "$PEER1_OUT" "$PEER2_OUT" "$PEER3_OUT" "$PEER1_OUT2"
}
trap cleanup EXIT

mkfifo "$PEER1_FIFO" "$PEER2_FIFO" "$PEER3_FIFO"

echo -e "${YELLOW}Starting 3-peer mesh...${NC}"

ADVPP_PEER_ID=peer-1 ADVPP_LISTEN=127.0.0.1:9100 \
    ./chat-peer-1 < "$PEER1_FIFO" > "$PEER1_OUT" 2>&1 &
PEER1_PID=$!
sleep 1

ADVPP_PEER_ID=peer-2 ADVPP_LISTEN=127.0.0.1:9101 ADVPP_BOOTSTRAP=127.0.0.1:9100 \
    ./chat-peer-2 < "$PEER2_FIFO" > "$PEER2_OUT" 2>&1 &
PEER2_PID=$!
sleep 1

ADVPP_PEER_ID=peer-3 ADVPP_LISTEN=127.0.0.1:9102 ADVPP_BOOTSTRAP=127.0.0.1:9100 \
    ./chat-peer-3 < "$PEER3_FIFO" > "$PEER3_OUT" 2>&1 &
PEER3_PID=$!
sleep 2

echo -e "${GREEN}All peers started.${NC}"
echo ""

# --- Step 1: single continuous writer per peer for this phase --------------
# peer-1: send the message, wait for gossip to settle, then exit (one FIFO
# open for the whole phase, no reopen race).
echo -e "${YELLOW}Sending message from peer-1, then peer-2 reads it live...${NC}"
(echo "2"; sleep 0.3; echo "msg from peer-1 before restart"; sleep 2; echo "4") > "$PEER1_FIFO" &
# peer-2: idle until the message has had time to gossip over, then read,
# then exit (also one FIFO open for the whole phase).
(sleep 1.5; echo "1"; sleep 0.5; echo "4") > "$PEER2_FIFO" &

wait "$PEER1_PID" 2>/dev/null || true
wait "$PEER2_PID" 2>/dev/null || true

if [ ! -f "chat_peer_peer-1.db" ]; then
    echo -e "${RED}FAIL: chat_peer_peer-1.db was not created.${NC}"
    FAIL=1
else
    echo -e "${GREEN}OK: chat_peer_peer-1.db exists on disk.${NC}"
fi

if grep -q "msg from peer-1 before restart" "$PEER2_OUT"; then
    echo -e "${GREEN}OK: peer-2 has the message (live gossip).${NC}"
else
    echo -e "${RED}FAIL: peer-2 does not have the message.${NC}"
    FAIL=1
fi

# --- Step 2: restart peer-1 and verify it recovers from SQLite ------------
echo -e "${YELLOW}Restarting peer-1...${NC}"
mkfifo "$PEER1_FIFO" 2>/dev/null || true
ADVPP_PEER_ID=peer-1 ADVPP_LISTEN=127.0.0.1:9100 \
    ./chat-peer-1 < "$PEER1_FIFO" > "$PEER1_OUT2" 2>&1 &
PEER1_PID=$!
sleep 1

echo -e "${YELLOW}Checking peer-1 for the recovered message...${NC}"
(echo "1"; sleep 0.5; echo "4") > "$PEER1_FIFO" &
wait "$PEER1_PID" 2>/dev/null || true

if grep -q "recovered 1 message" "$PEER1_OUT2"; then
    echo -e "${GREEN}OK: peer-1 logged SQLite recovery of the message.${NC}"
else
    echo -e "${RED}FAIL: peer-1 did not log recovery from SQLite.${NC}"
    FAIL=1
fi

if grep -q "msg from peer-1 before restart" "$PEER1_OUT2"; then
    echo -e "${GREEN}OK: peer-1's 'Read messages' shows the recovered message.${NC}"
else
    echo -e "${RED}FAIL: peer-1's 'Read messages' does not show the recovered message.${NC}"
    FAIL=1
fi

# --- Cleanup ---------------------------------------------------------------
kill "$PEER3_PID" 2>/dev/null || true
wait 2>/dev/null || true

echo ""
echo -e "${YELLOW}=== Verification ===${NC}"
echo ""
echo "peer-1 output (before restart):"
tail -20 "$PEER1_OUT" || echo "(no output)"
echo ""
echo "peer-2 output:"
tail -20 "$PEER2_OUT" || echo "(no output)"
echo ""
echo "peer-1 output (after restart):"
tail -20 "$PEER1_OUT2" || echo "(no output)"
echo ""

if [ "$FAIL" -eq 0 ]; then
    echo -e "${GREEN}=== Task 6 restart recovery test: PASS ===${NC}"
else
    echo -e "${RED}=== Task 6 restart recovery test: FAIL ===${NC}"
fi

exit "$FAIL"
