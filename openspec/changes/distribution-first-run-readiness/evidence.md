# Distribution readiness implementation evidence

## 2026-11-11 initial implementation

- Root installer now verifies selected CLI/desktop payloads against the release manifest before use. Legacy mode also refuses missing internal checksums. Failed command substitutions propagate errors. Running OffGrid or llama-server and inspection errors refuse replacement; process termination and systemd stop were removed.
- Root and Unix desktop/archive payload copying now preserves app-relative audio/web and execution assets. Generated archive helper refuses running work. Windows CLI helper refuses active OffGrid/inference processes and process-inspection errors before replacement.
- `node --test scripts/maintenance/desktop-installers.test.mjs`: six tests passed, including generated archive execution with redirected disposable paths, active-process refusal, payload preservation and Windows inspection failure. The fixture prevents unrecognized absolute destination paths. No native install was performed.
- CLI UI, speech and inference discovery now includes executable-relative archive layouts. Explicit configuration remains first; execution pack discovery already supports executable-relative layouts and validates identity.
- `bash scripts/maintenance/test-installer-entrypoints.sh` passed valid fallback/checksum fixtures, corrupted/missing/duplicate/unavailable manifest refusal before extraction, process-state refusal and legacy service fixtures.
- `bash scripts/maintenance/test-executor-install.sh` passed version retention, full UI/audio payload copies, and pre-effect invalid version rejection.
- Focused discovery tests passed, followed by full `go test ./internal/audio ./internal/inference ./internal/server ./internal/execution` (all passed).
- Documentation links and strict OpenSpec validation passed. No native qualification claimed by these fixtures.

## Installation inventory

Existing application directory: `C:/Users/phil/AppData/Local/Programs/OffGrid LLM Desktop`. Initial process enumeration found no OffGrid/llama-server process. This must be rechecked immediately before installation. No current-user code-signing certificate was found; no private keys or credential values were read. No installed application changes have been made at this stage.

## Remaining

See tasks.md: runtime API verification, artifact trust and preview identity, full project/package verification, installed upgrade and actual response verification remain. Windows installer release qualification, signing/notarization and other native OS qualification are not established by this progress.

## 2026-11-11 local commit and first-use follow-up

- User requested a commit before 23:40. Local commit `2054693b` was created at
  23:36:15 +08:00, without pushing. It contains verified installer/hardware
  selection, payload discovery, guidance and fixtures. The exact staged snapshot
  passed all four Go audio/inference/server/execution suites, nine installer
  fixtures, root/executor Bash fixtures, documentation/source-size checks and
  release naming/context/finalization fixtures. See the installer change's
  verification.md for scope and test-environment corrections.
- Current first-use source adds explicit preparation with mode/size/progress,
  retry and cancellation. Native probes inherit cancellation and readiness is
  invalidated by configuration changes or a removed runtime. Focused runtime
  manager tests passed, including native helper success, failed help exit status,
  cancelled probe, corrupt download, single-flight refusal and removed binaries.
- Onboarding completion/dismissal is scoped to workspace and account, with a
  one-time binding of legacy presentation preferences. Saved chat history no
  longer counts as a new runtime verification. No conversations are migrated.
- TypeScript, API generation/check, translation integrity and UI production build
  passed. Playwright passed all 25 runtime-preparation/chat-readiness scenarios,
  then both returned-first-reply and denied-browser-storage scenarios (27 total).
  These use isolated browser/API fixtures, not actual model inference.
- The broader current-source Go run encountered
  `TestProcessProviderBlockedPipeWriteHonorsStopContext` at 5.86 seconds against
  its two-second bound. The unchanged test subsequently passed three isolated
  repetitions. This is not dismissed as a pass: the full run/server result and
  a serial full inference rerun still need collection before task 3.1 is checked.
- No application installation or public release was performed in this follow-up.

## 2026-11-12 preview build and native preparation

- Continued with an isolated source worktree at
  `D:/offgrid-distribution-preview-20261112-001` because other tasks were editing
  the shared workspace. Its initial source snapshot records 2,785 files; explicit
  build-runner and route-contract corrections were applied to both trees.
- Added native artifact trust inspection and a fail-closed release gate. The
  workflow downloads actual draft assets (including reused files), checks their
  GitHub SHA-256, then verifies Authenticode/publisher pin or Apple signature,
  team, Gatekeeper and stapled ticket. Six verifier/preview fixtures passed;
  Windows native inspection reported the cached local installer unsigned and
  release mode rejected it. This is not evidence of public release signing.
- Built `0.4.29-preview.20261111161627595.g2054693b`, with matching desktop,
  service, executor and terminal identity and verified packaged runtime/UI
  payloads. `preview.json` identifies the dirty source and UI build. Both NSIS
  and portable artifacts are unsigned and explicitly not release-qualified.
  NSIS SHA-256: `3269a0900333fa8d1fbab47215cebe1b8848334e97e27fbdc4074f01d48cb5fb`.
  Portable SHA-256: `35a90f6394779ee970b457034c7978fe2498971df387d6cb968a71c584a21614`.
- Build execution exposed two runner issues, both corrected: relative packaging
  hooks need the desktop working directory, and Go's `-trimpath` omitted linker
  metadata needed by the existing service-version gate. Failures now exit with
  a nonzero process status. The successful build passed the original gates.
- Frozen-source desktop suite: 201 passed, one directory-symlink test skipped.
  Installer plus trust/preview fixtures: 15 passed. API check, TypeScript and
  production UI build passed. Documentation: 355 files, no issues; source-size
  check passed. Workflow actionlint passed before the final credential-scoping
  adjustment; that adjustment still needs the final rerun.
- Runtime/chat browser suite passed all 25 cases. A dependency junction initially
  caused Vite font-access errors; dependencies were copied into the isolated
  worktree and the full 25-case suite passed again without that environment issue.
- A real isolated service began with no engine, inspected missing CPU readiness
  without writing files, then explicitly downloaded and digest-verified all
  18,506,923 bytes of pinned b10516. Its native capability probe reached ready.
  No model was downloaded. Evidence: isolated worktree
  `build/runtime-download-evidence.json`; only the owned fixture process stopped.
- Full Go run passed every package except the server's frozen route manifest,
  which needed the newly added `/api/v2/runtime` registration and count 153.
  Reviewed and updated exactly that entry; the complete Go rerun is in progress.
  The earlier inference pipe-cancellation failure did not recur in this run.
- Packaged desktop preparation/real-response qualification is in progress.
  The user's installed application has not been changed by this task. Other
  OffGrid build/install activity was observed and must be rechecked before an
  upgrade. Publisher credentials and native macOS qualification remain absent.

### Final suite follow-up

- `go test -p 1 ./...` in the isolated worktree passed after the route-contract
  update; server took 111.939 seconds, other unchanged packages used their passing
  cache. Full log: `build/distribution-go-check-final.log`. The earlier full run's
  only failure was the now-corrected route manifest.
- Final workflow actionlint and all six verifier/preview fixtures passed again.
- Packaged engine preparation succeeded, but the first Qwen2.5 3B response was
  correctly refused by the memory guard. Available machine memory was about
  2.25 GiB afterward. The UI retained the prompt and settings and offered a
  smaller model. This is not counted as successful inference. No memory guard
  was disabled and no unrelated process was closed. Next qualification must use
  a model fitting actual available memory or a less-loaded machine.

### Native installer and GPU follow-up

- User confirmed another installation task is still running. This task must not
  replace the main installation until that work has finished. The final preview
  should incorporate the later main-worktree language/model-choice edits before
  replacing that task's build; six app files had changed since the frozen source.
- A real CUDA-backed Qwen2.5 3B reply was produced by the preview, then by the
  disposable installed qualification build. Preparation reported NVIDIA ready,
  the response was saved, and setup completion was recorded. The full native
  harness is not yet passing: its older empty-composer Send-button assertion and
  image file-input selector needed alignment with the current UI. Those test
  selectors were corrected without changing product behavior. GPU evidence must
  name its actual backend rather than the harness's former hardcoded CPU label.
- Added opt-in `--gpu` to the native reply harness. Its isolated Windows
  environment now retains ProgramFiles/ProgramW6432: reproducing NVML under the
  previous minimal environment failed, while either machine installation root
  restored detection. Personal OffGrid settings/paths/auth remain excluded.
- The first disposable NSIS install copied and SHA-256-verified all 1,186 payload
  files and registered the expected unique app ID/version/path. Its test harness
  initially assumed an optional uninstall-registry InstallLocation property;
  verification was corrected to the actual bounded UninstallString.
- That first disposable installer exposed a real isolation error: despite its
  unique app ID/executable, the original package name caused it to overwrite
  `offgrid-llm-desktop-updater/installer.exe`. The installed application's exe,
  app.asar, service and UI matched the other task's terminal-language build.
  After verifying those identities, the cache was restored from that task's
  actual installer, only while the cached bytes still matched our test artifact.
  Restoration SHA-256: `F5E6CE9D36AD16B747FFBA5AED08D17A3517332B38D7CFBB7E275FB1A6F4A9D2`.
  Receipt: isolated worktree `build/cache-restoration.json`. This cache write and
  restoration are explicitly disclosed; application/workspace files were not
  written. The first qualification app was uninstalled and its registry key
  removed; the restored main cache remained unchanged during uninstall.
- A v2 disposable installer is building with a separate `extraMetadata.name`,
  and a builder-level assertion checks its actual updater-cache directory.
  Its native install verification also checks the separate cache and unchanged
  main cache. This v2 run, full native reply/restart completion and cleanup remain
  pending; task 5.1 is deliberately still unchecked.

### Completed disposable qualification and remaining native guard gap

- The v2 installer completed with its separate package/cache identity. All 1,186
  installed payload files matched their source hashes, its own registration and
  cache were verified, and the main installer cache remained unchanged.
  Installer SHA-256:
  `DCEF39B50314164D1D3F4C5E3670E9E61FE5A4AA520716960296888775B101E2`.
  Evidence: isolated worktree `build/isolated-installer-v2/install-evidence.json`.
- The full native reply/restart harness passed from that temporary installation:
  NVIDIA preparation ready, one Qwen2.5 3B submission, persisted reply
  `hello there` in 9,340 ms, workspace identity retained after restart, saved
  answer not replayed, and image draft preserved. Unsupported image inference
  was refused without creating a turn. This does not qualify successful vision
  inference or a CPU model reply. Result:
  `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-native-reply-m5liok/result.json`.
- The temporary application was uninstalled. The original harness's 20-second
  observation window expired before cleanup completed; subsequent read-only
  verification confirmed both installation directory and registration absent,
  with the main cache still matching the restored installer hash. No repeat
  uninstall or manual deletion was used. Evidence:
  `build/isolated-installer-v2/uninstall-evidence.json` in the isolated worktree.
- Acceptance review found that native Setup also needs explicit handling for a
  separately running `offgrid.exe` or `llama-server.exe` and failed process
  inspection. Task 1.2 is reopened; successful helper fixtures alone do not close
  the requirement covering all maintained installer entry points.
- The concurrent installation task has added its own native process hook and
  cooperative desktop shutdown flow in the shared tree. Our overlapping new
  hook was removed from the shared tree, preserving that task's code; its three
  installer tests pass. A fail-closed candidate remains only in the isolated
  worktree. Its two tests pass, including actual PowerShell execution with
  mocked process enumeration, but it has not been compiled or qualified as a
  native installer and is not integrated into the current main source.
- Main upgrade remains pending the user's confirmation that the other install
  task has finished. Current source must be reconciled and rebuilt afterward.
  Actual publisher signing and native macOS qualification remain external gates.

### Native service-guard qualification

- Reinspection of `git show HEAD:apps/desktop/assets/installer.nsh` confirms the
  cooperative desktop shutdown hook was already committed. Earlier wording
  attributing it to newly arriving concurrent edits, and the initial concern
  about electron-builder's default force-stop path, were inaccurate. The actual
  missing coverage is separate service/worker detection and explicit reporting
  of failed process inspection. The existing hook is preserved.
- Added a native `nsProcess` guard in a separate include and a disposable native
  qualification harness. The shared installer does not yet include it. The
  isolated installer candidate checks the desktop, `offgrid.exe` and
  `llama-server.exe` before replacement and preserves the existing user-approved
  cooperative desktop quit flow. Unknown enumeration results stop with code 3;
  a matching active process stops with code 2. No PowerShell or name-based
  termination is used by the new guard.
- Thirteen compiled NSIS guard cases passed: absence permits a temporary marker,
  a live harmless owned process prevents the marker and survives, its subsequent
  exit permits the marker, and simulated inspection errors prevent the marker.
  Evidence: `C:/Users/phil/AppData/Local/Temp/offgrid-installer-guard-g42KzB/result.json`.
  The three existing installer tests also passed against the isolated candidate.
- Complete candidate Setup qualification is still in progress. Its first build
  failed because NSIS did not resolve a mixed-separator include path; the include
  was corrected to a Windows separator. This was a build failure, not a passing
  installer run. The shared installation remains untouched in this follow-up.
- The corrected full NSIS candidate built successfully. Artifact SHA-256:
  `1C5C77261F423644DD27E9AD7703BDC8B8FEA3A3046894195FEA82B8B62B079D`.
  Silent installation with a harmless owned `offgrid.exe` fixture refused with
  code 2 in 4,369 ms. The process survived; an existing sentinel file remained
  byte-identical; no payload files or uninstall registration appeared; the main
  and qualification updater caches stayed unchanged. Evidence: isolated
  worktree `build/isolated-installer-v3/refusal-evidence.json`.
- The first full refusal harness expected the target directory itself to remain
  absent. NSIS `.onInit` calls `SetOutPath $INSTDIR` before the process hook, so
  it can create an empty directory even on refusal. Inspection found an empty
  directory and no registration or cache change. The subsequent test checks
  existing file preservation and absence of added payload instead. This does
  not claim zero filesystem effects from starting native Setup.
- The isolated candidate preserves the original desktop quit hook and adds
  explicit inspection-error checks plus desktop/service/worker checks before
  replacement. The shared `installer.nsh` remains exactly at its committed
  version; only the new reusable guard and native test harness are present in
  the main worktree. Integration and a positive install of the final combined
  build remain pending the other installation task. Task 1.2 remains open.

### 0.4.29 release preparation and thread coordination

- The user expanded the goal to install, commit and release, then explicitly
  requested waiting for the other thread to finish. That thread is identified
  as `01a1b5d3-f785-77b2-8a32-20be2caca35a`, current turn
  `01a1cb73-9180-7212-a3a5-9a0346d73baa`. Read-only turn records and observed
  packaging processes confirm it is still preparing the localization install.
- The process guard is now integrated into the shared installer, retaining the
  committed quit flow. Its three existing installer tests passed. A temporary
  shared version bump to 0.4.29 interfered with the other task's package/service
  version check; only that version bump was restored to 0.4.28. The other thread
  rebuilt successfully with explicit 0.4.28 metadata. Further shared-source
  edits, installation and publication wait for that thread to finish.
- An isolated 2,798-file snapshot retains the proposed 0.4.29 version. It passed
  full `go test -p 1 ./...`, API contract, translation structure/completeness/
  consistency, TypeScript and production web build. Desktop tests: 202 passed,
  one platform skip. Logs: `build/release-0.4.29-go.log`,
  `build/release-0.4.29-web-build.log`, and
  `build/release-0.4.29-desktop-tests.log` in this isolated worktree.
- The concurrent task made further translation/test edits after that snapshot.
  Refresh it when the task finishes and rerun checks affected by those changes;
  these passing results are not evidence for unseen future source changes.
- GitHub reports 0.4.28 as the latest release. Repository secret names contain
  only DOCKERHUB_TOKEN and no repository variables: the Windows/macOS publisher
  credentials required by the new trust gate remain absent. No secret values
  were read. No new commit, tag, push or release was made.

### Final 0.4.29 local upgrade

- The other thread's turn completed at 2026-11-11 19:16:26 UTC. Its final
  localization source was incorporated before the 0.4.29 build. The final UI
  identity is `0cad8aa77f74918fb83a82079231be04e61124f8109908934dbae3053292f9d4`.
  Seven source/documentation/test files changed after the first release snapshot;
  no Go application code changed after its passing full run.
- Built both Windows artifacts with matching desktop/service/execution/terminal
  version 0.4.29. All original package verification hooks passed. Setup SHA-256:
  `e30d6807ab58c36e92ca6a78d7ea8dee7bf722f6d7c8ed44de705939676cb021`.
  Portable SHA-256:
  `5ef6c87c3e190def73b2eaea2b4902f010932c1eb151c187c5e547c2f9c7fd6e`.
  Native inspection reports both NotSigned and not-release-qualified. These are
  local candidates built before the final commits, not signed public artifacts.
- Final desktop checks: 202 passed, one platform skip. Translation regression
  tests: 11 passed. Installer/trust/preview fixtures: 15 passed. Documentation,
  source-size, repository hygiene, whitespace and strict change validation passed.
- Full browser run: 1,569 passed, 32 skipped, five failed. Two failures came from
  launching a standalone service without its packaged PDF worker. Two older
  account/workspace tests needed to explicitly dismiss the newly scoped setup
  dialog; they now assert it appears before continuing their privacy checks.
  The localization audit attempted to click background disclosures behind Help;
  traversal now scopes itself to the open modal. The affected five test files
  then passed using the packaged service: 108 passed, one skip. Product code was
  unchanged by these corrections. This is a full run plus a focused rerun, not
  a second all-green full run. Logs use `build/release-0.4.29-browser-` in the
  isolated worktree; post-build changes are recorded as test-only SHA-256 deltas.
- The main installed upgrade exited 0. All 1,186 payload files matched SHA-256;
  registration and executable version both identify 0.4.29. All 576 workspace
  files retained metadata, and files at most 16 MiB also retained exact hashes.
  The main installer cache matches the new Setup. The application stayed closed.
  Evidence: isolated worktree
  `build/release-0.4.29-windows/installed-upgrade-20261111-193202/result.json`.
  Rollback installer retained from the completed localization task:
  `D:/offgrid-llm/build/localization-completion-20261112/installers/OffGrid-0.4.28-windows-x64-setup.exe`.
- The installed executable then passed the native reply/restart harness in a
  disposable profile. NVIDIA preparation reached ready; one Qwen2.5 3B request
  produced `hello there` in 3,432 ms. Saved reply, first-answer completion,
  restart and image draft preservation passed; unsupported vision was refused
  without creating a turn. The normal workspace stayed unchanged, the source
  model retained its hash, and the test instance closed. Evidence:
  `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-native-reply-wY7FS1/result.json`.
- Installed service SHA-256:
  `8cd0892b2a62c50bfd0553ffd6ddabc023406807633ecbe307f02c9b2b6cef48`;
  installed desktop SHA-256:
  `3b44236aaea6bc35e02c2ffecc55d23e8388fd0debb801ba1b285b8ebcf4ade5`.
  Native macOS signing/notarization, exact-revision remote CI and publication
  remain incomplete. No successful CPU-model reply or vision qualification is
  inferred from the GPU test.

### Reviewed source checkpoint

- Created focused local commits after the verified upgrade: `c2192d54` runtime
  preparation, `0df992d2` setup/appearance/localization, `145a005b` native upgrade
  guards and artifact trust, and `24f0cd7` version 0.4.29 and candidate notes.
  Package versions and lockfiles agree. The staged snapshots passed whitespace,
  credential-pattern, large-file and binary checks. Trust/preview regression
  fixtures passed again (six tests); maintained documentation had zero findings.
- The package source differs from the final commits only in the documented
  browser-test corrections, release/specification documentation and removal of
  trailing blank lines from the theme license notice. The installed artifact
  hashes above continue to identify the tested pre-commit candidate; they are
  not hashes of a future CI build. No application behavior was changed after
  installed qualification.
- GitHub still lists only `DOCKERHUB_TOKEN` among repository secret names and
  no repository variables. Windows/macOS signing and notarization credentials
  remain unavailable. The latest published version is still 0.4.28. The exact
  committed source must pass remote CI before a release can be qualified.

### First exact-source CI attempt

Run `43090219230` on `c27df6d05cfa2b12a6f4d07f1a6ce6e58f69cf25`
passed pinned parser/OCR builds, the offline document corpus, Python checks on
Windows/Linux and PDF-worker checks on Windows/macOS/Linux. The native macOS
desktop unit step failed because the new translation tests import TypeScript
before that job installed the renderer's dependencies. The workflow now runs
its existing locked renderer dependency install before those tests, removing
the duplicate install from the later renderer build. Local desktop checks
passed again (202 passed, one platform skip). This changes CI ordering only;
it does not change the installed application's behavior. The corrected revision
requires a new full CI run; the first run is not qualifying evidence.
