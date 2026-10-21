# Tasks

## 1. Freeze documentation and checker baselines

- [x] 1.1 Inventory maintained `docs/advanced` links, the reliability log's major headings/byte count, active versus archived OpenSpec changes, and current size exceptions in `baseline-evidence.md`; verify `node scripts/maintenance/check-docs.mjs`, `openspec list --json`, and `openspec list --specs --json` results are recorded without treating checked tasks as qualification.
- [x] 1.2 Add regression tests for staged removals and additions to the application-layout checker before changing it; verify `node --test scripts/maintenance/check-application-layout.test.mjs` reproduces the old failure and passes after the fix.

## 2. Give maintained docs clear destinations

- [x] 2.1 Add `docs/{architecture,operations,contributing,quality}/README.md` entry points with current owners and explicit status language; link them from `docs/README.md` and the root README, then verify `node scripts/maintenance/check-docs.mjs` has zero missing maintained links.
- [x] 2.2 Move selected current `docs/advanced` guides to their responsible destinations and leave short old-path compatibility stubs; preserve technical content, update active source/CI references, and verify `node scripts/maintenance/check-docs.mjs` plus `rg -n 'docs/advanced/' README.md CONTRIBUTING.md docs scripts .github` reveal no obsolete maintained owner claims.
- [x] 2.3 Keep `docs/ROADMAP.md` the single roadmap, remove stale target dates only from maintained guidance, and explain current spec, active change, user guide and evidence roles in `CONTRIBUTING.md`/`openspec/README.md`; verify docs check and a manual link review.

## 3. Split qualification evidence without changing its meaning

- [x] 3.1 Split `docs/advanced/product-reliability-plan.md` into a `docs/quality` index, capability evidence parts and release evidence parts; retain source provenance and an exact section/byte reconciliation manifest, then verify the generated content comparison and `node scripts/maintenance/check-docs.mjs` pass.
- [x] 3.2 Update maintained readiness/feature pages to link the new evidence owners and distinguish implemented, automatic tests and real-hardware qualification; preserve release notes and record unverified checks, then verify docs check plus targeted `rg` for former reliability-log references.

## 4. Enforce a reviewable repository size and path policy

- [x] 4.1 Fix the application-layout checker to skip removed working-tree paths while retaining stale-reference detection for present staged and untracked sources; verify staged-move and stale-path tests and run `node scripts/maintenance/check-application-layout.mjs` on this dirty worktree.
- [x] 4.2 Add a size-ratchet checker and test cases for `cmd/*/main.go` over 150, new handwritten source over 600, bounded allowlist growth, invalid owner/date, and generated/historical exemptions; verify its unit tests and current-tree check pass.
- [x] 4.3 Document the allowlist owner, responsibility, review date and baseline for justified existing large files; integrate the size checker into CI/root contributor commands and verify the corresponding checker tests, docs check, and `go list ./...` pass.

## 5. Reconcile completed OpenSpec changes

- [x] 5.1 Audit each currently complete active change for its evidence, delta sync state and blocking gaps; record decisions in `openspec-archive-audit.md`, leaving incomplete or unqualified changes active; verify `openspec list --json` and strict validation for archive candidates.
- [x] 5.2 Sync durable delta requirements and archive only verified completed changes one at a time using the installed OpenSpec workflows; verify main specs with `openspec list --specs --json`, archive locations, and `node scripts/maintenance/check-docs.mjs` after path depth changes.

## 6. Cross-surface checks and handoff

- [x] 6.1 Run `go test ./... -count=1`, `go build -trimpath ./cmd/offgrid`, web API/check/build, desktop and companion unit tests, Python SDK tests/compileall, repository checkers, docs check, `go list ./...`, and `git diff --check`; record exact pass/fail/unsupported results in `verification-evidence.md`.
- [x] 6.2 Validate `repository-documentation-hygiene` strictly, review preserved public CLI/API/desktop/installer paths and no installed-user effects, and record any blocking finding before archival. Deployment, desktop replacement, commits, publication and real-hardware qualification are separate actions.
