# HuggingFace Hub Integration

OffGrid LLM now features **direct HuggingFace Hub integration**, giving you access to thousands of models without waiting for approval from centralized registries like Ollama's.

## Why This Matters

**OffGrid Advantage over Ollama:**
-  **No waiting** - Pull any GGUF model from HuggingFace instantly
-  **Search by metrics** - Find models by downloads, likes, size, quantization
-  **Automatic discovery** - Browse thousands of models with smart filtering
-  **Direct downloads** - No intermediary servers or approval process
-  **Community-driven** - Access bleeding-edge models the day they're released

**Ollama's limitation:** Models must be manually uploaded and approved by Ollama's team before you can use them.

## Features

### 1. Model Search

Search HuggingFace Hub with powerful filtering:

```bash
# Basic search
offgrid search llama

# Filter by author (e.g., TheBloke who publishes GGUF versions)
offgrid search mistral --author TheBloke

# Filter by quantization level
offgrid search --quant Q4_K_M --author TheBloke

# Sort by popularity
offgrid search --sort downloads --limit 10

# Sort by recency
offgrid search --sort modified --limit 20
```

**Search Options:**
- `-a, --author <name>` - Filter by author (e.g., "TheBloke")
- `-q, --quant <type>` - Filter by quantization (Q4_K_M, Q5_K_S, etc.)
- `-s, --sort <field>` - Sort by: downloads, likes, created, modified
- `-l, --limit <n>` - Limit results (default: 20)
- `--all` - Include gated models (require approval)

**Example Output:**
```
Found 15 models:

1. TheBloke/Llama-2-7B-Chat-GGUF
   ↓ 2.5M downloads  ❤ 342 likes  │  Recommended: Q4_K_M (3.8 GB)
   Available: Q4_K_M (3.8GB), Q5_K_M (4.6GB), Q6_K (5.5GB), Q8_0 (7.2GB), F16 (13.5GB)
   Download: offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF --file llama-2-7b-chat.Q4_K_M.gguf

2. TheBloke/Mistral-7B-Instruct-v0.2-GGUF
   ↓ 1.8M downloads  ❤ 218 likes  │  Recommended: Q4_K_M (4.1 GB)
   ...
```

### 2. Direct Download from HuggingFace

Download any GGUF model directly:

```bash
# Download specific file
offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF --file llama-2-7b-chat.Q4_K_M.gguf

# Download with quantization filter (shows menu if multiple matches)
offgrid download-hf TheBloke/Mistral-7B-Instruct-v0.2-GGUF --quant Q4_K_M

# Just model ID - interactive file selection
offgrid download-hf TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF
```

**Progress Display:**
```
[Package] Fetching model info: TheBloke/Llama-2-7B-Chat-GGUF

📥 Downloading llama-2-7b-chat.Q4_K_M.gguf (3.8 GB)
  Progress: 47.2% (1.8 / 3.8 GB) · 12.3 MB/s

[OK] Download complete!
  Model saved to: /var/lib/offgrid/models/llama-2-7b-chat.Q4_K_M.gguf

  Run with: offgrid run llama-2-7b-chat.Q4_K_M.gguf
```

### 3. Interactive CLI Chat

Chat with models directly from your terminal (like `ollama run`):

```bash
# Start interactive chat
offgrid run llama-2-7b-chat.Q4_K_M.gguf

# Or use model name from catalog
offgrid run tinyllama-1.1b-chat
```

**Features:**
-  Streaming responses (real-time token generation)
-  Conversation history maintained
-  Commands: `exit`, `quit`, `clear`
-  Works with any loaded model

**Example Session:**
```
🚀 Starting interactive chat with llama-2-7b-chat.Q4_K_M.gguf
Type 'exit' to quit, 'clear' to reset conversation

Connecting to inference engine...

You: What is quantum computing?
Assistant: Quantum computing is a revolutionary approach to computation that uses 
quantum mechanical phenomena like superposition and entanglement to process 
information. Unlike classical computers that use bits (0 or 1), quantum computers 
use quantum bits or qubits...

You: Give me a simple analogy
Assistant: Think of it like searching a massive library. A classical computer 
would check each book one by one. A quantum computer is like having a ghost that 
can check all books simultaneously...

You: clear
Conversation cleared.

You: exit
Goodbye!
```

### 4. Model Benchmarking

Test model performance on your hardware:

**CLI (upcoming):**
```bash
# Quick benchmark
offgrid benchmark llama-2-7b-chat.Q4_K_M.gguf

# Custom settings
offgrid benchmark llama-2-7b-chat.Q4_K_M.gguf --prompt-tokens 512 --output-tokens 128 --iterations 5
```

**API Endpoint:**
```bash
# Benchmark via API
curl -X POST http://localhost:11611/v1/benchmark \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-2-7b-chat.Q4_K_M.gguf",
    "prompt_tokens": 512,
    "output_tokens": 128,
    "iterations": 3
  }'
```

**Response:**
```json
{
  "model": "llama-2-7b-chat.Q4_K_M.gguf",
  "config": {
    "prompt_tokens": 512,
    "output_tokens": 128,
    "iterations": 3
  },
  "results": {
    "avg_prompt_tokens_per_sec": 847.3,
    "avg_generation_tokens_per_sec": 23.4,
    "avg_total_time_ms": 5821,
    "avg_memory_mb": 4235,
    "runs": [
      {
        "prompt_tokens_per_sec": 856.2,
        "generation_tokens_per_sec": 24.1,
        "total_time_ms": 5654,
        "memory_used_mb": 4187
      },
      ...
    ]
  },
  "system": {
    "cpu_percent": 89.2,
    "memory_mb": 12458,
    "memory_total_mb": 16384
  }
}
```

### 5. Search API Endpoint

Integrate HuggingFace search into your applications:

**GET Request:**
```bash
curl "http://localhost:11611/v1/search?query=llama&author=TheBloke&sort=downloads&limit=10"
```

**POST Request (advanced filtering):**
```bash
curl -X POST http://localhost:11611/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mistral",
    "author": "TheBloke",
    "quantization": "Q4_K_M",
    "min_downloads": 100000,
    "max_size": 5368709120,
    "sort_by": "downloads",
    "limit": 20,
    "only_gguf": true,
    "exclude_gated": true
  }'
```

**Response:**
```json
{
  "total": 15,
  "results": [
    {
      "model": {
        "id": "TheBloke/Llama-2-7B-Chat-GGUF",
        "downloads": 2534821,
        "likes": 342,
        "tags": ["llama", "chat", "gguf"],
        "library_name": "gguf",
        "pipeline_tag": "text-generation"
      },
      "gguf_files": [
        {
          "filename": "llama-2-7b-chat.Q4_K_M.gguf",
          "size": 4081004224,
          "size_gb": 3.8,
          "quantization": "Q4_K_M",
          "parameter_size": "7B",
          "is_chat": true,
          "download_url": "https://huggingface.co/TheBloke/Llama-2-7B-Chat-GGUF/resolve/main/llama-2-7b-chat.Q4_K_M.gguf"
        }
      ],
      "best_variant": {
        "filename": "llama-2-7b-chat.Q4_K_M.gguf",
        "quantization": "Q4_K_M",
        "size_gb": 3.8
      },
      "score": 3847.2
    }
  ]
}
```

## Quantization Guide

Understanding GGUF quantization levels:

| Quantization | Quality | Size | Use Case |
|--------------|---------|------|----------|
| **Q2_K** | Low | Smallest | Extreme resource constraints |
| **Q3_K_M** | Medium-Low | Small | Mobile, edge devices |
| **Q4_0** | Good | Medium-Small | Good balance, fast |
| **Q4_K_M** | **Recommended** | Medium | Best quality/size ratio |
| **Q5_K_M** | Very Good | Medium-Large | Better quality, more RAM |
| **Q6_K** | Excellent | Large | High quality needs |
| **Q8_0** | Near-Perfect | Very Large | Maximum quality |
| **F16** | Perfect | Huge | Research, benchmarking |

**Recommendation:** Start with **Q4_K_M** for most use cases. It provides excellent quality at a reasonable size.

## Common Workflows

### Discover and Install a New Model

```bash
# 1. Search for models
offgrid search "coding" --author TheBloke --sort downloads

# 2. Download the best match
offgrid download-hf TheBloke/CodeLlama-7B-Instruct-GGUF --quant Q4_K_M

# 3. Test it interactively
offgrid run codellama-7b-instruct.Q4_K_M.gguf

# 4. Benchmark performance
curl -X POST http://localhost:11611/v1/benchmark \
  -d '{"model":"codellama-7b-instruct.Q4_K_M.gguf"}'
```

### Find the Best Model for Your Hardware

```bash
# Search for small models (< 4GB)
offgrid search --quant Q4_K_M | grep "3GB\|2GB\|1GB"

# Or search for specific size
offgrid search llama --author TheBloke | grep "3."

# Download and benchmark
offgrid download-hf TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF --quant Q4_K_M
offgrid benchmark tinyllama-1.1b-chat.Q4_K_M.gguf
```

### Stay Updated with Latest Models

```bash
# Find recently updated models
offgrid search --sort modified --limit 20

# Search for specific architecture
offgrid search "mistral" --sort modified

# Get bleeding-edge releases
offgrid search --sort created --limit 10
```

## API Integration

Use OffGrid's search in your applications:

**Python Example:**
```python
import requests

# Search for models
response = requests.post('http://localhost:11611/v1/search', json={
    'query': 'llama',
    'quantization': 'Q4_K_M',
    'min_downloads': 100000,
    'limit': 10
})

models = response.json()['results']

# Pick the most popular
best_model = models[0]
print(f"Best model: {best_model['model']['id']}")
print(f"Downloads: {best_model['model']['downloads']}")

# Download it
file_info = best_model['best_variant']
download_url = file_info['download_url']
# ... implement download logic
```

## Advantages Over Ollama

| Feature | OffGrid LLM | Ollama |
|---------|-------------|--------|
| Model Discovery | [Yes] Search HuggingFace directly | [No] Limited to Ollama registry |
| Model Availability | [Yes] 10,000+ GGUF models | [Limited] ~100 curated models |
| New Model Access | [Done] Instant (day of release) | [Failed] Wait for approval/upload |
| Search Filters | [Done] Size, quant, downloads, likes | [Warning] Limited filtering |
| API Access | [Done] Full search API | [Failed] No search API |
| CLI Chat | [Done] Built-in (`offgrid run`) | [Done] (`ollama run`) |
| Benchmarking | [Done] Automated benchmarks | [Failed] Manual only |
| Custom Models | [Done] Any GGUF from anywhere | [Warning] Manual Modelfile needed |

## Technical Details

**Implementation:**
- `internal/models/huggingface.go` - HuggingFace API client
- `cmd/offgrid/main.go` - CLI commands (search, download-hf, run)
- `internal/server/server.go` - API endpoints (/v1/search, /v1/benchmark)

**API Client Features:**
- Smart relevance scoring (downloads, likes, recency)
- Automatic GGUF file parsing
- Best variant selection (Q4_K_M/Q5_K_M preference)
- Progress tracking for downloads
- Comprehensive error handling

**Search Algorithm:**
```
Score = (downloads / 1000) + (likes * 10) + recency_bonus + query_relevance_bonus
Recency Bonus = (6 - months_since_update) * 50 (if < 6 months)
Query Relevance = +200 if query matches model ID
Gated Penalty = score * 0.5
```

## Next Steps

1. **Try it out:**
   ```bash
   offgrid search llama
   offgrid download-hf TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF --quant Q4_K_M
   offgrid run tinyllama-1.1b-chat.Q4_K_M.gguf
   ```

2. **Integrate into your app:**
   - Use `/v1/search` API to let users discover models
   - Use `/v1/benchmark` to recommend models for user hardware
   - Build model galleries, rankings, recommendations

3. **Explore advanced features:**
   - Automatic model recommendations based on hardware
   - Model comparison and benchmarking
   - Community ratings and reviews (future)

## See Also

- [API Documentation](API.md)
- [Model Setup Guide](MODEL_SETUP.md)
- [Deployment Guide](DEPLOYMENT.md)
