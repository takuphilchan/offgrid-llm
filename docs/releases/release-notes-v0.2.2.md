# OffGrid LLM v0.2.2 Release Notes

**Release Date:** January 2026

## Highlights

This release introduces **REST API for sessions**, **comprehensive statistics endpoint**, and an **enhanced Python SDK**.

---

## New Features

### Sessions REST API

New endpoints for managing conversation sessions:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/sessions` | GET | List all sessions |
| `/v1/sessions` | POST | Create new session |
| `/v1/sessions/{name}` | GET | Get session details |
| `/v1/sessions/{name}` | DELETE | Delete session |
| `/v1/sessions/{name}/messages` | POST | Add message to session |

```bash
# Create a new session
curl -X POST http://localhost:11611/v1/sessions \
  -H "Content-Type: application/json" \
  -d '{"name": "my-chat"}'

# Add a message
curl -X POST http://localhost:11611/v1/sessions/my-chat/messages \
  -H "Content-Type: application/json" \
  -d '{"role": "user", "content": "Hello!"}'
```

### Comprehensive Statistics Endpoint

New `/v1/stats` endpoint provides detailed server metrics:

```json
{
  "server": {
    "uptime": "2h15m30s",
    "uptime_seconds": 8130,
    "start_time": "2025-12-03T10:00:00Z",
    "version": "0.2.2",
    "current_model": "phi-3-mini"
  },
  "inference": {
    "models": { ... },
    "aggregate": {
      "total_requests": 1250,
      "total_tokens": 45000,
      "avg_response_ms": 850,
      "model_count": 3
    }
  },
  "system": {
    "os": "linux",
    "arch": "amd64",
    "cpu_cores": 8,
    "total_memory_gb": 32
  },
  "resources": {
    "cpu_usage_percent": 25.5,
    "memory_used_mb": 4096,
    "memory_usage_percent": 12.8
  },
  "cache": { ... },
  "rag": {
    "enabled": true,
    "documents": 15
  }
}
```

---

## Python SDK v0.1.2

The Python SDK has been significantly enhanced with new features:

### API Key Authentication

```python
import offgrid

# With API key authentication
client = offgrid.Client(api_key="your-secret-key")
```

### Session Management

```python
# Access sessions through the client
sessions = client.sessions

# Create a new session
session = sessions.create("my-chat")

# List all sessions
all_sessions = sessions.list()

# Chat with context preservation
response = sessions.chat_with_session("my-chat", "Hello!")
response = sessions.chat_with_session("my-chat", "What did I just say?")

# Add messages manually
sessions.add_message("my-chat", "user", "Custom message")

# Get session details
details = sessions.get("my-chat")

# Delete session
sessions.delete("my-chat")
```

### Automatic Retry Logic

```python
# Built-in retry with exponential backoff
# Automatically retries failed requests up to 3 times
# Delays: 1s, 2s, 4s between retries
response = client.chat("Hello!")  # Retries automatically on failure
```

### Server Statistics

```python
# Get comprehensive server statistics
stats = client.stats()
print(f"Uptime: {stats['server']['uptime']}")
print(f"Total requests: {stats['inference']['aggregate']['total_requests']}")
print(f"CPU usage: {stats['resources']['cpu_usage_percent']}%")
```

### Installation

```bash
pip install offgrid --upgrade
```

---

## Breaking Changes

None in this release.

---

## Bug Fixes

- Fixed model cache statistics reporting
- Improved error handling in session management
- Enhanced request validation for chat completions

---

## Upgrade Guide

### From v0.2.1

1. Update the server:
   ```bash
   # Rebuild from source
   cd offgrid-llm
   go build -o offgrid ./cmd/offgrid
   ```

2. Update Python SDK:
   ```bash
   pip install offgrid --upgrade
   ```

3. (Optional) Enable API authentication:
   ```bash
   export OFFGRID_API_KEY="your-secret-key"
   ```

---

## What's Next

- Enhanced PDF parsing with better text extraction
- WebSocket support for real-time streaming
- Multi-user session isolation
- Rate limiting and usage quotas
- Prometheus metrics export

---

## Contributors

Thanks to all contributors who made this release possible!

---

## Full Changelog

See [GitHub Releases](https://github.com/takuphilchan/offgrid-llm/releases/tag/v0.2.2) for the complete list of changes.
