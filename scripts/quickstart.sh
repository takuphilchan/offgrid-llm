#!/bin/bash
set -e

echo "🌐 OffGrid LLM - Quick Start Script"
echo "=================================="
echo

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21.5 or higher."
    exit 1
fi

echo "✅ Go detected: $(go version)"
echo

# Build the project
echo "🔨 Building OffGrid LLM..."
make build

# Create models directory
MODELS_DIR="${HOME}/.offgrid-llm/models"
echo "📁 Creating models directory: ${MODELS_DIR}"
mkdir -p "${MODELS_DIR}"

# Check if any models exist
MODEL_COUNT=$(find "${MODELS_DIR}" -name "*.gguf" 2>/dev/null | wc -l)

if [ "$MODEL_COUNT" -eq 0 ]; then
    echo
    echo "⚠️  No models found in ${MODELS_DIR}"
    echo
    echo "Would you like to download TinyLlama 1.1B (~700MB)? (y/n)"
    read -r response
    
    if [[ "$response" =~ ^[Yy]$ ]]; then
        echo "📥 Downloading TinyLlama 1.1B Q4_K_M..."
        
        if command -v wget &> /dev/null; then
            wget -q --show-progress \
                "https://huggingface.co/TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF/resolve/main/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf" \
                -P "${MODELS_DIR}/"
            echo "✅ Model downloaded successfully!"
        elif command -v curl &> /dev/null; then
            curl -L --progress-bar \
                "https://huggingface.co/TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF/resolve/main/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf" \
                -o "${MODELS_DIR}/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf"
            echo "✅ Model downloaded successfully!"
        else
            echo "❌ Neither wget nor curl found. Please download a model manually."
            echo "   See docs/MODEL_SETUP.md for instructions."
        fi
    else
        echo "ℹ️  Skipping download. You can add models later to ${MODELS_DIR}"
        echo "   See docs/MODEL_SETUP.md for instructions."
    fi
else
    echo "✅ Found ${MODEL_COUNT} model(s) in ${MODELS_DIR}"
fi

echo
echo "🎉 Setup complete!"
echo
echo "To start the server:"
echo "  ./offgrid"
echo
echo "Or use:"
echo "  make run"
echo
echo "Once running, test with:"
echo "  curl http://localhost:11611/health"
echo "  curl http://localhost:11611/v1/models"
echo
echo "For more information:"
echo "  - Model setup: docs/MODEL_SETUP.md"
echo "  - README: README.md"
echo
