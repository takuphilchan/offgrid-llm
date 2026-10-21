# Proposal

## Why

The browser renderer, Electron host, and JavaScript computer companion are maintained as three applications but currently sit at unrelated top-level paths. Moving them together under `apps/` makes ownership and build inputs legible without changing the product or its persisted state.

## What Changes

- Move `web/app` to `apps/web`, `desktop` to `apps/desktop`, and `computer` to `apps/computer`; leave no tracked source or wrapper projects at the former paths. An ignored, running desktop build may remain at its old location until it can be retired safely.
- Update all maintained build, Docker, CI, release, packaging, test, generated-schema, installer, and contributor paths. Preserve package identities, versions, lockfiles, artifact destinations, renderer sharing, and desktop/companion trust boundaries.
- Keep each npm package and lockfile independent; do not introduce npm workspaces.
- Update maintained path documentation while leaving historical release notes and recorded evidence intact.

## Capabilities

### New Capabilities

None. This is a source-layout refactor; it introduces no new product behavior.

### Modified Capabilities

None. No maintained behavioral requirement changes; `skip_specs: true` is declared.

## Impact

Impacts source imports and relative paths in the three applications, `cmd/offgrid`, `internal/server`, Docker, CI/release workflows, installer/build/test scripts, and maintained documentation. Public API/CLI/Python contracts, application identity, installed locations, user data, and release names remain unchanged. The concurrently modified application files in the primary checkout must be moved without overwriting their contents.
