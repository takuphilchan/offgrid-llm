# Release Notes - v0.2.0

**Release Date:** November 29, 2025

## Overview

Version 0.2.0 introduces a full RAG (Retrieval-Augmented Generation) system with Knowledge Base management, a streamlined UI with consolidated navigation, and CLI feature parity for knowledge base operations.

---

## What's New

### Knowledge Base (RAG) System

**Document Management**
- Upload and ingest documents directly from the web UI
- Supported formats: `.txt`, `.md`, `.markdown`, `.json`, `.csv`, `.html`, `.htm`
- Smart chunking with configurable overlap for optimal retrieval
- Persistent storage with automatic restore on server restart

**Hybrid Search**
- 70% semantic similarity + 30% keyword matching (BM25)
- MMR (Maximal Marginal Relevance) reranking for diverse results
- Configurable minimum score threshold (default: 0.20)
- Top-K retrieval with relevance scoring

**Chat Integration**
- Auto-retrieval injects relevant context into conversations
- Source citations displayed with each response
- Toggle RAG on/off per conversation
- Works with any loaded embedding model

### UI Improvements

**Consolidated Navigation**
- Reduced from 7 tabs to 5 tabs for cleaner interface
- Chat, Knowledge, Benchmark, Models, Terminal

**Sessions Panel**
- Collapsible drawer within Chat tab
- Quick access to saved conversations
- Session management without leaving chat

**Knowledge Tab Redesign**
- Simplified workflow: select model to auto-enable RAG
- Document upload with drag-and-drop support
- Real-time document list with remove capability
- Developer Tools section (collapsible) for embeddings testing

**New Chat Dialog**
- Three-button horizontal layout: Cancel | Save & New | New Chat
- Option to save current conversation before starting new
- Prevents accidental loss of chat history

### CLI Knowledge Base Commands

New `kb` command (aliases: `knowledge`, `rag`) for terminal-based management:

```bash
offgrid kb status              # Show RAG status and stats
offgrid kb list                # List all indexed documents
offgrid kb add ./document.md   # Ingest a document
offgrid kb search "query"      # Search the knowledge base
offgrid kb remove <id>         # Remove a document by ID
offgrid kb clear               # Clear all documents
```

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/rag/status` | GET/POST | Get RAG status and statistics |
| `/v1/rag/enable` | POST | Enable RAG with embedding model |
| `/v1/rag/disable` | POST | Disable RAG |
| `/v1/documents/ingest` | POST | Add document to knowledge base |
| `/v1/documents/search` | POST | Search documents semantically |

---

## Technical Details

### RAG Architecture

- **Vector Store**: In-memory with cosine similarity
- **Chunking**: 512 tokens with 50-token overlap
- **Scoring**: Hybrid (semantic + BM25 keyword)
- **Reranking**: MMR with lambda=0.7 for diversity
- **Persistence**: JSON storage at `~/.offgrid-llm/models/rag/knowledge_base.json`

### New Files

```
internal/rag/
  chunker.go      # Document chunking logic
  document.go     # Document types and management
  engine.go       # RAG engine with hybrid search
  vectorstore.go  # Vector storage and similarity
internal/server/
  rag_handlers.go # API endpoint handlers
internal/tools/
  executor.go     # Tool execution framework
  manager.go      # Tool registration and dispatch
```

---

## Installation

### Upgrade from v0.1.9

```bash
# Pull latest and rebuild
git pull origin main
go build -o offgrid ./cmd/offgrid
```

### Fresh Install

**Desktop Application**

```bash
# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.sh | bash

# Windows (PowerShell as Admin)
irm https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.ps1 | iex
```

**CLI Only**

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/scripts/install.sh | bash
```

---

## Quick Start with RAG

```bash
# Start the server
offgrid serve

# In another terminal, add documents
offgrid kb add ./docs/manual.md
offgrid kb add ./notes/research.txt

# Check status
offgrid kb status

# Search your knowledge base
offgrid kb search "how to configure settings"
```

Or use the web UI at `http://localhost:11611`:
1. Go to **Knowledge** tab
2. Select an embedding model (RAG enables automatically)
3. Upload documents
4. Return to **Chat** and ask questions about your documents

---

## Breaking Changes

None. This release is backward compatible with v0.1.x configurations.

---

## Known Issues

- Large documents (>1MB) may take several seconds to chunk and index
- Embedding model must support the embedding API endpoint

---

## Contributors

Thanks to all contributors who made this release possible.

---

## Full Changelog

See [GitHub Commits](https://github.com/takuphilchan/offgrid-llm/compare/v0.1.9...v0.2.0) for the complete list of changes.
