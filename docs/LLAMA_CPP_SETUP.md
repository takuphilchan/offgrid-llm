# llama.cpp Integration Guide

## Overview

offgrid-llm supports two inference modes:

1. **Mock Mode** (default) - Returns pre-programmed responses for testing
2. **llama.cpp Mode** - Real LLM inference using GGUF models

## Quick Setup (Mock Mode)

The default build uses mock responses - no additional setup needed:

```bash
make build
./offgrid
```

## Full Setup (Real Inference)

### Prerequisites

- GCC/Clang compiler
- CMake (optional, for optimized builds)
- Git

### Step 1: Install llama.cpp

```bash
# Clone llama.cpp
cd /opt  # or your preferred directory
git clone https://github.com/ggerganov/llama.cpp
cd llama.cpp

# Build llama.cpp
make

# Install to system (optional)
sudo make install
```

### Step 2: Set Environment Variables

```bash
# Add llama.cpp to include path
export C_INCLUDE_PATH=/opt/llama.cpp:$C_INCLUDE_PATH
export LIBRARY_PATH=/opt/llama.cpp:$LIBRARY_PATH
export LD_LIBRARY_PATH=/opt/llama.cpp:$LD_LIBRARY_PATH

# Make permanent (add to ~/.bashrc or ~/.zshrc)
echo 'export C_INCLUDE_PATH=/opt/llama.cpp:$C_INCLUDE_PATH' >> ~/.bashrc
echo 'export LIBRARY_PATH=/opt/llama.cpp:$LIBRARY_PATH' >> ~/.bashrc
echo 'export LD_LIBRARY_PATH=/opt/llama.cpp:$LD_LIBRARY_PATH' >> ~/.bashrc
```

### Step 3: Build offgrid-llm with llama.cpp Support

```bash
cd /path/to/offgrid-llm

# Build with llama.cpp support
go build -tags llama -o offgrid ./cmd/offgrid

# Or use make
make build-llama
```

### Step 4: Configure & Run

```bash
# Create config (optional - enables llama.cpp by default)
./offgrid config init

# Edit config to disable mock mode
nano ~/.offgrid-llm/config.yaml
# Set: use_mock_engine: false

# Download a model
./offgrid download tinyllama-1.1b-chat

# Start server
./offgrid
```

## GPU Acceleration (Optional)

### CUDA (NVIDIA GPUs)

```bash
cd /opt/llama.cpp

# Build with CUDA support
make LLAMA_CUBLAS=1

# Set GPU layers in config
./offgrid config init
nano ~/.offgrid-llm/config.yaml
# Set: enable_gpu: true
#      num_gpu_layers: 35  # Adjust based on your GPU VRAM
```

### Metal (Apple Silicon)

```bash
cd /opt/llama.cpp

# Build with Metal support
make LLAMA_METAL=1
```

### OpenCL / ROCm (AMD GPUs)

```bash
cd /opt/llama.cpp

# Build with CLBlast
make LLAMA_CLBLAST=1
```

## Configuration Options

Edit `~/.offgrid-llm/config.yaml`:

```yaml
# Use real llama.cpp instead of mock
use_mock_engine: false

# Model settings
max_context_size: 4096  # Context window
num_threads: 8          # CPU threads (set to physical cores)

# GPU settings (optional)
enable_gpu: true
num_gpu_layers: 35      # Layers to offload to GPU

# Resource limits
max_memory_mb: 8192     # Max RAM for models
max_models: 2           # Max simultaneously loaded models
```

## Troubleshooting

### Build Error: "common.h: No such file or directory"

```bash
# Ensure C_INCLUDE_PATH is set correctly
echo $C_INCLUDE_PATH

# Should include /opt/llama.cpp or your llama.cpp path
export C_INCLUDE_PATH=/opt/llama.cpp:$C_INCLUDE_PATH
```

### Runtime Error: "cannot open shared object file"

```bash
# Ensure LD_LIBRARY_PATH is set
echo $LD_LIBRARY_PATH

# Add llama.cpp to library path
export LD_LIBRARY_PATH=/opt/llama.cpp:$LD_LIBRARY_PATH

# Or copy libraries to system path
sudo cp /opt/llama.cpp/*.so /usr/local/lib/
sudo ldconfig
```

### Model Loading Fails

```bash
# Check model file exists and is valid GGUF format
ls -lh ~/.offgrid-llm/models/

# Verify SHA256 (if available)
sha256sum ~/.offgrid-llm/models/tinyllama-1.1b-chat.Q4_K_M.gguf

# Try with more verbose logging
OFFGRID_LOG_LEVEL=debug ./offgrid
```

### Out of Memory

```bash
# Reduce context size
nano ~/.offgrid-llm/config.yaml
# Set: max_context_size: 2048

# Or use smaller quantization
./offgrid download tinyllama-1.1b-chat Q4_K_S  # Smaller than Q4_K_M

# Or enable GPU offloading
# Set: enable_gpu: true
#      num_gpu_layers: 20  # Start low, increase gradually
```

## Performance Tuning

### Optimal Thread Count

```bash
# Find CPU core count
nproc

# Set threads = physical cores (not logical)
# For 8-core CPU with hyperthreading (16 logical):
nano ~/.offgrid-llm/config.yaml
# Set: num_threads: 8
```

### GPU Offloading Sweet Spot

Start with partial offloading and measure:

```bash
# Profile with different layer counts
OFFGRID_GPU_LAYERS=10 ./offgrid  # Test
OFFGRID_GPU_LAYERS=20 ./offgrid  # Test
OFFGRID_GPU_LAYERS=35 ./offgrid  # Test

# Monitor VRAM usage
nvidia-smi -l 1  # NVIDIA
```

### Quantization Trade-offs

| Quantization | Size  | Quality | Speed |
|--------------|-------|---------|-------|
| Q4_K_S       | Small | Good    | Fast  |
| Q4_K_M       | Medium| Better  | Medium|
| Q5_K_M       | Larger| Great   | Slower|
| Q8_0         | Large | Best    | Slow  |

## Makefile Targets

```bash
make build              # Build without llama.cpp (mock mode)
make build-llama        # Build with llama.cpp support
make test               # Run tests
make clean              # Clean build artifacts
```

## Verifying Installation

```bash
# Check if built with llama.cpp
./offgrid config show

# Should show:
# - "Using llama.cpp engine" when starting server
# - Real responses instead of "This is a mock response"

# Test inference
curl -X POST http://localhost:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tinyllama-1.1b-chat",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Next Steps

- [Download Models](../README.md#model-catalog)
- [API Documentation](../README.md#api-endpoints)
- [Web Dashboard](../README.md#web-dashboard)
