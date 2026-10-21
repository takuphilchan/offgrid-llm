# Tasks

## 1. Inventory and baseline

- [x] 1.1 Record `python/` distribution name/version, import/API surface, worker script hashes, active runtime path references, and current SDK/worker test results in `baseline-evidence.md`; verify `python/pyproject.toml`, `python/offgrid/__init__.py`, and `python/tests` against the record without calling a live service.
- [x] 1.2 Inventory tracked, dirty, and untracked files under `python/` and `dev/`; classify every `dev/scripts` file by owner and side effects, and audit the moves retrospectively using staged Git blob identity, current dirty diffs, worker hashes, and consumer checks. Record the map and the missing complete pre-move dirty-file hash manifest in `baseline-evidence.md`; do not claim the original chronological hash gate was met.

## 2. Python client and worker separation

- [x] 2.1 Move the SDK package, metadata, README, examples, and client tests to `sdk/python` using preserved source files; update SDK-local relative references. Verify name/version/import metadata and run `python3 -m pytest sdk/python/tests/test_client.py` in an isolated environment without a live service.
- [x] 2.2 Move `audio_service.py` and `offgrid_provider_worker.py` to `runtimes/python`, and worker fixtures to `runtimes/python/tests`; update explicit path tests/discovery without adding an executable fallback. Verify pre/post worker hashes before path edits, run `python3 -m pytest runtimes/python/tests`, and `go test ./internal/inference ./internal/models`.
- [x] 2.3 Build the SDK wheel in isolated state, inspect its members for `offgrid` and absence of workers, then install into a temporary target and verify `import offgrid`, distribution identity/version, and representative public methods. Run `python3 -m compileall sdk/python runtimes/python`; preserve independent 0.2.12 policy.
- [x] 2.4 Update Python SDK, runtime-provider, setup and API docs for the new paths and separate SDK/runtime test commands; verify `node scripts/maintenance/check-docs.mjs` and a path search report no active old Python source paths.

## 3. Development files by ownership

- [x] 3.1 Move maintained test harnesses and fixture servers from `dev/scripts` to `scripts/test`, updating their root-resolution logic and all CI/source/package consumers. Verify `node --check` for moved JavaScript, relevant `node --test` suites, and `go test ./internal/server` for embedded helper paths.
- [x] 3.2 Move build helpers to `scripts/build`, release checks/installer smoke helpers to `scripts/release`, and docs/branding/hash/version helpers to `scripts/maintenance`; update direct and relative consumers. Verify representative build/docs/release fixture commands and `npm.cmd --prefix apps/desktop test` without running installers or downloads.
- [x] 3.3 Move the native contract Dockerfile to `docker/test/`, the metadata tool to `tools/model-package-metadata`, and maintained demos to `examples/clients/`; review developer-only experiments individually. Verify `go list ./...`, `go test ./...`, and Docker test-context/path checks without starting native desktop control.
- [x] 3.4 Retire `dev/Makefile`, stale `dev/README.md`, and `dev/install.sh` only after proving unique installer behavior exists in the canonical root installer or maintained helpers. Preserve `scripts/install.sh` URL; verify `bash scripts/maintenance/test-installer-entrypoints.sh` and no tracked `dev/` source remains.
- [x] 3.5 Document every maintained script's purpose, prerequisites, side effects and CI/contributor/release owner in `scripts/README.md`; update desktop/computer/contributor docs and verify `node scripts/maintenance/check-docs.mjs` reports no broken links.

## 4. Root commands and CI

- [x] 4.1 Add the root delegating `Makefile` targets `build`, `test`, `test-go`, `test-web`, `test-desktop`, `test-computer`, `test-python`, `check-docs`, and `lint` without install/deploy/live-service side effects. Verify `make -n` target expansions and execute safe representative targets; document direct Windows commands in `docs/advanced/BUILDING.md`.
- [x] 4.2 Add Linux and Windows Python CI jobs using supported Python, isolated SDK/worker tests and wheel/import checks; verify workflow syntax and path/fixture checks. Keep explicit live-service tests out of default discovery.
- [x] 4.3 Add a maintained developer/Python path check with negative fixtures, historical exceptions, and CI wiring; verify negative fixtures fail and the real tree passes. Keep the existing application layout and repository hygiene checks passing.

## 5. Integration and review

- [x] 5.1 Run `go test ./...`, web API/type/build checks, desktop/companion tests, Python SDK and worker protocol tests, docs/path checks, OpenSpec strict validation, and relevant release fixtures; record actual commands, results and unavailable checks in `verification-evidence.md`.
- [x] 5.2 Review source-file inventories, worker hashes, SDK wheel contents, API/CLI/schema identity, installer URLs and release asset names against the baseline; record any compatibility discrepancy or unverified platform test. Confirm no commit, deployment or installed-user modification occurred.
- [x] 5.3 Perform a separate scope/design/spec review against the diff and scenario tests; keep blocking findings open and do not mark hardware/provider qualification complete from source or fixture checks.

No model download, training, native desktop input, installed-user data change, service replacement, commit, push, or publication is authorized by this checklist.
