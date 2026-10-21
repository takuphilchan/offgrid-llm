#!/bin/bash
# Example: Using OffGrid LLM's HuggingFace Integration
# This script demonstrates the full workflow from search to chat

set -e

echo "=========================================="
echo "OffGrid LLM - HuggingFace Integration Demo"
echo "=========================================="
echo ""

# 1. Search for models
echo "1. Searching for TinyLlama models..."
echo ""
./offgrid search "tinyllama" --author TheBloke --limit 5
echo ""

# 2. Search with quantization filter
echo "2. Finding Q4_K_M quantized models..."
echo ""
./offgrid search "llama" --quant Q4_K_M --limit 3
echo ""

# 3. Use the search API
echo "3. Testing the search API endpoint..."
echo ""
curl -s "http://localhost:11611/v1/search?query=mistral&author=TheBloke&limit=3" | jq '.'
echo ""

# 4. Download a small model (commented out - uncomment to actually download)
echo "4. Download example (commented out):"
echo "   ./offgrid download-hf TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF --quant Q4_K_M"
echo ""

# Uncomment below to actually download:
# echo "Downloading TinyLlama..."
# ./offgrid download-hf TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF --quant Q4_K_M

# 5. Interactive chat (commented out - requires downloaded model)
echo "5. Interactive chat example (commented out):"
echo "   ./offgrid run tinyllama-1.1b-chat.Q4_K_M.gguf"
echo ""

# 6. Benchmark API example
echo "6. Benchmark API example (requires model to be loaded):"
echo ""
cat << 'EOF'
curl -X POST http://localhost:11611/v1/benchmark \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tinyllama-1.1b-chat.Q4_K_M.gguf",
    "prompt_tokens": 256,
    "output_tokens": 64,
    "iterations": 3
  }'
EOF
echo ""

# 7. Advanced search with multiple filters
echo "7. Advanced search - coding models under 5GB:"
echo ""
cat << 'EOF'
curl -X POST http://localhost:11611/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "code",
    "author": "TheBloke",
    "max_size": 5368709120,
    "quantization": "Q4_K_M",
    "sort_by": "downloads",
    "limit": 5
  }' | jq '.results[] | {id: .model.id, downloads: .model.downloads, size_gb: .best_variant.size_gb}'
EOF
echo ""

echo "=========================================="
echo "Demo Complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Start the server: ./offgrid serve"
echo "  2. Search for models: ./offgrid search <query>"
echo "  3. Download a model: ./offgrid download-hf <model-id> --quant Q4_K_M"
echo "  4. Chat with it: ./offgrid run <model-file>"
echo ""
echo "Documentation:"
echo "  - docs/HUGGINGFACE_INTEGRATION.md - Full guide"
echo "  - docs/NEW_FEATURES.md - Feature summary"
echo "  - ./offgrid help - CLI help"
echo ""
