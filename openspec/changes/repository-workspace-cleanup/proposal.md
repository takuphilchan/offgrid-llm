# Proposal

## Why

The application and tooling source trees have moved, but this dirty working tree still contains nested generated output, stale source-path metadata, and misleading backup references. A whole-tree cleanup must preserve unfinished feature work and installed-user state while making the maintained layout auditable.

## What Changes

- Record a reproducible inventory of staged, unstaged, untracked, and ignored files plus source hashes before cleanup.
- Reconcile active path metadata, documentation ownership, and confirmed redundant repository rules without changing product behavior or public entry points.
- Run source and compatibility checks before moving nested build/dependency/test outputs to a recoverable quarantine outside the repository.
- Record the final source and Git delta, retained backups, and verification limits. No output is permanently deleted.

## Capabilities

No spec-level behavior changes. Existing `repository-hygiene`, `documentation-governance`, `development-workflows`, and `installer-entrypoint-compatibility` requirements continue to apply. This structural/tooling change uses `skip_specs: true` rather than inventing a product requirement.

## Impact

Repository metadata, maintained documentation, cleanup evidence, and ignored local build outputs are affected. The public API, CLI, SDK, desktop permissions, model files, installed-user data, deployment, and release artifacts are not changed. Existing active OpenSpec changes and Git index content are preserved.
