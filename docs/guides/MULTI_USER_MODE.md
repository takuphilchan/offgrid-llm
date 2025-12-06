# Multi-User Mode Guide

OffGrid LLM supports both single-user and multi-user modes. By default, it runs in **single-user mode** for simplicity, making it perfect for local AI workflows without the overhead of user management.

## Quick Start

### Single-User Mode (Default)

When you run OffGrid without any special configuration, it operates in single-user mode:

```bash
# Start server in single-user mode (default)
offgrid serve
```

In single-user mode:
- No login required
- No user management UI
- All features available without authentication
- Perfect for personal/local use

### Multi-User Mode

To enable multi-user features, set the `OFFGRID_MULTI_USER` environment variable:

```bash
# Enable multi-user mode
export OFFGRID_MULTI_USER=true
offgrid serve

# Or inline
OFFGRID_MULTI_USER=true offgrid serve
```

In multi-user mode:
- User management UI available (Users tab)
- Metrics dashboard available (Metrics tab)
- User authentication supported
- API key management per user
- Role-based access control (Admin, User, Viewer, Guest)

## Configuration Options

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `OFFGRID_MULTI_USER` | `false` | Enable multi-user mode |
| `OFFGRID_REQUIRE_AUTH` | `false` | Require authentication for all requests |
| `OFFGRID_GUEST_ACCESS` | `true` | Allow guest access when auth not required |

### Config File

You can also set these in your config file (`~/.offgrid-llm/config.yaml`):

```yaml
multi_user_mode: true
require_auth: false
guest_access: true
```

**Note:** Environment variables take precedence over config file values.

## User Roles

| Role | Permissions |
|------|-------------|
| **Admin** | Full access - manage users, models, RAG, sessions |
| **User** | Standard access - chat, models, RAG, own sessions |
| **Viewer** | Read-only - chat, view models |
| **Guest** | Minimal - chat only |

## CLI Commands

### List Users (Multi-User Mode Required)

```bash
# In single-user mode, this shows a helpful message
offgrid users
# Output: "User management is disabled in single-user mode..."

# In multi-user mode
OFFGRID_MULTI_USER=true offgrid users
```

## API Endpoints

### System Configuration

```bash
# Check current mode
curl http://localhost:11611/v1/system/config
```

Response:
```json
{
  "multi_user_mode": false,
  "require_auth": false,
  "guest_access": true,
  "features": {
    "users": false,
    "metrics": true,
    "agent": true,
    "lora": true
  }
}
```

### User Management (Multi-User Mode)

```bash
# List users
curl http://localhost:11611/v1/users

# Get current user
curl http://localhost:11611/v1/users/me

# Create user
curl -X POST http://localhost:11611/v1/users \
  -H "Content-Type: application/json" \
  -d '{"username": "alice", "password": "secret", "role": "user"}'

# Login
curl -X POST http://localhost:11611/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "alice", "password": "secret"}'
```

## UI Features

### Single-User Mode

The sidebar shows:
- Chat (default)
- Models
- RAG
- Sessions
- Benchmark
- Terminal
- **Advanced**
  - Agent
  - LoRA

### Multi-User Mode

The sidebar shows all of the above plus:
- **Admin**
  - Users (user management)
  - Metrics (system monitoring)
- Auth status in sidebar footer

## Best Practices

1. **Local Development**: Use single-user mode (default)
2. **Shared Server**: Enable multi-user mode with `OFFGRID_MULTI_USER=true`
3. **Production**: Enable multi-user mode + require auth with `OFFGRID_REQUIRE_AUTH=true`

## Security Notes

- API keys are generated automatically for each user
- API keys are hashed before storage (original shown only once)
- Passwords are hashed with bcrypt-style algorithm
- Sessions expire after 24 hours by default
