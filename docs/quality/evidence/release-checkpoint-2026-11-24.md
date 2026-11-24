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

## First remote CI result and corrections

The checkpoint was committed as `9c587e450a53e47f8d51b2bfa3efec7d193588d5`
and pushed to `main`. [CI run 45350120847](https://github.com/takuphilchan/offgrid-llm/actions/runs/45350120847)
failed; no tag or release was created from it. Failures were retained and
investigated rather than bypassing the exact-source release gate:

- macOS native compilation lacked `sys/fcntl.h`, which declares the existing
  `filesec_*` APIs and `FILESEC_ACL`. The include is corrected; native validation
  requires the next macOS runner result.
- Windows extended path syntax does not normalize forward slashes. An explicit
  mixed-separator regression reproduced the hosted-runner failure. Normalizing
  separators before opening the physical path preserves the volume-GUID
  ownership check. Focused checks passed, then the full Windows execution suite
  passed in 73.206 seconds (`build/ci-windows-execution-final-20261124.log`).
- The Python fixture compared a canonical installation path against a Windows
  8.3 alias. Its assertion now requires the resolved path; outside-installation
  rejection remains tested. Linux Python checks passed: 199 tests and 14 subtests
  (`build/release-python-fixed-20261124.log`); the Windows rerun remains required.
- Ubuntu hosted AppArmor policy denied the real user/PID/mount namespace tests.
  The disposable CI VM now explicitly provisions that prerequisite and performs
  a fail-fast PID1/procfs probe. Product policy is unchanged, and unsupported
  hosts still fail closed. The workflow passes actionlint; hosted behavior must
  be verified by the next CI run. Two local stdio fixtures initially exceeded
  their unchanged 60-second helper-build deadlines on `/mnt/d`. From a fresh
  native Linux export of the identical source, with a normal Go build warmup,
  both passed (executor 1.391 seconds, execution 1.613 seconds); no assertions or
  deadlines changed. Original logs are retained, with the successful invocation
  in `build/release-ci-namespace-native-fs-20261124.log`.

## Second remote CI result and corrections

[CI run 45358141297](https://github.com/takuphilchan/offgrid-llm/actions/runs/45358141297)
tested `d36533b27145015df259fcdb201841f4eca9557b`. Python on both platforms,
all three document-worker runners, OCR packaging, container builds and the
native parser passed. Linux integration and Windows/macOS native client jobs
still failed; no release was created.

- Linux can expose an established TCP tuple with inode zero before the server
  accepts it. Ownership verification now waits within its existing bounded
  deadline, without sending HTTP until identity is proven. A real 200 ms
  delayed-accept regression reproduces the race. Full processowner/inference
  race suites passed in 4.855/40.725 seconds; evidence is retained in
  `build/release-connection-race-20261124.log`.
- The Windows desktop monitor fixture lacked a referenced event-loop handle.
  It now owns a bounded wait for the actual unreferenced production monitor.
  Full desktop checks passed: 292 passed, 4 opt-in skips, no failures or
  cancellations (`build/ci-windows-desktop-monitor-final-20261124.log`).
- The macOS account fixture now resolves and preserves Go compiler cache paths
  before isolating HOME, preventing helper builds from placing read-only module
  caches inside disposable application state. Windows executor checks passed
  in 13.034 seconds (`build/release-account-cache-20261124.log`); native macOS
  confirmation remains pending alongside its other execution/terminal failures.

These corrections are incremental; they do not establish a passing release gate.

## Included follow-up work and outstanding CI findings

The subsequent release pass committed and pushed:

- `9af507e6`: provider ownership and native test fixture corrections.
- `89b92a8e`: redacted protected-terminal diagnostics and fixture cleanup.
- `64d3e81b`: verified offline-kit preparation and safe model imports.
- `28f24b70`: accurate package/trust-report counts and preview release procedure.
- `9be55324`: supervised Kokoro speech and Qwen language normalization.

The additional [speech and kit evidence](offline-kit-corrections-2026-11-24.md)
records exact automated scope and remaining real-model/media qualification.
Fresh committed kit checks passed (models 4.566 s, CLI 1.904 s, schema checker
1.300 s), as did six builder tests and the web API contract check. Focused
server audio/model checks passed in 0.725 s. These are not installed-payload tests.

[CI 45402239314](https://github.com/takuphilchan/offgrid-llm/actions/runs/45402239314)
passed the earlier Windows native and desktop checks but failed packaged
terminal navigation at the `Return to Chat` locator. Linux advanced to the
translation checks, which found four missing messages in preview locales.
macOS execution contracts remain under investigation. The next diagnostic run
confirmed that protected input succeeds but its terminal-state restoration
assertion fails; it did not establish successful restoration. These failures
remain release blockers until corrected and verified on the final source.

The next run, [45416730931](https://github.com/takuphilchan/offgrid-llm/actions/runs/45416730931),
failed earlier on Linux due to a checksum-server HTTP/2 transport error. Checksum
verification remains enabled. Windows exposed a concurrent registry-directory
creation race: an initial missing-path probe followed by another process's
creation was incorrectly rejected as `execution_path_outside_workspace`.
Commit `27a90d36` adds a bounded retry with physical identity verification;
appearing files, symlinks and junctions still fail closed. A deterministic
negative control reproduced the CI error, then the Windows host-protection suite
passed ten repetitions (44.758 s), focused/subprocess checks passed fifty
repetitions (6.012 s), and Linux race checks passed three repetitions (4.179 s).
Logs are retained as `build/host-admission-path-negative.log`,
`build/host-admission-path-fixed.log`, `build/host-admission-path-repeat-final.log`
and `build/host-admission-path-linux-login.log`. Initial WSL invocation errors
are retained separately and do not count as passing tests.

The contract-test correction batch passed Windows execution (36.520 s) and
executor checks (7.146 s). Darwin arm64 test binaries cross-compiled, which is
not native runtime proof. Linux helper builds on `/mnt/d` exceeded their unchanged
60-second limits; a fresh native Linux snapshot of the same HEAD plus reviewed
patch and two added contract tests passed execution (7.022 s), executor (1.315 s)
and real isolated PTY tests (8.574 s). The snapshot is retained at
`/home/phil/.cache/offgrid-native-contract-MS8FfO`; its run is recorded in
`build/macos-contract-native-final.log`. macOS terminal restoration remains
unresolved: the next run includes configuration-only before/after state details
without relaxing the exact comparison or exposing passphrase bytes.

Commit `be8f320d` completes four voice messages in twenty preview locales and
updates stale terminal navigation selectors. All 29 locales have 1,719 authored
messages; native-language review remains pending. Eleven quality tests and the
isolated headless terminal display passed. This does not replace packaged native
PTY verification on the Windows runner.
