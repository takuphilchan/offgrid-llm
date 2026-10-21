# Proposal

## Why

Generated output, obsolete development deployment files, and competing installer entry points make the repository harder to review and can mislead contributors. Establish a verified hygiene and compatibility baseline before moving application source or decomposing the CLI and server.

## What Changes

- Remove the tracked `bin/offgrid` binary from the repository index, keep `bin/` ignored for local builds, and ignore currently exposed Go/Python cache output.
- Add a repository check that rejects tracked generated output and Go packages discovered beneath build/cache/test-result locations; move temporary generated Go sources to an ignored work area.
- Remove the obsolete Go 1.21 development Docker/Compose example, stale installer note, and disabled workflow snapshots only after checking their maintained replacements. Preserve historical release notes.
- Make contributor, security, conduct, support, issue, and pull-request guidance discoverable at the repository root and on GitHub.
- Preserve root `install.sh` as the canonical public installer and the existing `scripts/install.sh` URL as a compatibility entry point. Inventory and port any unique supported GPU/systemd behavior before replacing the latter; keep service names, installation locations, environment variables, and release assets stable.
- Relocate maintained systemd unit sources to `installers/systemd/` and update maintained references.

This is Phase 1 only. It does **not** move web, desktop, computer, Python, CLI, or server source; change user-facing interfaces; fix unrelated defects; rewrite Git history; commit; deploy; or publish.

## Capabilities

### New Capabilities

- `repository-hygiene`: Maintained-source and generated-output boundaries enforced by repository checks.
- `installer-entrypoint-compatibility`: Root and legacy public Unix installer entry points retain their supported observable behavior.

### Modified Capabilities

None. The current main-spec inventory is empty.

## Impact

Affected areas include `.gitignore`, `bin/`, `build/`, development Docker files, GitHub community/workflow files, root and legacy installers, systemd unit source paths, repository checks/CI, and maintained contributor/setup documentation. Public CLI/API contracts, installed-user state, package metadata, and release artifact names remain unchanged.

The structural work is isolated in a clean worktree because the primary worktree contains unfinished provider and native-workflow changes. The later Python-runtime move depends on finishing or safely integrating that active provider work; it is not part of this change.
