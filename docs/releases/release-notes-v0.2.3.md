# OffGrid LLM v0.2.3 Release Notes

**Release Date:** January 2026

## Highlights

This release introduces **AI Agents with MCP support**, **single-user mode as default**, **enhanced authentication**, and **comprehensive metrics**.

---

## New Features

### AI Agent System

Autonomous AI agents that can use tools to complete tasks:

```bash
# Run an agent task via CLI
offgrid agent run "Calculate 15 * 23 + 47" --model llama3

# List available tools
offgrid agent tools
```

**Built-in Tools:**
| Tool | Description |
|------|-------------|
| `calculator` | Evaluate mathematical expressions |
| `current_time` | Get current date/time |
| `read_file` | Read file contents |
| `write_file` | Write content to files |
| `list_files` | List directory contents |
| `shell` | Execute shell commands |
| `http_get` | Make HTTP GET requests |

**API Endpoints:**
```bash
# Run agent
POST /v1/agents/run
{
  "model": "llama3",
  "prompt": "Your task here",
  "style": "react",
  "max_steps": 10
}

# List tools
GET /v1/agents/tools

# Toggle tool
PATCH /v1/agents/tools
{"name": "shell", "enabled": false}
```

### MCP Server Integration

Connect external tools via [Model Context Protocol](https://modelcontextprotocol.io/):

```bash
# Add MCP server via CLI
offgrid agent mcp add filesystem "npx -y @modelcontextprotocol/server-filesystem /tmp"

# List MCP servers
offgrid agent mcp list

# Via API
POST /v1/agents/mcp
{
  "name": "filesystem",
  "url": "npx -y @modelcontextprotocol/server-filesystem /tmp"
}
```

### Single-User Mode (Default)

OffGrid now runs in **single-user mode by default** for simpler local AI workflows:

- No login required
- Users/Metrics tabs hidden in UI
- CLI shows helpful message for multi-user features

**Enable multi-user mode:**
```bash
# Via environment variable
export OFFGRID_MULTI_USER=true
offgrid serve

# Or in config.yaml
multi_user_mode: true
```

### System Configuration API

New endpoint to check feature flags and configuration:

```bash
GET /v1/system/config
```

Response:
```json
{
  "multi_user_mode": false,
  "require_auth": false,
  "guest_access": true,
  "version": "0.2.3",
  "features": {
    "users": false,
    "metrics": true,
    "agent": true,
    "lora": true
  }
}
```

### Enhanced UI

- **Agent Tab**: Run agents, manage tools, configure MCP servers
- **Metrics Tab**: Real-time stats, Prometheus metrics viewer (multi-user mode)
- **LoRA Tab**: Register and manage LoRA adapters
- **Dynamic sidebar**: Shows/hides features based on mode

---

## Improvements

### Authentication
- Fixed API key persistence bug
- Improved bypass paths middleware
- Added password field to user creation form
- Show/hide password toggle in login

### Environment Variable Overrides
- Config file values can now be overridden by environment variables
- Added `OFFGRID_MULTI_USER`, `OFFGRID_REQUIRE_AUTH`, `OFFGRID_GUEST_ACCESS`

### CLI Enhancements
- `offgrid users` shows helpful message in single-user mode
- `offgrid agent` subcommands for agent management
- Better error messages and guidance

---

## Documentation

New guides added:
- [AI Agent Guide](../guides/AGENT_GUIDE.md) - Using agents and MCP
- [Metrics Guide](../guides/METRICS_GUIDE.md) - Prometheus metrics and monitoring
- [Multi-User Mode Guide](../guides/MULTI_USER_MODE.md) - Authentication and user management

Updated documentation:
- API Reference with new endpoints
- CLI Reference with agent commands

---

## API Changes

### New Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/system/config` | GET | Get feature flags and configuration |
| `/v1/agents/run` | POST | Run an agent task |
| `/v1/agents/tools` | GET | List available tools |
| `/v1/agents/tools` | PATCH | Toggle tool enabled state |
| `/v1/agents/mcp` | GET | List MCP servers |
| `/v1/agents/mcp` | POST | Add MCP server |
| `/v1/agents/mcp/{name}` | DELETE | Remove MCP server |
| `/v1/agents/mcp/test` | POST | Test MCP server connection |

---

## Breaking Changes

None - this release is fully backward compatible.

---

## Upgrade Guide

### From v0.2.2

1. Update the binary:
```bash
# Using installer
curl -sSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/scripts/install.sh | bash

# Or build from source
git pull
go build -o offgrid ./cmd/offgrid/
sudo cp offgrid /usr/local/bin/
```

2. (Optional) Enable multi-user mode if needed:
```bash
export OFFGRID_MULTI_USER=true
```

3. Restart the server:
```bash
offgrid serve
```

---

## Known Issues

- MCP servers require Node.js/npx for most official servers
- LoRA adapters must be in GGUF format (convert with llama.cpp tools)

---

## Contributors

Thanks to all contributors who made this release possible!

---

See [GitHub Releases](https://github.com/takuphilchan/offgrid-llm/releases/tag/v0.2.3) for the complete list of changes.
