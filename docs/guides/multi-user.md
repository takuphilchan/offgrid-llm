# Authentication and shared-workspace access

The default quickstart is an unauthenticated local workspace bound to loopback.
It is not a safe shared-network configuration. Enabling multi-user features
alone does not require authentication: configure both intentionally.

## Prepare a protected service

1. Identify its actual data directory, service owner, and configuration.
2. Stop the service and make a [verified backup](../advanced/workspace-recovery.md).
3. Create the initial administrator using the same data root with multi-user
   mode enabled. The legacy user CLI accesses local files, not the HTTP service.
4. Configure authentication before allowing other machines to reach the service.
5. Start it, verify authorized access and rejection of unauthenticated requests,
   then configure a trusted TLS reverse proxy/firewall if remote access is needed.

Example administrator creation for Bash; replace the data path with the **stopped**
workspace's actual root:

```bash
OFFGRID_MULTI_USER=true OFFGRID_DATA_DIR=/path/to/data offgrid users create admin admin
```

PowerShell equivalent:

```powershell
$env:OFFGRID_MULTI_USER = 'true'
$env:OFFGRID_DATA_DIR = 'C:\path\to\data'
offgrid users create admin admin
```

The command prints initial credentials. Store them privately; do not paste them
in issues or screenshots. Do not run it concurrently with the service.
For Docker, use the same image and data volume, not a host-side empty workspace;
see the [authenticated Compose example](../setup/docker.md#production-stack).

## Configure and connect

Set these in the service's environment:

```text
OFFGRID_MULTI_USER=true
OFFGRID_REQUIRE_AUTH=true
OFFGRID_GUEST_ACCESS=false
OFFGRID_HOST=127.0.0.1
```

Keep loopback when a local reverse proxy provides network access. Direct remote
binding needs its own trusted-interface/firewall configuration. Do not expose
an unauthenticated API while setting up users.

The browser uses the login flow and an HTTP-only session cookie. Service-aware
CLI commands use `OFFGRID_SERVER_URL` and `OFFGRID_API_KEY`. API clients can use
a bearer key; see [API authentication](../reference/api.md#authentication).
Never put a credential into a public URL, shell history example, or renderer storage.

If using a YAML config, set its path with `OFFGRID_CONFIG`; do not assume that
writing a file changes a running process. Environment settings can override the
file. Inspect `offgrid config show` with the same environment.

## Roles and limits

Current built-in role permissions are defined in
[users.go](../../internal/users/users.go):

| Role | Capability set |
| --- | --- |
| Admin | Administration, chat, model management, knowledge management, all sessions, statistics |
| User | Chat, model listing, knowledge access, own sessions, statistics |
| Viewer | Chat, model listing, statistics; this is not a no-inference role |
| Guest | Chat |

Individual permissions and endpoint authorization still apply. Agent execution,
MCP configuration, and computer-control administration are privileged surfaces;
ordinary account creation does not grant unrestricted tools or desktop input.
OS-local consent remains independent of service authentication.

### What is and is not isolated

- Authenticated conversations are owner-scoped. Administrators with
  `sessions:all` can access legacy unowned conversations.
- Conversation names remain installation-wide; duplicate names conflict.
- Tasks check initiating actor ownership. Approvals cannot be transferred between users.
- The current knowledge index is shared among callers granted RAG access.
  Uploading a document does **not** create a private collection.
- Separate projects, explicit memberships, and complete collection isolation
  in the roadmap must not be assumed from the existence of roles.

Do not use a shared instance for mutually untrusted private document collections
until the relevant isolation gates are implemented and qualified.

## User maintenance

With the service stopped and its real environment configured:

```sh
offgrid users list
offgrid users info USER_ID
offgrid users quota USER_ID
```

`users create NAME ROLE` takes a positional role. `users reset-key USER_ID`
rotates a credential; update dependent clients deliberately.
`users delete USER_ID` removes an account. Inspect the target and retain a backup
before either mutation. Account removal is not secure erasure of backups,
audit records, or external tool data.

## Troubleshoot access

| Symptom | Check |
| --- | --- |
| Login not required | Verify `OFFGRID_REQUIRE_AUTH` on the actual running service, not just the shell running a client |
| CLI returns 401 | Confirm the intended service and a valid privately supplied API key |
| CLI returns 403 | Check role/permission; do not disable authentication to clear it |
| A conversation is missing | Check owner and workspace identity; another user's resource can return 404 |
| Desktop refuses connection | Match desktop/service version, API, and UI build; authentication does not bypass compatibility |
| Another service owns storage | Stop the legitimate owner for maintenance; do not delete its lock file |

For network deployment and backup/rollback, follow
[deployment](../advanced/DEPLOYMENT.md). For remaining multi-user qualification,
see the [reliability plan](../advanced/product-reliability-plan.md).
