#!/bin/bash
# Test script for embeddings endpoint

set -e

PORT=${OFFGRID_PORT:-11611}
BASE_URL="http://localhost:$PORT"

echo "🧪 Testing OffGrid Embeddings API"
echo "=================================="
echo ""

# Test 1: Health check
echo "1️⃣ Testing health endpoint..."
curl -s "$BASE_URL/health" | jq -r '.status' | grep -q "healthy" && echo "✅ Server is healthy" || echo "❌ Server health check failed"
echo ""

# Test 2: List models
echo "2️⃣ Checking available embedding models..."
curl -s "$BASE_URL/v1/models" | jq -r '.data[] | select(.id | test("embed|minilm|bge|e5|nomic")) | .id' | head -5
echo ""

# Test 3: Generate embedding for single text
echo "3️⃣ Testing single text embedding..."
RESPONSE=$(curl -s -X POST "$BASE_URL/v1/embeddings" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test-model",
    "input": "Hello, world!"
  }')

# Check if response has embeddings
if echo "$RESPONSE" | jq -e '.data[0].embedding' > /dev/null 2>&1; then
  DIMS=$(echo "$RESPONSE" | jq -r '.data[0].embedding | length')
  echo "✅ Embedding generated: $DIMS dimensions"
  echo "   First 5 values: $(echo "$RESPONSE" | jq -r '.data[0].embedding[:5]')"
else
  echo "❌ Failed to generate embedding"
  echo "Response: $RESPONSE"
fi
echo ""

# Test 4: Batch embeddings
echo "4️⃣ Testing batch embeddings..."
RESPONSE=$(curl -s -X POST "$BASE_URL/v1/embeddings" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test-model",
    "input": ["First text", "Second text", "Third text"]
  }')

if echo "$RESPONSE" | jq -e '.data | length' > /dev/null 2>&1; then
  COUNT=$(echo "$RESPONSE" | jq -r '.data | length')
  echo "✅ Batch embeddings generated: $COUNT embeddings"
  TOTAL_TOKENS=$(echo "$RESPONSE" | jq -r '.usage.total_tokens')
  echo "   Total tokens: $TOTAL_TOKENS"
else
  echo "❌ Failed to generate batch embeddings"
  echo "Response: $RESPONSE"
fi
echo ""

# Test 5: Error handling
echo "5️⃣ Testing error handling..."
RESPONSE=$(curl -s -X POST "$BASE_URL/v1/embeddings" \
  -H "Content-Type: application/json" \
  -d '{
    "input": "Missing model field"
  }')

if echo "$RESPONSE" | jq -e '.error' > /dev/null 2>&1; then
  ERROR_MSG=$(echo "$RESPONSE" | jq -r '.error.message // .error')
  echo "✅ Proper error handling: $ERROR_MSG"
else
  echo "❌ Expected error response"
fi
echo ""

echo "=================================="
echo "✨ Embeddings API test complete!"
