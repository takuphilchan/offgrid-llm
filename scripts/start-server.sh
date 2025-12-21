#!/bin/bash
# Start OffGrid server (development mode)
# Kills any running instances and starts the locally built binary

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# IMPORTANT: Preserve the original user's home directory when running with sudo
# This ensures models are found in the user's ~/.offgrid-llm/models/ directory
if [ -n "$SUDO_USER" ]; then
    REAL_HOME=$(getent passwd "$SUDO_USER" | cut -d: -f6)
    export HOME="$REAL_HOME"
    export OFFGRID_MODELS_DIR="$REAL_HOME/.offgrid-llm/models"
    export OFFGRID_DATA_DIR="$REAL_HOME/.offgrid-llm"
    echo "Running as sudo - using $SUDO_USER's home: $REAL_HOME"
fi

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Comprehensive cleanup function
cleanup_processes() {
    # Kill anything using port 11611 (main server port)
    if command -v lsof &> /dev/null; then
        lsof -ti:11611 2>/dev/null | xargs -r kill -9 2>/dev/null || true
    fi
    if command -v fuser &> /dev/null; then
        fuser -k -9 11611/tcp 2>/dev/null || true
    fi
    
    # Kill llama-server processes on ports 42382-42391 (model cache ports)
    for port in $(seq 42382 42391); do
        if command -v lsof &> /dev/null; then
            lsof -ti:$port 2>/dev/null | xargs -r kill -9 2>/dev/null || true
        fi
    done
    
    # Kill ALL offgrid processes
    pkill -9 -f "offgrid serve" 2>/dev/null || true
    pkill -9 -f "offgrid$" 2>/dev/null || true
    pkill -9 -f "/bin/offgrid" 2>/dev/null || true
    killall -9 offgrid 2>/dev/null || true
    
    # Kill ALL llama-server processes (these are child processes of offgrid)
    pkill -9 -f "llama-server" 2>/dev/null || true
    killall -9 llama-server 2>/dev/null || true
    
    # Wait for processes to die
    sleep 1
}

# Cleanup any running instances
echo -e "${YELLOW}Stopping any running OffGrid instances...${NC}"
cleanup_processes

# Check if bin/offgrid exists, if not build it
if [ ! -f "$PROJECT_ROOT/bin/offgrid" ]; then
    echo -e "${YELLOW}bin/offgrid not found. Building...${NC}"
    cd "$PROJECT_ROOT"
    go build -o bin/offgrid ./cmd/offgrid
    if [ $? -ne 0 ]; then
        echo -e "${RED}Build failed!${NC}"
        exit 1
    fi
    echo -e "${GREEN}Build successful!${NC}"
fi

cd "$PROJECT_ROOT"

# Final check - fail early if port is still occupied
if command -v lsof &> /dev/null && lsof -ti:11611 &>/dev/null; then
    echo -e "${RED}Error: Port 11611 is still in use. Process info:${NC}"
    lsof -i:11611
    echo -e "${YELLOW}Trying one more force kill...${NC}"
    lsof -ti:11611 2>/dev/null | xargs -r kill -9 2>/dev/null || true
    sleep 2
    if lsof -ti:11611 &>/dev/null; then
        echo -e "${RED}Failed to free port 11611. Please manually kill the process.${NC}"
        exit 1
    fi
fi

echo -e "${GREEN}Starting OffGrid server from bin/offgrid...${NC}"
echo ""

# Handle signals to ensure cleanup on exit
trap 'echo ""; echo "Shutting down..."; cleanup_processes; exit 0' INT TERM

./bin/offgrid serve
