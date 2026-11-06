# Embedding Models Guide

This guide explains how to use embedding models with OffGrid for semantic search, document similarity, and RAG (Retrieval Augmented Generation).

In the development Chat composer, **Context & response** contains Knowledge
readiness and the explicit retrieval switch. **Set up Knowledge** can take you to
this setup without discarding your draft. **Return to your draft** rechecks
readiness but does not turn retrieval on or send the question. A previously chosen
retrieval switch is not silently cleared by a failed readiness check. The service
searches the permitted knowledge base; setup does not create a document-only scope.
The return shortcut is held only for the current account/workspace until reload.
Drafts are separate for each account and identified workspace. Older unassigned
drafts require **Restore previous drafts here**; that copy-only operation preserves
originals and newer drafts, and does not enable Knowledge or send a question.

## What are Embeddings?

Embeddings are vector representations of text that capture semantic meaning. They enable:
- **Semantic Search**: Find similar documents even with different words
- **Document Q&A**: Search knowledge bases, manuals, logs
- **RAG**: Enhance LLM responses with relevant context
- **Deduplication**: Find similar/duplicate content
- **Clustering**: Group related texts together

## Quick Start

### Web UI (recommended)

1. Start OffGrid and open `http://127.0.0.1:11611/ui/#/knowledge`.
2. Review the proposed local search model. OffGrid preserves the configured
   model or prefers an installed embedding model. **Change search model** opens
   optional choices; opening this page does not download or activate anything.
3. Choose **Enable** for an installed model, or **Install and enable** after
   reviewing the download size. OffGrid tracks the download and activates search
   when the model is ready.
4. Add up to 20 documents in one selection. Each file shows its own result;
   unavailable formats are identified before transfer. **Document formats**
   explains local reader availability, including missing PDF/OCR support.
5. Inspect a source, then choose **Ask using Knowledge**, or select particular
   ready documents and ask about those sources.

Uploaded files are saved before extraction. **Unfinished imports** shows saved
files that need attention when you return. Resolve the reported problem, then
choose **Try again** without uploading again, or remove the saved import. Removing
an import does not delete a document already published in Knowledge.

Leaving the page stops dispatching further files; the current request may finish.
A lost response stops the batch and shows an unknown result. Check the saved
import and library before uploading again. Processing is request-owned: a service
restart reports interrupted work and waits for an explicit retry. It does not
automatically replay imports. Clearing recent results only dismisses that panel.
Older services without durable imports retain their existing upload behavior.

Knowledge foregrounds your documents, index readiness and **View source**.
**Manage Knowledge** contains the active embedding model and disabling retrieval;
each document's **Manage document** contains reindexing, chunk details and deletion.
These controls stay available according to your permissions, including source
inspection and deletion while retrieval is disabled.

**Ask using Knowledge** opens Chat with retrieval explicitly selected and preserves
the existing draft. It never sends the draft. Retrieval searches the permitted
knowledge base, not just a selected document; the composer states that scope and
checks readiness again. Ordinary navigation to Chat does not enable retrieval.

### CLI

```bash
# List available embedding models
offgrid models list --filter embedding

# Download a lightweight model (42MB, 384 dimensions)
offgrid download all-minilm-l6-v2
offgrid kb enable all-minilm-l6-v2

# Or use the multilingual model (about 411MB, 1024 dimensions)
offgrid download bge-m3
offgrid kb enable bge-m3

# Index retained source files
offgrid kb add ./docs/manual.pdf
offgrid kb status
```

### Document extraction limits

Import quality depends on extraction as well as embeddings. PDF extraction now
requires the app-local `offgrid-document-worker`, with a 30-second deadline and
bounded output. Missing workers report `document_parser_unavailable`; the service
does not run a program from PATH or silently fall back to regex extraction.
Worker packaging is still under qualification. Scans and mixed visual content
need the local OCR worker and a verified language pack. The current worker supports
the pinned English pack; language setup is explicit and never downloads silently.
Missing language data and partial recognition remain actionable import states.
See the [document format matrix](document-formats.md) for tested coverage and limits.
Keep original documents and inspect extracted content before relying on answers.

Successful new file imports save their original bytes with the document and
index. **View source → Download original file** retrieves those bytes, verified
against their digest. This uses the Knowledge workspace's existing read access;
it is not a private Chat attachment. Older documents may have extracted text
without an original, which the source viewer states explicitly. Reindexing keeps
the original unchanged; deleting the document deletes its retained original too.
External backups keep their own retention policy and are not erased by this action.

Original files and pending uploads share a 2 GiB Knowledge storage limit, in
addition to the per-file limit below. A failed storage transaction does not
publish the document. Pending imports are visible only to the authenticated
uploader with Knowledge management permission. Published documents retain the
existing workspace access policy; private collection policies remain unfinished.
The import list returns the most recent 100 records, with a workspace limit of
4,096 records. Remove unneeded import records when that limit is reached.

XLSX extraction follows workbook sheet order and includes sheet names and cell
references, inline/shared text, formulas and stored cached values, up to 100,000
populated cells per sheet. Formulas are
not recalculated; numeric/date formatting, charts, images and macros are not
interpreted. PPTX follows presentation slide order and extracts text, not rendered
diagrams. Legacy binary XLS is explicitly unsupported; convert it to XLSX first.

`offgrid kb status --json` (also `GET /v1/rag/status`) includes `document_formats`,
independently of embedding setup. `available` means a built-in extractor exists;
`installed_unchecked` means the packaged PDF worker is present, not that it has
passed runtime or document checks. `unavailable` identifies a missing dedicated
adapter. Qualification remains `unverified` until the applicable packaged tests
pass. OCR availability and installed languages are reported separately; model
vision support does not provide document OCR. Legacy DOC/XLS/PPT, ODF, email and
image documents are not silently decoded as plain text.

New extraction results retain original-file digests and parser revisions.
Spreadsheet sheets retain their names and order, and slide units retain their
presentation order. Flat text uses a section locator, not invented page numbers.
New document uploads now retain these boundaries through chunking, database
reload and explicit reindexing. Chunks do not cross units; their offsets are
UTF-8 byte offsets into retained extracted text and permit exact read-back.
Search locators include the source kind/index, original digest and parser
revision. New XLSX citations also identify the actual cells intersecting each
excerpt, including after context truncation. **View source** on a saved citation
checks its revision, digest, byte range and content hash before displaying the
exact passage. A stale citation cannot silently open an unrelated document prefix.
Existing indexes are not rewritten or given invented page or cell locators.
This establishes mechanical source fidelity, not model-answer accuracy, OCR
quality or complete enterprise qualification.

When a mixed PDF cannot be fully read, **Unfinished imports → View source** shows
the retained pages and identifies those needing attention. This is review only:
partial text is not presented as a complete indexed document. Import the required
language pack or resolve the reported issue, then choose **Try again** explicitly.
CLI users can inspect the same result with `offgrid kb review-import ID`.

Document uploads are limited to 50 MiB per file, extracted text to 16 MiB, and
expanded Office archives to 64 MiB / 10,000 entries. Invalid, encrypted or
unsupported documents must not be treated as successfully indexed. See the
[document evidence](../quality/evidence/document-ingestion-2026-11-01.md) for
tested behavior and remaining enterprise-readiness work.

### Generate embeddings directly

```bash
curl -X POST http://localhost:11611/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "model": "all-minilm-l6-v2",
    "input": "Hello world"
  }'
```

### Use in Python

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

##  Available Embedding Models

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

### BGE M3 (Multilingual)
- **Size**: about 411MB (Q4_K_M)
- **Dimensions**: 1024
- **Best for**: Multilingual retrieval and longer knowledge sources
- **RAM**: 2GB minimum
- **Languages**: Multilingual

```bash
offgrid download bge-m3
offgrid kb enable bge-m3
```

---

## API Reference

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

## Use Cases

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

## Configuration

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

## Performance Tips

### 1. Choose the Right Model

- **Low RAM / High throughput**: `all-minilm-l6-v2` (42MB)
- **Best accuracy**: `nomic-embed-text-v1` (262MB)
- **Multilingual**: `bge-m3` (about 411MB)

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

## Offline Deployment

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

## Troubleshooting

### Model not found

```bash
# List downloaded models
offgrid models list

# Download if needed
offgrid download all-minilm-l6-v2
```

If a catalog download returns HTTP 404, upgrade OffGrid or refresh the
container image before retrying. Older catalogs referenced repositories that
did not publish the expected GGUF artifacts. A partial download is retained
for resumable failures; a 404 is a source-catalog error, not a retryable partial
transfer.

### Out of memory

Use a smaller model:
```bash
offgrid download all-minilm-l6-v2  # 42MB instead of 262MB
```

### Slow performance

1. **Use GPU if available** (automatic)
2. **Batch your requests** (up to 32 at once)
3. **Cache embeddings** for repeated texts
4. **Use smaller model** for faster inference

---

## Integration Examples

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

## Further Reading

- [Sentence Transformers Documentation](https://www.sbert.net/)
- [Retrieval Augmented Generation (RAG)](https://arxiv.org/abs/2005.11401)
- [Cosine Similarity Explained](https://en.wikipedia.org/wiki/Cosine_similarity)
- [Vector Search Algorithms](https://www.pinecone.io/learn/vector-search/)

---

## Support

- **Issues**: https://github.com/takuphilchan/offgrid-llm/issues
- **Discussions**: https://github.com/takuphilchan/offgrid-llm/discussions
- **Documentation**: https://github.com/takuphilchan/offgrid-llm/tree/main/docs
