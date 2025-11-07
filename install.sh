#!/bin/bash
set -e

# OffGrid LLM Installation Script
# Usage: ./install.sh [--user|--system]

BINARY="offgrid"
INSTALL_MODE="${1:---system}"

# Colors for clean output
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'
CYAN='\033[36m'
GREEN='\033[32m'
RED='\033[31m'
YELLOW='\033[33m'

# Print functions
print_header() {
    echo ""
    echo -e "${BOLD}${CYAN}╭─────────────────────────────────────────────────╮${RESET}"
    echo -e "${BOLD}${CYAN}│                                                 │${RESET}"
    echo -e "${BOLD}${CYAN}│          OffGrid LLM Installation               │${RESET}"
    echo -e "${BOLD}${CYAN}│                                                 │${RESET}"
    echo -e "${BOLD}${CYAN}╰─────────────────────────────────────────────────╯${RESET}"
    echo ""
}

print_success() {
    echo -e "${GREEN}✓${RESET} $1"
}

print_error() {
    echo -e "${RED}✗${RESET} $1"
}

print_info() {
    echo -e "${DIM}•${RESET} $1"
}

print_step() {
    echo ""
    echo -e "${BOLD}$1${RESET}"
}

print_divider() {
    echo -e "${DIM}─────────────────────────────────────────────────${RESET}"
}

print_header

# Check for Go installation
if ! command -v go &> /dev/null; then
    print_error "Go is not installed"
    echo ""
    print_info "Please install Go 1.21 or higher"
    print_info "Download from: ${CYAN}https://golang.org/dl/${RESET}"
    echo ""
    exit 1
fi

GO_VERSION=$(go version | grep -oP 'go\d+\.\d+' | grep -oP '\d+\.\d+')
print_success "Go $GO_VERSION detected"

case "$INSTALL_MODE" in
    --system)
        print_step "System Installation"
        print_info "Installing to /usr/local/bin (requires sudo)"
        echo ""
        
        # Build binary
        print_info "Building binary..."
        make build > /dev/null 2>&1
        print_success "Build complete"
        
        # Install to system
        print_info "Installing to system..."
        sudo install -m 755 "$BINARY" "/usr/local/bin/$BINARY"
        
        # Verify installation
        if command -v offgrid &> /dev/null; then
            VERSION=$(offgrid --version 2>/dev/null || echo 'dev')
            
            echo ""
            echo -e "${BOLD}${GREEN}╭─────────────────────────────────────────────────╮${RESET}"
            echo -e "${BOLD}${GREEN}│                                                 │${RESET}"
            echo -e "${BOLD}${GREEN}│              Installation Complete              │${RESET}"
            echo -e "${BOLD}${GREEN}│                                                 │${RESET}"
            echo -e "${BOLD}${GREEN}╰─────────────────────────────────────────────────╯${RESET}"
            echo ""
            echo -e "  ${DIM}Location${RESET}  /usr/local/bin/offgrid"
            echo -e "  ${DIM}Version${RESET}   $VERSION"
            echo ""
            
            # Ask if user wants to setup systemd service
            if command -v systemctl &> /dev/null; then
                echo -e "${BOLD}Setup Options${RESET}"
                echo ""
                read -p "$(echo -e ${CYAN}Would you like to install as a systemd service? [y/N]: ${RESET})" -n 1 -r
                echo ""
                
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    print_step "Setting up systemd service"
                    
                    # Create systemd service file
                    sudo tee /etc/systemd/system/offgrid-llm.service > /dev/null <<EOF
[Unit]
Description=OffGrid LLM - Edge Inference Orchestrator
After=network.target
Documentation=https://github.com/takuphilchan/offgrid-llm

[Service]
Type=simple
User=$USER
ExecStart=/usr/local/bin/offgrid serve
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
                    
                    print_success "Service file created"
                    
                    # Reload systemd and enable service
                    sudo systemctl daemon-reload
                    sudo systemctl enable offgrid-llm.service
                    print_success "Service enabled (will start on boot)"
                    
                    # Ask to start now
                    read -p "$(echo -e ${CYAN}Start the service now? [Y/n]: ${RESET})" -n 1 -r
                    echo ""
                    
                    if [[ ! $REPLY =~ ^[Nn]$ ]]; then
                        sudo systemctl start offgrid-llm.service
                        sleep 2
                        
                        if systemctl is-active --quiet offgrid-llm.service; then
                            print_success "Service started successfully"
                            echo ""
                            echo -e "${BOLD}Service Status${RESET}"
                            echo -e "  ${CYAN}systemctl status offgrid-llm${RESET}      Check status"
                            echo -e "  ${CYAN}systemctl stop offgrid-llm${RESET}        Stop service"
                            echo -e "  ${CYAN}systemctl restart offgrid-llm${RESET}     Restart service"
                            echo -e "  ${CYAN}journalctl -u offgrid-llm -f${RESET}      View logs"
                            echo ""
                            echo -e "${BOLD}Server Running${RESET}"
                            echo -e "  ${DIM}Endpoint${RESET}  http://localhost:11611"
                            echo -e "  ${DIM}Web UI${RESET}    http://localhost:11611/ui"
                            echo ""
                        else
                            print_error "Service failed to start"
                            echo -e "  ${DIM}Check logs:${RESET} ${CYAN}journalctl -u offgrid-llm -n 50${RESET}"
                        fi
                    else
                        echo ""
                        echo -e "${DIM}Start manually with:${RESET} ${CYAN}sudo systemctl start offgrid-llm${RESET}"
                        echo ""
                    fi
                else
                    echo ""
                    echo -e "${BOLD}Quick Start${RESET}"
                    echo ""
                    echo -e "  ${CYAN}offgrid catalog${RESET}        Browse available models"
                    echo -e "  ${CYAN}offgrid quantization${RESET}   Learn about quantization"
                    echo -e "  ${CYAN}offgrid serve${RESET}          Start the server"
                    echo ""
                fi
            else
                echo -e "${BOLD}Quick Start${RESET}"
                echo ""
                echo -e "  ${CYAN}offgrid catalog${RESET}        Browse available models"
                echo -e "  ${CYAN}offgrid quantization${RESET}   Learn about quantization"
                echo -e "  ${CYAN}offgrid serve${RESET}          Start the server"
                echo ""
            fi
            
            echo -e "${DIM}Run ${CYAN}offgrid help${DIM} for all commands${RESET}"
            echo ""
        else
            print_error "Installation failed - 'offgrid' not found in PATH"
            exit 1
        fi
        ;;
        
    --user)
        print_step "User Installation"
        print_info "Installing to user bin (no sudo required)"
        echo ""
        
        # Install using go install
        print_info "Building and installing..."
        make install > /dev/null 2>&1
        print_success "Build complete"
        
        GOPATH=$(go env GOPATH)
        GOBIN="$GOPATH/bin"
        
        # Check if Go bin is in PATH
        if [[ ":$PATH:" == *":$GOBIN:"* ]]; then
            VERSION=$(offgrid --version 2>/dev/null || echo 'dev')
            
            echo ""
            echo -e "${BOLD}${GREEN}╭─────────────────────────────────────────────────╮${RESET}"
            echo -e "${BOLD}${GREEN}│                                                 │${RESET}"
            echo -e "${BOLD}${GREEN}│              Installation Complete              │${RESET}"
            echo -e "${BOLD}${GREEN}│                                                 │${RESET}"
            echo -e "${BOLD}${GREEN}╰─────────────────────────────────────────────────╯${RESET}"
            echo ""
            echo -e "  ${DIM}Location${RESET}  $GOBIN/offgrid"
            echo -e "  ${DIM}Version${RESET}   $VERSION"
            echo ""
            echo -e "${BOLD}Quick Start${RESET}"
            echo ""
            echo -e "  ${CYAN}offgrid catalog${RESET}        Browse available models"
            echo -e "  ${CYAN}offgrid serve${RESET}          Start the server"
            echo ""
            echo -e "${DIM}Run ${CYAN}offgrid help${DIM} for all commands${RESET}"
            echo ""
        else
            echo ""
            echo -e "${BOLD}${YELLOW}╭─────────────────────────────────────────────────╮${RESET}"
            echo -e "${BOLD}${YELLOW}│                                                 │${RESET}"
            echo -e "${BOLD}${YELLOW}│         Installation Complete - Setup PATH      │${RESET}"
            echo -e "${BOLD}${YELLOW}│                                                 │${RESET}"
            echo -e "${BOLD}${YELLOW}╰─────────────────────────────────────────────────╯${RESET}"
            echo ""
            echo -e "  ${DIM}Location${RESET}  $GOBIN/offgrid"
            echo ""
            echo -e "${BOLD}Add to PATH${RESET}"
            echo ""
            echo -e "  ${DIM}1.${RESET} Add to ${CYAN}~/.bashrc${RESET} or ${CYAN}~/.zshrc${RESET}"
            echo -e "     ${DIM}export PATH=\"\$PATH:$GOBIN\"${RESET}"
            echo ""
            echo -e "  ${DIM}2.${RESET} Reload your shell"
            echo -e "     ${CYAN}source ~/.bashrc${RESET}"
            echo ""
            echo -e "  ${DIM}3.${RESET} Verify installation"
            echo -e "     ${CYAN}which offgrid${RESET}"
            echo ""
            echo -e "${DIM}Or use the full path:${RESET}"
            echo -e "  ${CYAN}$GOBIN/offgrid catalog${RESET}"
            echo ""
        fi
        ;;
        
    *)
        echo -e "${BOLD}Usage${RESET}"
        echo ""
        echo -e "  ${CYAN}$0${RESET} [--user|--system]"
        echo ""
        echo -e "${BOLD}Options${RESET}"
        echo ""
        echo -e "  ${CYAN}--system${RESET}   Install to /usr/local/bin (requires sudo) ${DIM}[default]${RESET}"
        echo -e "  ${CYAN}--user${RESET}     Install to \$GOPATH/bin (no sudo required)"
        echo ""
        echo -e "${BOLD}Examples${RESET}"
        echo ""
        echo -e "  ${CYAN}$0${RESET}              System-wide installation"
        echo -e "  ${CYAN}$0 --system${RESET}     System-wide installation (explicit)"
        echo -e "  ${CYAN}$0 --user${RESET}       User installation"
        echo ""
        exit 1
        ;;
esac
