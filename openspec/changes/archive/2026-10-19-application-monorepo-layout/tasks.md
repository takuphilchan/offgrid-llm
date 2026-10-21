# Tasks

## 1. Inventory and preservation

- [x] 1.1 Record current package names, versions, desktop ID/protocol/resource destinations, OpenAPI checksum, and representative CLI/API baselines in `baseline-evidence.md`; verify the inventory against `web/app/package.json`, `desktop/package.json`, `computer/package.json`, and `pkg/api/openapi.yaml`.
- [x] 1.2 Inventory dirty tracked and untracked files under all three source trees, confirm `apps/web`, `apps/desktop`, and `apps/computer` do not exist, and record pre-move hashes; verify the inventory covers each file that will move.

## 2. Move the applications and their own references

- [x] 2.1 Move `web/app` to `apps/web` using `git mv`, preserving dirty and untracked content; verify post-move hashes, unchanged package identity, and that no tracked source remains under `web/app`.
- [x] 2.2 Update `apps/web` Vite output, desktop compatibility import, and schema/test paths; verify `npm.cmd --prefix apps/web run api:check`, `check`, and `build` pass with output in `apps/web/dist`.
- [x] 2.3 Move `desktop` to `apps/desktop` using `git mv`, preserving dirty and untracked content; verify post-move hashes and unchanged package ID, version, protocol, and resource destinations.
- [x] 2.4 Update `apps/desktop` hooks, relative build/UI/companion paths, and desktop-specific guidance; verify `npm.cmd --prefix apps/desktop test` plus `node --check apps/desktop/main.js` and `preload.js` pass.
- [x] 2.5 Move `computer` to `apps/computer` using `git mv`, preserving dirty and untracked content; update companion-local paths and tests; verify `npm.cmd --prefix apps/computer test` passes and package identity is unchanged.

## 3. Integrate maintained consumers

- [x] 3.1 Update Go CLI/server source path defaults and associated tests for the moved UI/companion; verify `go test ./cmd/offgrid ./internal/server` and `go test ./...` pass without changing API or CLI contracts.
- [x] 3.2 Update Dockerfiles, Compose/validation, installer/build scripts, `.dockerignore`, `.gitignore`, and `.gitattributes` for `apps/`; verify `docker compose config --quiet`, Docker validation, installer fixtures, and repository hygiene checks where available.
- [x] 3.3 Update CI and release workflows plus packaging/test scripts for the three new paths while preserving output artifact names; verify release-context/container fixtures, workflow syntax, and package metadata/resource destinations.
- [x] 3.4 Update maintained README, contributor, setup, architecture, CLI/API, and app documentation paths; preserve historical release/evidence text (repairing only broken source links); verify `node dev/scripts/check-docs.mjs` reports no broken maintained links.
- [x] 3.5 Add a maintained-path check rejecting obsolete source paths in active configuration, scripts, source and user documentation; verify its negative fixtures fail and the real tree passes, with explicit migration/historical exemptions.

## 4. Integration and review

- [x] 4.1 Run API contract, TypeScript/build, desktop/companion, Go, Docker-context, documentation, path-hygiene, and OpenSpec strict checks (including `node dev/scripts/check-docs.mjs`); record commands and actual results in `verification-evidence.md`.
- [x] 4.2 Review the diff for lost dirty files, unintended package/lockfile/version changes, obsolete source directories, and release artifact naming; compare inventories and capture unresolved platform packaging/installed-app checks as unverified.
- [x] 4.3 Independently review proposal/design/task scope against the diff and record blocking findings or limitations; do not mark a task complete merely because an OpenSpec check passes.

No training, native desktop control, installed-user data changes, deployment, commit, push, or release is authorized by this checklist.
