# Design

## Context

See `proposal.md` for motivation. `web/app` builds the renderer into `web/dist`; `desktop` packages that bundle and the `computer` companion. CI, release automation, Docker, Go source, installer scripts, and docs contain literal references to all three. The primary worktree has concurrent uncommitted changes inside the applications, so stale copies from a separate worktree cannot safely replace it. See `docs/advanced/ARCHITECTURE.md` and `docs/advanced/client-contracts.md` for existing service and client boundaries.

## Goals / Non-Goals

**Goals:** Move source to `apps/{web,desktop,computer}` in the primary checkout, retain every dirty tracked and untracked application file, and update maintained path consumers so the same build/packaging/test behavior resolves from the new locations.

**Non-Goals:** Change UI, native controls, consent, API/CLI contracts, package identities or versions, npm dependency policy, stored data, installed locations, release asset names, or the unfinished provider-runtime work. No live install, deployment, or publication.

## Decisions

1. **Move the actual source trees with Git, not stale copied trees.** Preflight source/destination identity and untracked files, capture a path/content inventory, use `git mv` for source directories where possible, and compare post-move hashes. If an ignored running build locks a directory (as `desktop/dist` did on Windows), move its tracked source paths individually with `git mv` and untracked source paths intact, leaving the ignored executable untouched until it can be retired safely. This preserves active edits. Alternative—copying from the clean refactor worktree—would overwrite newer work; permanent compatibility wrapper projects would defeat the new layout.
2. **Keep three independent npm projects.** Their package manifests and lockfiles move unchanged except for source-relative paths. No root workspace or package reinstall during the move. CI installs each separately. This avoids changing Electron packaging/dependency graphs during a path-only migration.
3. **Distinguish source-relative from runtime destinations.** The renderer builds into `apps/web/dist`. Service and Docker inputs, `OFFGRID_UI_DIR` in test harnesses, and Electron `from` paths point there. Packaged `ui`, `computer-pack`, `computer`, and `bin/offgrid` destinations stay unchanged. Identity manifests, trust checks, and installed-user directories are not rewritten.
4. **Update path owners in place and test contracts.** Update package scripts, Vite compatibility import, desktop packaging hooks, companion paths, Go CLI/server defaults, Docker, CI/release jobs, installer/build/fixture scripts, and maintained docs. Keep historical release notes and historical reliability evidence as records. Add a path-hygiene check only for maintained source/configuration, exempting explicit migration text.
5. **Validate in layers before claiming the move complete.** Compare pre/post package metadata and representative API/CLI identity hashes; run npm API generation/check, TypeScript/build, desktop/companion tests, Go tests, Docker context, CI script fixtures and docs checks. Installed desktop/platform packaging remains unverified if isolated machines are unavailable.

## Risks / Trade-offs

- **Missed relative path** → Inventory references across tracked files, update by ownership rather than blind global replacement, then run path scan and build/packaging checks.
- **Dirty/untracked work lost in a move** → Verify exact destination absence, preserve untracked files, compare content hashes before/after, and do not reset/stash unrelated work.
- **Package identity changes accidentally** → Compare name, version, app ID, protocol, resource destinations, and release naming before/after; do not rewrite lockfile dependency graphs.
- **Test outputs or `node_modules` remain at old locations** → Treat ignored generated output as local state, not tracked source; rebuild and clean only with explicit scope. The moved package directories retain their local dependencies when possible. A running `desktop/dist` may keep the old ignored directory alive temporarily; it must contain no maintained source after this change.
- **Platform package behavior not proven by compilation** → Record Windows/macOS/Linux package and installed-app checks separately as unverified until run in isolated environments.

## Migration Plan

This is a source-tree migration, not a user-data migration. First capture baselines and move the three directories without overwriting other work. Update consumers and validate. If validation fails, retain the worktree and fix paths; do not switch deployed binaries or silently roll back user edits. A later separately authorized deployment would package the new source layout while retaining the established installed resource paths.
