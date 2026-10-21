#!/bin/bash
# OffGrid LLM Installation Script
# Comprehensive installation with GPU support, llama.cpp compilation, and systemd integration

set -eu

# Installation options
FORCE_CPU_ONLY=false
FORCE_GPU=false

# Lock file to prevent concurrent installations
LOCK_FILE="/tmp/offgrid-install.lock"

# Cleanup function
cleanup() {
    rm -f "$LOCK_FILE"
}

# Set trap to cleanup on exit
trap cleanup EXIT

# Color definitions (matching CLI brand colors)
BRAND_PRIMARY='\033[38;5;45m'      # Bright cyan (#00d4ff)
BRAND_SECONDARY='\033[38;5;141m'   # Purple (#af87ff)
BRAND_ACCENT='\033[38;5;226m'      # Yellow (#ffff00)
BRAND_SUCCESS='\033[38;5;78m'      # Green (#5fd787)
BRAND_ERROR='\033[38;5;196m'       # Red (#ff005f)
BRAND_MUTED='\033[38;5;240m'       # Gray (#585858)
RESET='\033[0m'
BOLD='\033[1m'
DIM='\033[2m'

# Progress tracking
TOTAL_STEPS=14
CURRENT_STEP=0
START_TIME=$(date +%s)

# ASCII Art Banner
print_banner() {
    echo -e "${BRAND_PRIMARY}${BOLD}"
    cat << "EOF"
    ╔═══════════════════════════════════════════════════════════════╗
    ║                                                               ║
    ║     ██████╗ ███████╗███████╗ ██████╗ ██████╗ ██╗██████╗      ║
    ║    ██╔═══██╗██╔════╝██╔════╝██╔════╝ ██╔══██╗██║██╔══██╗     ║
    ║    ██║   ██║█████╗  █████╗  ██║  ███╗██████╔╝██║██║  ██║     ║
    ║    ██║   ██║██╔══╝  ██╔══╝  ██║   ██║██╔══██╗██║██║  ██║     ║
    ║    ╚██████╔╝██║     ██║     ╚██████╔╝██║  ██║██║██████╔╝     ║
    ║     ╚═════╝ ╚═╝     ╚═╝      ╚═════╝ ╚═╝  ╚═╝╚═╝╚═════╝      ║
    ║                                                               ║
    ║                  E D G E   I N F E R E N C E                  ║
    ║                                                               ║
    ╚═══════════════════════════════════════════════════════════════╝
EOF
    echo -e "${RESET}"
    echo -e "${BRAND_MUTED}    Offline-first AI for edge environments${RESET}"
    echo ""
}

# Print Functions
print_header() {
    echo ""
    echo -e "${BRAND_PRIMARY}◆${RESET} ${BOLD}$1${RESET}"
    echo -e "${BRAND_MUTED}$(printf '─%.0s' {1..60})${RESET}"
    echo ""
}

print_success() { echo -e "${BRAND_SUCCESS}✓${RESET} $1"; }
print_error() { echo -e "${BRAND_ERROR}✗${RESET} $1" >&2; }
print_info() { echo -e "${BRAND_PRIMARY}→${RESET} $1"; }
print_warning() { echo -e "${BRAND_ACCENT}⚡${RESET} $1"; }
print_step() { echo -e "${BOLD}${BRAND_PRIMARY}▸${RESET} $1"; }
print_divider() { echo -e "${BRAND_MUTED}$(printf '━%.0s' {1..70})${RESET}"; }
print_dim() { echo -e "${DIM}$1${RESET}"; }

# Progress tracking functions
get_elapsed_time() {
    local now=$(date +%s)
    local elapsed=$((now - START_TIME))
    printf "%02d:%02d" $((elapsed / 60)) $((elapsed % 60))
}

print_progress() {
    CURRENT_STEP=$((CURRENT_STEP + 1))
    local percent=$((CURRENT_STEP * 100 / TOTAL_STEPS))
    local elapsed=$(get_elapsed_time)

    echo ""
    echo -e "${BRAND_MUTED}╭────────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_MUTED}│${RESET} ${BOLD}Step ${CURRENT_STEP}/${TOTAL_STEPS}${RESET} ${BRAND_MUTED}[${BRAND_SUCCESS}$(printf '█%.0s' $(seq 1 $((percent / 5))))$(printf '░%.0s' $(seq 1 $((20 - percent / 5))))${BRAND_MUTED}] ${BRAND_ACCENT}${percent}%${RESET} ${BRAND_MUTED}│ ${DIM}Elapsed: ${elapsed}${RESET}"
    echo -e "${BRAND_MUTED}╰────────────────────────────────────────────────────────────────────╯${RESET}"

    print_header "$1"
    if [ -n "$2" ]; then
        echo -e "${DIM}  Estimated time: $2${RESET}"
        echo ""
    fi
}

# Usage/Help
usage() {
    echo -e "${BRAND_MUTED}$(printf '━%.0s' {1..70})${RESET}"
    echo ""
    echo -e "${BRAND_PRIMARY}◆${RESET} ${BOLD}Usage${RESET}"
    echo -e "${BRAND_MUTED}$(printf '─%.0s' {1..60})${RESET}"
    echo "  ./install.sh [OPTIONS]"
    echo ""
    echo -e "${BRAND_PRIMARY}◆${RESET} ${BOLD}Options${RESET}"
    echo -e "${BRAND_MUTED}$(printf '─%.0s' {1..60})${RESET}"
    echo "  --cpu-only          Force CPU-only mode (skip GPU detection)"
    echo "  --gpu               Force GPU mode (fail if no GPU detected)"
    echo "  --help, -h          Show this help message"
    echo ""
    echo -e "${BRAND_PRIMARY}◆${RESET} ${BOLD}Examples${RESET}"
    echo -e "${BRAND_MUTED}$(printf '─%.0s' {1..60})${RESET}"
    echo -e "  ${BRAND_MUTED}\$${RESET} ./install.sh                    # Auto-detect GPU"
    echo -e "  ${BRAND_MUTED}\$${RESET} ./install.sh --cpu-only         # CPU-only mode"
    echo -e "  ${BRAND_MUTED}\$${RESET} ./install.sh --gpu              # Require GPU"
    echo ""
    echo -e "${BRAND_MUTED}$(printf '━%.0s' {1..70})${RESET}"
    echo ""
    exit 0
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --cpu-only)
                FORCE_CPU_ONLY=true
                print_info "CPU-only mode enabled"
                shift
                ;;
            --gpu)
                FORCE_GPU=true
                print_info "GPU mode required"
                shift
                ;;
            --help|-h)
                usage
                ;;
            *)
                print_error "Unknown option: $1"
                usage
                ;;
        esac
    done

    # Validate conflicting options
    if [ "$FORCE_CPU_ONLY" = true ] && [ "$FORCE_GPU" = true ]; then
        print_error "Cannot use --cpu-only and --gpu together"
        exit 1
    fi
}

# Error Handler
handle_error() {
    print_error "Installation failed at: $1"
    print_info "Check the error message above for details"
    exit 1
}

trap 'handle_error "line $LINENO"' ERR

# Dependency Checks
SOURCE_INSTALL_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "$SOURCE_INSTALL_ROOT/scripts/build/source-install-build.sh"
source "$SOURCE_INSTALL_ROOT/installers/source-install-system.sh"

display_summary() {
    local total_time=$(get_elapsed_time)

    echo ""
    echo ""
    print_divider
    echo -e "${BRAND_SUCCESS}${BOLD}"
    cat << "EOF"
    ╔═══════════════════════════════════════════════════════════════╗
    ║                                                               ║
    ║              ✓  INSTALLATION COMPLETE  ✓                      ║
    ║                                                               ║
    ╚═══════════════════════════════════════════════════════════════╝
EOF
    echo -e "${RESET}"
    print_divider

    echo ""
    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}SYSTEM INFORMATION${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  Architecture     ${BRAND_SECONDARY}$ARCH${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  Operating System ${BRAND_SECONDARY}$NAME $VERSION${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  GPU Type         ${BRAND_SECONDARY}${GPU_TYPE}${RESET}"
    if [ "$GPU_TYPE" != "none" ]; then
        echo -e "${BRAND_PRIMARY}│${RESET}  GPU Info         ${BRAND_SECONDARY}${GPU_INFO}${RESET}"
    fi
    echo -e "${BRAND_PRIMARY}│${RESET}  Inference Mode   ${BRAND_SUCCESS}REAL LLM${RESET} ${DIM}(via llama.cpp)${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  Install Time     ${BRAND_ACCENT}${total_time}${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"

    echo ""

    # Get internal port
    local INTERNAL_PORT="(random)"
    if [ -f "/etc/offgrid/llama-port" ]; then
        INTERNAL_PORT=$(cat /etc/offgrid/llama-port)
    fi

    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}SERVICE ENDPOINTS${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  llama-server     ${BRAND_MUTED}http://127.0.0.1:${INTERNAL_PORT}${RESET} ${DIM}(internal only)${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  OffGrid API      ${BRAND_SECONDARY}http://localhost:11611${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  Web UI           ${BRAND_SECONDARY}http://localhost:11611/ui${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"

    echo ""

    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}SECURITY${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} llama-server bound to 127.0.0.1 only (internal IPC)"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} Random high port ${INTERNAL_PORT} not exposed externally"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} Only OffGrid port 11611 is publicly accessible"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} Inference runtime isolated behind the OffGrid service"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"

    echo ""

    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}QUICK START COMMANDS${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# Start interactive chat${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}offgrid run <model>${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# List available models${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}offgrid list${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# Download a model${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}offgrid download tinyllama${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# Start with session auto-save${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}offgrid run <model> --save my-session${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"

    echo ""

    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}SERVICE MANAGEMENT${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# Check service status${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}sudo systemctl status offgrid-llm${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}sudo systemctl status llama-server${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# View logs${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}sudo journalctl -u offgrid-llm -f${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}sudo journalctl -u llama-server -f${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# Restart services${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}sudo systemctl restart offgrid-llm${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"

    echo ""

    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}HEALTH CHECK${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}curl http://localhost:11611/health${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"

    echo ""

    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}NEXT STEPS${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_ACCENT}1.${RESET} Visit ${BRAND_SECONDARY}http://localhost:11611/ui${RESET} in your browser"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_ACCENT}2.${RESET} Test the health endpoint"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_ACCENT}3.${RESET} Try a chat completion"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"

    echo ""
    print_divider
    echo ""
    echo -e "${BRAND_SUCCESS}${BOLD}  🚀 OffGrid LLM is ready to use!${RESET}"
    echo -e "${DIM}     Installed in ${total_time} • Real LLM inference enabled${RESET}"
    echo ""
    print_divider
    echo ""
}

# Main Installation Flow
main() {
    # Parse command line arguments first
    parse_args "$@"

    # Show banner
    clear
    print_banner

    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}OffGrid LLM Installation${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  This installer will:"
    echo -e "${BRAND_PRIMARY}│${RESET}    • Check system dependencies"
    echo -e "${BRAND_PRIMARY}│${RESET}    • Detect GPU hardware"
    echo -e "${BRAND_PRIMARY}│${RESET}    • Install Go 1.21+"
    echo -e "${BRAND_PRIMARY}│${RESET}    • Build llama.cpp inference engine"
    echo -e "${BRAND_PRIMARY}│${RESET}    • Build and install OffGrid LLM"
    echo -e "${BRAND_PRIMARY}│${RESET}    • Configure systemd services"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}Estimated time: 10-15 minutes${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"
    echo ""

    # Check if running as root (not recommended but check dependencies will need sudo)
    if [ "$EUID" -eq 0 ]; then
        print_warning "Running as root - this is not recommended"
        print_info "Please run as a regular user with sudo access"
    fi

    # Check if another instance is running using lock file
    if [ -f "$LOCK_FILE" ]; then
        LOCK_PID=$(cat "$LOCK_FILE" 2>/dev/null || echo "")
        if [ -n "$LOCK_PID" ] && kill -0 "$LOCK_PID" 2>/dev/null; then
            print_error "Another installation appears to be running"
            print_info "PID: $LOCK_PID"
            print_info "If this is incorrect, remove $LOCK_FILE and try again"
            exit 1
        else
            print_info "Removing stale lock file"
            rm -f "$LOCK_FILE"
        fi
    fi

    # Create lock file with current PID
    echo $$ > "$LOCK_FILE"

    echo ""
    print_info "Starting installation..."
    sleep 1

    # Pre-flight checks
    check_dependencies
    detect_architecture
    detect_os
    detect_gpu

    # Show pre-flight summary
    echo ""
    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}PRE-FLIGHT CHECK COMPLETE${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} System dependencies verified"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} Architecture: ${BRAND_SECONDARY}${ARCH}${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} Operating System: ${BRAND_SECONDARY}${NAME}${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} GPU: ${BRAND_SECONDARY}${GPU_TYPE}${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"
    echo ""

    print_info "Proceeding with installation..."
    sleep 2

    # Build and install
    install_build_deps
    install_go
    install_nvidia_drivers
    build_llama_cpp
    build_offgrid
    install_binary

    # System setup
    setup_user
    setup_config
    setup_llama_server_service
    setup_systemd_service
    start_service

    # Install shell completions
    install_completions

    # Summary
    display_summary
}

# Run main installation
main "$@"
