#!/bin/bash
# Test streaming chat completion

echo "🧪 Testing Streaming Chat Completion"
echo "======================================"
echo ""

# Start server in background if not running
if ! lsof -Pi :11611 -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    echo "Starting server..."
    ./offgrid serve &
    SERVER_PID=$!
    sleep 2
    echo "Server started (PID: $SERVER_PID)"
    CLEANUP=true
else
    echo "Server already running on port 11611"
    CLEANUP=false
fi

echo ""
echo "Sending streaming request..."
echo ""

curl -N http://localhost:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mock-model",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello! How are you?"}
    ],
    "stream": true,
    "temperature": 0.7
  }'

echo ""
echo ""
echo "======================================"
echo "✅ Streaming test complete"

# Cleanup
if [ "$CLEANUP" = true ]; then
    echo "Stopping server..."
    kill $SERVER_PID 2>/dev/null
fi
