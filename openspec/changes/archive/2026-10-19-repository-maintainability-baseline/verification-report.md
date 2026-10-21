# Verification report: repository-maintainability-baseline

Reviewed against `proposal.md`, `design.md`, both delta specs, `tasks.md`, the Phase 1 diff, and the commands recorded in `baseline-evidence.md` on 2026-10-19. This is implementation review, not a claim of installed-platform qualification.

| Dimension | Result |
| --- | --- |
| Completeness | 17/17 tasks checked; all 7 added requirements have an implementation location. |
| Correctness | 7/7 requirement mappings reviewed; all 11 scenarios have code-path and/or isolated fixture evidence. Real installer execution is unverified. |
| Coherence | Six design decisions followed; source moves are separated from later application/CLI/server refactors. |

## Requirement and scenario mapping

| Requirement | Evidence |
| --- | --- |
| Canonical and legacy entry points | `install.sh`, `scripts/install.sh`, and `scripts/maintenance/test-installer-entrypoints.sh` cover local and piped legacy invocation, forwarded environment and arguments, and root default behavior. |
| Unique legacy behavior retained | Root `detect_gpu_variant`, `download_cli_bundle`, `verify_legacy_checksums`, and `setup_legacy_systemd`; fixtures cover GPU/CPU/AVX2 fallback, checksum mismatch, unsupported architecture, extraction failure, `AUTOSTART=yes/no`, and rejected systemd start. The developer source-build flow remains available through split helpers. |
| Relocated unit identity | `installers/systemd/` contains the unchanged existing units and a checked `offgrid@.service` template; installed targets in scripts remain the same. |
| Generated output untracked | `.gitignore`, removal of indexed `bin/offgrid`, and tested `scripts/maintenance/check-repository-hygiene.mjs`. |
| Go output excluded | `go list ./...` and the checker reject output packages while allowing maintained `scripts/build/` helpers. |
| Project guidance discoverable | Root community files, four GitHub issue templates, PR template, root README links, and documentation/template checks. |
| Historical records unchanged | No release-note diff, no history rewrite, and no installer or installed workspace execution. |

## Issues and limits

- **Warning:** Release installation and systemd behavior are verified by isolated command fixtures and source review, not by installing on disposable Linux/macOS hosts. Before claiming platform qualification, run an offline release-bundle install in disposable identities and verify resulting binaries, UI, service state, and rollback behavior. The older unit examples retain pre-existing path/platform assumptions.
- **Warning:** GitHub private vulnerability reporting and the existing README's Discord invitation could not be confirmed externally here. `SECURITY.md` describes the private-report option conditionally and provides a no-details public fallback; maintainers should verify those project settings and links before publication.
- **Suggestion:** The installer fixture sources root function definitions by a marked file boundary. If the installer is restructured later, replace that test mechanism with a dedicated sourceable library to avoid a comment-dependent extraction.

No critical implementation mismatch was found. The checked work was reconciled into the dirty primary worktree after path-by-path hash comparison; its unrelated provider/native changes remain untouched. The result is not a release-readiness or native-install certification.
