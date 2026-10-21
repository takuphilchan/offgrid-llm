# Design

## Context

See `proposal.md` for motivation and `specs/development-workflows/spec.md` for the observable contract. `python/pyproject.toml` defines the independent `offgrid` 0.2.12 client; `python/tests/test_provider_worker.py` currently executes a separate worker source file. Native provider admission takes an explicit absolute interpreter and script path and validates their reviewed bytes, so moving the source must not create a silent fallback. `dev/scripts` is referenced by CI, desktop packaging, Go tests, maintained docs, and other OpenSpec changes. `dev/Makefile` and `dev/README.md` contain stale install/release guidance; Phase 1 already extracted the canonical install helpers. See `docs/advanced/ARCHITECTURE.md` and `docs/advanced/client-contracts.md` for service boundaries.

## Goals / Non-Goals

**Goals:** One public SDK tree, one worker tree, responsibility-specific maintained tooling, a small root command façade, and current-path CI/docs. Preserve the bytes and interfaces of moved executable code unless a relative path must change.

**Non-Goals:** Change inference behavior, worker wire events, Python public APIs or version, install/user-data locations, model acquisition, release assets, native permission flow, or the unfinished provider scheduler. This change does not run live native tests or publish a package.

## Decisions

1. **Move the actual dirty files, not a stale worktree copy.** Record tracked/untracked inventory and worker hashes for `python/` and `dev/`; use Git renames for tracked files and preserve untracked files explicitly. Verify content after prefix mapping before making path edits. A complete pre-move hash manifest for every dirty file was not captured; the user accepted a retrospective staged-blob, dirty-diff, worker-hash and consumer audit with that limitation recorded, rather than claiming the original chronological gate passed. Keep temporary build outputs ignored. Copying a clean branch would lose provider and test edits in the primary checkout.
2. **Separate distributable SDK from runtime workers.** Move the SDK project to `sdk/python` with unchanged package identity, version and import tree. Move both executable workers to `runtimes/python`; place worker protocol tests beside them, while client tests remain under `sdk/python/tests`. The SDK build includes only `offgrid` and its examples/docs, not service workers. Update explicit trusted-worker paths, tests and docs without accepting a default executable path or weakening digest validation. The user explicitly chose this move before provider-runtime completion; the active provider change will consume the new path.
3. **Classify scripts by their real side effects and owners.** Put test harnesses and fixture servers under `scripts/test/`, build/source generation under `scripts/build/`, release validation and installer-smoke automation under `scripts/release/`, and docs/branding/hash/version checks under `scripts/maintenance/`. Move the native contract Dockerfile to `docker/test/`, metadata tool to `tools/model-package-metadata`, and maintained client demos to `examples/clients/`. Review ambiguous `dev/scripts` files individually; do not turn interactive download, installer, or live-desktop helpers into default tests. Keep `scripts/install.sh` as the published compatibility URL and root `install.sh` as canonical.
4. **Replace stale developer entry points with a small delegating root Makefile.** `make build`, `test`, `test-go`, `test-web`, `test-desktop`, `test-computer`, `test-python`, `check-docs`, and `lint` call the same maintained direct commands. They do not install, deploy, download a model, or target a live service. The old `dev/Makefile` is not copied wholesale because it hardcodes 0.1.9, creates a root binary, and exposes destructive install/clean targets. Windows documentation lists `go`, `npm.cmd`, `node`, and Python commands directly.
5. **Make CI and path checks prove the relocation.** Add Python matrix checks on Linux and Windows using a supported Python version; run SDK and protocol fixture tests without `test_live.py` contacting a personal service. Update active scripts, workflow paths, Docker build context, docs, and source tests; add a maintained-path check with historical evidence exemptions. Existing application/source path checks continue to run.

## Risks / Trade-offs

- **Path drift across many script consumers** → Inventory all active references, update by owner, run syntax/tests and a negative-fixture path checker; preserve archived historical commands.
- **SDK accidentally bundles the worker** → Inspect built distribution contents and import from an isolated installation target; keep worker code outside `sdk/python` and test separately.
- **Worker trust or protocol changes during move** → Compare pre/post hashes, update only explicit path references, run protocol/cancellation/malformed-message fixtures and existing Go provider tests. No automatic runtime fallback.
- **Interactive scripts have hidden side effects** → Classify and document each; do not invoke installers, downloads, native-control, or live-service scripts for routine verification.
- **Cross-platform availability** → Run Linux/Windows SDK checks where available and retain macOS/native provider qualification as unverified; CI configuration alone is not execution proof.

## Migration Plan

Capture identities and source inventories, move SDK and workers, update the worker tests and provider references, then rehome scripts/tools/examples/container and update every active consumer. Add Makefile and CI only after direct commands pass. Retain old worktree state and audit staged Git blob identity, dirty diffs, worker hashes and tests for source continuity; do not represent this as a complete pre-move dirty-file hash comparison. If a path check fails, repair the same tree; never silently repoint to a stale worker or replay a user task. This is source migration only—no installed-user migration, deployment, commit, or release.
