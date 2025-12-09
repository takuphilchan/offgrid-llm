# OffGrid LLM

**Run powerful AI models completely offline on your own computer.**

[![License: MIT](https://img.shields.io/badge/License-MIT-10b981.svg?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-0078D4.svg?style=flat-square)](https://github.com/takuphilchan/offgrid-llm/releases)
[![PyPI](https://img.shields.io/pypi/v/offgrid?style=flat-square&color=3776AB)](https://pypi.org/project/offgrid/)

No cloud. No subscriptions. No data leaving your machine.

<p align="center">
  <img src="docs/images/chat-page.png" alt="OffGrid LLM Chat Interface" width="800"/>
</p>

---

## Features

- **100% Offline** - Your data never leaves your computer
- **Modern Chat UI** - Clean, responsive web interface
- **Voice Assistant** - Multi-language speech-to-text and text-to-speech (18+ languages)
- **Python Library** - Simple API for scripting and automation
- **Knowledge Base (RAG)** - Chat with your documents
- **USB Transfer** - Move models between air-gapped systems
- **Cross-Platform** - Linux, macOS, and Windows support

---

## Install

**One command to install everything:**

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```

This installs CLI + Desktop App + Voice Assistant (Whisper STT + Piper TTS).

For non-interactive installation:
```bash
# Install everything (default)
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | NONINTERACTIVE=yes bash

# CLI + Voice only (no desktop)
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | DESKTOP=no NONINTERACTIVE=yes bash
```

**Then open:** http://localhost:11611

### Python Library

```bash
pip install offgrid
```

---

## Screenshots

<table>
  <tr>
    <td align="center">
      <img src="docs/images/chat-page.png" alt="Chat Interface" width="400"/>
      <br/>
      <b>Chat Interface</b>
    </td>
    <td align="center">
      <img src="docs/images/models-page.png" alt="Model Management" width="400"/>
      <br/>
      <b>Model Management</b>
    </td>
  </tr>
</table>

---

## Usage

```python
import offgrid

# Connect to server
client = offgrid.Client()  # localhost:11611

# Or custom server
client = offgrid.Client(host="http://192.168.1.100:11611")

# Chat
response = client.chat("Hello!")
print(response)

# Specify model
response = client.chat("Hello!", model="Llama-3.2-3B-Instruct")

# Streaming
for chunk in client.chat("Tell me a story", stream=True):
    print(chunk, end="", flush=True)

# With options
response = client.chat(
    "Explain quantum computing",
    model="Llama-3.2-3B-Instruct",
    system="You are a physics teacher",
    temperature=0.7,
    max_tokens=500
)
```

### Model Management

```python
# List models
for model in client.list_models():
    print(model["id"])

# Search HuggingFace
results = client.models.search("llama", ram=8)

# Download
client.models.download(
    "bartowski/Llama-3.2-3B-Instruct-GGUF",
    "Llama-3.2-3B-Instruct-Q4_K_M.gguf"
)

# Import/Export USB
client.models.import_usb("/media/usb")
client.models.export_usb("model-name", "/media/usb")
```

### Knowledge Base (RAG)

```python
# Add documents
client.kb.add("notes.txt")
client.kb.add("meeting", content="Meeting notes...")
client.kb.add_directory("./docs")

# Chat with context
response = client.chat("Summarize the meeting", use_kb=True)

# Search documents
results = client.kb.search("deadline")
```

### Embeddings

```python
embedding = client.embed("Hello world")
embeddings = client.embed(["Hello", "World"])
```

### Voice Assistant

```python
# Speech-to-text (transcribe audio)
text = client.audio.transcribe("recording.wav")
print(text["text"])

# With language hint
text = client.audio.transcribe("spanish.wav", language="es")

# Text-to-speech (generate audio)
audio = client.audio.speak("Hello world!", voice="en_US-amy-medium")
with open("hello.wav", "wb") as f:
    f.write(audio)

# List available voices & models
voices = client.audio.voices()
models = client.audio.whisper_models()
```

**CLI:**
```bash
offgrid audio transcribe recording.wav           # Transcribe audio
offgrid audio speak "Hello!" -o greeting.wav     # Generate speech
offgrid audio voices                              # List voices
```

---

## Web UI & CLI

After installing the desktop app:

**Web Interface:** `http://localhost:11611/ui/`

**Command Line:**
```bash
offgrid list                    # List models
offgrid search llama --ram 8    # Search HuggingFace
offgrid download-hf repo --file model.gguf
offgrid run model-name          # Interactive chat
offgrid serve                   # Start server
```

---

## System Requirements

| RAM | Models |
|-----|--------|
| 4GB | 1B-3B parameters |
| 8GB | 7B parameters |
| 16GB+ | 13B+ parameters |

GPU optional. Supports NVIDIA (CUDA), AMD (ROCm), Apple Silicon (Metal), Vulkan.

---

## Documentation

| Guide | Description |
|-------|-------------|
| [Python Library](python/README.md) | Full Python API reference |
| [Quick Start](docs/QUICKSTART.md) | Get running in 5 minutes |
| [CLI Reference](docs/CLI_REFERENCE.md) | All commands |
| [API Reference](docs/API.md) | REST API endpoints |

**Docker:** [docs/DOCKER.md](docs/DOCKER.md) · **Contributing:** [dev/CONTRIBUTING.md](dev/CONTRIBUTING.md)

---

## License

MIT License - [LICENSE](LICENSE)

**Built with [llama.cpp](https://github.com/ggerganov/llama.cpp)**
