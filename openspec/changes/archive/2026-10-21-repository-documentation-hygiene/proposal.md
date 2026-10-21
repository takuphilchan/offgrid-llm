# Proposal

## Why

The reorganized source tree now has a clear application owner, but maintained documentation and repository checks still mix current instructions, long qualification logs, and historical paths. Contributors need a stable map of current behavior and evidence without treating an archived plan as release proof.

## What Changes

- Add maintained architecture, operations, contributing, and quality documentation entry points while preserving old `docs/advanced` URLs through short compatibility stubs or retaining genuinely current documents in place.
- Split the oversized reliability log by capability and release evidence, retaining a navigable index and exact historical claims rather than silently converting unverified claims into guarantees.
- Keep one authoritative roadmap, clarify current specs versus active changes versus user guides versus release evidence, and archive only changes whose tasks and verification actually support completion.
- Add a responsibility-based source-size check: `cmd/*/main.go` at most 150 lines; new handwritten production files over 600 lines require review; existing cohesive exceptions require an owner, responsibility, and review date. Generated and historical files remain exempt.
- Fix repository checks that mistake staged removals for readable current files, then wire documentation/size checks into maintained contributor and CI paths.

## Capabilities

### New Capabilities

- `documentation-governance`: Maintained documentation has discoverable owners, current-path navigation, separated qualification evidence, and explicit OpenSpec/current-doc distinctions.

### Modified Capabilities

- `repository-hygiene`: Add a responsibility-based size ratchet and ensure path checkers inspect the working tree correctly during staged source moves.

## Impact

Documentation, OpenSpec main specs and completed-change archives, maintenance scripts/tests, and CI/contributor instructions change. There are no public API, CLI, runtime, desktop permission, user-data, deployment, model-download, commit, or release changes in this phase. Historical release notes and installed-user state remain untouched. Acceptance requires active links and paths to resolve, source-size checks to distinguish allowlisted existing files from new files, and the applicable repository/CI checks to pass; unavailable platform checks remain explicitly unverified.
