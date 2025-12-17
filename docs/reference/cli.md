# OffGrid LLM - Complete CLI Reference

## Overview

All CLI commands now feature a consistent industrial/brutalist design with comprehensive error handling, helpful messages, and actionable guidance.

## Quick Reference

### Model Discovery
```bash
offgrid search llama --author TheBloke    # Search HuggingFace
offgrid catalog                            # Browse curated catalog
```

### Model Management
```bash
offgrid download tinyllama-1.1b-chat Q4_K_M        # From catalog
offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF  # From HuggingFace
offgrid list                                        # Show installed
offgrid remove tinyllama-1.1b-chat.Q4_K_M          # Delete model
```

### Import/Export
```bash
offgrid import /media/usb                          # Import from USB
offgrid export tinyllama-1.1b-chat.Q4_K_M /media/usb  # Export to USB
```

### Knowledge Base (RAG)
```bash
offgrid kb status                          # Show RAG status
offgrid kb list                            # List documents
offgrid kb add ./docs/manual.md            # Add a document
offgrid kb search "how to configure"       # Search knowledge base
offgrid kb remove <id>                     # Remove document
offgrid kb clear                           # Clear all documents
```

### Inference
```bash
offgrid serve                              # Start HTTP server
offgrid run tinyllama-1.1b-chat.Q4_K_M    # Interactive chat
offgrid benchmark tinyllama-1.1b-chat.Q4_K_M  # Performance test
```

### AI Agents (New)
```bash
offgrid agent run "Calculate factorial of 10"  # Run agent task
offgrid agent tools                             # List available tools
offgrid agent mcp list                          # List MCP servers
offgrid agent mcp add name "npx -y @modelcontextprotocol/server-filesystem /tmp"
```

### User Management (Multi-User Mode)
```bash
offgrid users                              # List users (requires OFFGRID_MULTI_USER=true)
offgrid users create alice                 # Create a user
offgrid users delete <user-id>             # Delete a user
```

### Information
```bash
offgrid info          # System information
offgrid quantization  # Learn about quantization
offgrid help          # Command help
```

## Design Features

### Visual Theme
- **Box Drawing**: Lines and dividers for clear sections
- **Icons**: Consistent symbols for status and actions
- **Structure**: Clear hierarchy with consistent spacing
- **Brutalist**: Minimal, functional, industrial aesthetic

###  Error Handling
Every command validates inputs and provides:
- [X] Clear error explanation
-  Available options when applicable
-  Helpful tips and next steps
-  Suggestions for fix

### Smart Feedback
- Model existence validation before operations
- File size and space calculations
- Progress indicators for long operations
- Available models listing on errors
- Helpful command suggestions

## Command Details

### `offgrid list`

Shows all installed models with sizes and metadata.

**Output:**
```
[Package] Installed Models
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Found 1 model(s):

  • tinyllama-1.1b-chat.Q4_K_M · 637.8 MB · Q4_K_M

Total size: 637.8 MB

Next steps:
  • Start chat:       offgrid run <model-name>
  • Start server:     offgrid serve
  • Benchmark model:  offgrid benchmark <model-name>
```

**Empty State:**
```
No models installed in /home/user/.offgrid-llm/models

Get started:
  • Search HuggingFace:  offgrid search llama
  • Download model:      offgrid download-hf <model-id>
  • Browse catalog:      offgrid catalog
```

---

### `offgrid run <model-name>`

Interactive chat interface with a model.

**Features:**
- Model existence validation
- Server connectivity check
- Box-drawing chat UI
- Conversation history
- Commands: `exit`, `clear`

**Chat Interface:**
```
 Starting interactive chat with tinyllama-1.1b-chat.Q4_K_M
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Commands: 'exit' to quit, 'clear' to reset conversation
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[Lightning] Connecting to inference engine... [OK]

┌─ You
│ Hello!
└─

┌─ Assistant
│ Hi! How can I help you today?
└─
```

**Error (Model Not Found):**
```
[X] Model not found: nonexistent-model

Available models:
  • tinyllama-1.1b-chat.Q4_K_M

Tip: Use 'offgrid list' to see all installed models
```

---

### `offgrid search <query> [flags]`

Search HuggingFace Hub for GGUF models.

**Flags:**
- `--author <name>`: Filter by author
- `--quantization <type>`: Filter by quantization (Q4_K_M, Q5_K_M, etc.)
- `--sort <field>`: Sort by downloads, likes, or recent
- `--limit <n>`: Limit results (default: 10)

**Example:**
```bash
**Example:**
```bash
offgrid search "llama 7b chat" --limit 5
```

**Output:**
```
Searching HuggingFace Hub...
```

**Output:**
```
 Searching HuggingFace Hub...

Found 5 GGUF models:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. TheBloke/Llama-2-7B-Chat-GGUF
   Downloads: 2.5M
   GGUF: Q4_K_M, Q5_K_M, Q6_K, Q8_0

[Download with: offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF]
```

---

### `offgrid download-hf <model-id> [--file <filename>]`

Download GGUF models directly from HuggingFace.

**Features:**
- Auto-detects GGUF files
- File selection UI for multiple files
- Progress tracking
- Resume support (partial)
- Automatically fetches matching vision projectors (mmproj) from the source repo or a curated fallback catalog (e.g., koboldcpp/mmproj) so VLM downloads stay single-step

**Vision fallback coverage:**
- Qwen2.5-VL (3B/7B, including VLM-R1 builds)
- Qwen2-VL (2B/7B) and Qwen2.5-VL vision variants
- LLaMA 3 vision checkpoints (8B) and legacy LLaVA 7B/13B adapters
- Gemma 3 (4B/12B/27B), MiniCPM, Pixtral 12B, Mistral Small 24B, Yi 34B, Obsidian 3B

To add more adapters, edit `internal/models/projector_fallbacks.go` (no rebuild of docs required) and list the model-id/filename substrings that should map to a particular projector file.

**Example:**
```bash
offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF --file llama-2-7b-chat.Q4_K_M.gguf
```

**File Selection UI:**
```
[Package] TheBloke/Llama-2-7B-Chat-GGUF
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Found 4 GGUF files:

  1. llama-2-7b-chat.Q4_K_M.gguf · Q4_K_M
  2. llama-2-7b-chat.Q5_K_M.gguf · Q5_K_M
  3. llama-2-7b-chat.Q6_K.gguf · Q6_K
  4. llama-2-7b-chat.Q8_0.gguf · Q8_0

Select file (1-4) or 'q' to quit:
```

---

### `offgrid remove <model-id>`

Remove an installed model with confirmation.

**Interactive:**
```
  Remove Model
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Model:  tinyllama-1.1b-chat.Q4_K_M
Path:   /home/user/.offgrid-llm/models/tinyllama-1.1b-chat.Q4_K_M.gguf
Size:   637.8 MB will be freed

[Warning]  This action cannot be undone. Continue? (y/N):
```

**After Deletion:**
```
[OK] Removed tinyllama-1.1b-chat.Q4_K_M

2 model(s) remaining
```

---

### `offgrid import <path>`

Import models from USB/SD card or directory.

**Single File:**
```bash
offgrid import /media/usb/tinyllama-1.1b-chat.Q4_K_M.gguf
```

**Directory:**
```bash
offgrid import /media/usb
```

**Progress:**
```
Scanning /media/usb

Found 2 model file(s):

  1. tinyllama-1.1b-chat.Q4_K_M (Q4_K_M) · 637.8 MB
  2. llama-2-7b-chat.Q5_K_M (Q5_K_M) · 4.5 GB

Importing models...

  [OK] tinyllama-1.1b-chat.Q4_K_M.gguf
  [OK] llama-2-7b-chat.Q5_K_M.gguf

[OK] Imported 2 model(s) to /home/user/.offgrid-llm/models
```

**Error (Path Not Found):**
```
[X] Path not found: /media/unknown

Common USB/SD mount points:
  • Linux:   /media/<username>/<device>
  • macOS:   /Volumes/<device>
  • Windows: D:\ E:\ F:\

Tip: Use 'ls /media' or 'mount' to find your device
```

---

### `offgrid export <model-id> <destination>`

Export model to USB/SD card.

**Example:**
```bash
offgrid export tinyllama-1.1b-chat.Q4_K_M /media/usb
```

**Output:**
```
[Package] Export Model
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Model:  tinyllama-1.1b-chat.Q4_K_M
From:   /home/user/.offgrid-llm/models/tinyllama-1.1b-chat.Q4_K_M.gguf
To:     /media/usb/tinyllama-1.1b-chat.Q4_K_M.gguf
Size:   637.8 MB

  Progress: 100.0% · 637.8 MB / 637.8 MB

[OK] Export complete
  Location: /media/usb/tinyllama-1.1b-chat.Q4_K_M.gguf
```

---

### `offgrid benchmark <model-id>`

Benchmark model performance.

**Output:**
```
[Lightning] Benchmark Model
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Model Information
  Name:          tinyllama-1.1b-chat.Q4_K_M
  Path:          /home/user/.offgrid-llm/models/tinyllama-1.1b-chat.Q4_K_M.gguf
  Size:          637.8 MB
  Quantization:  Q4_K_M

Performance Metrics
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  ⏳ This feature requires llama.cpp integration

  Metrics will include:
    • Model load time
    • Tokens per second (inference speed)
    • Memory usage (RAM/VRAM)
    • First token latency
    • Context processing speed

  Next steps:
    1. Ensure server is running: offgrid serve
    2. Use API endpoint: curl http://localhost:11611/v1/benchmark
```

---

### `offgrid catalog`

Browse curated model catalog.

**Output:**
```
 Model Catalog
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

tinyllama-1.1b-chat [Star]
  TinyLlama 1.1B Chat · 1.1B parameters · 2 GB RAM minimum
  Compact model for low-resource environments
  Variants: Q4_K_M (0.6 GB), Q5_K_M (0.7 GB)

llama-2-7b-chat [Star]
  Llama 2 7B Chat · 7B parameters · 8 GB RAM minimum
  Meta's open-source chat model, good balance of quality and size
  Variants: Q4_K_M (3.8 GB), Q5_K_M (4.5 GB)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Usage:
  offgrid download <model-id> [quantization]

Or search HuggingFace for more models:
  offgrid search llama --author TheBloke
```

---

### `offgrid help`

Show command reference.

**Output:**
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Usage
  offgrid [command]

Commands
  serve              Start HTTP inference server (default)
  search <query>     Search HuggingFace for models
  download <id>      Download a model from catalog
  download-hf <id>   Download from HuggingFace Hub
  run <model>        Interactive chat with a model
  import <path>      Import model(s) from USB/SD card
  export <id> <path> Export a model to USB/SD card
  remove <id>        Remove an installed model
  list               List installed models
  catalog            Show available models
  benchmark <id>     Benchmark model performance
  quantization       Explain quantization levels
  config <action>    Manage configuration
  info               Show system information
  help               Show this help

Examples
  offgrid search llama --author TheBloke
  offgrid download tinyllama-1.1b-chat
  offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF
  offgrid run tinyllama-1.1b-chat.Q4_K_M
  offgrid import /media/usb
  offgrid benchmark tinyllama-1.1b-chat.Q4_K_M

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

---

## Error Handling Examples

### Model Not Found
All model-related commands validate existence:
```
[X] Model not found: fake-model

Available models:
  • tinyllama-1.1b-chat.Q4_K_M
  • llama-2-7b-chat.Q5_K_M
```

### Missing Arguments
Usage help is shown automatically:
```
OFFGRID-LLM v0.1.6
Edge Inference Orchestrator

Usage: offgrid run <model-name>

Examples:
  offgrid run tinyllama-1.1b-chat.Q4_K_M
  offgrid run llama-2-7b-chat.Q5_K_M

Tip: Use 'offgrid list' to see available models
```

### Connection Errors
Server connectivity issues are handled gracefully:
```
[X] Cannot connect to inference server

Make sure:
  • Server is running: offgrid serve
  • Port 11611 is not blocked
  • Firewall allows connections

Check server status: curl http://localhost:11611/health
```

### HuggingFace Errors
Network and API issues show helpful guidance:
```
[X] Failed to fetch model info from HuggingFace

Possible causes:
  • Network connectivity issues
  • Model repository is private or doesn't exist
  • HuggingFace API temporarily unavailable

Try:
  • Check internet connection
  • Verify model ID: hub.com/<model-id>
  • Search for models: offgrid search <query>
```

---

## Tips & Best Practices

### Quick Start Workflow
```bash
# 1. Search for a model
offgrid search llama --author TheBloke

# 2. Download it
offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF

# 3. Start chatting
offgrid run llama-2-7b-chat.Q4_K_M
```

### Offline Workflow
```bash
# Prepare on connected machine
offgrid search llama --author TheBloke
offgrid download-hf TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF
offgrid export tinyllama-1.1b-chat.Q4_K_M /media/usb

# Use on air-gapped machine
offgrid import /media/usb
offgrid run tinyllama-1.1b-chat.Q4_K_M
```

### Server Workflow
```bash
# Start server
offgrid serve

# In another terminal
curl http://localhost:11611/v1/models
curl http://localhost:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Hello!"}]}'
```

---

## Visual Design Guide

### Icons Used
-  Launch/Start
- [Lightning] Speed/Performance
- [OK] Success
- [X] Error
- [Package] Package/Model
- ⏬ Download
-  Search
-  Delete
-  Catalog
- ⏳ In Progress
-  Warning
-  Likes
-  Date

### Box Drawing
```
━━━━━━━━━━  Separator (thick horizontal)
┌─          Top-left corner
│           Vertical line
└─          Bottom-left corner
```

### Typography
- **Bold**: Commands, file names
- `Code`: Literal values, flags
- Regular: Descriptions, help text

---

## AI Agent Commands

### `offgrid agent run`

Run an AI agent to complete a task autonomously.

```bash
offgrid agent run "Calculate the factorial of 10"
offgrid agent run "Search for Llama models and summarize" --model qwen2.5-7b
```

**Options:**
- `--model`: Model to use for agent reasoning
- `--style`: Agent style (`react` or `cot`)
- `--max-steps`: Maximum reasoning steps (default: 10)

### `offgrid agent tools`

List available tools for agents.

```bash
offgrid agent tools
```

### `offgrid agent mcp`

Manage MCP (Model Context Protocol) servers.

```bash
# List MCP servers
offgrid agent mcp list

# Add MCP server
offgrid agent mcp add filesystem "npx -y @modelcontextprotocol/server-filesystem /tmp"

# Remove MCP server
offgrid agent mcp remove filesystem

# Test MCP server connection
offgrid agent mcp test "npx -y @modelcontextprotocol/server-memory"
```

---

## User Management Commands

> **Note:** User management requires multi-user mode. Enable with `OFFGRID_MULTI_USER=true`.

### `offgrid users`

List all users (multi-user mode only).

```bash
# In single-user mode (shows helpful message)
offgrid users

# In multi-user mode
OFFGRID_MULTI_USER=true offgrid users
```

### `offgrid users create`

Create a new user.

```bash
offgrid users create alice --role user
offgrid users create bob --role admin
```

### `offgrid users delete`

Delete a user by ID.

```bash
offgrid users delete <user-id>
```

---

## Accessibility

- **Plain Text**: All output is screen-reader friendly
- **Icons**: Optional, meaningful even without icons
- **NO_COLOR**: Respects NO_COLOR environment variable
- **Consistent**: Predictable structure across commands

---

## Future Enhancements

- [ ] Color support (with NO_COLOR respect)
- [ ] Progress bars (spinner, percentage)
- [ ] Tab completion for bash/zsh
- [ ] Command aliases (`ls`, `rm`, etc.)
- [ ] JSON output mode (`--json`)
- [ ] Verbose mode (`--verbose`)
- [ ] Quiet mode (`--quiet`)
- [ ] Configuration profiles
