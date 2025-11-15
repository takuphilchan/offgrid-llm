# Running on 4GB RAM

OffGrid LLM works on systems with as little as 4GB RAM by using smaller models and efficient quantization.

## Quick Start

**Find models for your RAM:**
```bash
offgrid search llama --ram 4
offgrid search mistral --ram 4
```

## Recommended Models for 4GB RAM

### 1B Models (~1GB RAM)
- Llama 3.2 1B Instruct (Q4_K_M)
- Qwen2.5 1.5B Instruct (Q4_K_M)
- Phi-2 2.7B (Q4_K_M)

**Download:**
```bash
offgrid search "llama 1b" --ram 4 --limit 3
offgrid download-hf MaziyarPanahi/Llama-3.2-1B-Instruct-GGUF --file Llama-3.2-1B-Instruct.Q4_K_M.gguf
```

### 3B Models (~2GB RAM)
- Llama 3.2 3B Instruct (Q4_K_M)
- Phi-3 Mini 3.8B (Q4_K_M)
- StableLM 3B (Q4_K_M)

**Download:**
```bash
offgrid search "llama 3b" --ram 4
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF --file Llama-3.2-3B-Instruct-Q4_K_M.gguf
```

## Understanding Quantization

**Q2_K** - Smallest files, lowest quality (~0.5GB for 3B models)
**Q3_K** - Small files, acceptable quality (~0.6GB for 3B models)
**Q4_K_M** - Recommended balance (~0.8GB for 3B models)
**Q5_K** - Better quality, larger files (~1GB for 3B models)

For 4GB RAM, stick with Q4_K_M or lower quantizations.

## RAM Usage Breakdown

```
1B model (Q4_K_M):
  Model file: ~0.6GB
  Runtime overhead: ~0.4GB
  Total RAM: ~1GB

3B model (Q4_K_M):
  Model file: ~1.5GB
  Runtime overhead: ~0.5GB
  Total RAM: ~2GB
```

## Performance Tips

**Reduce context window** (saves RAM):
```bash
offgrid run model.gguf --context 2048  # Default is 4096
```

**Disable unnecessary features:**
- Close other applications
- Use CPU-only mode if GPU has limited VRAM
- Avoid very long conversations (clear context periodically)

## What to Expect

**1B models:**
- Good for: Simple tasks, code completion, basic Q&A
- Not good for: Complex reasoning, long-form writing

**3B models:**
- Good for: Most everyday tasks, coding help, summarization
- Not good for: Very complex reasoning, specialized knowledge

## Troubleshooting

**Out of memory errors:**
```bash
# Use smaller model
offgrid search --ram 4 --limit 5

# Reduce context
offgrid run model.gguf --context 1024

# Use lighter quantization
offgrid search llama --quant Q3_K_M
```

**Slow performance:**
- This is normal on CPU-only with 4GB RAM
- 1B models respond in 2-5 seconds
- 3B models may take 5-15 seconds per response

## Why This Matters

4GB RAM support means OffGrid LLM works on:
- Budget laptops from 2015-2020
- Raspberry Pi 4 (4GB model)
- Entry-level computers worldwide
- Older hardware still in use

This makes AI accessible regardless of economic constraints or hardware availability.

## Next Steps

- [Model Setup Guide](MODEL_SETUP.md)
- [Performance Tuning](advanced/PERFORMANCE.md)
- [HuggingFace Integration](guides/HUGGINGFACE_INTEGRATION.md)
