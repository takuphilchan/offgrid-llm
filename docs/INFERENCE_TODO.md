# Real Inference Integration TODO

## Current Status

The project currently runs in **mock mode** which returns placeholder responses instead of actual LLM inference. This is a temporary state while we resolve version compatibility issues.

## Problem

The `go-skynet/go-llama.cpp` binding (v0.0.0-20240314183750-6a8041ef6b46, March 2024) expects:
- `grammar-parser.h` from llama.cpp's `common/` directory
- Older llama.cpp API structure

The current llama.cpp (November 2024) has:
- Refactored grammar parsing into different structure
- No `grammar-parser.h` header file
- Updated API interfaces

**Build Error:**
```
binding.cpp:5:10: fatal error: grammar-parser.h: No such file or directory
```

## Solution Options

### Option 1: Pin llama.cpp to Compatible Version ⚡ (Fastest)
**Pros:**
- Quick fix - just checkout older commit
- Proven working combination

**Cons:**
- Missing latest llama.cpp features/optimizations
- Security updates not included

**Steps:**
1. Find exact llama.cpp commit hash used in go-llama.cpp submodule (circa March 2024)
2. Checkout that specific commit in `~/llama.cpp`
3. Rebuild llama.cpp libraries
4. Re-enable llama build in `install.sh`

### Option 2: Update go-llama.cpp Binding 🔨 (Medium effort)
**Pros:**
- Can use latest llama.cpp
- Learn the codebase better

**Cons:**
- Requires C++ knowledge
- Need to understand binding internals
- Ongoing maintenance burden

**Steps:**
1. Fork `go-skynet/go-llama.cpp`
2. Update `binding.cpp` to work with modern llama.cpp
3. Remove `grammar-parser.h` dependency
4. Use new llama.cpp grammar API
5. Test thoroughly
6. Update go.mod to use fork

### Option 3: Switch to llama-cpp-python + CGO Wrapper 🐍 (Different approach)
**Pros:**
- Well-maintained Python binding
- Active development
- Better documentation

**Cons:**
- Requires Python runtime
- More complex deployment
- Performance overhead of Python wrapper

**Steps:**
1. Install llama-cpp-python in system Python
2. Create CGO wrapper calling Python C API
3. Or use subprocess to call llama-cpp-python CLI
4. Update inference engine

### Option 4: Direct CGO Integration 🎯 (Most control)
**Pros:**
- Complete control over llama.cpp integration
- No third-party binding dependency
- Can use latest llama.cpp features

**Cons:**
- Most work required
- Need to write all the bindings
- Higher maintenance burden

**Steps:**
1. Write CGO wrappers for llama.h functions
2. Implement model loading, tokenization, inference
3. Handle memory management carefully
4. Add error handling
5. Create proper Go API

### Option 5: Use llama.cpp Server Mode 🌐 (Service architecture)
**Pros:**
- Clean separation of concerns
- llama.cpp handles all inference
- Easy to update llama.cpp independently

**Cons:**
- Extra process to manage
- Network overhead (even if localhost)
- More complex deployment

**Steps:**
1. Build llama.cpp server binary
2. Add systemd service for llama-cpp-server
3. Update offgrid to proxy to llama.cpp HTTP API
4. Handle server lifecycle

## Recommended Approach

**Start with Option 1** for quick wins, then move to **Option 4** for production:

1. **Phase 1: Get it working** (1-2 hours)
   - Pin to compatible llama.cpp commit
   - Enable real inference
   - Verify with TinyLlama model

2. **Phase 2: Production ready** (1-2 days)
   - Write direct CGO bindings to llama.h
   - Remove go-llama.cpp dependency  
   - Add comprehensive error handling
   - GPU acceleration support

3. **Phase 3: Optimization** (ongoing)
   - Benchmark performance
   - Add model caching
   - Streaming improvements
   - Multi-model support

## Testing Plan

Once real inference is enabled:

1. **Smoke Test**
   ```bash
   curl -X POST http://localhost:11611/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{
       "model": "tinyllama-1.1b-chat.Q4_K_M",
       "messages": [{"role": "user", "content": "What is 2+2?"}]
     }'
   ```
   Should return actual TinyLlama response (not "This is a mock response")

2. **Streaming Test**
   ```bash
   curl -X POST http://localhost:11611/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{
       "model": "tinyllama-1.1b-chat.Q4_K_M",
       "messages": [{"role": "user", "content": "Count to 10"}],
       "stream": true
     }'
   ```
   Should stream tokens as SSE events

3. **Load Test**
   - Multiple concurrent requests
   - Model switching
   - Memory usage monitoring

## Resources

- llama.cpp repo: https://github.com/ggerganov/llama.cpp
- go-llama.cpp repo: https://github.com/go-skynet/go-llama.cpp  
- llama.cpp examples: https://github.com/ggerganov/llama.cpp/tree/master/examples
- CGO documentation: https://go.dev/blog/cgo

## Timeline

- **Phase 1** - Compatible llama.cpp: Next session (1-2 hours)
- **Phase 2** - Direct CGO bindings: 1-2 days focused work
- **Phase 3** - Optimization: Ongoing

## Current Workaround

Mock mode provides:
- API compatibility testing
- Frontend development
- Performance baseline measurements
- Installation verification

All infrastructure is ready - just need to swap the inference backend!
