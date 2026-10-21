#!/bin/bash
# Test P2P on a single machine by running two OffGrid instances
#
# Instance 1: Port 11611 (default), P2P port 9090, Discovery port 9091 (shared)
# Instance 2: Port 11612, P2P port 9092, Discovery port 9091 (shared)
#
# Both instances share the same discovery port so they can find each other
# They share the same models directory but have separate data dirs

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OFFGRID="$PROJECT_ROOT/bin/offgrid"

# Create temp directories for each instance
INSTANCE1_DIR=$(mktemp -d)
INSTANCE2_DIR=$(mktemp -d)

# Shared discovery port - both instances must use the same one
DISCOVERY_PORT=9091

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     OffGrid P2P Local Test             ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""
echo -e "${GREEN}Instance 1:${NC} Port 11611, P2P 9090, Discovery $DISCOVERY_PORT"
echo -e "${GREEN}Instance 2:${NC} Port 11612, P2P 9092, Discovery $DISCOVERY_PORT"
echo ""
echo "Data directories:"
echo "  Instance 1: $INSTANCE1_DIR"
echo "  Instance 2: $INSTANCE2_DIR"
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up...${NC}"
    
    # Kill background processes
    if [ ! -z "$PID1" ]; then
        kill $PID1 2>/dev/null || true
    fi
    if [ ! -z "$PID2" ]; then
        kill $PID2 2>/dev/null || true
    fi
    
    # Remove temp directories
    rm -rf "$INSTANCE1_DIR" "$INSTANCE2_DIR" 2>/dev/null || true
    
    echo "Done."
}

trap cleanup EXIT

# Check if offgrid binary exists
if [ ! -f "$OFFGRID" ]; then
    echo "Building offgrid..."
    cd "$PROJECT_ROOT" && go build -o bin/offgrid ./cmd/offgrid
fi

# Start Instance 1
echo -e "${GREEN}Starting Instance 1...${NC}"
OFFGRID_PORT=11611 \
OFFGRID_P2P_ENABLED=true \
OFFGRID_P2P_PORT=9090 \
OFFGRID_DISCOVERY_PORT=$DISCOVERY_PORT \
OFFGRID_DATA_DIR="$INSTANCE1_DIR" \
"$OFFGRID" serve --quiet &
PID1=$!

sleep 2

# Start Instance 2 - uses a DIFFERENT P2P port but SAME discovery port
echo -e "${GREEN}Starting Instance 2...${NC}"
OFFGRID_PORT=11612 \
OFFGRID_P2P_ENABLED=true \
OFFGRID_P2P_PORT=9092 \
OFFGRID_DISCOVERY_PORT=$DISCOVERY_PORT \
OFFGRID_DATA_DIR="$INSTANCE2_DIR" \
"$OFFGRID" serve --quiet &
PID2=$!

# Wait for both to start
sleep 3

echo ""
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo -e "${GREEN}Both instances running!${NC}"
echo ""

# Test P2P status on both instances
echo -e "${YELLOW}Testing P2P status...${NC}"
echo ""

echo "Instance 1 P2P status:"
curl -s http://localhost:11611/v1/p2p/status | python3 -m json.tool 2>/dev/null || echo "  (endpoint not available)"
echo ""

echo "Instance 2 P2P status:"
curl -s http://localhost:11612/v1/p2p/status | python3 -m json.tool 2>/dev/null || echo "  (endpoint not available)"
echo ""

# Wait for peer discovery (mDNS takes a moment)
echo -e "${YELLOW}Waiting for peer discovery (5 seconds)...${NC}"
sleep 5

# Check peers
echo ""
echo "Instance 1 discovered peers:"
curl -s http://localhost:11611/v1/p2p/peers | python3 -m json.tool 2>/dev/null || echo "  (no peers or endpoint not available)"
echo ""

echo "Instance 2 discovered peers:"
curl -s http://localhost:11612/v1/p2p/peers | python3 -m json.tool 2>/dev/null || echo "  (no peers or endpoint not available)"
echo ""

# List models on network
echo -e "${YELLOW}Models available across P2P network:${NC}"
curl -s http://localhost:11611/v1/p2p/models | python3 -m json.tool 2>/dev/null || echo "  (no models or endpoint not available)"
echo ""

echo -e "${BLUE}════════════════════════════════════════${NC}"
echo ""
echo "Interactive mode - press Enter to test verification, or Ctrl+C to exit"
read -r

# Test model verification if models exist
MODELS=$(curl -s http://localhost:11611/v1/models | python3 -c "import sys,json; m=json.load(sys.stdin).get('models',[]); print(m[0]['id'] if m else '')" 2>/dev/null)

if [ ! -z "$MODELS" ]; then
    echo -e "${YELLOW}Testing model verification for: $MODELS${NC}"
    curl -s -X POST http://localhost:11611/v1/models/verify \
        -H "Content-Type: application/json" \
        -d "{\"model_id\": \"$MODELS\"}" | python3 -m json.tool
else
    echo "No models installed to test verification."
fi

echo ""
echo "Press Ctrl+C to stop both instances and clean up."

# Keep running until interrupted
wait
