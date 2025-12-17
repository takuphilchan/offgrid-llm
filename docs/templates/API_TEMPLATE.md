# API Endpoint Name

> `METHOD /path/to/endpoint`

Brief description of what this endpoint does.

## Request

### URL

```
METHOD /v1/endpoint/{parameter}
```

### Path Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `parameter` | string | Yes | Description of path parameter |

### Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `20` | Maximum items to return |
| `offset` | int | `0` | Pagination offset |
| `filter` | string | `""` | Filter expression |

### Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Content-Type` | Yes | Must be `application/json` |
| `Authorization` | No | Bearer token for authentication |

### Request Body

```json
{
  "field1": "string",
  "field2": 123,
  "field3": true,
  "nested": {
    "subfield": "value"
  },
  "array": ["item1", "item2"]
}
```

#### Body Parameters

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `field1` | string | Yes | Description of field1 |
| `field2` | integer | No | Description of field2 |
| `field3` | boolean | No | Description of field3 |
| `nested.subfield` | string | No | Nested field description |

## Response

### Success Response

**Code:** `200 OK`

```json
{
  "success": true,
  "data": {
    "id": "abc123",
    "created_at": "2024-01-01T00:00:00Z",
    "result": "value"
  },
  "meta": {
    "total": 100,
    "limit": 20,
    "offset": 0
  }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `success` | boolean | Whether the request succeeded |
| `data.id` | string | Unique identifier |
| `data.created_at` | string | ISO 8601 timestamp |
| `meta.total` | integer | Total available items |

### Error Responses

#### 400 Bad Request

```json
{
  "success": false,
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Validation failed",
    "details": [
      {
        "field": "field1",
        "message": "field1 is required"
      }
    ]
  }
}
```

#### 401 Unauthorized

```json
{
  "success": false,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Invalid or missing authentication token"
  }
}
```

#### 404 Not Found

```json
{
  "success": false,
  "error": {
    "code": "NOT_FOUND",
    "message": "Resource not found"
  }
}
```

#### 500 Internal Server Error

```json
{
  "success": false,
  "error": {
    "code": "INTERNAL_ERROR",
    "message": "An unexpected error occurred"
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Request validation failed |
| `UNAUTHORIZED` | 401 | Authentication required |
| `FORBIDDEN` | 403 | Permission denied |
| `NOT_FOUND` | 404 | Resource not found |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Server error |

## Examples

### cURL

```bash
curl -X POST "http://localhost:8080/v1/endpoint/param" \
  -H "Content-Type: application/json" \
  -d '{
    "field1": "value",
    "field2": 123
  }'
```

### Python

```python
import requests

response = requests.post(
    "http://localhost:8080/v1/endpoint/param",
    json={
        "field1": "value",
        "field2": 123
    }
)

data = response.json()
print(data["data"]["id"])
```

### Go

```go
import (
    "bytes"
    "encoding/json"
    "net/http"
)

payload := map[string]interface{}{
    "field1": "value",
    "field2": 123,
}

body, _ := json.Marshal(payload)
resp, err := http.Post(
    "http://localhost:8080/v1/endpoint/param",
    "application/json",
    bytes.NewBuffer(body),
)
```

### JavaScript

```javascript
const response = await fetch('http://localhost:8080/v1/endpoint/param', {
    method: 'POST',
    headers: {
        'Content-Type': 'application/json',
    },
    body: JSON.stringify({
        field1: 'value',
        field2: 123,
    }),
});

const data = await response.json();
console.log(data.data.id);
```

## Rate Limiting

| Limit Type | Value | Window |
|------------|-------|--------|
| Requests | 100 | per minute |
| Burst | 20 | concurrent |

Rate limit headers:
- `X-RateLimit-Limit`: Maximum requests per window
- `X-RateLimit-Remaining`: Remaining requests in window
- `X-RateLimit-Reset`: Unix timestamp when window resets

## Changelog

| Version | Changes |
|---------|---------|
| 0.5.0 | Added `field3` parameter |
| 0.3.0 | Initial release |

## See Also

- [Authentication](./AUTH.md)
- [Error Handling](./ERRORS.md)
- [Rate Limiting](./RATE_LIMITS.md)
