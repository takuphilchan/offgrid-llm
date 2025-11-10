# Quick Reference Guide - New Features

## Prompt Templates

Use pre-built templates for common tasks without writing prompts from scratch.

### List All Templates
```bash
offgrid template list
```

### View Template Details
```bash
offgrid template show code-review
offgrid template show summarize
```

### Use a Template
```bash
# Interactive mode - prompts for each variable
offgrid template apply code-review

# Example workflow:
$ offgrid template apply translate
text: Hello, how are you?
from: English
to: Spanish
# Generated prompt shown
```

### Available Templates

| Template | Purpose | Variables |
|----------|---------|-----------|
| `summarize` | Condense text | text, length |
| `code-review` | Review code for issues | code, language |
| `translate` | Translate languages | text, from, to |
| `explain` | Explain concepts | concept, level, style |
| `brainstorm` | Generate ideas | topic, count, focus |
| `debug` | Find bugs | code, language, error |
| `document` | Generate docs | code, language, style |
| `refactor` | Improve code | code, language, focus |
| `test` | Generate tests | code, language, framework |
| `cli` | Create CLI tools | description, language, features |

## Model Aliases

Create friendly shortcuts for long model names.

### Create Aliases
```bash
offgrid alias set chat llama-2-7b-chat.Q4_K_M.gguf
offgrid alias set code codellama-7b.Q4_K_M.gguf
offgrid alias set tiny tinyllama-1.1b.Q4_K_M.gguf
```

### List Aliases
```bash
offgrid alias list
```

### Remove Alias
```bash
offgrid alias remove chat
```

### Usage
```bash
# Use alias instead of full name
offgrid run chat
offgrid benchmark code
```

## Favorites

Star frequently used models for quick access.

### Add to Favorites
```bash
offgrid favorite add llama-2-7b-chat.Q4_K_M.gguf
offgrid favorite add mistral-7b-instruct.Q4_K_M.gguf
```

### List Favorites
```bash
offgrid favorite list
# ★ llama-2-7b-chat.Q4_K_M.gguf
# ★ mistral-7b-instruct.Q4_K_M.gguf
```

### Remove from Favorites
```bash
offgrid favorite remove llama-2-7b-chat.Q4_K_M.gguf
```

## Batch Processing

Process multiple prompts in parallel from JSONL files.

### Input Format (JSONL)
```json
{"id": "req1", "model": "model.gguf", "prompt": "What is AI?", "options": {"temperature": 0.7}}
{"id": "req2", "model": "model.gguf", "prompt": "Explain ML", "options": {"max_tokens": 100}}
```

### Process Batch
```bash
# Default concurrency (4 workers)
offgrid batch process input.jsonl output.jsonl

# Custom concurrency
offgrid batch process input.jsonl output.jsonl --concurrency 8
```

### Output Format
```json
{
  "id": "req1",
  "model": "model.gguf",
  "prompt": "What is AI?",
  "response": "Artificial Intelligence is...",
  "error": "",
  "duration_ms": 1234,
  "tokens_per_sec": 45.6
}
```

### View Results
```bash
cat output.jsonl | jq .
cat output.jsonl | jq -r '.response'
```

## Response Caching

Automatic caching of responses for faster repeated queries.

### Check Cache Statistics
```bash
curl http://localhost:11611/cache/stats | jq .
```

Response:
```json
{
  "enabled": true,
  "entries": 42,
  "max_entries": 1000,
  "ttl_seconds": 3600,
  "hits": 156,
  "misses": 84,
  "hit_rate": "65.00%"
}
```

### Clear Cache
```bash
curl -X POST http://localhost:11611/cache/clear
```

### Cache Behavior
- **Capacity**: 1000 entries (LRU eviction)
- **TTL**: 1 hour (configurable)
- **Cleanup**: Automatic every 15 minutes
- **Key**: Hash of (model + prompt + parameters)

## Example Workflows

### Code Review Workflow
```bash
# 1. Apply code-review template
offgrid template apply code-review

# 2. Paste your code when prompted
# 3. Get detailed review output
```

### Translation Workflow
```bash
# Create batch file for multiple translations
cat > translations.jsonl << 'EOF'
{"id":"1","model":"llama-2","prompt":"Translate to Spanish: Hello"}
{"id":"2","model":"llama-2","prompt":"Translate to French: Hello"}
{"id":"3","model":"llama-2","prompt":"Translate to German: Hello"}
EOF

# Process all at once
offgrid batch process translations.jsonl results.jsonl --concurrency 3
```

### Model Management Workflow
```bash
# 1. Download models
offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF

# 2. Create alias
offgrid alias set llama2 llama-2-7b-chat.Q4_K_M.gguf

# 3. Mark as favorite
offgrid favorite add llama-2-7b-chat.Q4_K_M.gguf

# 4. Use with alias
offgrid run llama2
```

## API Integration

### Using Templates via API
Templates are applied client-side. Use the CLI or integrate template generation into your application.

### Monitoring Cache Performance
```python
import requests

# Check cache stats
stats = requests.get('http://localhost:11611/cache/stats').json()
print(f"Cache hit rate: {stats['hit_rate']}")
print(f"Active entries: {stats['entries']}/{stats['max_entries']}")

# Clear cache if needed
if stats['hit_rate'] < '50%':
    requests.post('http://localhost:11611/cache/clear')
```

### Batch Processing via API
Use the CLI `batch process` command or implement your own batch processor using the inference endpoints:
```python
import asyncio
import aiohttp

async def process_batch(prompts):
    async with aiohttp.ClientSession() as session:
        tasks = [
            session.post('http://localhost:11611/v1/completions', 
                        json={'model': 'model.gguf', 'prompt': p})
            for p in prompts
        ]
        responses = await asyncio.gather(*tasks)
        return [await r.json() for r in responses]
```

## Tips & Best Practices

1. **Templates**: Modify defaults by creating custom templates based on built-in ones
2. **Aliases**: Use short, memorable names (e.g., `chat`, `code`, `tiny`)
3. **Favorites**: Star models you use >80% of the time
4. **Batch Processing**: 
   - Use concurrency = number of CPU cores for best performance
   - Keep prompts similar length for even distribution
5. **Caching**:
   - Great for demos, testing, repeated queries
   - Clear cache when changing model parameters
   - Monitor hit rate to optimize prompt patterns
