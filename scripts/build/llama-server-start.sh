#!/bin/bash
# Start llama-server with the first available model

MODELS_DIR="${HOME}/.offgrid-llm/models"
PORT=42382  # Uncommon port in high range to avoid conflicts

# Find first .gguf model
MODEL=$(find "$MODELS_DIR" -name "*.gguf" -type f | sort -h | head -1)

if [ -z "$MODEL" ]; then
    echo "No models found in $MODELS_DIR"
    exit 1
fi

echo "Starting llama-server with model: $(basename "$MODEL")"
echo "Port: $PORT"

exec llama-server \
    --model "$MODEL" \
    --port $PORT \
    --host 127.0.0.1 \
    -c 4096 \
    --n-gpu-layers 0 \
    --threads 4
