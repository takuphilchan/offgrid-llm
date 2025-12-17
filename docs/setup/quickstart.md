# OffGrid LLM - Quick Start Guide

Get OffGrid LLM running in under 5 minutes.

---

## What is OffGrid LLM?

**Run powerful AI language models completely offline** - no internet required after setup.

Perfect for:
-  Privacy-conscious users
- 🏔️ Remote/offline environments
- 🏢 Air-gapped enterprise systems
- 🎓 Educational institutions
-  Local development

**Features:**
- 100% offline operation
- GPU acceleration (CUDA, ROCm, Metal, Vulkan)
- OpenAI-compatible API
- Modern web UI
- USB model transfer
- Cross-platform (Linux, macOS, Windows)

---

## Installation Options

Choose the method that fits your needs:

### 🚀 Option 1: One-Line Install (Recommended)

**Best for:** Most users - installs everything in one command

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```

**What you get:**
- CLI tools and server
- Desktop app with system tray
- Voice Assistant (Whisper STT + Piper TTS)

**Then open:** http://localhost:11611

---

### 🐳 Option 2: Docker

**Best for:** Isolated environment, production deployment

```bash
# Clone and run
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
docker-compose up -d

# Access UI
open http://localhost:11611
```

**Done!** See [Docker Guide](docker.md) for GPU support.

---

### 💻 Option 3: Desktop App Only

**Best for:** Non-technical users who just want the app

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.sh | bash
```

**Windows (PowerShell as Administrator):**
```powershell
irm https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.ps1 | iex
```

---

### ⌨️ Option 4: CLI Only (Minimal)

**Best for:** Servers, headless systems, developers
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/scripts/install.sh | bash

**Manual installation:**
```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
sudo ./dev/install.sh
```

---

## First Steps After Installation

### 1. Access the Web UI

Open your browser to:
```
http://localhost:11611/ui/
```

You should see the OffGrid LLM interface.

### 2. Download Your First Model

**Option A: Via Web UI (Easiest)**

1. Click the **Models** tab
2. Search for a model (try "tinyllama" or "phi")
3. Click **Download**
4. Wait for download to complete

**Option B: Via CLI**

```bash
# Download a small model to test (1.5GB)
offgrid download qwen2.5:0.5b-instruct-q4_k_m

# Download a popular model (4GB)
offgrid download llama3.2:3b-instruct-q4_k_m

# Download for coding (4GB)
offgrid download deepseek-coder:6.7b-instruct-q4_k_m
```

**Recommended starter models:**
- `qwen2.5:0.5b-instruct-q4_k_m` (1.5GB) - Fast, great for testing
- `llama3.2:3b-instruct-q4_k_m` (4GB) - Good quality, moderate size
- `phi3:3.8b-instruct-q4_k_m` (2.3GB) - Microsoft's efficient model

### 3. Start Chatting

1. Click the **Chat** tab
2. Select your downloaded model from the dropdown
3. Type a message and press Enter

**Example prompts:**
```
Explain quantum computing in simple terms

Write a Python function to calculate fibonacci numbers

What's the difference between Docker and virtual machines?
```

---

## Common Use Cases

### Local Development Assistant

```bash
# Chat about code
Ask: "How do I implement a REST API in Go?"

# Generate code
Ask: "Write a React component for a todo list"

# Debug code
Ask: "Why is this Python code throwing an error: [paste code]"
```

### Document Analysis

```bash
# Summarize
Ask: "Summarize this article: [paste text]"

# Extract info
Ask: "Extract key points from: [paste text]"
```

### Creative Writing

```bash
# Story generation
Ask: "Write a short sci-fi story about AI"

# Email drafts
Ask: "Write a professional email requesting a meeting"
```

---

## Understanding the Interface

### Chat Tab
- **Model Selector:** Choose which AI model to use
- **System Prompt:** Set personality/behavior (optional)
- **Temperature:** Creativity level (0.0-2.0)
- **Stream:** Real-time response vs. wait for complete response

### Sessions Tab
- Save and load different conversations
- Each session has its own history

### Models Tab
- Download new models
- View installed models
- Delete models you don't need

### Terminal Tab
- Run CLI commands from the UI
- Monitor downloads
- Check system status

---

## Tips for Best Results

### 1. Choose the Right Model Size

**Low RAM (4-8GB):** Use 0.5B-1B models
```bash
offgrid download qwen2.5:0.5b-instruct-q4_k_m
```

**Medium RAM (8-16GB):** Use 3B-7B models
```bash
offgrid download llama3.2:3b-instruct-q4_k_m
```

**High RAM (16GB+):** Use 13B+ models
```bash
offgrid download llama3.1:13b-instruct-q4_k_m
```

### 2. Adjust Temperature

- **0.0-0.3:** Focused, deterministic (code, facts)
- **0.7-1.0:** Balanced (general chat)
- **1.5-2.0:** Creative (stories, brainstorming)

### 3. Use System Prompts

Click **System Prompt** dropdown to set behavior:
- **Expert Coder:** For programming help
- **Creative Writer:** For stories and content
- **Research Assistant:** For analysis
- **Custom:** Define your own

### 4. Enable Streaming

Toggle **Stream** ON for:
- Real-time responses
- Better user experience
- Ability to stop generation

---

## USB Model Transfer (Offline Deployment)

Perfect for air-gapped systems or slow internet.

### Export Models to USB

1. Insert USB drive
2. Click **Models** tab
3. Scroll to **USB Model Transfer**
4. Click **Browse** and select USB path
5. Click **Export All Models**

### Import Models from USB

1. Insert USB with models
2. Click **Models** tab
3. Click **Browse** and select USB path
4. Click **Import All**

Models are verified with SHA256 checksums for integrity.

---

## Troubleshooting

### "No models found"

**Solution:** Download a model first
```bash
offgrid download qwen2.5:0.5b-instruct-q4_k_m
```

### "Model loading takes forever"

**Reasons:**
- Large model on slow storage
- Not enough RAM
- CPU-only mode (use smaller model)

**Solutions:**
```bash
# Check available RAM
free -h  # Linux
vm_stat  # macOS

# Download smaller model
offgrid download qwen2.5:0.5b-instruct-q4_k_m
```

### "Server not responding"

```bash
# Check if running
offgrid status

# Restart server
sudo systemctl restart offgrid-llm

# Check logs
journalctl -u offgrid-llm -f
```

### "Out of memory"

**Solutions:**
1. Close other applications
2. Use a smaller model
3. Reduce `max_tokens` in settings
4. Add swap space (Linux)

### Port 11611 already in use

```bash
# Find what's using the port
sudo lsof -i :11611

# Kill the process
sudo kill -9 <PID>

# Or change OffGrid port
export OFFGRID_PORT=8080
```

---

## Next Steps

### Learn More
-  [Full Documentation](../README.md)
- 🐳 [Docker Deployment](docker.md)
-  [API Reference](../reference/api.md)
-  [Performance Tuning](../advanced/performance.md)

### Advanced Topics
- [Custom Model Setup](../guides/models.md)
- [GPU Acceleration](../advanced/cpu-tuning.md)
- [Production Deployment](../advanced/deployment.md)
- [Building from Source](../advanced/building.md)

### Get Help
-  [GitHub Discussions](https://github.com/takuphilchan/offgrid-llm/discussions)
-  [Report Issues](https://github.com/takuphilchan/offgrid-llm/issues)
-  [Documentation](https://github.com/takuphilchan/offgrid-llm/tree/main/docs)

---

## Example Workflows

### Workflow 1: Code Review Assistant

```bash
# 1. Download coding model
offgrid download deepseek-coder:6.7b-instruct-q4_k_m

# 2. Set system prompt to "Expert Coder"

# 3. Paste code and ask:
"Review this code for security issues and best practices:
[paste your code]"
```

### Workflow 2: Document Summarizer

```bash
# 1. Download general model
offgrid download llama3.2:3b-instruct-q4_k_m

# 2. Set system prompt to "Research Assistant"

# 3. Ask:
"Summarize the key points from this document in bullet points:
[paste document]"
```

### Workflow 3: Offline Documentation

```bash
# 1. Download multiple specialized models
offgrid download deepseek-coder:6.7b-instruct-q4_k_m  # Code
offgrid download llama3.2:3b-instruct-q4_k_m          # General
offgrid download phi3:3.8b-instruct-q4_k_m            # Efficient

# 2. Use different models for different tasks
# 3. Keep all chat history in sessions
```

---

**Ready to get started?** Pick an installation method above and you'll be running your own AI in minutes! 
