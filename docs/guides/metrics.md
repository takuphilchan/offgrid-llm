# Metrics & Monitoring

OffGrid LLM provides comprehensive metrics for monitoring system health, performance, and usage.

## Accessing Metrics

### Prometheus Format

```bash
curl http://localhost:11611/metrics
```

Returns metrics in Prometheus format, suitable for scraping by Prometheus, Grafana, or other monitoring tools.

### System Stats API

```bash
curl http://localhost:11611/v1/system/stats
```

Returns real-time system statistics in JSON format:

```json
{
  "cpu_percent": 2.5,
  "memory_bytes": 52428800,
  "memory_total": 8589934592,
  "heap_alloc": 45000000,
  "goroutines": 15,
  "gc_cycles": 42,
  "models_loaded": 1,
  "rag_documents": 5,
  "active_sessions": 3,
  "total_users": 2,
  "admin_users": 1,
  "uptime_seconds": 3600,
  "requests_total": 150,
  "websocket_connections": 2,
  "tokens_generated": 50000,
  "errors_total": 3,
  "avg_latency_ms": 125.5
}
```

### UI Dashboard

In multi-user mode (`OFFGRID_MULTI_USER=true`), access the **Metrics** tab in the web UI for:
- Real-time request counts
- Average latency
- Token generation stats
- Error tracking
- Resource usage (CPU, Memory, Disk)
- Active connections
- Raw Prometheus metrics viewer

## Available Metrics

### Request Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `offgrid_requests_total` | Counter | Total HTTP requests by method, path, status |
| `offgrid_request_duration_seconds` | Histogram | Request duration by method, path |
| `offgrid_requests_in_flight` | Gauge | Currently processing requests |

### LLM Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `offgrid_tokens_input_total` | Counter | Total input tokens processed |
| `offgrid_tokens_output_total` | Counter | Total output tokens generated |
| `offgrid_tokens_per_request` | Histogram | Tokens per request distribution |
| `offgrid_generation_duration_seconds` | Histogram | Time to generate responses |
| `offgrid_tokens_per_second` | Histogram | Token generation speed |

### Model Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `offgrid_model_loaded` | Gauge | Whether a model is currently loaded (0/1) |
| `offgrid_model_load_time_seconds` | Histogram | Time to load models |

### RAG Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `offgrid_rag_queries_total` | Counter | Total RAG queries |
| `offgrid_rag_query_duration_seconds` | Histogram | RAG query duration |
| `offgrid_rag_documents_total` | Gauge | Number of indexed documents |
| `offgrid_rag_chunks_total` | Gauge | Number of document chunks |
| `offgrid_rag_embeddings_total` | Counter | Total embeddings generated |

### Session Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `offgrid_active_sessions` | Gauge | Currently active sessions |
| `offgrid_sessions_created_total` | Counter | Total sessions created |

### User Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `offgrid_active_users` | Gauge | Users active in last 24 hours |
| `offgrid_total_users` | Gauge | Total registered users |
| `offgrid_quota_exceeded_total` | Counter | Quota limit violations |

### Error Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `offgrid_errors_total` | Counter | Total errors by method, path, status |

### Resource Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `offgrid_memory_usage_bytes` | Gauge | Current memory usage |
| `offgrid_cpu_usage_percent` | Gauge | Current CPU usage |
| `offgrid_disk_usage_bytes` | Gauge | Disk space used by models |

### WebSocket Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `offgrid_websocket_connections` | Gauge | Active WebSocket connections |
| `offgrid_websocket_messages_total` | Counter | Total WebSocket messages |

## Prometheus Integration

### Example Prometheus Config

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'offgrid'
    static_configs:
      - targets: ['localhost:11611']
    scrape_interval: 15s
    metrics_path: '/metrics'
```

### Example Queries

```promql
# Request rate per second
rate(offgrid_requests_total[5m])

# Average request latency
histogram_quantile(0.95, rate(offgrid_request_duration_seconds_bucket[5m]))

# Token generation rate
rate(offgrid_tokens_output_total[5m])

# Error rate
rate(offgrid_errors_total[5m])
```

## Grafana Dashboard

You can create a Grafana dashboard using these metrics. Key panels to include:

1. **Request Rate** - `rate(offgrid_requests_total[5m])`
2. **Latency P95** - `histogram_quantile(0.95, rate(offgrid_request_duration_seconds_bucket[5m]))`
3. **Tokens/sec** - `rate(offgrid_tokens_output_total[5m])`
4. **Active Sessions** - `offgrid_active_sessions`
5. **Memory Usage** - `offgrid_memory_usage_bytes`
6. **Model Status** - `offgrid_model_loaded`

## Alerting

Example alert rules:

```yaml
groups:
  - name: offgrid
    rules:
      - alert: HighErrorRate
        expr: rate(offgrid_errors_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate on OffGrid"
          
      - alert: ModelNotLoaded
        expr: offgrid_model_loaded == 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "No model loaded"
          
      - alert: HighMemoryUsage
        expr: offgrid_memory_usage_bytes > 4294967296
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Memory usage above 4GB"
```

## Health Checks

### Liveness Probe

```bash
curl http://localhost:11611/health
# Returns: {"status":"ok"} with 200 OK
```

### Readiness Probe

```bash
curl http://localhost:11611/ready
# Returns: {"status":"ready"} with 200 OK when ready to serve
```

### Kubernetes Probes

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 11611
  initialDelaySeconds: 10
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /ready
    port: 11611
  initialDelaySeconds: 5
  periodSeconds: 5
```

## Best Practices

1. **Set up alerts** for error rates and model availability
2. **Monitor memory** usage to prevent OOM issues
3. **Track token rates** to understand usage patterns
4. **Use dashboards** for at-a-glance status
5. **Retain metrics** for trend analysis
