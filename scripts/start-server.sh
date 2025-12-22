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
    # Try with sudo if available to catch processes from other sessions
    if command -v lsof &> /dev/null; then
        lsof -ti:11611 2>/dev/null | xargs -r kill -9 2>/dev/null || true
        sudo lsof -ti:11611 2>/dev/null | xargs -r sudo kill -9 2>/dev/null || true
    fi
    if command -v fuser &> /dev/null; then
        fuser -k -9 11611/tcp 2>/dev/null || true
        sudo fuser -k -9 11611/tcp 2>/dev/null || true
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

# Store the server PID so we can properly clean it up
SERVER_PID=""

# Handle signals to ensure cleanup on exit
cleanup_and_exit() {
    echo ""
    echo "Shutting down..."
    
    # If server is running, send SIGTERM first for graceful shutdown
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        echo "Sending SIGTERM to offgrid server (PID: $SERVER_PID)..."
        kill -TERM "$SERVER_PID" 2>/dev/null || true
        
        # Wait up to 5 seconds for graceful shutdown
        for i in $(seq 1 50); do
            if ! kill -0 "$SERVER_PID" 2>/dev/null; then
                echo "Server stopped gracefully"
                break
            fi
            sleep 0.1
        done
        
        # Force kill if still running
        if kill -0 "$SERVER_PID" 2>/dev/null; then
            echo "Force killing server..."
            kill -9 "$SERVER_PID" 2>/dev/null || true
            wait "$SERVER_PID" 2>/dev/null || true
        fi
    fi
    
    # Final cleanup of any remaining processes
    cleanup_processes
    
    exit 0
}

# Trap all signals that should trigger cleanup:
# INT = Ctrl+C, TERM = kill, HUP = terminal closed, EXIT = script exit
# Note: TSTP (Ctrl+Z) is handled specially - we convert it to a full exit
trap 'cleanup_and_exit' INT TERM HUP EXIT

# Disable Ctrl+Z (SIGTSTP) - force users to use Ctrl+C for clean shutdown
# This prevents zombie processes from Ctrl+Z suspending
trap 'echo ""; echo "Use Ctrl+C to stop the server (Ctrl+Z disabled)"; ' TSTP

# Start server in background so we can capture its PID
./bin/offgrid serve &
SERVER_PID=$!

# Wait for the server process
wait $SERVER_PID
EXIT_CODE=$?

# Clear the EXIT trap before normal exit to avoid double cleanup
trap - EXIT

# Server exited on its own
if [ $EXIT_CODE -ne 0 ]; then
    echo -e "${RED}Server exited with code $EXIT_CODE${NC}"
fi

cleanup_processes
exit $EXIT_CODE
