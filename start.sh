#!/bin/bash
# OffGrid LLM - Launch Script
# Starts the Go backend and opens the UI

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${CYAN}◆ Starting OffGrid LLM${NC}"
echo ""

# Check if offgrid binary exists
if [ ! -f "./offgrid" ]; then
    echo -e "${YELLOW}⚠ Binary not found. Building...${NC}"
    make build
fi

# Start the server in background
echo -e "${GREEN}✓${NC} Starting backend server on port 11611..."
./offgrid server > /tmp/offgrid-server.log 2>&1 &
SERVER_PID=$!

# Wait for server to be ready
echo -e "${CYAN}→${NC} Waiting for server to start..."
sleep 2

# Check if server is running
if ! ps -p $SERVER_PID > /dev/null; then
    echo -e "${YELLOW}✗${NC} Server failed to start. Check /tmp/offgrid-server.log"
    cat /tmp/offgrid-server.log
    exit 1
fi

echo -e "${GREEN}✓${NC} Backend server started (PID: $SERVER_PID)"
echo ""

# Open UI in browser
UI_URL="http://localhost:11611/ui"
echo -e "${CYAN}◆${NC} Opening UI at: ${GREEN}$UI_URL${NC}"
echo ""

# Try to open browser
if command -v xdg-open > /dev/null; then
    xdg-open "$UI_URL" 2>/dev/null || true
elif command -v open > /dev/null; then
    open "$UI_URL" 2>/dev/null || true
elif command -v wslview > /dev/null; then
    wslview "$UI_URL" 2>/dev/null || true
else
    echo -e "${YELLOW}→${NC} Please open ${GREEN}$UI_URL${NC} in your browser"
fi

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}OffGrid LLM is running!${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "  UI:     ${GREEN}$UI_URL${NC}"
echo -e "  API:    ${GREEN}http://localhost:11611${NC}"
echo -e "  Health: ${GREEN}http://localhost:11611/health${NC}"
echo ""
echo -e "  Server PID: ${YELLOW}$SERVER_PID${NC}"
echo -e "  Logs: ${YELLOW}/tmp/offgrid-server.log${NC}"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop the server${NC}"
echo ""

# Trap Ctrl+C to kill server
trap "echo ''; echo -e '${CYAN}→${NC} Stopping server...'; kill $SERVER_PID 2>/dev/null; echo -e '${GREEN}✓${NC} Server stopped'; exit 0" INT TERM

# Keep script running
tail -f /tmp/offgrid-server.log
