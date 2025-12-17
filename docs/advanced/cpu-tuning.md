# CPU Performance Optimization

Most users don't have a GPU - this guide helps you get the best performance from your CPU.

## Quick Start

**Check your CPU threads:**
```bash
offgrid info
```

**Optimize automatically:**
```bash
offgrid run model-name --optimize-cpu
```

This automatically sets the best thread count for your CPU.

---

## Understanding CPU Inference

**CPU-only inference is perfectly fine** for:
- Small models (1B-3B parameters)
- Medium models (7B-8B) on modern CPUs
- General chat and text generation

**GPU is only needed for:**
- Very large models (13B+)
- Real-time processing
- Image generation

---

## Thread Configuration

**Rule of thumb:**
- Use **physical cores** (not threads/hyperthreads)
- Leave 1-2 cores for your OS

**Examples:**
- 4-core CPU: Use 3 threads
- 6-core CPU: Use 5 threads
- 8-core CPU: Use 6-7 threads
- 12-core+ CPU: Use cores - 2

**Set threads manually:**
```bash
offgrid run model-name --threads 6
```

---

## Model Selection for CPU

### Budget Hardware (4GB RAM, 2-4 cores)

**Best models:**
```bash
# Llama 1B - Fastest, great quality
offgrid download-hf MaziyarPanahi/Llama-3.2-1B-Instruct-GGUF \
  --file Llama-3.2-1B-Instruct.Q4_K_M.gguf

# Phi-2 2.7B - Better quality, still fast
offgrid download-hf TheBloke/phi-2-GGUF \
  --file phi-2.Q4_K_M.gguf
```

**Performance:**
- 1B models: 10-20 tokens/sec
- 3B models: 5-10 tokens/sec

### Mid-range (8GB RAM, 4-8 cores)

**Recommended:**
```bash
# Llama 3B - Best balance
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf

# Mistral 7B - Higher quality
offgrid download-hf TheBloke/Mistral-7B-Instruct-v0.2-GGUF \
  --file mistral-7b-instruct-v0.2.Q4_K_M.gguf
```

**Performance:**
- 3B models: 8-15 tokens/sec
- 7B models: 3-8 tokens/sec

### Higher-end (16GB+ RAM, 8+ cores)

**Best options:**
```bash
# Mistral 7B with better quantization
offgrid search mistral --quant Q5_K_M

# Llama 13B for best quality
offgrid search llama-2-13b --quant Q4_K_M
```

**Performance:**
- 7B models (Q5): 5-10 tokens/sec
- 13B models (Q4): 2-5 tokens/sec

---

## Quantization Guide

Quantization reduces model size and speeds up inference. For CPU:

**Q2_K** - Smallest, fastest, lower quality
- Use when: Very limited RAM (<2GB available)
- Speed: 2-3x faster than Q4
- Quality: Noticeable degradation

**Q3_K_M** - Good balance for old hardware
- Use when: 2-4GB RAM, older CPU
- Speed: 1.5-2x faster than Q4
- Quality: Minor degradation

**Q4_K_M** - **Recommended for most users**
- Use when: 4GB+ RAM
- Speed: Good baseline
- Quality: Excellent, hard to tell from original

**Q5_K_M** - Higher quality
- Use when: 8GB+ RAM, modern CPU
- Speed: Slightly slower than Q4
- Quality: Very close to original

**Q8_0** - Maximum quality
- Use when: 16GB+ RAM, powerful CPU
- Speed: 2x slower than Q4
- Quality: Nearly identical to original

**Quick comparison:**
```bash
# Mistral 7B sizes and speed (approximate)
Q2_K: 2.5GB, 10 tok/s on 4-core CPU
Q4_K_M: 4.1GB, 5 tok/s on 4-core CPU
Q5_K_M: 5.0GB, 4 tok/s on 4-core CPU
Q8_0: 7.2GB, 2.5 tok/s on 4-core CPU
```

---

## Performance Tips

### 1. Use Smaller Context Windows

**Lower context = faster inference:**
```bash
offgrid run model-name --context 2048
```

**Context size guide:**
- 2048 tokens: Basic chat (fast)
- 4096 tokens: Default (balanced)
- 8192 tokens: Long conversations (slower)

### 2. Adjust Batch Size

**Smaller batches for older CPUs:**
```bash
offgrid run model-name --batch 256
```

**Batch size guide:**
- 128: Old/slow CPUs
- 256: Budget CPUs
- 512: Default (modern CPUs)
- 1024+: High-end CPUs

### 3. Enable Memory Locking (Linux/macOS)

**Prevents swapping to disk:**
```bash
offgrid run model-name --mlock
```

Only use if you have enough RAM. Check first:
```bash
offgrid info  # Shows available RAM
```

### 4. Use Memory Mapping

**Enabled by default** - loads models faster:
```bash
offgrid run model-name --mmap
```

Disable only if you have issues:
```bash
offgrid run model-name --no-mmap
```

---

## Real-World Examples

### Example 1: Student Laptop (4GB RAM, Intel i3)

**Goal:** Run a helpful AI for homework

```bash
# Search for small models
offgrid search llama --ram 4

# Download 1B model (fastest)
offgrid download-hf MaziyarPanahi/Llama-3.2-1B-Instruct-GGUF \
  --file Llama-3.2-1B-Instruct.Q4_K_M.gguf

# Run optimized for dual-core (2 physical cores)
offgrid run Llama-3.2-1B-Instruct.Q4_K_M \
  --threads 2 \
  --context 2048 \
  --batch 256
```

**Expected:** 10-15 tokens/sec, very usable

### Example 2: Office Desktop (8GB RAM, Intel i5)

**Goal:** Code assistant and writing help

```bash
# Download 3B model (best balance)
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf

# Run on quad-core (4 physical cores)
offgrid run Llama-3.2-3B-Instruct-Q4_K_M \
  --threads 3 \
  --optimize-cpu
```

**Expected:** 8-12 tokens/sec, smooth experience

### Example 3: Home Server (16GB RAM, AMD Ryzen)

**Goal:** High-quality local AI for family

```bash
# Download 7B model with better quantization
offgrid download-hf TheBloke/Mistral-7B-Instruct-v0.2-GGUF \
  --file mistral-7b-instruct-v0.2.Q5_K_M.gguf

# Run on 6-core (6 physical cores)
offgrid run mistral-7b-instruct-v0.2.Q5_K_M \
  --threads 5 \
  --context 4096 \
  --mlock
```

**Expected:** 5-8 tokens/sec, high quality

### Example 4: Raspberry Pi 4/5 (4GB-8GB RAM)

**Goal:** Truly offline AI on $50 hardware

```bash
# Use smallest viable model
offgrid download-hf MaziyarPanahi/Llama-3.2-1B-Instruct-GGUF \
  --file Llama-3.2-1B-Instruct.Q3_K_M.gguf

# Optimize for ARM CPU
offgrid run Llama-3.2-1B-Instruct.Q3_K_M \
  --threads 3 \
  --context 1024 \
  --batch 128
```

**Expected:** 3-5 tokens/sec on Pi 5, slower on Pi 4

---

## Troubleshooting

### Problem: Too slow (<1 token/sec)

**Solutions:**
1. Use smaller model (1B instead of 7B)
2. Lower quantization (Q3_K instead of Q4_K)
3. Reduce threads (might be too many)
4. Lower context size (--context 2048)
5. Reduce batch size (--batch 256)

### Problem: Out of memory

**Solutions:**
1. Close other applications
2. Use more aggressive quantization (Q2_K)
3. Switch to smaller model
4. Don't use --mlock
5. Check: `offgrid info`

### Problem: Model loads slowly

**Solutions:**
1. Enable mmap (should be default)
2. Move model to SSD (not HDD)
3. Use smaller model
4. First load is always slower (model file is cached)

### Problem: System freezes

**Solutions:**
1. Reduce thread count (--threads 2)
2. Don't use --mlock
3. Increase swap space
4. Use smaller model

---

## Benchmarking

**Test your setup:**
```bash
offgrid benchmark model-name
```

**Compare different settings:**
```bash
# Test different thread counts
offgrid benchmark model-name --threads 2
offgrid benchmark model-name --threads 4
offgrid benchmark model-name --threads 6

# Test different batch sizes
offgrid benchmark model-name --batch 256
offgrid benchmark model-name --batch 512
```

**What's good performance?**
- 1B model: 8+ tokens/sec = good
- 3B model: 5+ tokens/sec = good
- 7B model: 3+ tokens/sec = good
- 13B model: 1+ tokens/sec = usable

---

## When GPU Actually Helps

**GPU acceleration is worth it for:**
- Models 7B and larger
- Real-time applications
- Batch processing
- Multiple concurrent users
- Vision models (future)

**Not worth it for:**
- 1B-3B models (CPU is fine)
- Occasional use
- Single user
- Budget constraints

**Check GPU support:**
```bash
offgrid info  # Shows GPU if detected
```

---

## Summary

**For most users (CPU-only):**
1. Use 1B-3B models with Q4_K_M quantization
2. Set threads = physical cores - 1
3. Use default context (4096) or lower (2048)
4. Enable mlock if you have enough RAM
5. Expect 5-15 tokens/sec (very usable)

**This is perfectly fine!** Modern small models are surprisingly capable. You don't need a GPU for a great experience.

**More help:**
- System requirements: `offgrid info`
- Find models for your RAM: `offgrid search --ram 4`
- Model recommendations: [4GB_RAM.md](low-memory.md)
- Performance tuning: [PERFORMANCE.md](performance.md)
