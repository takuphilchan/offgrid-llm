# Scripts Directory

This directory contains utility scripts for building, installing, and managing OffGrid LLM.

## Files

### Installation Scripts

- **`install.sh`** - Main installation script for CLI deployment
  - Detects OS, CPU, and GPU automatically
  - Builds and installs OffGrid LLM binary
  - Installs web UI
  - Optionally sets up auto-start service
  - Usage: `curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/scripts/install.sh | bash`

### Build Scripts

- **`build-all.sh`** - Cross-platform build automation
  - Builds binaries for Linux, macOS (AMD64/ARM64), and Windows
  - Creates optimized production builds
  - Outputs to `build/` directory

### Server Management

- **`start-server.sh`** - Manual server startup script
  - Starts OffGrid LLM server with default configuration
  - Useful for testing and development
  - Alternative to systemd service

## Usage Examples

**Install OffGrid LLM:**
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/scripts/install.sh | bash
```

**Build for all platforms:**
```bash
./scripts/build-all.sh
```

**Start server manually:**
```bash
./scripts/start-server.sh
```

## Related Documentation

- [Installation Guide](../docs/INSTALLATION.md) - Complete installation documentation
- [Building Guide](../docs/advanced/BUILDING.md) - Build from source instructions
- [Docker Deployment](../docker/README.md) - Containerized deployment option
