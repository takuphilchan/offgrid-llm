# CLI execution boundary verification (2026-10-20)

## Checks run in this worktree

| Check | Result |
| --- | --- |
| `go test ./cmd/offgrid ./internal/cli -count=1` | Passed after final root handoff and wrapper removal. |
| `go test ./... -count=1` | Passed; all tested Go packages, including agents, inference, server and service clients, reported `ok` or no test files. |
| `go vet ./cmd/offgrid ./internal/cli` | Passed. |
| `go build -trimpath -o build/dev/offgrid.exe ./cmd/offgrid` | Passed; output is an ignored local development binary, not a release artifact. |
| Pinned Phase 4 baseline comparison | 24 safe commands/aliases matched current executable exactly on exit code, stdout and stderr. Includes the eight recorded baseline fixtures plus migrated `run`, benchmark/compare, agent, model, session, knowledge and log help/invalid usage. Both binaries ran with `NO_COLOR=1`; no installed service or personal document was used. |
| `git diff --check` and `git diff --cached --check` | Both exited 0. Git emitted LF-to-CRLF normalization warnings for existing tracked files, but no whitespace errors. Untracked files are not covered by Git diff checks. |
| `node scripts/maintenance/check-docs.mjs` | Passed: 185 Markdown files, zero problems. |
| `openspec validate cli-execution-boundary --strict --no-interactive` | Passed. This checks OpenSpec structure, not application behavior. |

The focused command subprocess tests additionally exercise isolated service
errors, global flag placement, `NO_COLOR`, cancellation mapping and safe
aliases. A no-user-data server fixture starts a mock-engine service on a
random loopback port, observes `/health`, and shuts it down. Process adapter
tests use fakes and do not install or launch external integrations.

## Unavailable or not claimed

`go test -race ./cmd/offgrid ./internal/cli -count=1` could not run in this
Windows shell: `CGO_ENABLED=0` and `gcc` is unavailable; Go reported
`-race requires cgo`. Linux/macOS signal delivery and native integration
lifecycle have not been qualified here. The installed service, desktop app,
personal model directory and user documents were not used for verification.
No deployment, desktop replacement, commit, push or release was performed.

## Final boundary review

- `cmd/offgrid/main.go` is the sole production owner of `os.Args` and
  `os.Exit`; `runRoot` returns an exit code. Registered families are selected
  through `internal/cli`'s registry, while the root switch retains only
  unmigrated legacy commands and the one existing server constructor.
- Construction of the OS environment reads configuration and captures
  callbacks; it does not create directories, start processes or submit work.
  The exercised subprocess cases are read-only or invalid-usage cases, and
  direct fixtures use temporary storage, fake runners or isolated HTTP.
- `internal/serviceclient` remains the authenticated service boundary.
  Profiles, task submissions, artifacts and control use the configured
  service client; historical legacy agent lists keep their prior localhost
  client. No CLI route was added around service authorization.
- Review found a transport compatibility mismatch in `run` and `metrics`
  during the handoff. Their former default HTTP client behavior was restored;
  benchmark/compare already preserve it. Focused tests, the full Go suite,
  trimpath build and all 24 safe-command comparisons passed after that fix.
- The concurrent provider/agent work compiles and passes the full Go suite
  with this boundary. This does not qualify its native runtime behavior.

No blocking source finding remains for this structural change. Process-global
legacy presentation state, remaining unmigrated root commands and the private
server constructor are explicitly deferred to their owning phases; this
review does not claim concurrent in-process CLI execution or native-platform
qualification.
