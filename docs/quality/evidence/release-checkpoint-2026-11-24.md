# Integrated release checkpoint — 2026-11-24

The user authorized committing and pushing all pending implementation work and
releasing a new version before continuing the transformation. This checkpoint
preserves the complete implementation, tests, OpenSpec changes and requirement
evidence. It does not close M0–M8 or outstanding model/hardware/acoustic gates.

Inventory before this report: 969 changed source/test/documentation files,
13,813,624 bytes. Credential-pattern and binary-output review found no new
credentials or binaries; the sole file over 2 MiB is the maintained translation
registry. Ignored builds, private backups, model assets and `_work` artwork are
not part of the commit. Independent bounded core, client and release-workflow
reviews found no new confirmed blocker; these are not exhaustive audits.

Fresh local checks:

- Full Windows `go test ./...` passed; log `build/release-go-test-20261124.log`.
- Desktop: 292 passed, 4 opt-in skips, zero failures; `build/release-desktop-test-20261124.log`.
- Computer companion: 238 passed, 6 opt-in skips, zero failures; `build/release-computer-test-20261124.log`.
- API schema check, source layout, source-size and maintained documentation passed.
- Release-context, exact-source CI, finalization and asset-access tests passed
  through native WSL invocation. Windows CI-gate and six trust/preview tests passed.
  Bash-dependent tests first failed through the Windows shell bridge; no tests or
  gates were weakened to resolve that invocation problem.
- Repository hygiene initially detected an ignored November 12 diagnostic Go
  package under `build/document-assessment-20261112`. Its two source files were
  retained byte-for-byte with `.go.source` extensions, outside Go package
  discovery; the unchanged hygiene check then passed. Retention hashes are in
  `build/release-diagnostic-retention-20261124.json`.
- The current renderer, voice/UI regressions and local candidate installation
  have separate [bounded evidence](voice-barge-in-and-auto-ui-2026-11-24.md).

Publication remains pending until the exact committed source passes remote CI
and the existing unsigned-preview workflow builds/verifies its complete artifact
and container set. v0.4.33 is intended as a prerelease; no stable-alias promotion,
signed publisher claim or complete-transformation claim is authorized by passing
these local checks alone. Known limitations remain in the release notes and
master requirement tracker.
