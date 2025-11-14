# Auto-Start Service for OffGrid LLM

OffGrid LLM includes automatic startup functionality for the llama-server inference engine using systemd on Linux systems.

## What Gets Auto-Started?

When you install OffGrid LLM using the installer script, a systemd service is automatically configured to:

1. **Start llama-server on boot** - The inference engine starts automatically when your system boots
2. **Use the first available model** - Automatically loads the smallest GGUF model from `~/.offgrid-llm/models/`
3. **Run on a secure port** - Uses port `48081` (uncommon, high port) by default
4. **Restart on failure** - Automatically restarts if the service crashes
5. **Run as your user** - Runs with your user permissions for security

## Installation

The auto-start service is automatically configured when you install using:

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash
```

Or when installing from a USB package using the `install.sh` script.

### What Gets Installed

1. **Startup Script**: `/usr/local/bin/llama-server-start.sh`
   - Finds available models
   - Reads port configuration
   - Starts llama-server with optimal settings

2. **Systemd Service**: `/etc/systemd/system/llama-server@.service`
   - Per-user service template
   - Automatic restart on failure
   - Security hardening enabled

3. **Port Configuration**: `/etc/offgrid/llama-port`
   - Default: `48081`
   - Can be customized

## Service Management

### Check Service Status

```bash
sudo systemctl status llama-server@$USER
```

### Start Service Now

```bash
sudo systemctl start llama-server@$USER
```

### Stop Service

```bash
sudo systemctl stop llama-server@$USER
```

### Restart Service

```bash
sudo systemctl restart llama-server@$USER
```

### Disable Auto-Start

```bash
sudo systemctl disable llama-server@$USER
```

### Re-enable Auto-Start

```bash
sudo systemctl enable llama-server@$USER
```

### View Logs

```bash
# Recent logs
sudo journalctl -u llama-server@$USER -n 50

# Follow logs in real-time
sudo journalctl -u llama-server@$USER -f

# Logs since boot
sudo journalctl -u llama-server@$USER -b
```

## Configuration

### Change Port

To use a different port:

1. Edit the port configuration:
   ```bash
   echo "8081" | sudo tee /etc/offgrid/llama-port
   ```

2. Restart the service:
   ```bash
   sudo systemctl restart llama-server@$USER
   ```

### Specify a Model

By default, the service auto-loads the smallest model. To use a specific model, edit the startup script:

```bash
sudo nano /usr/local/bin/llama-server-start.sh
```

Change the `MODEL_FILE` line to specify your preferred model:

```bash
MODEL_FILE="$MODELS_DIR/your-preferred-model.gguf"
```

### Adjust Resources

Edit the startup script to modify:
- **Context size**: Change `-c 4096` to your desired context window
- **Thread count**: Change `--threads 4` to match your CPU cores
- **GPU layers**: Add `--n-gpu-layers 32` for GPU acceleration

Example with GPU:

```bash
sudo nano /usr/local/bin/llama-server-start.sh
```

Add to the `llama-server` command:

```bash
exec llama-server \
    --model "$MODEL_FILE" \
    --port "$PORT" \
    --host 127.0.0.1 \
    -c 4096 \
    --threads 8 \
    --n-gpu-layers 32 \
    --metrics
```

## Troubleshooting

### Service Won't Start

Check if models are available:

```bash
ls -lh ~/.offgrid-llm/models/
```

If no models exist, download one:

```bash
offgrid download tinyllama-1.1b-chat
```

### Check Why Service Failed

```bash
sudo systemctl status llama-server@$USER
sudo journalctl -u llama-server@$USER -n 100
```

### llama-server Not Found

Ensure llama.cpp is installed:

```bash
which llama-server
```

If not found, reinstall:

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash
```

### Port Already in Use

Change the port configuration:

```bash
echo "48082" | sudo tee /etc/offgrid/llama-port
sudo systemctl restart llama-server@$USER
```

### Permission Issues

Ensure the service runs as your user:

```bash
sudo systemctl cat llama-server@$USER | grep User
```

Should show: `User=%i` (which expands to your username)

## Manual Installation

If you didn't use the installer but want to set up auto-start:

1. **Create startup script**:
   ```bash
   sudo curl -o /usr/local/bin/llama-server-start.sh \
     https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/scripts/llama-server-start.sh
   sudo chmod +x /usr/local/bin/llama-server-start.sh
   ```

2. **Create systemd service**:
   ```bash
   sudo tee /etc/systemd/system/llama-server@.service > /dev/null << 'EOF'
   [Unit]
   Description=Llama.cpp HTTP Server for OffGrid LLM
   After=network.target

   [Service]
   Type=simple
   User=%i
   Environment="HOME=/home/%i"
   ExecStart=/usr/local/bin/llama-server-start.sh
   Restart=always
   RestartSec=5s
   StandardOutput=journal
   StandardError=journal

   # Security hardening
   NoNewPrivileges=true
   PrivateTmp=true

   [Install]
   WantedBy=multi-user.target
   EOF
   ```

3. **Configure port**:
   ```bash
   sudo mkdir -p /etc/offgrid
   echo "48081" | sudo tee /etc/offgrid/llama-port
   ```

4. **Enable and start**:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable llama-server@$USER
   sudo systemctl start llama-server@$USER
   ```

## Security Considerations

### Why Port 48081?

- **Uncommon**: Not typically used by other applications
- **High port**: Above 1024, doesn't require root
- **Non-standard**: Less likely to be scanned by attackers

### Localhost Only

The service binds to `127.0.0.1` (localhost) by default, meaning:
- Only accessible from the local machine
- Not exposed to the network
- Safe from remote attacks

### User Isolation

The service runs as your user, not root:
- Limited permissions
- Can only access your files
- Isolated from system resources

### Security Hardening

The systemd service includes:
- `NoNewPrivileges=true` - Prevents privilege escalation
- `PrivateTmp=true` - Isolated temporary directory

## Platform Support

### Linux with systemd

✅ **Fully supported** - All distributions with systemd (Ubuntu, Debian, Fedora, Arch, etc.)

### Linux without systemd

⚠️ **Manual setup required** - Use init.d or your init system

### macOS

⚠️ **Not included** - Use `launchd` or run manually. See [macOS Auto-Start Guide](./MACOS_AUTOSTART.md) (coming soon)

### Windows

⚠️ **Not included** - Use Task Scheduler or Windows Services. See [Windows Auto-Start Guide](./WINDOWS_AUTOSTART.md) (coming soon)

## Benefits

1. **Zero-config startup** - Works out of the box after installation
2. **Survives reboots** - Always available after system restart
3. **Automatic recovery** - Restarts if crashes occur
4. **Optimal defaults** - Pre-configured for best performance
5. **User-specific** - Each user can have their own instance

## See Also

- [Installation Guide](INSTALLATION.md)
- [Quick Start](QUICKSTART_HF.md)
- [Model Setup](MODEL_SETUP.md)
- [Performance Tuning](PERFORMANCE.md)
