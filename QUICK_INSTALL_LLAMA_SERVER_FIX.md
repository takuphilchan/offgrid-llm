# Quick Installer llama-server Fix

## Problem

The quick installer (`installers/install.sh`) downloads pre-built llama-server binaries from the official llama.cpp releases. These binaries were compiled with `BUILD_SHARED_LIBS=ON`, which creates dynamic backend libraries that fail to load at runtime with the error:

```
llama_model_load_from_file_impl: no backends are loaded
```

## Root Cause

The official llama.cpp release binaries use dynamic backend libraries (`.so` files) that are separate from the main binary. When llama-server starts, it can't find or load these backend libraries, resulting in:

```
llama_model_load_from_file_impl: no backends are loaded. 
hint: use ggml_backend_load() or ggml_backend_load_all() to load a backend before calling this function
```

## Solution Options

### Option 1: Build llama-server from Source (Current Workaround)

Until properly-built pre-built binaries are available, users can build llama-server with static backends:

```bash
# Install dependencies
sudo apt-get update
sudo apt-get install -y build-essential cmake git

# Clone and build llama.cpp with static backends
git clone https://github.com/ggml-org/llama.cpp
cd llama.cpp
mkdir build && cd build

# Build with STATIC backends (BUILD_SHARED_LIBS=OFF)
cmake .. -DBUILD_SHARED_LIBS=OFF \
         -DGGML_VULKAN=ON \
         -DCMAKE_BUILD_TYPE=Release

cmake --build . --config Release -j $(nproc)

# Install the properly-built binary
sudo cp bin/llama-server /usr/local/bin/llama-server
```

### Option 2: Use Production Installer

The production installer (`dev/install.sh`) builds llama-server from source with `BUILD_SHARED_LIBS=OFF` and sets up systemd services for auto-start:

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/dev/install.sh | bash
```

This creates:
- `llama-server.service` - Auto-starts llama-server on port 8081
- `offgrid-llm.service` - Auto-starts OffGrid API server on port 11611

### Option 3: Provide Properly-Built Pre-Built Binaries (Future)

**Recommended long-term solution:** The offgrid-llm repository should provide its own pre-built llama-server binaries compiled with `BUILD_SHARED_LIBS=OFF`. This would require:

1. GitHub Actions workflow to build llama-server with static backends
2. Create releases with properly-built binaries for each platform:
   - `llama-server-linux-x64-static.zip`
   - `llama-server-linux-x64-vulkan-static.zip`
   - `llama-server-macos-arm64-static.zip`
   - etc.
3. Update `installers/install.sh` to download from offgrid-llm releases instead of llama.cpp releases

## Current Status

- ✅ `dev/install.sh` (production installer) - FIXED with BUILD_SHARED_LIBS=OFF
- ⚠️ `installers/install.sh` (quick installer) - Downloads broken binaries
- ✅ `offgrid run` command - FIXED to auto-start offgrid serve (architectural fix)

## User Workaround

Until the pre-built binaries are fixed, quick installer users should:

1. Run the quick installer as normal:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash
   ```

2. Build and install proper llama-server binary (see Option 1 above)

3. Start llama-server manually:
   ```bash
   llama-server -m ~/.offgrid-llm/models/YOUR_MODEL.gguf --port 8081 --host 127.0.0.1 &
   ```

4. Use offgrid commands (offgrid run will auto-start offgrid serve):
   ```bash
   offgrid run YOUR_MODEL
   ```

## Architecture Notes

OffGrid LLM uses a two-tier architecture:

1. **llama-server** (port 8081) - llama.cpp HTTP server, handles actual inference
2. **offgrid serve** (port 11611) - OffGrid API server, proxies to llama-server

The `offgrid run` command now auto-starts `offgrid serve` if it's not running, but users must start llama-server separately (unless using the production installer with systemd).
