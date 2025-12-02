# OffGrid LLM Python Client

Python client library for [OffGrid LLM](https://github.com/takuphilchan/offgrid-llm) - Run AI models completely offline.

## Installation

```bash
pip install offgrid-llm
```

## Quick Start

```python
import offgrid_llm as offgrid

# Simple one-liner
response = offgrid.chat("What is Python?")
print(response)

# List available models
models = offgrid.list_models()
for m in models:
    print(f"- {m['id']}")
```

## Full Usage

### Chat

```python
from offgrid_llm import Client

client = Client()

# Basic chat
response = client.chat("Explain quantum computing")
print(response)

# With system prompt
response = client.chat(
    "Write a poem about AI",
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

### Model Management

```python
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

# List installed models
for model in client.models.list():
    print(model['id'])

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
client.kb.add_directory("./docs", extensions=[".md", ".txt"])

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
from offgrid_llm import Client

# Default: localhost:11611
client = Client()

# Custom host/port
client = Client(host="192.168.1.100", port=11611)

# Custom timeout (for slow models)
client = Client(timeout=600)  # 10 minutes
```

## Error Handling

```python
from offgrid_llm import Client, OffGridError

client = Client()

try:
    response = client.chat("Hello")
except OffGridError as e:
    print(f"Error: {e.message}")
    if e.code:
        print(f"Code: {e.code}")
```

## Convenience Functions

The module provides top-level functions for quick use:

```python
import offgrid_llm as offgrid

# These use a default client (localhost:11611)
offgrid.chat("Hello!")
offgrid.complete("Once upon a time")
offgrid.embed("Some text")
offgrid.list_models()
offgrid.search("llama", ram=8)
offgrid.download("repo", "file.gguf")
offgrid.info()
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
