# Host protection integration verification — 2026-11-22

Status: affected Windows and Linux race suites pass on the corrected frozen
source. This verifies an implementation increment, not complete browser file
integrity, administrative recovery, platform qualification or the transformation.

## Source and scope

`build/host-ancestry-checkpoint-20261122.tar` contains 4,267 source files, SHA-256
`87a5bfaf10349326271cccf1327cb78187caf53709a3b52a90dab1c3b98ac619`.
Every extracted file was checked against the accompanying manifest before tests.
Windows used `build/host-ancestry-windows-20261122`; Linux used an isolated
home-directory stage, `/home/phil/.offgrid-host-ancestry.zXHHDa`. Fixture state is
disposable; no personal browser, installed workspace or live service was used.

This snapshot includes shared registry admission/release/reclamation, native
storage checks, read-only inspection and CLI, isolated activation cancellation,
and the namespace-aware ancestry correction. It also includes the server fixture
fix that waits for the subprocess stderr copier before reading diagnostics.
Subsequent voice work and the Chromium patch prototype are separately qualified.

The same 4,267-file source snapshot also passed compile-only checks for all six
packages below on **Darwin amd64 and arm64** (12 combinations, `CGO_ENABLED=0`,
`go test -p 1 -c`). Log: `build/host-ancestry-darwin-compile-20261122.log`.
No Darwin executable was run; native macOS ownership, ACL, filesystem and worker
behavior remain unqualified.

Implementation and focused proof:

- [Registry admission and cleanup](host-protection-registry-2026-11-22.md).
- [Read-only inspection](host-protection-inspection-2026-11-22.md).
- [Linux ancestry witnesses](linux-supervised-storage-ancestry-2026-11-22.md).
- [Isolated activation cancellation](isolated-activation-cancellation-2026-11-22.md).

## Actual full package results

| Package | Windows | Linux race |
|---|---|---|
| `internal/storage` | PASS 7.882 s | PASS 5.882 s |
| `internal/execution` | PASS 31.550 s | PASS 106.246 s |
| `cmd/offgrid-computer` | PASS 5.851 s | PASS 2.116 s |
| `cmd/offgrid-executor` | PASS 6.255 s | PASS 10.097 s |
| `internal/computer` | PASS 1.746 s | PASS 3.012 s |
| `internal/server` | PASS 133.546 s | PASS 261.541 s |

Windows command: `go test -p 1 -count=1 ./internal/storage ./internal/execution ./cmd/offgrid-computer ./cmd/offgrid-executor ./internal/computer ./internal/server`.
Linux used the same packages with `-race -json`, `GOMAXPROCS=4`, `CGO_ENABLED=1`
and `GOPROXY=off`. Raw logs:
`build/host-ancestry-full-windows-20261122.log` and
`build/host-ancestry-linux-race-20261122.jsonl`.

Linux recorded zero failed tests and 41 skips: installed model/native/document
and client journeys, encrypted-backup external CLI/qualification cases, real
Docker cases, metadata/foreign-owner opt-ins, and subprocess helper entry points.
These are not passing qualifications. The separately enabled actual namespace
foreign-owner negative passed as recorded in its focused evidence. Real Docker
preflight later became unavailable; its failed attempt is retained in the
activation evidence, with no container created and no timeout relaxation.

Independent source review found no new blocker in namespace witness validation,
read-only inspection or worker cleanup wiring. Review does not replace runtime
or platform checks.

## Preserved failure and repair

The earlier 4,251-file snapshot (`b3b0cacd...`) passed Windows but failed Linux
executor/server integration. Startup rejected `/tmp` as namespace UID 65534;
the error-reporting branch also read stderr concurrently with its writer.
`build/host-model-linux-race-20261122.jsonl` retains both failures. The correction
uses exact host-validated ancestor identities through the private supervisor
channel, never general trust of UID 65534, and joins stderr copying before
inspection. The table above is a new full run, not an isolated rerun relabeled
as the original passing.

Companion and desktop sources were unchanged from that earlier manifest:
all 243 relevant files matched their hashes. Full companion checks passed
238/244 with six opt-in skips (43.225 s), and desktop passed 290/294 with four
opt-in skips (3.096 s). Raw logs are
`build/host-registry-companion-full-20261122.log` and
`build/host-registry-desktop-full-20261122.log`.

## Remaining gates

Unrestricted commands, terminals, MCP and plugins still need real filesystem
containment. Missing-owner settlement and browser-profile retirement are not
implemented by inspection. Native macOS execution, installed packaging,
power-loss durability, the unchanged 512 MiB browser transport contract,
real-model/hardware, soak and human qualification remain open. No roadmap
milestone, release or full browser-transfer task is certified here.
