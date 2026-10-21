# Implementation checkpoint (2026-10-20)

Progress is 10/16 checked tasks. This is not the completed Phase 5 boundary.

- Task 1.1: pinned the Phase 4-built CLI and its safe output/exit oracle.
- Task 1.2: added isolated subprocess coverage for flag order, `NO_COLOR`,
  aliases and JSON errors; existing tests cover authenticated service errors.
  The Unix interrupt fixture is skipped on Windows because Go cannot send
  `os.Interrupt` to a child there. Windows cancellation mapping has a unit test,
  not a real Ctrl+C platform qualification.
- Task 2.1: added `internal/cli.Environment`, typed command runner, invocation
  options, the `Execute` boundary, and direct dependency/cancellation tests.
  `cmd/offgrid` still owns the runtime entry; it has not yet been wired through
  `Execute`.
- Tasks 2.2–2.3: shared terminal/error/JSON presentation uses injected streams
  and options. One alias registry covers all 47 original root cases. Flag
  parsing no longer mutates `os.Args` or sets `NO_COLOR` in the process
  environment; a subprocess test checks the latter. Existing command families
  still use transitional wrappers and process-global JSON mode. Animated
  progress output belongs to the later TUI family.
- Task 3.1: version, help, search, service-backed model listing, doctor and
  info rendering moved into `internal/cli`. Direct tests use injected streams,
  isolated HTTP, authenticated service fixtures and fake observations. The OS
  adapters use read-only model scanning. `doctor` no longer creates a
  `.test_write` file; it reports storage presence with writability explicitly
  untested. This deliberate safety change is not claimed byte-identical to old
  doctor output. Tests assert missing storage remains missing, and `info`
  retains the old empty-model JSON `null` shape.

After these edits, `go test ./...` passed. The eight originally recorded safe
executable fixtures, three safe search captures, and four isolated
`info`/`status` captures matched the pinned executable byte for byte including
exits. `git diff --check` exited 0 (with Git CRLF conversion warnings for
unrelated existing files). No installed service or desktop app was started or
replaced.

- Task 3.2: moved authenticated session/conversation/export, knowledge,
  profile, agent submission/control/artifact and computer service operations to
  `internal/cli`. `cmd/offgrid` retains thin OS adapters while Phase 5 is in
  progress. The injected execution boundary has one service-client factory,
  explicit terminal/interactive callbacks and exclusive output-file adapters;
  missing dependencies fail closed. Existing command tests and new isolated
  HTTP fixtures verify actor credentials, redirect rejection, JSON shapes,
  exit codes 0/1/2/130, artifact digest/no-overwrite, and no local-file
  fallback. `go test ./... -count=1` passed on Windows. A newly built
  `build/dev/offgrid.exe` matched the pinned baseline's output and exit codes
  for ten safe CLI captures (including `kb --help` and `session --help`).
  This is source/test evidence only; service or installed-desktop qualification
  was not performed. Tasks 3.3 onward remain.

Task 3.3's original wording conflicted with this change's structural-only
scope: it requested actor-scoped service access although `audit` and `config`
are local commands and `/metrics` has its own existing access behavior. The
proposal, design and task now explicitly preserve those local boundaries and
defer any new service API or authorization policy to a separate change. This
resolved the planning conflict before task 3.3 was implemented and verified.

- Task 3.3: moved config, metrics, users, and audit into `internal/cli`,
  deleting their former command implementations and leaving only the OS-backed
  adapter in `cmd/offgrid`. The adapter injects local configuration, user and
  quota stores, audit logger/file discovery, cryptographic entropy, and HTTP.
  It does not add a service endpoint or change `/metrics` authentication.
  Legacy local-admin diagnostics retain their original process status, while
  the command functions now return explicit typed outcomes. Config's legacy
  printed errors retain status 1 without double rendering. Direct tests use a
  disposable user/audit directory, stubbed HTTP, and injected configuration;
  they cover user creation/listing/JSON/quota/deletion, audit
  show/stats/verify/export, config init/show/invalid paths, and offline/live
  metrics presentation. A rebuilt CLI matched the pinned Phase 4 executable
  byte for byte on stdout and stderr, with identical exit codes, for 17 safe
  config/metrics/users/audit captures. `go test ./... -count=1` passed after
  the command move; targeted CLI tests passed again after adding the final
  fixture assertions. No installed user store or application was used.

Task 4.1 is complete at the source/fixture level. Model download (including
the `download-hf` compatibility command), package operations, USB import,
remove, export, verification, quantization and batch execution now use
`internal/cli`. The OS adapter injects reviewed quantizer and batch process
calls, selected model-directory file operations, progress presentation and
the existing authenticated model service client. The unreachable legacy
Hugging Face download helper was removed; no active command called it. Direct
tests use disposable model and USB directories, fake quantizer and batch
callbacks, and local authenticated HTTP fixtures. A rebuilt CLI matched the
pinned Phase 4 executable byte for byte on stdout/stderr and exit code for
16 safe model/batch captures; four further alias/help captures also matched.
No installed model directory, service or desktop was used. `go test ./...
-count=1` passed after the full migration. Real download, quantization and
installed-system behavior are not claimed qualified by these fixtures.

At the initial task-4.2 checkpoint, work remained unchecked. The `chat` placeholder, audio
command implementation and integration list/setup/install logic now execute
inside `internal/cli`. Audio uses injected file, HTTP and engine adapters;
integration install uses an injected user-home and bundle installer. Legacy
agent list/workflow/tool/template parsing and presentation also use the shared
command boundary, with the task list still going through the authenticated
service client. Legacy MCP configuration management now uses injected local
file and clock dependencies; a disposable-file test covers add/list/enable/
disable/remove and a malformed entry that previously could panic. Legacy
terminal chat and interactive `run` internals still have OS adapters in
`cmd/offgrid`; this is
not a claim that the entire command family has moved. Run and benchmark
entry points now return typed outcomes instead of calling `os.Exit` directly.
New isolated tests cover audio, integration and legacy-agent callbacks,
authentication and malformed input. Six safe integration and five safe agent
executable captures matched the pinned Phase 4 binary on separate stdout,
stderr and exit status; the direct audio safe captures also matched. The
first integration comparison attempted to use `.ArgumentList` on Windows
PowerShell's older .NET and was invalid; the recorded six captures are from
the corrected `.Arguments` comparison. `go test ./... -count=1` passed after
these partial task-4.2 edits. No real integration installation, audio setup,
interactive model or installed application was used.
`go test ./... -count=1` passed again after the MCP move.

Benchmark execution and comparison have since moved from `cmd/offgrid` into
`internal/cli`; the command package retains only the OS entry adapters. The
implementation now uses injected terminal streams, local configuration and
HTTP client rather than global stdout/stderr and `http.DefaultClient`. Isolated
tests exercise one-iteration benchmark and two-model comparison against
disposable GGUF files and fake HTTP responses. Five safe help/alias captures
matched the pinned baseline on separate stdout, stderr and exit status.
Targeted CLI tests and `go test ./... -count=1` passed after this move. A
separate fake-transport test confirms interrupting benchmark execution maps to
exit 130 rather than hiding cancellation as a failed iteration. Task 4.2
was still unchecked because the
large interactive `run` implementation is not yet migrated.

Task 4.2 is complete at the source/fixture level. Interactive `run` parsing,
model selection, HTTP requests, session handling and plain-terminal loop now
live in `internal/cli`; `cmd/offgrid` supplies OS process/model loading,
hardware probing and Bubble Tea terminal rendering through callbacks. The
legacy agent terminal loop and help moved into the same boundary, while task
submission still calls the existing authenticated durable-task path. Tests
exercise missing adapters, isolated model directories, streamed replies,
TUI-to-plain fallback, cancellation without plain-mode replay, automatic
agent model selection and a single task
submission. A current executable matched the pinned Phase 4 binary exactly
on stdout, stderr and exit code for eight safe `run`, `chat` and `agent`
captures. `go test ./... -count=1` passed after the migration, and targeted
CLI tests passed after the final fallback fixture. No real model, service,
interactive TUI, installed workspace or desktop was exercised; those are not
claimed qualified by this checkpoint.

Task 4.3 is in progress and remains unchecked. The `logs` command now uses
the injected CLI boundary and a named OS process runner for `systemctl`,
`journalctl` and `tail`; direct fixtures use a fake runner and disposable log
file. The runner rejects unknown capabilities, arbitrary paths, invalid
follow flags and unbounded line counts before process creation. The existing
`computer setup` link now uses the same reviewed launcher with strict
`offgrid://computer` task-ID validation and context cancellation. Three safe
`logs` help captures matched the pinned executable exactly. Hermes and
OpenClaw launchers now reject oversized argument vectors and NUL-bearing
arguments before reaching their existing injectable process factories; a
direct test confirms rejection without spawning a process. This retains
their explicit CLI pass-through verbs and does not yet migrate their process
ownership to the per-invocation runner. Four additional safe Hermes,
OpenClaw, logs and computer-help captures matched the pinned executable on
stdout, stderr and exit status. A count outside
1–10000 now fails before launch, an intentional safety tightening not included
in the exact-output compatibility set. Hermes/OpenClaw process ownership and
other native helpers still require review, so this is not evidence that task
4.3 is done.

The active llama-server startup and model-transfer reload hooks were also
reviewed. Startup now launches the trusted binary with a bounded argv and
platform-specific detached-process settings, instead of interpolating model
paths into `sh -c`. Reload keeps its systemd route, but runs status/restart
and the selected-model write through named process capabilities; `sudo tee`
receives the path over stdin, while direct fallback uses argv. A fake-runner
test covers service discovery without spawning a process. A test
proves that a path containing spaces, quotes and shell metacharacters remains
one argument; invalid ports and paths fail before launch. Windows package
tests passed, and Linux/amd64 and macOS/arm64 CLI test binaries cross-compiled.
No native runtime was launched, so background lifecycle and actual systemd
behavior remain unverified. The remaining integration/native launchers still
prevent checking task 4.3.

The interactive `run` alias-download prompt was corrected after this review:
it now calls the existing service download command with an explicit `--file`
argument and propagates a typed error in the same command context. A fixture
checks the handoff without network access, so a nested `executeCommand` can
no longer terminate the process mid-session. After these changes,
`go test ./... -count=1` passed and a fresh executable matched the pinned
Phase 4 binary for 12 safe command captures on stdout, stderr and exit code.

Task 4.3 is complete for process-dispatch injection and isolated unit coverage.
Hermes and OpenClaw command, installer and smoke-test dispatch now share an
injectable launch hook; argument vectors (including executable and script
paths) are bounded and rejected before dispatch. Their existing explicit
CLI pass-through behavior remains. A fake launcher verifies both commands'
argv and propagated failures without running installed agents. The Hermes
smoke-test fixture now feeds the process output through the fake launcher
instead of spawning a helper subprocess. Computer-workspace opening uses an
injectable reviewed runner; fake tests verify the exact URI and reject an
invalid task ID before dispatch. Systemd/logging and llama reload already use
named process capabilities with fake-runner coverage, while quantization
uses an injected process callback and disposable model directory. The legacy
background server launcher is now injectable with a disposable log-root and
fake health probe. `go test ./cmd/offgrid ./internal/cli -count=1` passed after
these changes. No external integration installer, systemd service, browser,
native model process, real background server or installed-user workspace was
used; native lifecycle behavior remains unqualified. Moving these adapters to
per-invocation OS environment ownership belongs to the still-open process
entry task 5.1, not this launch-injection checkpoint.

Task 5.1 is in progress. `cmd/offgrid/main.go` now creates the interrupt
context, passes an immutable argv view to the root, and owns the only
production `os.Exit` in the command package. The root returns command exit
codes, including failures from migrated command wrappers. Legacy `init`,
appliance, diagnostic, version and default-server paths now return their
existing codes instead of calling `os.Exit` themselves. A safe current build
matched the pinned Phase 4 executable on exit code, stdout and stderr for 14
captures; `go test ./cmd/offgrid ./internal/cli -count=1` and `go test ./...
-count=1` passed. OS dependency construction and the final single-dispatch
handoff are not complete; task 5.1 remains unchecked. The `init --help`
capture in an earlier comparison could read the configured model directory,
so it was excluded from the final safe set and no installed-state conclusion
is drawn from it.

Task 5.3 is complete. `CONTRIBUTING.md`, `docs/repository-structure.md`,
`docs/advanced/ARCHITECTURE.md`, and `docs/advanced/cli-experience.md` now
identify the actual `internal/cli` command owner, the still-present
`cmd/offgrid` compatibility adapters and server startup, and the deferred
`internal/app` phase. `node scripts/maintenance/check-docs.mjs` checked 185
maintained Markdown files and reported zero problems. It does not check
external URLs or heading anchors.

Task 5.2 is complete at the isolated-source level. Both no-command default
and explicit `serve` route through one private adapter that calls the
existing `server.NewWithConfig(cfg).Start()`; no replacement constructor or
`internal/app` shell was created. A fake-adapter fixture verifies both routes
and startup failure return codes without touching user state. A second test
constructs the real server with a mock inference engine, temporary model/data
directories and a random loopback port; it observes `/health` and closes the
server cleanly. `go test ./cmd/offgrid -run
'TestDefaultAndExplicitServeUseOneLegacyAdapterWithoutUserState|TestDefaultServeStartsAndShutsDownWithIsolatedState'
-count=1 -v` passed. `go test ./cmd/offgrid ./internal/server -count=1`
also passed. This is not an installed desktop/container or native model test.

Task 5.1 is now complete at the source and isolated-fixture level. The OS
entry creates the interrupt context and `cli.Environment`; `internal/cli`
owns a single registry for migrated command families, with the root retaining
only unmigrated legacy branches and the private server adapter. Redundant
command wrappers for benchmark, agent, search, logs, audio, and batch were
removed from `cmd/offgrid`; its remaining OS-specific helpers construct
dependencies but do not execute an alternate migrated command path. Legacy
agent lists keep their historical localhost client, while profiles, tasks,
artifacts, and control retain the configured authenticated service client.
Benchmark retains its prior default HTTP timeout. `rg` finds one production
`os.Exit`, in `main.go`, and no production `os.Args` use outside `main.go`.
`go test ./cmd/offgrid ./internal/cli -count=1` passed. A fresh trimpath
build matched the pinned baseline exactly on exit code, stdout, and stderr
for 24 safe help/invalid-usage/alias captures, including `run`, `bench`,
`compare`, `agents`, `models`, `quant`, `sessions`, `knowledge`, and `log`.
Real service, native app, and model behavior are not implied by these fixtures.
