# Model Setup Guide

This guide explains how to download and set up models for OffGrid LLM.

## Quick Start

1. **Create models directory:**
   ```bash
   mkdir -p ~/.offgrid-llm/models
   ```

2. **Download a model** (choose one method below)

3. **Start the server:**
   ```bash
   ./offgrid
   ```

## Downloading Models

### Method 1: Using `curl` or `wget`

Download quantized models from Hugging Face:

```bash
# Example: Llama 2 7B Q4_K_M (Recommended - ~4GB)
wget https://huggingface.co/TheBloke/Llama-2-7B-GGUF/resolve/main/llama-2-7b.Q4_K_M.gguf \
  -P ~/.offgrid-llm/models/

# Example: Mistral 7B Q4_K_M (~4GB)
wget https://huggingface.co/TheBloke/Mistral-7B-Instruct-v0.2-GGUF/resolve/main/mistral-7b-instruct-v0.2.Q4_K_M.gguf \
  -P ~/.offgrid-llm/models/

# Example: Tiny Llama 1.1B Q4_K_M (Lightweight - ~700MB)
wget https://huggingface.co/TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF/resolve/main/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf \
  -P ~/.offgrid-llm/models/
```

### Method 2: Using Hugging Face CLI

```bash
# Install Hugging Face CLI
pip install huggingface-hub

# Download a model
huggingface-cli download TheBloke/Llama-2-7B-GGUF \
  llama-2-7b.Q4_K_M.gguf \
  --local-dir ~/.offgrid-llm/models/ \
  --local-dir-use-symlinks False
```

### Method 3: From USB/SD Card (Offline)

Perfect for air-gapped or offline environments:

```bash
# Copy from USB drive
cp /media/usb/models/*.gguf ~/.offgrid-llm/models/

# Or use the API endpoint (once server is running)
curl -X POST http://localhost:11611/v1/import \
  -F "file=@/media/usb/models/llama-2-7b.Q4_K_M.gguf" \
  -F "name=llama-2-7b.Q4_K_M.gguf"
```

## Recommended Models by Use Case

### Low Resource (< 4GB RAM)
- **TinyLlama 1.1B Q4_K_M** (~700MB)
  - Best for: Basic chat, simple tasks
  - Memory: ~1GB

### Standard (4-8GB RAM)
- **Llama 2 7B Q4_K_M** (~4GB)
  - Best for: General purpose, good quality
  - Memory: ~5GB
  
- **Mistral 7B Q4_K_M** (~4GB)
  - Best for: Code, instruction following
  - Memory: ~5GB

### High Performance (16GB+ RAM)
- **Llama 2 13B Q4_K_M** (~7.4GB)
  - Best for: Complex tasks, better reasoning
  - Memory: ~9GB

## Quantization Guide

Models come in different quantizations (compression levels):

- **Q4_0, Q4_K_M**: Best balance of size and quality (Recommended)
- **Q5_K_M**: Better quality, larger size
- **Q8_0**: Highest quality, largest size
- **Q2_K, Q3_K**: Smallest size, lower quality

Example: `model-name.Q4_K_M.gguf`

## Model Formats

OffGrid LLM supports:
- `.gguf` (Recommended - latest format)
- `.ggml` (Older format)
- `.bin` (Legacy)

## Verifying Installation

Once you've downloaded a model:

```bash
# List models
curl http://localhost:11611/v1/models

# Test chat completion
curl http://localhost:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-2-7b.Q4_K_M",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Offline Model Distribution

For deploying to offline environments:

### Prepare a USB Drive

```bash
# Create directory structure on USB
mkdir -p /media/usb/offgrid-llm/{binary,models,docs}

# Copy binary
cp ./offgrid /media/usb/offgrid-llm/binary/

# Copy models
cp ~/.offgrid-llm/models/*.gguf /media/usb/offgrid-llm/models/

# Copy this guide
cp docs/MODEL_SETUP.md /media/usb/offgrid-llm/docs/
```

### Deploy on Target Machine

```bash
# Copy from USB
cp -r /media/usb/offgrid-llm ~/ 

# Set up
cd ~/offgrid-llm
chmod +x binary/offgrid

# Create models directory and copy models
mkdir -p ~/.offgrid-llm/models
cp models/*.gguf ~/.offgrid-llm/models/

# Run
./binary/offgrid
```

## Troubleshooting

### Model not loading
- Check file permissions: `chmod 644 ~/.offgrid-llm/models/*.gguf`
- Verify disk space: `df -h`
- Check RAM available: `free -h`

### Out of memory
- Try a smaller model (TinyLlama)
- Use a more aggressive quantization (Q4_0, Q3_K)
- Close other applications

### Model not found
- Ensure model is in correct directory: `ls ~/.offgrid-llm/models/`
- Check environment variable: `echo $OFFGRID_MODELS_DIR`
- Restart server to rescan models

## Environment Variables

```bash
# Custom models directory
export OFFGRID_MODELS_DIR=/path/to/models

# Memory limit (MB)
export OFFGRID_MAX_MEMORY_MB=4096

# Number of threads
export OFFGRID_NUM_THREADS=4
```

## Popular Model Sources

- [TheBloke on Hugging Face](https://huggingface.co/TheBloke) - Large collection of GGUF models
- [Ollama Model Library](https://ollama.com/library) - Pre-configured models
- [LM Studio Community](https://lmstudio.ai/models) - Curated model collection

## Next Steps

- Read the [API Documentation](API.md)
- Learn about [P2P Model Sharing](P2P.md)
- Configure [Resource Limits](CONFIGURATION.md)
