# Server decomposition verification (2026-10-21)

Local Windows/amd64, isolated test state, no installed service or native desktop control:

| Check | Result |
| --- | --- |
| Pre-move `go test ./internal/server -count=1` | Pass |
| Post-move `go test ./internal/server -count=1` | Pass after handler/router split and again after RAG helper extraction |
| Frozen route and HTTP baseline tests | Pass; 136 registrations, four outer wrappers, 23 route groups, in-memory router health, auth, body limit, errors and SSE |
| `go test ./... -count=1` | Pass after main handler/router split and again after the final RAG helper edit |
| `go build -trimpath -o build/dev/offgrid.exe ./cmd/offgrid` | Pass after main handler/router split and again after the final RAG helper edit |
| `npm.cmd --prefix apps/web run api:check` | Pass |
| `node scripts/maintenance/check-docs.mjs` and its six tests | Pass, zero maintained-document problems |
| `git diff --check` | Pass (exit 0); Git printed existing CRLF normalization warnings |
| `go test -race ./internal/server -count=1` | Unavailable locally: Go reports `-race requires cgo`; `CGO_ENABLED=0`, and no `gcc` is installed. CI's Linux race job remains necessary. |
| `node scripts/maintenance/check-application-layout.mjs` | Fails reading previously moved `cmd/offgrid/model_package_commands.go` because the check enumerates cached deleted files. This is a separate repository-hygiene script issue, not a server compile failure; address in the later layout/hygiene phase. |
| `openspec validate server-mechanical-decomposition --strict --no-interactive` | Pass; no spec delta is expected for this mechanical change |

Tests and source review do not qualify installed-user behavior, real model inference, macOS/Linux native providers, or release readiness. No deploy, commit or publication was performed.
