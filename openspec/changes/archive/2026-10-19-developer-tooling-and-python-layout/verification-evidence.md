# Verification evidence (2026-10-19)

This is source/fixture evidence for the primary `D:/offgrid-llm` worktree. It is not release, native-hardware, or installed-app qualification. The old `D:/offgrid-llm-reorg` worktree is a separate older branch and was not used as the migration source.

## Checks that passed

| Area | Command or observation | Result |
| --- | --- | --- |
| Go source | `go list ./...` and `go test ./...` | Passed; relocated examples and metadata tool are included, with no generated build packages. A second full run also passed. |
| Go executables | `go build -trimpath -o build/dev/offgrid.exe ./cmd/offgrid` and equivalent `offgrid-computer.exe` build | Passed; output is ignored local build state. |
| Web | `npm.cmd --prefix apps/web run api:check`, `run check`, `run build` | Passed; Vite reported a non-blocking large-chunk warning. |
| Desktop/companion | `npm.cmd --prefix apps/desktop test`; `npm.cmd --prefix apps/computer test` | Passed, 71 and 73 tests respectively. No installed desktop app was started or replaced. |
| Python | `wsl ... make test-python check-docs` | 37 isolated client and 27 worker-protocol tests passed; six docs-checker fixtures passed. WSL Python was 3.14, not the CI 3.12 matrix. |
| SDK distribution | `python3 -m pip wheel --no-deps --no-build-isolation --wheel-dir <temporary> sdk/python`; `python3 scripts/test/check-python-package.py <wheel>` | Passed; `offgrid-0.2.12` installed into a temporary target, imported with its matching metadata, and contained no service worker. |
| Script syntax/fixtures | `node --check` over 28 JavaScript files in `scripts/test`, `scripts/release`, `scripts/maintenance`; `node --test` over docs, developer-layout, application-layout, hygiene, and native-harness fixtures; `bash -n` over moved build/version helpers | Passed. The combined Node fixture run had 19 passing tests before the new Docker-path negative fixture; that fixture passed separately afterward. |
| Maintained references | `node scripts/maintenance/check-docs.mjs`, `check-developer-layout.mjs`, `check-application-layout.mjs`, `check-repository-hygiene.mjs` | Passed; 174 maintained Markdown files, zero broken local links; all three source/hygiene checks clean. The developer checker has negative fixtures for obsolete paths, missing relocated source, and excluded Docker test context. |
| Root commands | `wsl ... make -n build test test-go test-web test-desktop test-computer test-python check-docs lint`; `wsl ... make test-python check-docs` | Passed; targets delegate to direct commands. Direct PowerShell-compatible commands are documented in `docs/advanced/BUILDING.md`. |
| Release/installer fixtures | `node scripts/release/test-release-ci.mjs`; WSL `node scripts/release/test-{finalize-release,release-context,release-asset-access,release-containers}.mjs`; WSL `bash scripts/maintenance/test-installer-entrypoints.sh` | Passed using fixture data; the deliberate negative installer-download scenario is expected. No release, install, or deployment was run. |
| CI/OpenSpec | Cached `actionlint` v1.7.12 against `.github/workflows/ci.yml`; `openspec validate --all --strict --no-interactive` | Passed; 18 OpenSpec items valid. CI is configured for Python 3.12 on Ubuntu and Windows but those runners have not executed this uncommitted change. |
| Diff hygiene | `git diff --check`; `git diff --cached --check` | No whitespace errors. Git reported CRLF normalization notices only. |

## Limits and failed attempts

- `docker build --check --network=none -f docker/test/native-contracts.Dockerfile .` loaded the relocated Dockerfile from the intended context but could not resolve `ubuntu:24.04` because Docker Hub refused the connection. The automated source-path and `.dockerignore` negative fixtures pass; a real image build remains unverified.
- `wsl ... make build` stopped at `go: No such file or directory` because this WSL distribution lacks Go. The equivalent Go binaries and web bundle built successfully with the installed Windows toolchain; the Make target itself was dry-run checked.
- `go test -race ./internal/server ./internal/agents ./internal/inference ./internal/models ./internal/storage` did not run: this Windows Go configuration has CGO disabled (`-race requires cgo`). This is not a passing race test.
- `go mod tidy -diff` could not fetch the already-declared `go-llama.cpp` module because `proxy.golang.org` was unreachable. It made no go.mod/go.sum changes and is not a proof of tidy state.
- Initial `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 ...` likewise could not reach the Go proxy; running the cached module source locally succeeded. `npm ci` was not rerun: existing locked dependencies were used for local application checks.
- The first bulk `node --check` command accidentally included a `.py` file because of PowerShell `-Include` behavior. It was corrected to select `.js`, `.mjs`, and `.cjs` extensions, and all 28 selected files then passed.
- Native Windows/macOS/Wayland installed-app checks, external Python CI runners, signing, and human usability checks were not performed. No personal desktop, installed-user state, model download, training, deployment, desktop replacement, commit, push, or publication was part of this verification.

## Source and compatibility review

- `git diff --cached --name-status --find-renames=100%` retains 73 exact (`R100`) moves in the relevant paths (71 developer/Python, two root helpers) and nine separately reviewed legacy deletions. The destination audit also found 30 unstaged modified SDK/script files and 13 untracked files, which remain present. `git ls-files -- dev python` returns zero. The two runtime worker SHA-256 values match the pre-move baseline exactly: audio `5639530306A19ECE16290A9552E5CF6DBD674D603F5387B59B959F008FA176C8` and provider `12CE9D5D0C8E0D4429F557ED8EAEB0248B155DD0F5568BA03957E64F7DF269B3`. The worker remains outside the SDK wheel.
- The current `pkg/api/openapi.yaml` SHA-256 is `7251A1E6CCF0B6CED676499F2533BDB82100FED3FA1830D65862F97CFBB43E67`, exactly the Phase 2 pre-move value. `npm ... api:check` also passed. Existing concurrent feature edits to other CLI/API files remain user work; this layout change did not claim to review those features.
- A fresh Windows CLI build emitted the Phase 1 normalized `--no-color --help` SHA-256 `C3E2DB20DD84B3CDF965B1893B30347DF019BC260DAF52E653A5CA87DA42A963`; `version` reported `0.4.18`, `windows/amd64`, CPU only. `--json model preview --category nonsense` exited 2 with the same `invalid_usage` JSON payload recorded in the Phase 1 baseline.
- Package identities remain `@offgrid/ui` 0.4.18, `offgrid-llm-desktop` 0.4.18, `@offgrid/computer-preview` 0.1.0, and Python `offgrid` 0.2.12. Desktop `appId` remains `com.offgrid.llm.desktop`, product name `OffGrid LLM Desktop`, protocol `offgrid`, and packaged `computer-pack`, `ui`, and `bin/offgrid` destinations. The Windows Setup/portable and Linux/macOS artifact-name templates are unchanged. Release archives still use `offgrid-${version}-${platform}-${arch}-${variant}-${cpu_features}` with the same `.zip`/`.tar.gz` split and Docker image `takuphilchan/offgrid-llm`.
- The canonical root `install.sh` and published `scripts/install.sh` paths remain; the latter still points to the maintained raw root installer URL. The isolated installer-entrypoint fixtures passed. Unique source/GPU/systemd helpers remain under `scripts/build` and `installers`, but no real installation was used as parity evidence.
- `git rev-parse HEAD` remains the original `68d36b22544f3ce5843cad53fb7f7bff6ea15eb1`. This pass did not commit, merge the separate worktree, deploy a container, replace an installed app, or modify installed-user models/conversations/data.

## Separate scope, design, and scenario review

The review compared `proposal.md`, `design.md`, and every `development-workflows` scenario with the staged move inventory, unstaged path edits, CI matrix, SDK wheel, executable-worker hashes, and the fixture tests. It found and corrected four in-scope issues: top-level build/version helpers still outside their designated owners, an obsolete quick-start downloader incorrectly placed among maintained client examples, the path checker missing Windows-style references and some active text-file types, and the script guide describing temporary-fixture checks as wholly read-only. The new negative fixtures and current-tree check pass after those corrections.

The reviewed paths still use explicit provider-worker selection and digest validation; no SDK worker fallback, installed-data migration, permission broadening, model download, or release action was introduced. The scenario matrix has evidence for SDK import/version, worker protocol and missing/altered-worker rejection (Go/provider tests), root direct/Make entry points, obsolete-path rejection with historical exemptions, and isolated Python fixtures. This was a separate self-review, not an external peer review or platform qualification.

Evidence limitation accepted for the revised task 1.2: an exact pre-move SHA-256 manifest for every dirty SDK/developer file was not captured. The staged exact renames, worker hashes, destination inventory, and current diffs provide retrospective continuity evidence, but cannot prove the original *before any move* chronology. The user approved documenting this limitation and continuing; the task is checked for the retrospective audit only. Docker image resolution, Windows/hosted Python CI execution, and real native/platform checks also remain unverified as described above; do not infer provider or hardware qualification from this source review.

## OpenSpec verification review

| Dimension | Result |
| --- | --- |
| Completeness | 17/17 tasks complete under the user-approved retrospective inventory criterion; all five added requirements have corresponding implementation. |
| Correctness | Ten scenarios have source or fixture coverage: SDK identity/import and independent versioning (`sdk/python/pyproject.toml`, `scripts/maintenance/update-version.sh`, wheel check); worker protocol and altered/missing identity (`runtimes/python/tests`, `internal/inference/native_package_provider_test.go`); root/direct commands (`Makefile`, `docs/advanced/BUILDING.md`); obsolete/historical paths (`check-developer-layout.mjs` and its negative fixtures); isolated Python CI (`.github/workflows/ci.yml`). Actual hosted Windows CI and installed-provider activation were not run locally. |
| Coherence | The actual moved files, explicit worker identity, source-path checker, and side-effect-specific script owners match the design. The design now records the accepted inventory evidence limitation rather than claiming a chronological hash manifest. |

No critical implementation or spec divergence was found in this source-level review. Remaining platform and Docker-image evidence gaps are listed above and remain unverified, not silently promoted to passes. This is a self-review, not independent external review.
