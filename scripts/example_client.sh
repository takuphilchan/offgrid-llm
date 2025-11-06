#!/bin/bash

# OffGrid LLM - Example API Client

BASE_URL="http://localhost:8080"

echo "🌐 OffGrid LLM - Example API Client"
echo "===================================="
echo

# Check if server is running
echo "1️⃣  Checking server health..."
HEALTH=$(curl -s "${BASE_URL}/health")
if [ $? -eq 0 ]; then
    echo "✅ Server is healthy: ${HEALTH}"
else
    echo "❌ Server is not running. Start it with: ./offgrid"
    exit 1
fi
echo

# List available models
echo "2️⃣  Listing available models..."
MODELS=$(curl -s "${BASE_URL}/v1/models")
echo "${MODELS}" | jq '.' 2>/dev/null || echo "${MODELS}"
echo

# Get model count
MODEL_COUNT=$(echo "${MODELS}" | jq '.data | length' 2>/dev/null)
if [ "$MODEL_COUNT" = "0" ] || [ -z "$MODEL_COUNT" ]; then
    echo "⚠️  No models available. Please add models to ~/.offgrid-llm/models/"
    echo "   See docs/MODEL_SETUP.md for instructions."
    exit 0
fi

echo "Found ${MODEL_COUNT} model(s)"
echo

# Get first model ID
MODEL_ID=$(echo "${MODELS}" | jq -r '.data[0].id' 2>/dev/null)
echo "Using model: ${MODEL_ID}"
echo

# Example 1: Chat Completion
echo "3️⃣  Testing chat completion..."
cat > /tmp/chat_request.json << EOF
{
  "model": "${MODEL_ID}",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "What is OffGrid LLM?"}
  ],
  "temperature": 0.7,
  "max_tokens": 100
}
EOF

CHAT_RESPONSE=$(curl -s -X POST "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d @/tmp/chat_request.json)

echo "Response:"
echo "${CHAT_RESPONSE}" | jq '.' 2>/dev/null || echo "${CHAT_RESPONSE}"
echo

# Example 2: Text Completion
echo "4️⃣  Testing text completion..."
cat > /tmp/completion_request.json << EOF
{
  "model": "${MODEL_ID}",
  "prompt": "The future of AI in offline environments is",
  "temperature": 0.7,
  "max_tokens": 50
}
EOF

COMPLETION_RESPONSE=$(curl -s -X POST "${BASE_URL}/v1/completions" \
  -H "Content-Type: application/json" \
  -d @/tmp/completion_request.json)

echo "Response:"
echo "${COMPLETION_RESPONSE}" | jq '.' 2>/dev/null || echo "${COMPLETION_RESPONSE}"
echo

# Cleanup
rm -f /tmp/chat_request.json /tmp/completion_request.json

echo "✅ API examples completed!"
echo
echo "For more examples, see the documentation."
