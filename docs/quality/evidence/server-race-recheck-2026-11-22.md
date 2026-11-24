# Server and CI race recheck — 2026-11-22

The current full server race suite and complete race-package set from
`.github/workflows/ci.yml` both passed. This establishes current execution
results; it does not explain or erase the earlier timeout failures in
[task-writer evidence](task-writer-reservation-2026-11-21.md), qualify skipped
runtime tests or establish stable application performance.

## Source and environment

Both runs restored and byte-verified the same 4,169 maintained source files from
`build/endpoint-damaged-recovery-checkpoint-20261122.tar`, SHA-256
`bb6d2e87b53a7024979d506b12af3a8b39ac762cce5958432dc0dedf876ca8b5`.
Ubuntu under WSL, Go 1.26.6, CGO enabled, GOPROXY off, GOMAXPROCS=4 and fresh
ext4 test state. No live installation, model weights, personal browser sessions
or real native input were used. Required opt-in fixture flags were not invented.

## Full server, unchanged eight-minute deadline

```sh
go test -race -json ./internal/server -count=1 -timeout=8m
```

PASS: JSON package elapsed 266.212 s; Go's final line reports 266.209 s.
Process wall time was 266.99 s, user CPU 235.00 s, system CPU 16.91 s,
maximum RSS 868,428 KiB, no swap, exit 0. The longest parent test was
`TestAutomaticDependentWorkerValidatesSealedEdits`, 44.25 s, PASS; desktop
observation/control workflow was 14.77 s and the 66-publication lifecycle was
9.30 s. Per-test timing is retained, not just a final successful exit.

Evidence:

- `build/endpoint-history-full-linux-race-20261122.jsonl`
- `build/endpoint-history-full-linux-race-20261122-resources.txt`
- `build/endpoint-damaged-and-full-linux-race-20261122.log`

The earlier runner first ran agents, then server/API/configuration; agents were
not concurrent with its failed server command. The new result is a different
source checkpoint and execution, so neither package overlap nor an intervening
code change is a proven explanation. No assertion was removed and this
eight-minute deadline was not increased.

## Complete CI race package set

The second run matches CI's package set, adding JSON output and `-count=1` to
force execution. It keeps Go's default ten-minute timeout, as CI does; it is
separate from the eight-minute server recheck above.

```sh
go test -race -json -count=1 ./internal/sessions ./internal/rag ./internal/server ./internal/users ./internal/cache ./internal/agents ./internal/runs ./internal/storage ./internal/serviceclient ./internal/models ./internal/audio ./internal/computer ./internal/execution ./internal/processowner ./internal/inference ./internal/integrations/... ./internal/cli ./internal/power ./internal/degradation ./cmd/offgrid ./cmd/offgrid-executor
```

All 22 packages PASS, no failed test event, process exit 0. Wall time 299.16 s,
user CPU 583.37 s, system CPU 57.42 s, maximum RSS 1,040,580 KiB, no swap.
This is the race-test step, not the complete CI workflow with installers,
runtime packaging, renderer/desktop, cross-platform and opt-in qualification.

| Package below repository root | Seconds | Skipped test/subtest events |
|---|---:|---:|
| internal/sessions | 3.488 | 0 |
| internal/rag | 75.105 | 30 |
| internal/server | 274.332 | 32 |
| internal/users | 4.504 | 0 |
| internal/cache | 1.025 | 0 |
| internal/agents | 138.267 | 0 |
| internal/runs | 1.012 | 0 |
| internal/storage | 6.116 | 3 |
| internal/serviceclient | 3.390 | 0 |
| internal/models | 2.639 | 2 |
| internal/audio | 2.150 | 0 |
| internal/computer | 3.363 | 0 |
| internal/execution | 79.747 | 4 |
| internal/processowner | 4.403 | 0 |
| internal/inference | 22.690 | 8 |
| internal/integrations | 1.195 | 2 |
| internal/integrations/verification | 1.579 | 0 |
| internal/cli | 4.815 | 0 |
| internal/power | 1.013 | 0 |
| internal/degradation | 1.014 | 0 |
| cmd/offgrid | 58.380 | 4 |
| cmd/offgrid-executor | 5.933 | 0 |

All 85 skip events were inspected and retained in
`build/transformation-ci-skips-20261122.json`. They include explicit model/runtime
qualification, packaged PDF/OCR workers/packs/corpora, administrator-provisioned
Ollama/ONNX factories, Docker isolation, installed integration hooks, pinned
release archives, cross-client/manual-browser fixtures, age interoperability,
PTY opt-ins and helper-only subprocess entry points. A skipped helper entry point
can have separate parent-subprocess coverage; a skipped real-runtime test is
not qualified by a fixture. These checks need their own prepared environment
and prior/prospective evidence, not a reinterpretation of this pass.

Detailed output and resource measurements:

- `build/transformation-ci-race-20261122.jsonl`
- `build/transformation-ci-race-20261122-resources.txt`
- `build/transformation-ci-race-20261122.log`

Current race checks no longer reproduce the prior server timeout. Its cause,
repeatability under controlled load, application latency/memory, 72-hour soak,
installed/platform and real-model qualification remain separate work. This
record does not complete a milestone or authorize commit, publication or release.

## Repository-wide normal Go and final endpoint renderer checks

The same verified source was subsequently checked with
`go test -json -count=1 ./...`: exit 0, 41 packages PASS, 22 packages with no
test files, 90 skipped test/subtest events and no failed package. Test-event
wall interval was 91.267 s; server passed in 68.706 s and agents in 25.892 s.
Logs: `build/transformation-all-go-20261122.jsonl`,
`build/transformation-all-go-20261122-stderr.txt` (empty), and
`build/transformation-all-go-20261122.log`. Packages with no test files are
compiled by this command; they do not acquire behavioral coverage from it.

The final production renderer build and all affected owner/history client/UI
tests passed: 28 tests in 47.1 s, including narrow multilingual reviews,
uncertain responses, stale history, recovery navigation, authorization and
malformed historical policy handling. Logs:
`build/endpoint-history-final-web-build-20261122.log` and
`build/endpoint-history-final-web-tests-20261122.log`. The final build retains
the existing bundle-size warning. These tests are distinct from the actual
fresh-service journeys documented in the history/recovery records and do not
represent the entire desktop/browser/native CI workflow.
