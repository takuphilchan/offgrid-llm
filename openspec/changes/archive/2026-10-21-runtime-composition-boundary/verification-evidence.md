# Verification evidence — 2026-10-21

## Ownership and compatibility review

- Production composition has one entry, `internal/app.New`, used by both CLI startup forms. `server.NewTransport` only borrows typed dependencies; `server.Handler`/`Serve` retain the existing router and one listener.
- `NewWithConfig`, `startupErr`, and `construction_legacy.go` were removed. In-package HTTP tests now construct explicit disposable dependencies. Workspace ownership, layout failure, bind conflict, cancellation, corrupt recovery and repeated-close cases are covered in `internal/app` tests.
- The application cleanup stack releases components in reverse acquisition order after transport/agent work drains. If that drain returns uncertain, the runtime retains component ownership.
- No route, public API schema, desktop permission policy or installed-user state was intentionally changed in this phase. Tests use temporary directories, a mock engine and loopback listeners; there was no live desktop/native action.

## Executed checks

| Check | Result |
| --- | --- |
| `go test ./internal/app ./internal/server ./cmd/offgrid -count=1` | Pass |
| `go test ./... -count=1` | Pass twice after removing the legacy constructor |
| `go test ./internal/server -run 'TestServerRouteRegistrationContract\|TestServerHTTPBaseline' -count=1` | Pass |
| `go build -trimpath -o build/dev/offgrid.exe ./cmd/offgrid` | Pass |
| `npm.cmd --prefix apps/web run api:check` | Pass |
| `npm.cmd --prefix apps/desktop test` | Pass, 71 tests |
| `node scripts/maintenance/check-docs.mjs` | Pass, 197 maintained Markdown files, 0 problems |
| `go list ./...` | Pass |
| `git diff --check` | Exit 0; Git emitted line-ending conversion warnings from the existing Windows working copy, not whitespace errors |
| `openspec validate runtime-composition-boundary --strict --no-interactive` | Pass; change intentionally has no spec deltas |
| `go test -race ./internal/app ./internal/server ./cmd/offgrid -count=1` | Unavailable locally: `-race requires cgo`; no supported C compiler was configured |

## Limits

This is source-level and isolated fixture evidence, not a deployment or real-hardware qualification. The broader repository worktree contains unrelated earlier changes; these checks do not imply those changes are committed, published or release-ready. Installed Windows/macOS/Linux desktop checks and human usability review were not run for this pure composition refactor.
