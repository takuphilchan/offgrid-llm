# Quick Start: HuggingFace Integration

Get started with OffGrid's HuggingFace model search and download in 5 minutes.

## Prerequisites

1. OffGrid LLM installed and built:
   ```bash
   cd offgrid-llm
   make build
   ```

2. Server running (optional for CLI, required for API):
   ```bash
   ./offgrid serve
   # Or if installed: sudo systemctl start offgrid
   ```

## 1. Search for Models (30 seconds)

### Basic Search
```bash
# Search by name
./offgrid search llama

# Search with author filter
./offgrid search mistral --author TheBloke

# Search specific quantization
./offgrid search --quant Q4_K_M --limit 10
```

### Advanced Search
```bash
# Most popular models
./offgrid search --sort downloads --limit 10

# Recently updated models
./offgrid search --sort modified --limit 10

# Combine filters
./offgrid search "coding" --author TheBloke --quant Q4_K_M --sort downloads
```

**Output Example:**
```
🔍 Searching HuggingFace Hub...

Found 15 models:

1. TheBloke/Llama-2-7B-Chat-GGUF
   ↓ 2.5M downloads  ❤ 342 likes  │  Recommended: Q4_K_M (3.8 GB)
   Available: Q4_K_M (3.8GB), Q5_K_M (4.6GB), Q6_K (5.5GB)
   Download: offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF --file llama-2-7b-chat.Q4_K_M.gguf

2. TheBloke/Mistral-7B-Instruct-v0.2-GGUF
   ↓ 1.8M downloads  ❤ 218 likes  │  Recommended: Q4_K_M (4.1 GB)
   ...
```

## 2. Download a Model (2-10 minutes)

### Quick Download
```bash
# Download best variant (Q4_K_M/Q5_K_M)
./offgrid download-hf TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF --quant Q4_K_M
```

### Specific File
```bash
# Download exact file
./offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF \
  --file llama-2-7b-chat.Q4_K_M.gguf
```

**Progress Display:**
```
📦 Fetching model info: TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF

📥 Downloading tinyllama-1.1b-chat.Q4_K_M.gguf (0.6 GB)
  Progress: 67.3% (0.4 / 0.6 GB) · 15.2 MB/s

✓ Download complete!
  Model saved to: /var/lib/offgrid/models/tinyllama-1.1b-chat.Q4_K_M.gguf

  Run with: offgrid run tinyllama-1.1b-chat.Q4_K_M.gguf
```

## 3. Chat with Model (instant)

### Interactive Terminal Chat
```bash
# Start chatting
./offgrid run tinyllama-1.1b-chat.Q4_K_M.gguf
```

**Chat Session:**
```
🚀 Starting interactive chat with tinyllama-1.1b-chat.Q4_K_M.gguf
Type 'exit' to quit, 'clear' to reset conversation

Connecting to inference engine...

You: What is machine learning?
Assistant: Machine learning is a subset of artificial intelligence that enables 
computers to learn from data without being explicitly programmed. It involves 
algorithms that improve automatically through experience...

You: Give me a simple example
Assistant: Sure! Think of email spam filters. Initially, you mark some emails 
as spam. The ML algorithm learns patterns from those examples - certain words, 
sender patterns, etc. Over time, it gets better at automatically detecting spam...

You: clear
Conversation cleared.

You: exit
Goodbye!
```

**Chat Commands:**
- Type normally to send messages
- `clear` - Reset conversation
- `exit` or `quit` - Exit chat

## 4. Benchmark Performance (1-2 minutes)

### Using the API
```bash
curl -X POST http://localhost:11611/v1/benchmark \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tinyllama-1.1b-chat.Q4_K_M.gguf",
    "prompt_tokens": 512,
    "output_tokens": 128,
    "iterations": 3
  }'
```

**Result:**
```json
{
  "model": "tinyllama-1.1b-chat.Q4_K_M.gguf",
  "results": {
    "avg_prompt_tokens_per_sec": 1247.3,
    "avg_generation_tokens_per_sec": 42.1,
    "avg_total_time_ms": 3821,
    "avg_memory_mb": 687
  }
}
```

## 5. Use the Search API

### Simple GET Request
```bash
curl "http://localhost:11611/v1/search?query=llama&author=TheBloke&limit=5"
```

### Advanced POST Request
```bash
curl -X POST http://localhost:11611/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mistral",
    "author": "TheBloke",
    "quantization": "Q4_K_M",
    "min_downloads": 100000,
    "sort_by": "downloads",
    "limit": 10
  }'
```

## Complete Workflow Example

```bash
# 1. Search for good coding models
./offgrid search "code" --author TheBloke --sort downloads --limit 5

# 2. Download the top result
./offgrid download-hf TheBloke/CodeLlama-7B-Instruct-GGUF --quant Q4_K_M

# 3. Test it interactively
./offgrid run codellama-7b-instruct.Q4_K_M.gguf
# Try: "Write a Python function to sort a list"

# 4. Benchmark it
curl -X POST http://localhost:11611/v1/benchmark \
  -d '{"model":"codellama-7b-instruct.Q4_K_M.gguf"}' | jq
```

## Common Tasks

### Find Small Models (< 2GB)
```bash
./offgrid search --quant Q4_K_M --sort downloads | grep "0\.[0-9]GB\|1\.[0-9]GB"
```

### Find Latest Models
```bash
./offgrid search --sort modified --limit 10
```

### Find Most Popular Models
```bash
./offgrid search --sort downloads --limit 10
```

### Find Specific Model Type
```bash
# Chat models
./offgrid search "chat" --author TheBloke

# Instruct models
./offgrid search "instruct" --author TheBloke

# Code models
./offgrid search "code" --author TheBloke
```

## Integration Example (Python)

```python
import requests
import json

# Server URL
BASE_URL = "http://localhost:11611"

# 1. Search for models
response = requests.post(f"{BASE_URL}/v1/search", json={
    "query": "llama",
    "quantization": "Q4_K_M",
    "sort_by": "downloads",
    "limit": 5
})

results = response.json()['results']

# 2. Show user the options
for i, result in enumerate(results, 1):
    model = result['model']
    best = result['best_variant']
    print(f"{i}. {model['id']}")
    print(f"   Downloads: {model['downloads']:,}")
    print(f"   Size: {best['size_gb']:.1f} GB")
    print(f"   Quantization: {best['quantization']}")
    print()

# 3. Let user pick, then benchmark
choice = int(input("Select model (1-5): ")) - 1
selected = results[choice]

print(f"\nBenchmarking {selected['model']['id']}...")
bench = requests.post(f"{BASE_URL}/v1/benchmark", json={
    "model": selected['best_variant']['filename'],
    "prompt_tokens": 256,
    "output_tokens": 64,
    "iterations": 3
})

results = bench.json()['results']
print(f"Prompt speed: {results['avg_prompt_tokens_per_sec']:.1f} tokens/sec")
print(f"Generation speed: {results['avg_generation_tokens_per_sec']:.1f} tokens/sec")
print(f"Memory usage: {results['avg_memory_mb']:.0f} MB")
```

## Tips & Best Practices

### Quantization Selection
- **Q4_K_M** - Best default (good quality, reasonable size)
- **Q5_K_M** - Better quality if you have extra RAM
- **Q3_K_M** - Smaller, lower quality (mobile/edge)
- **Q6_K** - High quality, large size
- **Q8_0** - Near-perfect, very large

### Search Tips
- Use `--author TheBloke` for well-tested GGUF conversions
- Sort by `downloads` to find proven models
- Sort by `modified` to find latest releases
- Combine filters to narrow down exactly what you need

### Performance
- Start with small models (TinyLlama 1.1B) to test setup
- Benchmark before deploying to production
- Q4_K_M is usually the sweet spot for most hardware

### Troubleshooting
```bash
# Server not responding?
sudo systemctl status offgrid

# Check logs
sudo journalctl -u offgrid -f

# Test server manually
./offgrid serve
```

## Next Steps

1. **Explore more models:**
   - Browse [TheBloke's HuggingFace](https://huggingface.co/TheBloke)
   - Search by task type (chat, code, reasoning)
   - Try different quantizations

2. **Build an application:**
   - Use the search API to let users discover models
   - Use benchmark API to recommend models
   - Integrate chat API for inference

3. **Read full docs:**
   - [HUGGINGFACE_INTEGRATION.md](HUGGINGFACE_INTEGRATION.md) - Complete guide
   - [NEW_FEATURES.md](NEW_FEATURES.md) - Feature overview
   - [API.md](API.md) - API reference

## Help & Support

```bash
# CLI help
./offgrid help
./offgrid search --help
./offgrid download-hf --help

# Check system info
./offgrid info

# Test setup
curl http://localhost:11611/health
```

## Summary Commands

```bash
# Essential commands you'll use most:
./offgrid search <query>                    # Find models
./offgrid download-hf <id> --quant Q4_K_M   # Download
./offgrid run <model>                        # Chat
./offgrid benchmark <model>                  # Test performance
```

That's it! You're now ready to discover and use any GGUF model from HuggingFace.
