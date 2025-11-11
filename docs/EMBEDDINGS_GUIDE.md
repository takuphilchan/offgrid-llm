# Embedding Models Guide

This guide explains how to use embedding models with OffGrid LLM for semantic search, document similarity, and RAG (Retrieval Augmented Generation).

## 🎯 What are Embeddings?

Embeddings are vector representations of text that capture semantic meaning. They enable:
- **Semantic Search**: Find similar documents even with different words
- **Document Q&A**: Search knowledge bases, manuals, logs
- **RAG**: Enhance LLM responses with relevant context
- **Deduplication**: Find similar/duplicate content
- **Clustering**: Group related texts together

## 🚀 Quick Start

### 1. Download an Embedding Model

```bash
# List available embedding models
offgrid models list --filter embedding

# Download a lightweight model (42MB, 384 dimensions)
offgrid download all-minilm-l6-v2

# Or a more powerful model (262MB, 768 dimensions)
offgrid download nomic-embed-text-v1
```

### 2. Generate Embeddings via API

```bash
curl -X POST http://localhost:11611/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "model": "all-minilm-l6-v2",
    "input": "Hello world"
  }'
```

### 3. Use in Python

```python
import requests

def get_embedding(text, model="all-minilm-l6-v2"):
    response = requests.post(
        "http://localhost:11611/v1/embeddings",
        json={"model": model, "input": text}
    )
    return response.json()["data"][0]["embedding"]

# Single text
embedding = get_embedding("How do I reset my password?")
print(f"Embedding dimensions: {len(embedding)}")

# Batch processing
texts = [
    "How do I reset my password?",
    "Forgot password recovery steps",
    "What is the weather today?"
]

response = requests.post(
    "http://localhost:11611/v1/embeddings",
    json={"model": "all-minilm-l6-v2", "input": texts}
)

for i, embedding_data in enumerate(response.json()["data"]):
    print(f"Text {i}: {len(embedding_data['embedding'])} dimensions")
```

---

## 📋 Available Embedding Models

### all-MiniLM-L6-v2 (Recommended for beginners)
- **Size**: 42MB (F16), 23MB (Q8_0)
- **Dimensions**: 384
- **Best for**: General semantic search, lightweight deployments
- **RAM**: 1GB minimum
- **Performance**: ~100-500 texts/second (CPU)

```bash
offgrid download all-minilm-l6-v2
```

### BGE Small EN v1.5 (High quality)
- **Size**: 64MB
- **Dimensions**: 384
- **Best for**: Retrieval, RAG applications, high accuracy
- **RAM**: 1GB minimum
- **Performance**: ~50-300 texts/second (CPU)

```bash
offgrid download bge-small-en-v1.5
```

### Nomic Embed Text v1 (Most powerful)
- **Size**: 262MB
- **Dimensions**: 768
- **Best for**: Long documents, high-accuracy retrieval
- **RAM**: 2GB minimum
- **Performance**: ~20-100 texts/second (CPU)
- **Context**: Up to 8192 tokens

```bash
offgrid download nomic-embed-text-v1
```

### E5 Small v2 (Multilingual)
- **Size**: 64MB
- **Dimensions**: 384
- **Best for**: Non-English text, multilingual search
- **RAM**: 1GB minimum
- **Languages**: 100+ languages

```bash
offgrid download e5-small-v2
```

---

## 🔌 API Reference

### Endpoint

```
POST /v1/embeddings
```

### Request Format

```json
{
  "model": "all-minilm-l6-v2",
  "input": "string or array of strings",
  "encoding_format": "float",  // Optional: "float" (default) or "base64"
  "user": "optional-user-id"
}
```

### Response Format

```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "embedding": [0.123, -0.456, ...],  // 384 or 768 floats
      "index": 0
    }
  ],
  "model": "all-minilm-l6-v2",
  "usage": {
    "prompt_tokens": 4,
    "total_tokens": 4
  }
}
```

### Batch Processing

```json
{
  "model": "all-minilm-l6-v2",
  "input": [
    "First document",
    "Second document",
    "Third document"
  ]
}
```

Response will contain embeddings in the same order as input.

---

## 💡 Use Cases

### 1. Document Similarity

```python
import numpy as np

def cosine_similarity(vec1, vec2):
    return np.dot(vec1, vec2) / (np.linalg.norm(vec1) * np.linalg.norm(vec2))

# Get embeddings
doc1_emb = get_embedding("Ship engine maintenance manual chapter 5")
doc2_emb = get_embedding("Marine diesel engine service procedures")
doc3_emb = get_embedding("Weather forecast for tomorrow")

# Calculate similarity
sim_1_2 = cosine_similarity(doc1_emb, doc2_emb)
sim_1_3 = cosine_similarity(doc1_emb, doc3_emb)

print(f"Doc1 vs Doc2: {sim_1_2:.3f}")  # High similarity (same topic)
print(f"Doc1 vs Doc3: {sim_1_3:.3f}")  # Low similarity (different topics)
```

### 2. Semantic Search (Offline Knowledge Base)

```python
import json

# Build knowledge base (one-time)
documents = [
    "Engine oil change interval: every 250 hours",
    "Fuel filter replacement: every 500 hours",
    "Battery maintenance: check monthly",
    # ... more documents
]

# Embed all documents
embeddings_db = []
for doc in documents:
    emb = get_embedding(doc)
    embeddings_db.append({"text": doc, "embedding": emb})

# Save to disk for offline use
with open("/data/knowledge_base.json", "w") as f:
    json.dump(embeddings_db, f)

# Search function
def search(query, top_k=5):
    query_emb = get_embedding(query)
    
    # Calculate similarities
    results = []
    for item in embeddings_db:
        score = cosine_similarity(query_emb, item["embedding"])
        results.append({"text": item["text"], "score": score})
    
    # Sort by score
    results.sort(key=lambda x: x["score"], reverse=True)
    return results[:top_k]

# Use it
results = search("How often should I change oil?")
for r in results:
    print(f"{r['score']:.3f}: {r['text']}")
```

### 3. RAG (Retrieval Augmented Generation)

```python
def rag_query(question, context_docs, llm_model="llama-2-7b-chat"):
    # 1. Find relevant context
    relevant = search(question, top_k=3)
    context = "\n".join([r["text"] for r in relevant])
    
    # 2. Build prompt with context
    prompt = f"""Context:
{context}

Question: {question}

Answer based only on the context above:"""
    
    # 3. Get LLM response
    response = requests.post(
        "http://localhost:11611/v1/chat/completions",
        json={
            "model": llm_model,
            "messages": [{"role": "user", "content": prompt}]
        }
    )
    
    return response.json()["choices"][0]["message"]["content"]

# Use it
answer = rag_query("When should I replace the fuel filter?", knowledge_base)
print(answer)
```

---

## ⚙️ Configuration

### GPU Acceleration

Embedding models can use GPU acceleration:

```python
# Models automatically use GPU if available
# To force CPU-only:
response = requests.post(
    "http://localhost:11611/v1/embeddings",
    json={
        "model": "all-minilm-l6-v2",
        "input": text,
        # GPU layers controlled server-side
    }
)
```

### Batch Size

The server processes embeddings in batches of 32 by default. For large batches, split into chunks:

```python
def embed_large_batch(texts, model="all-minilm-l6-v2", batch_size=32):
    all_embeddings = []
    
    for i in range(0, len(texts), batch_size):
        batch = texts[i:i+batch_size]
        response = requests.post(
            "http://localhost:11611/v1/embeddings",
            json={"model": model, "input": batch}
        )
        all_embeddings.extend([d["embedding"] for d in response.json()["data"]])
    
    return all_embeddings
```

---

## 🔧 Performance Tips

### 1. Choose the Right Model

- **Low RAM / High throughput**: `all-minilm-l6-v2` (42MB)
- **Best accuracy**: `nomic-embed-text-v1` (262MB)
- **Multilingual**: `e5-small-v2` (64MB)

### 2. Batch Your Requests

```python
# SLOW: One request per text
for text in texts:
    emb = get_embedding(text)  # Many API calls

# FAST: Batch requests
embeddings = get_embedding(texts)  # Single API call
```

### 3. Cache Embeddings

```python
import hashlib
import json

cache = {}

def get_embedding_cached(text, model="all-minilm-l6-v2"):
    # Create cache key
    key = hashlib.md5(f"{model}:{text}".encode()).hexdigest()
    
    if key in cache:
        return cache[key]
    
    # Generate embedding
    emb = get_embedding(text, model)
    cache[key] = emb
    return emb
```

### 4. Pre-compute Document Embeddings

For a fixed document set (manuals, logs, etc.), embed once and save:

```python
# One-time embedding
docs_with_embeddings = []
for doc in documents:
    docs_with_embeddings.append({
        "text": doc,
        "embedding": get_embedding(doc)
    })

# Save to disk
with open("embedded_docs.json", "w") as f:
    json.dump(docs_with_embeddings, f)

# Later: load from disk (no re-embedding needed)
with open("embedded_docs.json") as f:
    embedded_docs = json.load(f)
```

---

## 📦 Offline Deployment

### USB/SD Card Distribution

```bash
# On internet-connected machine:
offgrid download all-minilm-l6-v2
offgrid models export all-minilm-l6-v2 /media/usb/

# Pre-compute embeddings for your knowledge base
python embed_documents.py --input docs/ --output /media/usb/embedded_docs.json

# On offline machine:
offgrid models import /media/usb/all-minilm-l6-v2.gguf

# Use pre-computed embeddings
cp /media/usb/embedded_docs.json /var/lib/offgrid/data/
```

### P2P Distribution

```bash
# Machine with model shares over local network
offgrid p2p serve

# Other machines download
offgrid p2p download all-minilm-l6-v2
```

---

## 🐛 Troubleshooting

### Model not found

```bash
# List downloaded models
offgrid models list

# Download if needed
offgrid download all-minilm-l6-v2
```

### Out of memory

Use a smaller model:
```bash
offgrid download all-minilm-l6-v2  # 42MB instead of 262MB
```

Or use quantized version:
```bash
offgrid download all-minilm-l6-v2 --quantization Q8_0  # 23MB
```

### Slow performance

1. **Use GPU if available** (automatic)
2. **Batch your requests** (up to 32 at once)
3. **Cache embeddings** for repeated texts
4. **Use smaller model** for faster inference

---

## 🔗 Integration Examples

### Node.js

```javascript
const axios = require('axios');

async function getEmbedding(text) {
  const response = await axios.post('http://localhost:11611/v1/embeddings', {
    model: 'all-minilm-l6-v2',
    input: text
  });
  return response.data.data[0].embedding;
}
```

### Go

```go
import (
    "github.com/takuphilchan/offgrid-llm/pkg/api"
)

embedding, err := client.CreateEmbedding(ctx, &api.EmbeddingRequest{
    Model: "all-minilm-l6-v2",
    Input: "Hello world",
})
```

### Curl

```bash
curl -X POST http://localhost:11611/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{"model":"all-minilm-l6-v2","input":"test"}'
```

---

## 📚 Further Reading

- [Sentence Transformers Documentation](https://www.sbert.net/)
- [Retrieval Augmented Generation (RAG)](https://arxiv.org/abs/2005.11401)
- [Cosine Similarity Explained](https://en.wikipedia.org/wiki/Cosine_similarity)
- [Vector Search Algorithms](https://www.pinecone.io/learn/vector-search/)

---

## 💬 Support

- **Issues**: https://github.com/takuphilchan/offgrid-llm/issues
- **Discussions**: https://github.com/takuphilchan/offgrid-llm/discussions
- **Documentation**: https://github.com/takuphilchan/offgrid-llm/tree/main/docs
