# API Documentation

OffGrid LLM provides an OpenAI-compatible REST API for running language models offline.

## Python Library

The easiest way to use OffGrid LLM from Python:

```bash
pip install offgrid
```

```python
import offgrid

# Connect to server
client = offgrid.Client()  # localhost:11611

# Or custom server
client = offgrid.Client(host="http://192.168.1.100:11611")

# Chat
response = client.chat("Hello!")
print(response)

# Specify model
response = client.chat("Hello!", model="Llama-3.2-3B-Instruct")

# Streaming
for chunk in client.chat("Tell me a story", stream=True):
    print(chunk, end="", flush=True)

# Model management
client.models.download("repo/model", "file.gguf")
client.kb.add("document.txt")
```

See [Python Library Documentation](../python/README.md) for full API reference.

---

## REST API

### Base URL

```
http://localhost:11611
```

## Authentication

Currently, no authentication is required. This will be added in future versions for multi-user deployments.

## Endpoints

### Health Check

Check if the server is running and healthy.

**Endpoint:** `GET /health`

**Response:**
```json
{
  "status": "healthy"
}
```

---

### Root Information

Get basic server information.

**Endpoint:** `GET /`

**Response:**
```json
{
  "name": "OffGrid LLM",
  "version": "0.1.6",
  "status": "running"
}
```

---

### List Models

List all available models.

**Endpoint:** `GET /v1/models`

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "llama-2-7b.Q4_K_M",
      "object": "model",
      "created": 1699286400,
      "owned_by": "offgrid-llm"
    }
  ]
}
```

---

### Chat Completions

Create a chat completion (multi-turn conversation).

**Endpoint:** `POST /v1/chat/completions`

**Request Body:**
```json
{
  "model": "llama-2-7b.Q4_K_M",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "What is machine learning?"
    }
  ],
  "temperature": 0.7,
  "max_tokens": 150,
  "top_p": 0.95,
  "frequency_penalty": 0.0,
  "presence_penalty": 0.0
}
```

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `model` | string | Yes | - | ID of the model to use |
| `messages` | array | Yes | - | Array of message objects |
| `temperature` | float | No | 0.7 | Sampling temperature (0.0 to 2.0) |
| `max_tokens` | integer | No | - | Maximum tokens to generate |
| `top_p` | float | No | 0.95 | Nucleus sampling threshold |
| `frequency_penalty` | float | No | 0.0 | Penalize frequent tokens (-2.0 to 2.0) |
| `presence_penalty` | float | No | 0.0 | Penalize new tokens (-2.0 to 2.0) |
| `stop` | array | No | - | Up to 4 sequences where generation stops |
| `stream` | boolean | No | false | Stream responses (not yet implemented) |

**Message Object:**
```json
{
  "role": "user|assistant|system",
  "content": "Message content"
}
```

**Response:**
```json
{
  "id": "chatcmpl-1699286400",
  "object": "chat.completion",
  "created": 1699286400,
  "model": "llama-2-7b.Q4_K_M",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Machine learning is a subset of artificial intelligence..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 150,
    "total_tokens": 175
  }
}
```

**Finish Reasons:**
- `stop`: Natural completion
- `length`: Max tokens reached
- `content_filter`: Content filtered (future)

---

### Text Completions

Create a text completion (single prompt).

**Endpoint:** `POST /v1/completions`

**Request Body:**
```json
{
  "model": "llama-2-7b.Q4_K_M",
  "prompt": "Once upon a time",
  "temperature": 0.7,
  "max_tokens": 100
}
```

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `model` | string | Yes | - | ID of the model to use |
| `prompt` | string | Yes | - | Text prompt to complete |
| `temperature` | float | No | 0.7 | Sampling temperature (0.0 to 2.0) |
| `max_tokens` | integer | No | - | Maximum tokens to generate |
| `top_p` | float | No | 0.95 | Nucleus sampling threshold |
| `frequency_penalty` | float | No | 0.0 | Penalize frequent tokens |
| `presence_penalty` | float | No | 0.0 | Penalize new tokens |
| `stop` | array | No | - | Stop sequences |

**Response:**
```json
{
  "id": "cmpl-1699286400",
  "object": "text_completion",
  "created": 1699286400,
  "model": "llama-2-7b.Q4_K_M",
  "choices": [
    {
      "index": 0,
      "text": ", there was a young programmer who...",
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 4,
    "completion_tokens": 100,
    "total_tokens": 104
  }
}
```

---

### Embeddings

Generate vector embeddings for text.

**Endpoint:** `POST /v1/embeddings`

**Request Body:**
```json
{
  "model": "bge-m3-Q4_K_M",
  "input": ["Hello world", "How are you?"]
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | Yes | ID of the embedding model |
| `input` | string or array | Yes | Text or array of texts to embed |

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "embedding": [0.0023, -0.0134, ...],
      "index": 0
    },
    {
      "object": "embedding",
      "embedding": [0.0045, -0.0089, ...],
      "index": 1
    }
  ],
  "model": "bge-m3-Q4_K_M",
  "usage": {
    "prompt_tokens": 8,
    "total_tokens": 8
  }
}
```

---

## Error Responses

All errors follow a consistent format:

```json
{
  "error": {
    "message": "Model not found: invalid-model",
    "type": "api_error",
    "code": "model_not_found"
  }
}
```

**HTTP Status Codes:**
- `200`: Success
- `400`: Bad Request (invalid parameters)
- `404`: Not Found (model doesn't exist)
- `405`: Method Not Allowed
- `500`: Internal Server Error

---

## Usage Examples

### cURL

```bash
# List models
curl http://localhost:11611/v1/models

# Chat completion
curl http://localhost:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-2-7b.Q4_K_M",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'

# Text completion
curl http://localhost:11611/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-2-7b.Q4_K_M",
    "prompt": "The future of AI is"
  }'
```

### Python (requests)

```python
import requests

BASE_URL = "http://localhost:11611"

# List models
response = requests.get(f"{BASE_URL}/v1/models")
models = response.json()

# Chat completion
response = requests.post(
    f"{BASE_URL}/v1/chat/completions",
    json={
        "model": "llama-2-7b.Q4_K_M",
        "messages": [
            {"role": "user", "content": "Hello!"}
        ]
    }
)
result = response.json()
print(result["choices"][0]["message"]["content"])
```

### JavaScript (fetch)

```javascript
const BASE_URL = "http://localhost:11611";

// Chat completion
const response = await fetch(`${BASE_URL}/v1/chat/completions`, {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    model: "llama-2-7b.Q4_K_M",
    messages: [
      { role: "user", content: "Hello!" }
    ]
  })
});

const result = await response.json();
console.log(result.choices[0].message.content);
```

---

## OpenAI SDK Compatibility

OffGrid LLM is designed to be compatible with OpenAI client libraries. Simply point them to your local server:

### Python (openai package)

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:11611/v1",
    api_key="not-needed"  # Required by SDK but not used
)

response = client.chat.completions.create(
    model="llama-2-7b.Q4_K_M",
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)

print(response.choices[0].message.content)
```

### Node.js (openai package)

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://localhost:11611/v1',
  apiKey: 'not-needed'
});

const response = await client.chat.completions.create({
  model: 'llama-2-7b.Q4_K_M',
  messages: [
    { role: 'user', content: 'Hello!' }
  ]
});

console.log(response.choices[0].message.content);
```

---

## Configuration

See [Configuration Guide](CONFIGURATION.md) for environment variables and server settings.

## Rate Limiting

Currently, no rate limiting is implemented. This will be added in future versions.

## Future API Endpoints

Coming soon:
- `POST /v1/import` - Import models from USB/SD card
- `GET /v1/stats` - Server statistics and resource usage
- `GET /v1/peers` - List P2P peers (when P2P is enabled)
