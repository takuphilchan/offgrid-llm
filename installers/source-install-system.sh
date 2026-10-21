#!/usr/bin/env bash
# Source-install and service helpers for scripts/build/install-from-source.sh.

install_binary() {
    print_progress "Installing OffGrid LLM Binary" "~10 seconds"

    local INSTALL_DIR="/usr/local/bin"

    print_step "Installing binary to $INSTALL_DIR/offgrid..."
    sudo install -o0 -g0 -m755 offgrid "$INSTALL_DIR/offgrid"

    print_success "Binary installed to $INSTALL_DIR/offgrid"

    # Verify installation
    if command -v offgrid &> /dev/null; then
        INSTALLED_VERSION=$(offgrid --version 2>/dev/null || echo "unknown")
        print_success "Installation verified: $INSTALLED_VERSION"
    fi
}

# Create User and Groups
setup_user() {
    print_progress "Setting Up Service User" "~10 seconds"

    # Create offgrid user if it doesn't exist
    if ! id offgrid &> /dev/null; then
        print_step "Creating offgrid system user..."
        sudo useradd -r -s /bin/false -U -m -d /var/lib/offgrid offgrid
        print_success "User 'offgrid' created"
    else
        print_info "User 'offgrid' already exists"
    fi

    # Ensure /var/lib/offgrid exists with proper permissions
    sudo mkdir -p /var/lib/offgrid
    sudo chown offgrid:offgrid /var/lib/offgrid
    sudo chmod 755 /var/lib/offgrid

    # Add offgrid user to current user's primary group for model access
    if [ -n "$USER" ] && id "$USER" &>/dev/null; then
        USER_GROUP=$(id -gn "$USER")
        print_step "Adding offgrid user to '${USER_GROUP}' group for model access..."
        sudo usermod -aG "$USER_GROUP" offgrid 2>/dev/null || true
        print_success "offgrid user can now read models from user home directories"
    fi

    # Add to video and render groups for GPU access
    if [ "$GPU_TYPE" != "none" ]; then
        print_step "Adding offgrid user to GPU groups..."
        sudo usermod -aG video offgrid 2>/dev/null || true
        sudo usermod -aG render offgrid 2>/dev/null || true
    fi
}

# Create Systemd Service
# Setup llama-server systemd service
setup_llama_server_service() {
    print_progress "Setting Up llama-server Service" "~30 seconds"

    # Verify llama-server binary exists
    if [ ! -f "/usr/local/bin/llama-server" ]; then
        print_error "llama-server binary not found at /usr/local/bin/llama-server"
        print_warning "Skipping llama-server service setup"
        print_info "OffGrid LLM will run in mock mode"
        return 1
    fi

    # Verify llama-server can run
    if ! /usr/local/bin/llama-server --version &> /dev/null; then
        print_warning "llama-server binary exists but may have issues"
        print_info "Checking dependencies..."
        ldd /usr/local/bin/llama-server | grep "not found" || true
    fi

    local SERVICE_FILE="/etc/systemd/system/llama-server.service"

    # Generate a random high port for internal use only (49152-65535 dynamic/private range)
    # Using a specific seed based on hostname for consistency across restarts
    local RANDOM_PORT=$((49152 + $(hostname | md5sum | cut -c1-4 | xargs -I{} printf "%d" 0x{}) % 16384))
    print_info "Using internal port: $RANDOM_PORT (localhost-only, not externally accessible)"

    # Find a model to use (prefer tinyllama)
    local MODEL_PATH=""
    local MODELS_DIR="/var/lib/offgrid/models"

    # Create models directory if it doesn't exist
    sudo mkdir -p "$MODELS_DIR"
    sudo chown offgrid:offgrid "$MODELS_DIR"

    # Make models directory readable by all, writable by group
    # Use setgid so new files inherit the offgrid group
    sudo chmod 2775 "$MODELS_DIR"  # 2 = setgid bit

    # Create marker file to indicate system directory is ready
    sudo touch "$MODELS_DIR/.offgrid"
    sudo chown offgrid:offgrid "$MODELS_DIR/.offgrid"

    # Add current user to offgrid group for write access
    if ! groups | grep -q offgrid; then
        print_info "Adding current user to 'offgrid' group for model management..."
        sudo usermod -a -G offgrid "$USER"
        print_info "Group membership will be active in new terminal sessions"
    fi

    if [ -f "$MODELS_DIR/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf" ]; then
        MODEL_PATH="$MODELS_DIR/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf"
    elif [ -f "$MODELS_DIR/tinyllama-1.1b-chat.Q4_K_M.gguf" ]; then
        MODEL_PATH="$MODELS_DIR/tinyllama-1.1b-chat.Q4_K_M.gguf"
    elif [ -f "$MODELS_DIR/tinyllama.gguf" ]; then
        MODEL_PATH="$MODELS_DIR/tinyllama.gguf"
    else
        # Use the first .gguf file found
        MODEL_PATH=$(sudo find "$MODELS_DIR" -name "*.gguf" -type f 2>/dev/null | head -n 1)
    fi

    if [ -z "$MODEL_PATH" ]; then
        print_warning "No model found in $MODELS_DIR"
        print_info "You can download a model with:"
        print_dim "  wget https://huggingface.co/TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF/resolve/main/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf"
        print_dim "  sudo mv tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf $MODELS_DIR/"
        print_dim "  sudo chown offgrid:offgrid $MODELS_DIR/*.gguf"
        print_dim "  sudo systemctl restart llama-server"
        MODEL_PATH="$MODELS_DIR/model.gguf"
    else
        print_info "Using model: $(basename $MODEL_PATH)"
        # Fix permissions on model files
        sudo chmod 644 "$MODELS_DIR"/*.gguf 2>/dev/null || true
        sudo chown offgrid:offgrid "$MODELS_DIR"/*.gguf 2>/dev/null || true
    fi

    print_step "Creating llama-server service file at $SERVICE_FILE..."

    # Determine GPU layers based on GPU type
    local GPU_LAYERS="0"
    local EXTRA_ARGS=""
    if [ "$GPU_TYPE" = "nvidia" ]; then
        GPU_LAYERS="99"  # Offload all layers to GPU
        EXTRA_ARGS="--n-gpu-layers 99"
        print_info "Configured for NVIDIA GPU acceleration (99 layers offloaded)"
    elif [ "$GPU_TYPE" = "amd" ]; then
        GPU_LAYERS="99"
        EXTRA_ARGS="--n-gpu-layers 99"
        print_info "Configured for AMD GPU acceleration (99 layers offloaded)"
    else
        print_info "Configured for CPU-only inference"
    fi

    # Create startup script that reads active model from config
    print_step "Creating llama-server startup script..."
    local START_SCRIPT="/usr/local/bin/llama-server-start.sh"
    sudo tee "$START_SCRIPT" > /dev/null <<'START_SCRIPT_EOF'
#!/bin/bash
# llama-server startup script with dynamic model loading

ACTIVE_MODEL_FILE="/etc/offgrid/active-model"
SYSTEM_MODELS_DIR="/var/lib/offgrid/models"
LLAMA_PORT_FILE="/etc/offgrid/llama-port"

# Debug logging
exec 2>&1

echo "[$(date)] llama-server-start.sh starting..."

# Find the first available model if no active model is set
if [ -f "$ACTIVE_MODEL_FILE" ]; then
    MODEL_PATH=$(cat "$ACTIVE_MODEL_FILE")
    if [ ! -f "$MODEL_PATH" ]; then
        echo "[$(date)] Warning: Configured model not found: $MODEL_PATH"
        MODEL_PATH=""
    fi
fi

# If no active model or it doesn't exist, find the first .gguf file
if [ -z "$MODEL_PATH" ]; then
    echo "[$(date)] Searching for models in $SYSTEM_MODELS_DIR..."
    # Try system models directory first
    MODEL_PATH=$(find "$SYSTEM_MODELS_DIR" -maxdepth 1 -name "*.gguf" -type f 2>/dev/null | head -n 1)

    # If not found in system dir, search all user home directories
    if [ -z "$MODEL_PATH" ]; then
        echo "[$(date)] No models in system dir, searching user directories..."
        for user_home in /home/*; do
            if [ -d "$user_home/.offgrid-llm/models" ]; then
                echo "[$(date)] Checking $user_home/.offgrid-llm/models..."
                MODEL_PATH=$(find "$user_home/.offgrid-llm/models" -maxdepth 1 -name "*.gguf" -type f 2>/dev/null | head -n 1)
                [ -n "$MODEL_PATH" ] && break
            fi
        done
    fi

    if [ -z "$MODEL_PATH" ]; then
        echo "[$(date)] ERROR: No .gguf models found in $SYSTEM_MODELS_DIR or /home/*/.offgrid-llm/models"
        echo "[$(date)] Please download a model with: offgrid download-hf <repo> --file <file>"
        exit 1
    fi
    echo "[$(date)] Using first available model: $MODEL_PATH"
    # Save it as the active model for next time
    echo "$MODEL_PATH" > "$ACTIVE_MODEL_FILE"
fi

# Read port from config
if [ -f "$LLAMA_PORT_FILE" ]; then
    LLAMA_PORT=$(cat "$LLAMA_PORT_FILE")
else
    LLAMA_PORT=49233
fi

echo "[$(date)] Starting llama-server with model: $(basename $MODEL_PATH)"
echo "[$(date)] Port: $LLAMA_PORT"
echo "[$(date)] Model path: $MODEL_PATH"

# Check if model file is readable
if [ ! -r "$MODEL_PATH" ]; then
    echo "[$(date)] ERROR: Model file is not readable: $MODEL_PATH"
    ls -l "$MODEL_PATH"
    exit 1
fi

# Verify llama-server binary exists and is executable
if [ ! -x "/usr/local/bin/llama-server" ]; then
    echo "[$(date)] ERROR: llama-server not found or not executable at /usr/local/bin/llama-server"
    exit 1
fi

# Check library dependencies
echo "[$(date)] Checking library dependencies..."
if ! ldd /usr/local/bin/llama-server | grep -q "not found"; then
    echo "[$(date)] All dependencies satisfied"
else
    echo "[$(date)] WARNING: Missing dependencies:"
    ldd /usr/local/bin/llama-server | grep "not found"
fi

# Execute llama-server with the selected model
# Performance flags:
#   -c 2048: Context size (adjust based on RAM - lower uses less memory)
#   -b 256: Batch size (lower = faster time-to-first-token)
#   -fa: Flash attention (faster inference, less memory)
#   --cont-batching: Better throughput for concurrent requests
#   --cache-type-k/v q8_0: Quantized KV cache saves ~50% memory
#   --cache-prompt: Reuses prompt processing across requests (big speedup for chat)
echo "[$(date)] Executing llama-server..."
exec /usr/local/bin/llama-server -m "$MODEL_PATH" --port "$LLAMA_PORT" --host 127.0.0.1 -c 2048 -b 256 -fa --cont-batching --cache-type-k q8_0 --cache-type-v q8_0 --cache-prompt EXTRA_ARGS_PLACEHOLDER
START_SCRIPT_EOF

    # Replace EXTRA_ARGS_PLACEHOLDER with actual GPU args
    sudo sed -i "s/EXTRA_ARGS_PLACEHOLDER/$EXTRA_ARGS/" "$START_SCRIPT"
    sudo chmod +x "$START_SCRIPT"

    print_success "Created startup script at $START_SCRIPT"

    # Create initial active model config
    print_step "Setting up active model configuration..."
    sudo mkdir -p /etc/offgrid
    echo "$MODEL_PATH" | sudo tee /etc/offgrid/active-model > /dev/null
    sudo chmod 644 /etc/offgrid/active-model
    print_info "Active model: $(basename $MODEL_PATH)"

    sudo tee "$SERVICE_FILE" > /dev/null <<SERVICE_EOF
[Unit]
Description=llama.cpp Inference Server (Internal)
Documentation=https://github.com/ggerganov/llama.cpp
After=network-online.target
Wants=network-online.target
Before=offgrid-llm.service

[Service]
Type=simple
User=offgrid
Group=offgrid
WorkingDirectory=/var/lib/offgrid
ExecStart=/usr/local/bin/llama-server-start.sh
Restart=always
RestartSec=3

# Environment variables
Environment="PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
Environment="LD_LIBRARY_PATH=/usr/local/lib"
Environment="GGML_BACKEND_DL_PATH=/usr/local/lib"
Environment="LLAMA_SERVER_INTERNAL=1"

# Security hardening - balanced security with functionality
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=/var/lib/offgrid
ReadWritePaths=/etc/offgrid

[Install]
WantedBy=multi-user.target
SERVICE_EOF

    # Store the port in a config file for OffGrid to read
    print_step "Saving internal port configuration..."
    sudo mkdir -p /etc/offgrid
    echo "$RANDOM_PORT" | sudo tee /etc/offgrid/llama-port > /dev/null
    sudo chmod 644 /etc/offgrid/llama-port

    print_success "llama-server service created on internal port $RANDOM_PORT"

    print_step "Reloading systemd daemon..."
    sudo systemctl daemon-reload

    print_step "Enabling llama-server service..."
    sudo systemctl enable llama-server.service

    print_success "llama-server service enabled (will start on boot)"

    # Start the service now
    print_step "Starting llama-server service..."
    sudo systemctl start llama-server.service

    sleep 2

    if sudo systemctl is-active --quiet llama-server.service; then
        print_success "llama-server is running on internal port $RANDOM_PORT!"

        # Wait for server to be ready
        print_step "Waiting for llama-server to be ready..."
        for i in {1..10}; do
            if curl -s http://127.0.0.1:${RANDOM_PORT}/health | grep -q "ok"; then
                print_success "llama-server health check passed"
                print_info "Internal endpoint: http://127.0.0.1:${RANDOM_PORT} (not accessible externally)"
                return 0
            fi
            sleep 1
        done
        print_warning "llama-server is running but health check timed out"
    else
        print_error "Failed to start llama-server service"
        print_info "Check logs with: sudo journalctl -u llama-server.service -n 50"
    fi
}

setup_systemd_service() {
    print_progress "Setting Up OffGrid Systemd Service" "~20 seconds"

    local SERVICE_FILE="/etc/systemd/system/offgrid-llm.service"

    # Read the internal llama-server port
    local LLAMA_PORT=42382
    if [ -f "/etc/offgrid/llama-port" ]; then
        LLAMA_PORT=$(cat /etc/offgrid/llama-port)
        print_info "Configuring OffGrid to connect to llama-server on port $LLAMA_PORT"
    fi

    print_step "Creating service file at $SERVICE_FILE..."

    sudo tee "$SERVICE_FILE" > /dev/null <<SERVICE_EOF
[Unit]
Description=OffGrid LLM - Offline AI Inference Engine
Documentation=https://github.com/yourusername/offgrid-llm
After=network-online.target llama-server.service
Wants=network-online.target
Requires=llama-server.service

[Service]
Type=simple
User=offgrid
Group=offgrid
WorkingDirectory=/var/lib/offgrid
ExecStart=/usr/local/bin/offgrid serve
Restart=always
RestartSec=3

# Environment variables
Environment="PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
Environment="OFFGRID_PORT=11611"
Environment="OFFGRID_MODELS_DIR=/var/lib/offgrid/models"
Environment="LLAMA_SERVER_URL=http://127.0.0.1:${LLAMA_PORT}"

# Security settings - balanced security with functionality
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=/var/lib/offgrid

[Install]
WantedBy=multi-user.target
SERVICE_EOF

    print_success "Systemd service created"

    print_step "Reloading systemd daemon..."
    sudo systemctl daemon-reload

    print_step "Enabling offgrid-llm service..."
    sudo systemctl enable offgrid-llm.service

    print_success "Service enabled (will start on boot)"
}

# Setup Configuration
setup_config() {
    print_progress "Setting Up Configuration" "~10 seconds"

    local CONFIG_DIR="/var/lib/offgrid"
    local MODELS_DIR="$CONFIG_DIR/models"
    local WEB_DIR="$CONFIG_DIR/web/ui"

    print_step "Creating directories..."
    sudo mkdir -p "$MODELS_DIR"
    sudo mkdir -p "$WEB_DIR"

    print_step "Building and copying web UI..."
    if [ -f "apps/web/package-lock.json" ] && command -v npm >/dev/null 2>&1; then
        (cd apps/web && npm ci && npm run api:check && npm run build)
    fi
    if [ -f "apps/web/dist/index.html" ]; then
        sudo cp -r apps/web/dist/. "$WEB_DIR/"
        print_success "Web UI files copied"
    else
        print_warning "No generated UI found; run scripts/build/sync-ui.sh first"
    fi

    sudo chown -R offgrid:offgrid "$CONFIG_DIR"

    print_success "Configuration directories created"
}

# Install Shell Completions
install_completions() {
    print_progress "Installing Shell Completions" "~10 seconds"

    # Detect user's shell
    USER_SHELL=$(basename "$SHELL")

    case "$USER_SHELL" in
        bash)
            print_step "Installing Bash completions..."

            # Try user installation (safer, doesn't require sudo)
            mkdir -p "$HOME/.bash_completion.d"
            /usr/local/bin/offgrid completions bash > "$HOME/.bash_completion.d/offgrid" 2>/dev/null || {
                print_warning "Could not generate completions (offgrid binary might not be ready yet)"
                return
            }

            # Add sourcing to .bashrc if not already there
            if ! grep -q "\.bash_completion\.d/offgrid" "$HOME/.bashrc" 2>/dev/null; then
                echo "" >> "$HOME/.bashrc"
                echo "# OffGrid completions" >> "$HOME/.bashrc"
                echo "[ -f ~/.bash_completion.d/offgrid ] && source ~/.bash_completion.d/offgrid" >> "$HOME/.bashrc"
            fi

            print_success "Bash completions installed to ~/.bash_completion.d/offgrid"
            print_info "Restart your shell or run: source ~/.bashrc"
            ;;

        zsh)
            print_step "Installing Zsh completions..."

            ZSH_COMPLETIONS="${XDG_DATA_HOME:-$HOME/.local/share}/zsh/site-functions"
            mkdir -p "$ZSH_COMPLETIONS"

            /usr/local/bin/offgrid completions zsh > "$ZSH_COMPLETIONS/_offgrid" 2>/dev/null || {
                print_warning "Could not generate completions"
                return
            }

            print_success "Zsh completions installed to $ZSH_COMPLETIONS/_offgrid"
            print_info "Restart your shell or run: exec zsh"
            ;;

        fish)
            print_step "Installing Fish completions..."

            mkdir -p "$HOME/.config/fish/completions"
            /usr/local/bin/offgrid completions fish > "$HOME/.config/fish/completions/offgrid.fish" 2>/dev/null || {
                print_warning "Could not generate completions"
                return
            }

            print_success "Fish completions installed (active immediately)"
            ;;

        *)
            print_info "Shell: $USER_SHELL (manual completion setup required)"
            print_info "Run: offgrid completions <bash|zsh|fish> to generate"
            ;;
    esac
}

# Start Service
start_service() {
    print_progress "Starting OffGrid LLM Service" "~15 seconds"

    print_step "Starting offgrid-llm service..."
    sudo systemctl start offgrid-llm.service

    sleep 2

    if sudo systemctl is-active --quiet offgrid-llm.service; then
        print_success "Service started successfully"

        print_info "Service status:"
        sudo systemctl status offgrid-llm.service --no-pager -l | head -n 10
    else
        print_error "Service failed to start"
        print_info "Check logs with: sudo journalctl -u offgrid-llm.service -n 50"
        exit 1
    fi
}

# Display Summary
