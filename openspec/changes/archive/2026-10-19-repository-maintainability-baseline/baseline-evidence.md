# Phase 1 compatibility baseline

Captured on Windows/amd64 from the isolated `D:\offgrid-llm-reorg` worktree at commit `68d36b22544f3ce5843cad53fb7f7bff6ea15eb1`. The primary `D:\offgrid-llm` worktree had 197 modified/untracked entries at capture time and was not edited or stashed. No service was started; commands below did not use the installed workspace or port 11611. The baseline executable was built with `go build -trimpath -o <OS temporary directory>/offgrid-reorg-baseline-68d36b2.exe ./cmd/offgrid`.

## CLI and API contract

| Check | Baseline evidence |
| --- | --- |
| `<baseline executable> --no-color --help` | Exit 0. Header `OffGrid LLM 0.4.18`; global `--json`/`--no-color` shown. SHA-256 of UTF-8 output lines joined with LF and one final LF: `C3E2DB20DD84B3CDF965B1893B30347DF019BC260DAF52E653A5CA87DA42A963`. |
| `<baseline executable> version` | Exit 0; version `0.4.18`, platform `windows/amd64`, device `CPU only` in this environment. |
| `<baseline executable> --json model preview --category nonsense` | Exit 2; `{"error":{"code":"invalid_usage","message":"category must be language, embeddings, asr or tts","retryable":false}}`. |
| `<baseline executable> model --json preview --category nonsense` | Same exit 2 and JSON payload; validates global flag after subcommand. |
| `Get-FileHash pkg/api/openapi.yaml -Algorithm SHA256` | `DAC4ECECF9F6FCF91DB5B83CBF66698816832FBB6D3948672D6A743170111A5A`. |
| `git ls-files bin build dist` | `bin/offgrid` only (existing tracked binary to remove from the current tree, not history). |

The command dispatcher in `cmd/offgrid/main.go` maps these aliases without invoking them: `rm`/`delete` → `remove`, `bench` → `benchmark`, `fav` → `favorite`, `rag`/`knowledge` → `kb`, `server` → `serve`, `diagnose`/`check` → `doctor`, `setup` → `init`, `sessions` → `session`, `agents` → `agent`, and `log` → `logs`. The default no-command path starts the service. Destructive aliases were not executed just to capture a baseline.

## Application and release identity

| Source | Baseline |
| --- | --- |
| `web/app/package.json` | `@offgrid/ui` `0.4.18`; SHA-256 `9B13AE8FA56C2860E7F7DCDF2B62809182911D35D4B3253460AE30FC18424CCF` |
| `desktop/package.json` | `offgrid-llm-desktop` `0.4.18`; `appId` `com.offgrid.llm.desktop`; product `OffGrid LLM Desktop`; protocol `offgrid`; SHA-256 `68C5DB33F5BCE201EC1ED2889C3E5D97CBA698D68F502F13DA8E771A728B74A7` |
| `computer/package.json` | `@offgrid/computer-preview` `0.1.0`; SHA-256 `3E1B4050D842DB498A49E0DC899110CBC03AB3050D6B8F05B0A946C2881D40EE` |
| `python/pyproject.toml` | distribution `offgrid` `0.2.12` (independent version policy) |
| `.github/workflows/release-unified.yml` | bundle pattern `offgrid-${VERSION}-${platform}-${arch}-${variant}-${cpu_features}` plus `.tar.gz`/`.zip`; SHA-256 `B22F8802497A87B3C7229A50DDF7A23314825E46843E630588F99EFE7E45C264` |
| `.github/workflows/docker-publish.yml` | image name `takuphilchan/offgrid-llm` |
| Desktop package destinations | `computer-pack`, `computer`, `ui`, and `bin/offgrid` resources; Windows NSIS `${productName}-Setup-${version}.${ext}` and portable `${productName}-${version}-Portable.${ext}`; Linux/macOS `${productName}-${version}-${arch}.${ext}` |

## Installer and service identity

| Source | SHA-256 / relevant baseline |
| --- | --- |
| Root `install.sh` | `2B730B7A80B5761A4215F3C202ABEB99DBAED4BFD436343247A828BF0E470F38`; canonical public installer, `VERSION`, `NONINTERACTIVE`, `/usr/local/bin`, CLI/desktop/voice flow. |
| Legacy `scripts/install.sh` | `F24C8472814D17F93085179C5D4392E2548022B2C354B796F189369748042EA1`; published raw URL, `VERSION`, `AUTOSTART`, CPU/GPU bundle selection, installed `offgrid@${USER}.service`. |
| Developer `dev/install.sh` | `25A68A2E64BD787BA2BCE95244520CF2A2126C58D5A1AE0431691888717E58BD`; `--cpu-only`/`--gpu`, source builds, `llama-server.service` and `offgrid-llm.service`. |
| `scripts/offgrid.service` | SHA-256 `06BC12758AF45D614D8956A495398916C118B7D33E9E20B44AFBA0537C4798F6` |
| `dev/scripts/llama-server@.service` | SHA-256 `52B5F73FE6D14C6DA8271DDF78083F39F941632CCF6E04968978A5542C949D0F` |

These scripts are not interchangeable yet. In particular, replacing `scripts/install.sh` with a path-relative shim would break `curl .../scripts/install.sh | bash` and lose its service behavior. Installer consolidation requires isolated parity fixtures before removing the old implementation.

### Installer behavior inventory (before convergence)

| Behavior | Root `install.sh` | Published `scripts/install.sh` | Developer `dev/install.sh` |
| --- | --- | --- | --- |
| Entry/options | Root local or streamed Bash; `VERSION`, advertises `NONINTERACTIVE` but does not read it; no parsed options | Local or streamed Bash; `VERSION`, `AUTOSTART=yes/no/ask` (default `ask`); now has a side-effect-free `--help` for local/pipe fixture | Local source-build script; `--cpu-only`, `--gpu`, `--help`; no release `VERSION` flow |
| CPU/GPU | Always requests `cpu` bundle, feature detection `avx512/avx2/basic`; AVX-512 to AVX2 fallback | Detects Linux Vulkan/NVIDIA/AMD as `vulkan`, Apple Silicon as `metal`, otherwise `cpu`; GPU download falls back to CPU, then AVX2 when applicable | Compiles llama.cpp based on detected or forced GPU mode; may install build dependencies, Go, drivers, and NVIDIA support |
| Release names | `offgrid-${version}-${os}-${arch}-cpu-${features}` `.tar.gz` or `.zip`; optional desktop asset by OS/version | `offgrid-${version}-${os}-${arch}-${variant}-${features}` `.tar.gz` or `.zip`; no desktop asset | No release archive selection; builds source |
| Destinations | CLI and `llama-server` in `/usr/local/bin`; UI `/var/lib/offgrid/web/ui`; audio in `$HOME/.offgrid-llm/audio`; desktop app in user or `/Applications` | Same CLI, UI, and bundled audio locations; no desktop; `offgrid@.service` in `/etc/systemd/system`, `/etc/offgrid/llama-port` | `/usr/local/bin`, `/var/lib/offgrid`, system Go/library locations, `/etc/offgrid`, completion files, `llama-server.service` and `offgrid-llm.service` |
| Service lifecycle | Stops matching processes before CLI copy; no systemd setup | Stops `offgrid@${USER}` and processes; installs template; `AUTOSTART` controls enable/start, default asks; checks health | Creates separate service-user, llama and server units, enables/starts them |
| Failure behavior | Latest-version/download/extraction/copy errors fail; no bundle checksum check; desktop unavailable is optional | Latest-version, download, extraction, checksum mismatch (when checksum file exists), copy and unit-operation errors fail; absent checksum warns; no desktop | Source/dependency/service errors use script error handler; includes installer lock and driver installation |

The developer script is a separate source-build/admin path, not a substitute for either release installer. Its driver/package-manager and two-service behavior must remain available as focused development helpers until any replacement is proven; deleting or redirecting it to root would violate this inventory. The current legacy `--help` fixture invokes both `bash scripts/install.sh --help` and `bash -s -- --help < scripts/install.sh` with a temporary HOME. `wsl bash /mnt/d/offgrid-llm-reorg/scripts/maintenance/test-installer-entrypoints.sh` passed and performed no installation. It exercises entry-point transport, not full installer side effects; the latter need isolated command stubs before replacing the legacy implementation.

### Installer convergence and unit-source evidence

The published `scripts/install.sh` is now a small wrapper. A checkout invokes the neighboring root installer; a streamed invocation downloads the root installer from the maintained raw URL into a temporary file. It sets `OFFGRID_INSTALL_LEGACY=yes` so the canonical implementation retains the published entry point's GPU selection, CPU/AVX2 fallback, optional checksum validation, and `AUTOSTART=ask` default. Root `install.sh` still defaults to its prior CPU bundle and no systemd setup. The legacy path still skips desktop acquisition and optional Piper download; `VERSION` and explicit `AUTOSTART` pass through unchanged.

`scripts/maintenance/test-installer-entrypoints.sh` passed in WSL. It executes both final local and piped wrapper forms with `--help`, uses a fake local canonical script to verify forwarded `VERSION`, `AUTOSTART`, `OFFGRID_INSTALL_LEGACY`, and arguments, stubs downloads to check GPU/CPU/AVX2 selection and exhaustion, rejects unsupported architecture, archive-extraction failure, and checksum mismatch, and replaces `tee`, `mkdir`, `id`, and `systemctl` with in-process fixtures to check service commands for `AUTOSTART=yes/no` and failed start. It compares the embedded `offgrid@.service` unit against `installers/systemd/offgrid@.service`. No actual release download, installation, process stopping, or systemd action was performed. `bash -n` passed for root, legacy, developer, and relocated helper scripts. Installed service behavior on real Linux/macOS remains unverified.

The development-only source installer retains its `--cpu-only`/`--gpu` and source-build flow. Its dependency/build functions moved mechanically to `scripts/build/source-install-build.sh`; its install/service functions moved to `installers/source-install-system.sh`; `dev/install.sh` still handles arguments, presentation, and orchestration. A UTF-8 reconstruction of the three source sections was character-for-character equal to the original Git blob (`63,627` characters); no function body was intentionally changed. The earlier PowerShell-default-encoding split was discarded before this verified UTF-8 version. The developer script's installed unit names remain `llama-server.service` and `offgrid-llm.service`.

The existing `scripts/offgrid.service` and `dev/scripts/llama-server@.service` moved without content edits to `installers/systemd/`. Their SHA-256 digests still equal the baseline values above. `dev/scripts/install-llama-service.sh` now reads the relocated source; its installed target remains `/etc/systemd/system/llama-server@.service`. Maintained installer documentation links root `install.sh`; historical release notes were not edited. The older unit examples have pre-existing platform/path assumptions and were not presented as hardware-qualified by this structural change.

## Phase 1 integration checks

| Command | Result on isolated worktree |
| --- | --- |
| `go test ./... -count=1` | Passed every package on Windows/amd64; `internal/server` 78.936 s and `internal/agents` 61.525 s. |
| `GOPROXY=off go test -race ./internal/agents ./internal/server ./internal/inference ./internal/models` in WSL | Passed all four packages; server 116.425 s. Windows race could not start because CGO is disabled and no Windows C compiler is installed. |
| `go list ./...` | Passed; no `/build/` or `/_work/` package. |
| `node --test scripts/maintenance/check-repository-hygiene.test.mjs scripts/maintenance/check-community-templates.test.mjs` | Passed 8 tests. |
| `node scripts/maintenance/check-repository-hygiene.mjs` | Passed. Root build output remains ignored; maintained `scripts/build/` source is allowed. |
| `node dev/scripts/check-docs.mjs` | Passed: 146 maintained Markdown files, zero problems. |
| `bash scripts/maintenance/test-installer-entrypoints.sh` in WSL | Passed final local/piped wrapper, bundle fallback, checksum, and stubbed systemd fixtures. |
| `bash -n` on root/legacy/developer installers and helpers | Passed. |
| `docker compose -f docker/docker-compose.yml -f docker/docker-compose.dev.yml config --quiet` in WSL | Passed; no image build or live deployment performed. |
| `node dev/scripts/test-release-ci.mjs` | Passed on Windows. |
| `node dev/scripts/test-release-context.mjs` and `test-release-containers.mjs` | Passed in WSL. |
| PyYAML parse of `.github/workflows/ci.yml` | Passed. `actionlint` is not installed offline, so semantic workflow lint is unverified here. |
| `git diff --check` and `git diff --cached --check` | Passed. |
| `openspec validate repository-maintainability-baseline --strict` | Passed; this validates artifacts, not behavior. |

No real installation, service activation, platform packaging, CPU/GPU Docker image build, or installed-user workspace migration was exercised in this phase. `go mod tidy -diff` remains unavailable from the earlier cache/proxy limitation. These limits do not count as passing evidence for the full repository program or provider-runtime qualification.

## Phase 1 compatibility review and integration findings

`cmd/offgrid`, `pkg/api`, all three application package manifests, and the active release workflow were not edited. A fresh temporary build of `cmd/offgrid` produced the same normalized `--no-color --help` SHA-256 as the baseline (`C3E2DB20DD84B3CDF965B1893B30347DF019BC260DAF52E653A5CA87DA42A963`), the same `0.4.18` version/platform response, and the same representative JSON invalid-usage payload with exit 2. OpenAPI, web, desktop, computer, and release-workflow SHA-256 digests matched the baseline table. The release archive naming format and package/resource destination declarations are unchanged. `git diff HEAD -- docs/releases` was empty. No installer, desktop host, user model/data workspace, or service was started or replaced by this structural work.

The primary worktree still contains active provider/native-workflow changes. Its overlapping files are `README.md` and `dev/CONTRIBUTING.md`, both modified by the user to add a Discord community link. This isolated branch has incorporated those exact additions into its root README and relocated CONTRIBUTING content; `SUPPORT.md` refers to the README rather than inventing another invitation. All other Phase 1 paths are non-overlapping with the primary worktree at this review point. Integration into the primary checkout is a separate reconciliation step immediately after this review, not a Git merge or commit.

Unresolved qualifications: real Unix installer behavior and service lifecycle are fixture-tested but not installed on disposable Linux/macOS hosts; private vulnerability-reporting availability and the Discord invitation could not be confirmed by this environment; `actionlint`, Docker image builds, and desktop package builds were not run; existing example unit sources retain old path/platform assumptions; `go mod tidy -diff` remained unavailable. These are not claimed as passed or as production-readiness evidence. No commit, push, deployment, release, archive, or history rewrite has occurred.

### Primary-worktree reconciliation

After the isolated review, Phase 1 was transferred to `D:\offgrid-llm` without touching its in-progress provider/native implementation. The pre-transfer overlap review found only README, contributor, docs-index, and roadmap edits, all adding the project's Discord link; they were retained and combined with the path updates. A SHA-256 comparison of all 49 changed/new Phase 1 paths in the two worktrees was byte-for-byte equal. `bin/offgrid` was removed from the main Git index while its local binary remained on disk. Two existing ignored diagnostic directories, `build/consent-runtime-deploy` and `build/post-launch-diagnostic`, were moved intact to `_work/` after confirming no maintained reference or active process used their paths. Their binaries, UI files, and diagnostic JSON were preserved; no recursive delete was performed. The main-checkout repository checker then passed, the documentation checker reported 162 maintained files with zero problems, the installer fixtures passed, and strict OpenSpec validation passed. Main `git ls-files bin build dist` was empty. No commit, deployment, release, or installed application replacement occurred.

## Pre-refactor test results

| Command | Result |
| --- | --- |
| `go mod tidy -diff` | Unavailable: Windows module cache lacks `github.com/go-skynet/go-llama.cpp` and the `proxy.golang.org` connection timed out. WSL retry with `GOPROXY=off` found that module but lacks `golang.org/x/exp`; no module file changed. This is not evidence of a tidy diff or pass. |
| `go test ./cmd/offgrid -count=1` | Passed. |
| `go test ./internal/server -count=1` | Passed. |
| `go test ./... -count=1` (first run) | Passed all packages; `internal/server` 40.782 s, `internal/agents` 34.276 s. |
| `go test ./... -count=1` (second run) | Passed all packages; `internal/server` 34.751 s, `internal/agents` 27.012 s. |
| `go test ./internal/server -run '^TestModelOper' -count=3` | Passed (7.847 s). This repeated every matching model-operation test three times because the supplied plan did not name the one flaky case. It does not rule out future flakes. |

All checks after structural edits must compare against this baseline. `go run` adds its own `exit status 2` wrapper on a CLI failure, so exit-code comparison must use a built executable, not `go run`.

## Generated-source audit

The maintained scripts and workflows inspected on this branch build binaries under `build/` but do not copy or generate `.go` source there. A source search for `build/...*.go` outside the new planning artifacts found no maintained producer. `go list ./...` completed with no package beneath `/build/` or `/_work/`. The new `/_work/` ignore rule is available for future diagnostic generators; no existing source tree needed relocation in this phase.

## Obsolete development Docker audit

`dev/Dockerfile` used Go 1.21, an unpinned llama.cpp clone, and a handwritten dual-process startup script; `dev/docker-compose.yml` used version `0.1.0-alpha`. The maintained `docker/Dockerfile` uses Go 1.26.6, a pinned llama.cpp revision, built shared UI, and the current service lifecycle. CI and Docker publication reference `docker/Dockerfile`/`docker/Dockerfile.gpu`; the maintained developer overlay is `docker/docker-compose.dev.yml`. The old files were removed only in this worktree. `dev/README.md` now points to the maintained files. Historical release-note mentions remain untouched. `docker compose -f docker/docker-compose.yml -f docker/docker-compose.dev.yml config --quiet` passed in WSL; Windows PowerShell lacked `docker`. `node dev/scripts/check-docs.mjs` passed.

## Disabled release-workflow audit

The disabled bundle workflow built Linux/macOS bundles and an installer; the active `.github/workflows/release-unified.yml` builds Linux/macOS/Windows bundles, includes `install.sh` in each bundle, builds desktop assets, and finalizes releases behind CI/source checks. The disabled simple release workflow's build/package/publish path is likewise superseded by the active unified/finalize workflows. Both disabled files were removed from the current tree only. `node dev/scripts/test-release-ci.mjs` passed on Windows; `test-release-context.mjs` and `test-release-containers.mjs` passed under WSL (their Windows runs failed due path/temporary-directory behavior). PyYAML parsed all six active `.yml` files. `actionlint` could not run offline because its module was absent from the cache; semantic workflow lint remains an integration check. `git diff --name-only -- docs/releases` and its cached equivalent were empty.
