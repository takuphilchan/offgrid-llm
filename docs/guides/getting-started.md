# Getting Started with OffGrid LLM

> From zero to your first AI chat in 5 minutes.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Installation](#quick-installation)
- [Your First Chat](#your-first-chat)
- [Understanding the Basics](#understanding-the-basics)
- [Common Tasks](#common-tasks)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)

---

## Prerequisites

### Hardware Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| RAM       | 4 GB    | 16 GB       |
| Storage   | 10 GB   | 50+ GB      |
| CPU       | Any x64 | AVX2 support|

### Software Requirements

- **Operating System:** Linux, macOS, or Windows
- **Go:** 1.21+ (for building from source)
- **Git:** For cloning repository

---

## Quick Installation

### Option A: Pre-built Binary (Easiest)

```bash
# Download latest release
curl -fsSL https://github.com/yourusername/offgrid-llm/releases/latest/download/offgrid-linux-amd64 -o offgrid
chmod +x offgrid
sudo mv offgrid /usr/local/bin/
```

### Option B: Install Script

```bash
curl -fsSL https://raw.githubusercontent.com/yourusername/offgrid-llm/main/install.sh | bash
```

### Option C: Build from Source

```bash
# Clone repository
git clone https://github.com/yourusername/offgrid-llm.git
cd offgrid-llm

# Build
go build -o offgrid ./cmd/offgrid

# Install
sudo mv offgrid /usr/local/bin/
```

### Verify Installation

```bash
offgrid version
# Output: Version 0.2.8
```

---

## Your First Chat

### Step 1: Start the Server

```bash
offgrid serve
```

You should see:

```
🌐 Server starting...
📁 Models directory: ~/.offgrid-llm/models
🔌 Listening on: http://localhost:11611
```

### Step 2: Open the Web UI

Open your browser and navigate to:

```
http://localhost:11611
```

### Step 3: Download a Model

1. Click **"Models"** in the sidebar
2. Browse the available models
3. Click **Download** on a small model (e.g., TinyLlama 1.1B)
4. Wait for download to complete

### Step 4: Start Chatting!

1. Click **"Chat"** in the sidebar
2. Select your downloaded model from the dropdown
3. Type your message and press Enter

🎉 **Congratulations!** You're now running AI locally!

---

## Understanding the Basics

### How It Works

```
You type a message
         │
         ▼
┌─────────────────┐
│   Web Browser   │
│   (localhost)   │
└────────┬────────┘
         │ HTTP/WebSocket
         ▼
┌─────────────────┐
│  OffGrid Server │  ← Running on YOUR computer
│   Port 11611    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   LLM Model     │  ← Stored locally
│  (llama.cpp)    │
└────────┬────────┘
         │
         ▼
    AI Response
```

**Key Points:**
- Everything runs on YOUR computer
- No internet required (after model download)
- Your conversations stay private

### Directory Structure

After installation, OffGrid creates:

```
~/.offgrid-llm/
├── models/              # Downloaded model files
├── data/                # App data (chat history, etc.)
└── config.yaml          # Settings (created on first run)
```

### Default Configuration

| Setting        | Default Value | Description              |
|----------------|---------------|--------------------------|
| Port           | 11611         | HTTP server port         |
| Host           | localhost     | Bind address             |
| Context Size   | 4096          | Token context window     |
| GPU Layers     | 0             | Layers on GPU (0=CPU)    |

---

## Common Tasks

### Change Server Port

```bash
offgrid serve --port 8080
```

Or set environment variable:

```bash
export OFFGRID_PORT=8080
offgrid serve
```

### List Downloaded Models

```bash
offgrid models list
```

### Download a Model via CLI

```bash
offgrid models pull <model-name>
```

### Run a One-Shot Query

```bash
offgrid chat "What is the capital of France?"
```

### Enable GPU Acceleration

```bash
offgrid serve --gpu-layers 35
```

(Requires CUDA or Metal support)

### Use the REST API

```bash
curl http://localhost:11611/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tinyllama-1.1b",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

---

## Troubleshooting

### Server Won't Start

**Symptom:** `Error: port 11611 already in use`

**Solution:**
```bash
# Find what's using the port
lsof -i :11611

# Use a different port
offgrid serve --port 8080
```

### Model Download Fails

**Symptom:** Download hangs or errors

**Solutions:**
1. Check internet connection
2. Try a smaller model first
3. Download manually and place in `~/.offgrid-llm/models/`

### Out of Memory

**Symptom:** Process killed or crashes

**Solutions:**
1. Use a smaller model
2. Reduce context size: `--context-size 2048`
3. Close other applications

### Slow Generation

**Symptom:** Responses take a long time

**Solutions:**
1. Use a smaller/faster model
2. Enable GPU acceleration
3. Increase CPU threads: `--threads 8`

### Model Not Found

**Symptom:** `Error: model not found`

**Solution:**
```bash
# List available models
offgrid models list

# Check models directory
ls ~/.offgrid-llm/models/
```

---

## Next Steps

Now that you're up and running, explore these features:

### 📚 Enable RAG (Document Chat)

Add your documents to chat with them:

```bash
offgrid rag add ~/Documents/myfile.pdf
offgrid chat "What does my document say about...?"
```

**Guide:** [Embeddings Guide](embeddings.md)

### 🎤 Voice Input/Output

Talk to your AI:

```bash
offgrid serve --enable-voice
```

**Guide:** [Features Guide](features.md)

### 🤖 Run Agent Tasks

Let AI autonomously complete tasks:

```bash
offgrid agent "Research quantum computing and summarize"
```

**Guide:** [Agents Guide](agents.md)

### 🔌 Use the API

Integrate with your own applications:

**Guide:** [API Reference](../reference/api.md)

### 🖥️ Desktop App

Install the native desktop application:

**Guide:** [Desktop Installation](../../desktop/DESKTOP_INSTALL.md)

---

## Getting Help

- **Documentation:** [docs/README.md](../README.md)
- **Issues:** [GitHub Issues](https://github.com/takuphilchan/offgrid-llm/issues)
- **Contributing:** [CONTRIBUTING.md](../../dev/CONTRIBUTING.md)

---

**Happy chatting! 🎉**
