# Audit Logs

OffGrid LLM includes tamper-evident security audit logging for compliance and monitoring.

---

## Overview

Every significant action is logged with:
- Timestamp
- Event type
- User ID
- Details
- HMAC signature for tamper detection
- Chain hash linking to previous event

Logs are stored locally and can be exported for compliance reporting.

---

## Quick Start

```bash
offgrid audit show                    # View recent events
offgrid audit stats                   # Show statistics
offgrid audit verify                  # Check integrity
offgrid audit export-csv report.csv   # Export to CSV
```

---

## CLI Commands

### View Events

```bash
offgrid audit show [flags]
```

| Flag | Description |
|------|-------------|
| `--limit N` | Show last N events (default: 20) |
| `--type TYPE` | Filter by event type |

```bash
offgrid audit show --limit 50
offgrid audit show --type auth
offgrid audit show --type model_load
```

### Statistics

```bash
offgrid audit stats
```

Shows:
- Total events
- Events by type
- First/last event timestamps
- Chain integrity status

### Verify Integrity

```bash
offgrid audit verify
```

Checks the HMAC chain to detect any tampering.

Output:
- `OK` - Chain is intact
- `FAILED` - Tampering detected at specific event

### Export

**CSV format:**
```bash
offgrid audit export-csv /path/to/report.csv
```

**JSON format:**
```bash
offgrid audit export-json /path/to/report.json
```

---

## Event Types

| Type | Description |
|------|-------------|
| `auth` | Authentication attempts |
| `auth_success` | Successful login |
| `auth_failure` | Failed login |
| `model_load` | Model loaded |
| `model_unload` | Model unloaded |
| `inference` | Inference request |
| `config_change` | Configuration changed |
| `user_create` | User created |
| `user_delete` | User deleted |
| `api_key_create` | API key generated |
| `api_key_revoke` | API key revoked |
| `server_start` | Server started |
| `server_stop` | Server stopped |

---

## API

### List Events

```bash
curl "http://localhost:11611/v1/audit?limit=20&type=auth"
```

### Get Statistics

```bash
curl "http://localhost:11611/v1/audit/stats"
```

### Verify Chain

```bash
curl "http://localhost:11611/v1/audit/verify"
```

### Export

```bash
curl "http://localhost:11611/v1/audit/export?format=csv" > report.csv
curl "http://localhost:11611/v1/audit/export?format=json" > report.json
```

---

## Event Structure

Each event contains:

```json
{
  "id": "evt_abc123",
  "timestamp": "2024-01-15T10:30:00Z",
  "type": "model_load",
  "user_id": "user_xyz",
  "details": {
    "model": "llama3",
    "size": "3B"
  },
  "signature": "hmac-sha256:...",
  "chain_hash": "sha256:..."
}
```

---

## Tamper Detection

The audit log uses HMAC-SHA256 chaining:

1. Each event is signed with HMAC
2. Each event includes hash of previous event
3. Chain is verified on startup
4. Manual verification with `offgrid audit verify`

If tampering is detected:
- Verification reports the first broken link
- Event ID and timestamp of the issue
- Recommendation to investigate

---

## Storage

Audit logs are stored in:

```
~/.offgrid-llm/audit/
  events.log          # Main log file
  events.log.1        # Rotated logs
  chain.hash          # Latest chain hash
```

### Log Rotation

Logs rotate when they reach 10MB. Rotated logs are kept for 90 days by default.

Configure in `~/.offgrid-llm/config.yaml`:

```yaml
audit:
  enabled: true
  max_size_mb: 10
  retention_days: 90
  hmac_key: "your-secret-key"
```

---

## Compliance

Audit logs support compliance with:

- **SOC 2** - Access logging, change tracking
- **HIPAA** - Access controls, audit trails
- **GDPR** - Data access logging
- **PCI-DSS** - User activity tracking

### Recommended Practices

1. **Regular exports** - Weekly CSV exports to secure storage
2. **Integrity checks** - Daily `offgrid audit verify`
3. **Offsite backup** - Copy logs to separate system
4. **Access control** - Restrict audit log access
5. **Retention policy** - Keep logs per compliance requirements

---

## Integration

### Syslog

Forward to syslog for centralized logging:

```yaml
audit:
  syslog:
    enabled: true
    address: "localhost:514"
    facility: "local0"
```

### Webhook

Send events to external system:

```yaml
audit:
  webhook:
    enabled: true
    url: "https://your-siem.example.com/events"
    headers:
      Authorization: "Bearer token"
```

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| No events shown | Check if audit is enabled in config |
| Verification fails | Investigate tampering, check disk errors |
| Export fails | Check write permissions, disk space |
| Missing events | Check log rotation, retention settings |

---

## See Also

- [CLI Reference](../reference/cli.md)
- [Multi-User Guide](multi-user.md)
- [Security Best Practices](../advanced/security.md)
