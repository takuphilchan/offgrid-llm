# Repository Structure

This repository is organized as a single product with multiple distribution surfaces:

| Path | Purpose |
|------|---------|
| `cmd/offgrid/` | CLI entrypoint for the `offgrid` command. |
| `internal/` | Private Go application packages for server, inference, RAG, models, agents, audio, P2P, users, and supporting services. |
| `pkg/api/` | Public API types and the stable OpenAPI contract. |
| `web/app/` | React and TypeScript application source. |
| `web/dist/` | Generated UI bundle served by Go and packaged by Electron. |
| `desktop/` | Electron desktop wrapper and desktop-specific assets. |
| `python/` | Python SDK and examples. |
| `docs/` | User, operator, contributor, and architecture documentation. |
| `docker/` | Dockerfiles, Compose files, and container deployment helpers. |
| `installers/` | Platform installer scripts. |
| `scripts/` | Build, install, service, and developer utility scripts. |
| `examples/` | Standalone examples intended to compile with `go build ./...`. |
| `dev/` | Developer-only scripts, examples, and setup helpers. |

## Cleanup Rules

- Keep generated binaries, local module caches, model files, logs, and build outputs out of Git.
- Put user-facing examples in `examples/`; put experimental or maintainer-only examples in `dev/examples/`.
- Prefer OS-specific files with build tags for platform behavior, such as disk, USB, GPU, and process handling.
- Keep docs aligned with implementation status. If a feature still depends on mock mode or partial integration, say so directly in the relevant guide.
- Avoid growing large entrypoint files further. New CLI behavior should move toward focused command files or package-level helpers.
- Treat `pkg/api/openapi.yaml` as the stable HTTP source of truth and regenerate UI types after contract changes.
