# OffGrid Python Client

Python client library for [OffGrid LLM](https://github.com/takuphilchan/offgrid-llm) - Run AI models completely offline.

## Installation

```bash
pip install offgrid
```

## Quick Start

```python
import offgrid

# Connect to server
client = offgrid.Client()  # localhost:11611

# Chat
response = client.chat("What is Python?")
print(response)

# List available models
models = client.list_models()
for m in models:
    print(f"- {m['id']}")
```

## Full Usage

### Chat

```python
from offgrid import Client

client = Client()

# Basic chat (uses first available model)
response = client.chat("Explain quantum computing")
print(response)

# Specify model
response = client.chat("Hello!", model="Llama-3.2-3B-Instruct")

# With system prompt
response = client.chat(
    "Write a poem about AI",
    model="Llama-3.2-3B-Instruct",
    system="You are a creative poet.",
    temperature=0.9
)

# Streaming
for chunk in client.chat("Tell me a long story", stream=True):
    print(chunk, end="", flush=True)

# Full conversation
messages = [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello!"},
    {"role": "assistant", "content": "Hi there! How can I help?"},
    {"role": "user", "content": "What's the weather like?"}
]
response = client.chat(messages=messages)
```

### API Key Authentication

```python
from offgrid import Client

# With API key (if server has OFFGRID_API_KEY set)
client = Client(api_key="your-secret-key")

# All requests automatically include the Authorization header
response = client.chat("Hello!")
```

### Session Management

```python
# Sessions preserve conversation context on the server
sessions = client.sessions

# Create a new session
session = sessions.create("my-chat")

# List all sessions
for s in sessions.list():
    print(f"- {s['name']}: {len(s.get('messages', []))} messages")

# Chat with session (context is preserved)
response1 = sessions.chat_with_session("my-chat", "My name is Alice")
response2 = sessions.chat_with_session("my-chat", "What is my name?")
# response2 will correctly reference "Alice"

# Add messages manually
sessions.add_message("my-chat", "user", "Hello")
sessions.add_message("my-chat", "assistant", "Hi there!")

# Get session details
details = sessions.get("my-chat")
print(f"Messages: {len(details['messages'])}")

# Delete session
sessions.delete("my-chat")
```

### Server Statistics

```python
# Get comprehensive server statistics
stats = client.stats()

# Server info
print(f"Uptime: {stats['server']['uptime']}")
print(f"Version: {stats['server']['version']}")

# Inference metrics
print(f"Total requests: {stats['inference']['aggregate']['total_requests']}")
print(f"Total tokens: {stats['inference']['aggregate']['total_tokens']}")

# System resources
print(f"CPU usage: {stats['resources']['cpu_usage_percent']:.1f}%")
print(f"Memory: {stats['resources']['memory_used_mb']}MB")

# RAG status
if stats['rag']['enabled']:
    print(f"Documents: {stats['rag']['documents']}")
```

### Model Management

```python
# List installed models
for model in client.list_models():
    print(model['id'])

# Search for models
results = client.models.search("llama", ram=8)
for model in results:
    print(f"{model['id']} - {model['size_gb']}GB")

# Download a model
client.models.download(
    "bartowski/Llama-3.2-3B-Instruct-GGUF",
    "Llama-3.2-3B-Instruct-Q4_K_M.gguf",
    progress_callback=lambda pct, done, total: print(f"\r{pct:.1f}%", end="")
)

# Delete a model
client.models.delete("old-model")

# Import from USB
imported = client.models.import_usb("/media/usb")

# Export to USB
client.models.export_usb("Llama-3.2-3B-Instruct-Q4_K_M", "/media/usb")
```

### Knowledge Base (RAG)

```python
# Add documents
client.kb.add("notes.md")
client.kb.add("meeting", content="Meeting notes from today...")
client.kb.add_directory("./docs", extensions=[".md", ".txt", ".pdf"])

# List documents
for doc in client.kb.list():
    print(f"{doc['id']}: {doc['chunks']} chunks")

# Search
results = client.kb.search("project deadline")
for r in results:
    print(f"[{r['score']:.2f}] {r['content'][:100]}...")

# Chat with Knowledge Base context
response = client.chat(
    "What are the main action items from the meeting?",
    use_kb=True
)

# Remove documents
client.kb.remove("notes.md")
client.kb.clear()  # Remove all
```

### AI Agents (New in v0.2.3)

Run autonomous agents that can use tools to complete tasks:

```python
# Run an agent task
result = client.agent.run(
    "Calculate 127 * 48 + 356",
    model="llama3.2:3b"
)
print(result["result"])

# List available tools
tools = client.agent.tools()
for tool in tools:
    status = "✓" if tool["enabled"] else "✗"
    print(f"[{status}] {tool['name']}: {tool['description']}")

# Toggle tools on/off
client.agent.disable_tool("shell")  # Security: disable shell access
client.agent.enable_tool("calculator")

# Complex multi-step tasks
result = client.agent.run(
    "Read the VERSION file and tell me what version it is",
    model="llama3.2:3b",
    max_steps=5
)
```

**Built-in Tools:**
- `calculator` - Evaluate mathematical expressions
- `current_time` - Get current date/time
- `read_file` - Read file contents
- `write_file` - Write content to files
- `list_files` - List directory contents
- `shell` - Execute shell commands
- `http_get` - Make HTTP GET requests

### MCP Integration (New in v0.2.3)

Connect external tools via Model Context Protocol:

```python
# Add an MCP server
client.agent.mcp.add(
    "filesystem",
    "npx -y @modelcontextprotocol/server-filesystem /tmp"
)

# List configured servers
servers = client.agent.mcp.list()
for s in servers:
    print(f"{s['name']}: {s['url']}")

# Test a server connection
result = client.agent.mcp.test(url="npx -y @modelcontextprotocol/server-github")
print(f"Found {len(result.get('tools', []))} tools")

# Remove a server
client.agent.mcp.remove("filesystem")
```

### LoRA Adapters (New in v0.2.3)

Manage LoRA adapters for fine-tuned models:

```python
# List registered adapters
adapters = client.lora.list()
for a in adapters:
    print(f"{a['name']}: {a['path']}")

# Register a new adapter
client.lora.register(
    "coding-assistant",
    "/path/to/code-lora.gguf",
    scale=0.8
)

# Get adapter details
adapter = client.lora.get("coding-assistant")

# Remove an adapter
client.lora.remove("coding-assistant")
```

### Audio: Speech-to-Text & Text-to-Speech (New in v0.2.4)

Transcribe audio files and generate speech completely offline:

```python
# Setup: Download Whisper model for transcription
client.audio.setup_whisper("base")  # Options: tiny, base, small, medium, large

# Setup: Download a voice for text-to-speech  
client.audio.setup_piper("en_US-amy-medium")

# Transcribe audio (Speech-to-Text)
text = client.audio.transcribe("recording.wav", model="base")
print(f"Transcription: {text}")

# Transcribe with options
text = client.audio.transcribe(
    "recording.mp3",
    model="small",       # Larger = more accurate
    language="en",       # Optional: specify language
    response_format="text"  # text, json, verbose_json
)

# Generate speech (Text-to-Speech)
audio_data = client.audio.speak("Hello, how are you today?", voice="en_US-amy-medium")
with open("output.wav", "wb") as f:
    f.write(audio_data)

# Text-to-speech with options
audio_data = client.audio.speak(
    "Welcome to OffGrid!",
    voice="en_US-amy-medium",
    speed=1.0,              # 0.5 = slow, 2.0 = fast
    response_format="wav"   # wav, mp3, opus, flac
)

# List available voices
voices = client.audio.voices()
for v in voices:
    print(f"- {v['id']}: {v['language']}")

# List Whisper models
models = client.audio.models()
for m in models:
    status = "✓ installed" if m["installed"] else "✗ not installed"
    print(f"- {m['id']} ({m['size']}): {status}")

# Check audio status
status = client.audio.status()
print(f"Whisper installed: {status['whisper']['installed']}")
print(f"Piper installed: {status['piper']['installed']}")
```

**Available Whisper Models:**
| Model | Size | RAM | Speed | Quality |
|-------|------|-----|-------|---------|
| tiny | 75MB | ~1GB | Fastest | Basic |
| base | 142MB | ~1GB | Fast | Good |
| small | 466MB | ~2GB | Medium | Better |
| medium | 1.5GB | ~5GB | Slower | Great |
| large | 2.9GB | ~10GB | Slowest | Best |

**Popular Voices:**
- `en_US-amy-medium` - American English, female
- `en_US-ryan-medium` - American English, male
- `en_GB-alba-medium` - British English, female
- `de_DE-thorsten-medium` - German, male
- `fr_FR-siwis-medium` - French, female

### System Configuration (New in v0.2.3)

```python
# Get server configuration and feature flags
config = client.config()
print(f"Version: {config['version']}")
print(f"Multi-user mode: {config['multi_user_mode']}")
print(f"Agent enabled: {config['features']['agent']}")

# Get real-time system stats
stats = client.system_stats()
print(f"CPU: {stats['cpu_percent']}%")
print(f"Memory: {stats['memory_percent']}%")
```

### Model Loading Progress (New in v0.2.11)

Track model loading progress in real-time:

```python
# Get current loading status
progress = client.loading.progress()
print(f"Phase: {progress['phase']}")  # idle, loading, warmup, ready, failed
print(f"Progress: {progress['progress']}%")

# Wait for model to be ready with progress updates
def show_progress(p):
    print(f"\r{p['message']} ({p['progress']}%)", end="")

success = client.loading.wait_for_ready(
    timeout=120,
    progress_callback=show_progress
)

# Stream loading progress via SSE
for update in client.loading.stream():
    print(f"{update['phase']}: {update['progress']}%")
    if update['phase'] in ('ready', 'failed'):
        break

# Pre-warm a model into OS page cache for instant switching
client.loading.prewarm("/var/lib/offgrid/models/llama3.gguf")

# Or use the models helper
client.models.prewarm("llama3.2:3b")
```

### P2P Network (New in v0.2.11)

Discover and share models with other OffGrid nodes on your local network:

```python
# Check P2P status
status = client.p2p.status()
print(f"P2P enabled: {status['enabled']}")
print(f"Connected peers: {status['peer_count']}")

# List discovered peers
for peer in client.p2p.peers():
    print(f"{peer['hostname']} @ {peer['address']}")
    for model in peer['models']:
        print(f"  - {model}")

# List models available across the network
models = client.p2p.models()
for m in models:
    print(f"{m['model_id']} on {m['hostname']}")

# Download a model from a peer
client.p2p.download("llama3", peer_id="node-abc123")

# Verify model integrity with peer consensus
result = client.p2p.verify("llama3")
if result['valid']:
    print("Model verified against peer hashes")

# Broadcast a new model to peers
client.p2p.broadcast("my-custom-model")

# Enable/disable P2P
client.p2p.enable()
client.p2p.disable()
```

### Distributed RAG (New in v0.2.11)

Search knowledge bases across all connected P2P peers:

```python
# Search local knowledge base only
results = client.kb.search("project deadline")

# Search across all P2P peers
results = client.kb.search("API documentation", distributed=True)
for r in results:
    peer = r.get('peer_hostname', 'local')
    print(f"[{peer}] {r['content'][:100]}...")
```

### Embeddings

```python
# Single text
embedding = client.embed("Hello world")
print(f"Dimensions: {len(embedding)}")

# Multiple texts
embeddings = client.embed(["Hello", "World", "AI"])
```

### System Info

```python
# Check server health
if client.health():
    print("Server is running")

# Get detailed info
info = client.info()
print(f"Uptime: {info['uptime']}")
print(f"CPU: {info['system']['cpu_percent']}%")
print(f"Memory: {info['system']['memory_percent']}%")
```

## Configuration

```python
from offgrid import Client

# Default: localhost:11611
client = Client()

# Custom server URL
client = Client(host="http://192.168.1.100:11611")

# Just hostname (auto-adds http://)
client = Client(host="192.168.1.100:11611")

# With API key authentication
client = Client(api_key="your-secret-key")

# Custom timeout (for slow models)
client = Client(timeout=600)  # 10 minutes

# Combined options
client = Client(
    host="http://192.168.1.100:11611",
    api_key="your-secret-key",
    timeout=300
)
```

## Automatic Retry

The client automatically retries failed requests with exponential backoff:

- Up to 3 retry attempts
- Delays: 1s → 2s → 4s between retries
- Only retries on connection errors, not HTTP errors

```python
# Retries are automatic
response = client.chat("Hello!")  # Will retry on transient failures
```

## Error Handling

```python
from offgrid import Client, OffGridError

client = Client()

try:
    response = client.chat("Hello")
except OffGridError as e:
    print(f"Error: {e.message}")
    if e.code:
        print(f"Code: {e.code}")
```

## Requirements

- Python 3.8+
- OffGrid LLM server running (`offgrid serve`)
- No external dependencies (uses only stdlib)

## Links

- [OffGrid LLM](https://github.com/takuphilchan/offgrid-llm) - Main project
- [API Reference](https://github.com/takuphilchan/offgrid-llm/blob/main/docs/API.md)
- [Issue Tracker](https://github.com/takuphilchan/offgrid-llm/issues)

## License

MIT License
