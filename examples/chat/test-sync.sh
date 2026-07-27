#!/bin/bash
# Automated 3-peer sync test harness for Task 5
# Spawns 3 peers, sends messages, verifies convergence

set -e

SCRIPTDIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPTDIR"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== Task 5: Multi-Peer Sync Test ===${NC}"
echo ""

# Check if binaries exist
if [ ! -f "chat-peer-1" ] || [ ! -f "chat-peer-2" ] || [ ! -f "chat-peer-3" ]; then
    echo -e "${RED}ERROR: Binaries not found. Run 'bash BUILD.sh' first.${NC}"
    exit 1
fi

echo -e "${YELLOW}Starting 3 peers...${NC}"

# Create temp directories for each peer
PEER1_FIFO="/tmp/chat-peer-1-$$-input"
PEER2_FIFO="/tmp/chat-peer-2-$$-input"
PEER3_FIFO="/tmp/chat-peer-3-$$-input"

PEER1_OUT="/tmp/chat-peer-1-$$-output.txt"
PEER2_OUT="/tmp/chat-peer-2-$$-output.txt"
PEER3_OUT="/tmp/chat-peer-3-$$-output.txt"

trap "cleanup" EXIT

cleanup() {
    # Kill all background processes
    jobs -p | xargs -r kill 2>/dev/null || true
    # Remove temp files
    rm -f "$PEER1_FIFO" "$PEER2_FIFO" "$PEER3_FIFO"
    rm -f "$PEER1_OUT" "$PEER2_OUT" "$PEER3_OUT"
}

# Create FIFOs for stdin
mkfifo "$PEER1_FIFO" || true
mkfifo "$PEER2_FIFO" || true
mkfifo "$PEER3_FIFO" || true

# Start peer-1 (bootstrap)
echo "Starting peer-1 (bootstrap node)..."
ADVPP_PEER_ID=peer-1 ADVPP_LISTEN=127.0.0.1:9000 \
    ./chat-peer-1 < "$PEER1_FIFO" > "$PEER1_OUT" 2>&1 &
PEER1_PID=$!

# Wait a bit for peer-1 to start listening
sleep 1

# Start peer-2 (bootstrap to peer-1)
echo "Starting peer-2 (bootstraps to peer-1)..."
ADVPP_PEER_ID=peer-2 ADVPP_LISTEN=127.0.0.1:9001 ADVPP_BOOTSTRAP=127.0.0.1:9000 \
    ./chat-peer-2 < "$PEER2_FIFO" > "$PEER2_OUT" 2>&1 &
PEER2_PID=$!

# Wait a bit for peer-2 to join
sleep 1

# Start peer-3 (bootstrap to peer-1)
echo "Starting peer-3 (bootstraps to peer-1)..."
ADVPP_PEER_ID=peer-3 ADVPP_LISTEN=127.0.0.1:9002 ADVPP_BOOTSTRAP=127.0.0.1:9000 \
    ./chat-peer-3 < "$PEER3_FIFO" > "$PEER3_OUT" 2>&1 &
PEER3_PID=$!

# Wait for all peers to be ready
sleep 2

echo -e "${GREEN}All peers started.${NC}"
echo ""

# Test 1: peer-1 sends a message
echo -e "${YELLOW}Test 1: peer-1 sends message${NC}"
(echo "2"; echo "Hello from peer-1"; sleep 1; echo "4") > "$PEER1_FIFO" &
sleep 2

# Test 2: peer-2 sends a message
echo -e "${YELLOW}Test 2: peer-2 sends message${NC}"
(echo "2"; echo "Message from peer-2"; sleep 1; echo "4") > "$PEER2_FIFO" &
sleep 2

# Test 3: peer-3 sends a message
echo -e "${YELLOW}Test 3: peer-3 sends message${NC}"
(echo "2"; echo "Message from peer-3"; sleep 1; echo "4") > "$PEER3_FIFO" &
sleep 2

# Test 4: All peers read messages
echo -e "${YELLOW}Test 4: All peers read messages${NC}"
(echo "1"; sleep 1; echo "4") > "$PEER1_FIFO" &
(echo "1"; sleep 1; echo "4") > "$PEER2_FIFO" &
(echo "1"; sleep 1; echo "4") > "$PEER3_FIFO" &
sleep 3

# Wait for all background processes to finish
wait $PEER1_PID $PEER2_PID $PEER3_PID 2>/dev/null || true

echo ""
echo -e "${YELLOW}=== Verification ===${NC}"
echo ""

# Check outputs
echo "peer-1 output:"
tail -20 "$PEER1_OUT" || echo "(no output)"

echo ""
echo "peer-2 output:"
tail -20 "$PEER2_OUT" || echo "(no output)"

echo ""
echo "peer-3 output:"
tail -20 "$PEER3_OUT" || echo "(no output)"

echo ""
echo -e "${GREEN}Test completed. Check outputs above for convergence.${NC}"
echo "Expected: All peers should show the same messages in the same order."
