# Implementation evidence

Hermes blocker/deployment follow-up (2026-10-24): complete-read classification and
scoped native hook guidance now allow the model to correct display-prefix writes
while the independent verifier retains exact-byte checks. Two fresh source CLI
Hermes/Meta-Llama runs passed, followed by installed CLI/container Hermes and
OpenClaw/Qwen greeting/copy passes with independent file oracles. The user-approved
container/WSL CLI/provider deployment retained backups and all model IDs. Task
11.3 covers these contracted cases; older uncontracted/platform gates remain open.
See the maintained record for hashes, private evidence paths and warning details.

Contracted-launch follow-up (2026-10-24): supported hook integration and explicit
`--verify-contract` invocation now pass both actual installed runtime launch
fixtures, including false completion, cancellation and saved-config preservation.
OpenClaw `agent exec` failed lifecycle delivery; the approved contracted-only
`agent --local` path fixes that with read-only config composition and private
session state. The ordinary path is unchanged. Qwen/OpenClaw passed one real
greeting/copy; the initial Hermes/Meta-Llama copy was safely vetoed for incorrect
bytes, so real-workflow/commit gates remain open. See the dated maintained record
for exact private evidence paths, commands and upstream warnings. No installed
replacement, commit or publication occurred in this follow-up.

Host-verification follow-up (2026-10-24): the shared read-only verification core
and adversarial file/event fixtures pass Windows Go and WSL race checks. No
normal run path or installed plugin uses it yet; the explicit invocation
interface and supported hooks remain pending. This is task 11.1 implementation
evidence, not a repair claim for the existing live failures. See the maintained
record below for commands, scope, limitations and unchanged installed state.

Latest result-fidelity follow-up (2026-10-24): the separately authorized v2
container/CLI was activated, measured and rolled back after failing acceptance.
Frozen comparison: baseline 3/7, candidate 2/7. Real OpenClaw greetings and exact
file copies passed once each with Meta-Llama 8B and Qwen 7B; Hermes/Meta-Llama
still wrote incorrect content and falsely claimed an exact match. Native calls
were valid, but semantic fidelity was not. Models, data, external settings and
desktop were preserved. The running service and CLI are again seq2/v1; the
rejected candidate and backups remain recoverable. The maintained evidence below
records identities, independent outcomes, timings and private paths. Workflow
and commit gates remain open; no prompt-only qualification is claimed.

Latest 2026-10-24 follow-up: the approved seq1 container/CLI and provider-only
replacement preserved model/workspace inventories and external configuration
hashes. Qwen 7B/OpenClaw passed one real greeting and independent exact-file
workflow. Meta-Llama 8B/Hermes passed the greeting but invalid native calls were
rejected before effects. Missing lazy grammar for untagged JSON is being repaired
under reopened task 9.1; workflow and commit gates remain open. The maintained
record below preserves exact results, timings and private rollback references.

Latest template follow-up: the user-authorized r8 container and matching WSL CLI
explicitly enable `llama31-optional-tools-v1`. Runtime/template digest selection
preserves unknown templates and native tool/result semantics. The actual Hermes
greeting now answers normally with tools present. Independent sequential/file
verification still fails; this remains experimental, not workflow qualification.
The removed result-serialization experiment, final test results, exact build
identities and private recovery locations are recorded in the maintained evidence
below. No desktop replacement, commit or publication accompanied this pass.

Latest 2026-10-23 evidence: the CUDA service and WSL CLI are replaced with
q8 KV caching, hybrid CPU/GPU admission, bounded post-eviction measurements
and classified stream errors. The q4/q8 comparison reproduced corrupted
responses only with the tested q4 cache configuration. Meta-Llama's 65,536-token
q8 allocation is confirmed. Full Hermes/OpenClaw file workflows still fail
independent read-back despite successful process exits; task 5.4 remains open.
See the maintained record below for exact tests, timings and recovery paths.

The maintained record is
[External integration foundation — 2026-10-22](../../../docs/quality/evidence/external-integrations-2026-10-22.md).

It distinguishes implemented contracts, automated fixtures, installed-runtime
interoperability with deterministic inference, and actual local-model results.
The real OpenClaw 3B file workflow failed independent verification; Hermes
real-model qualification was context/resource blocked. Neither is presented as
qualified by a passing configuration or smoke test. Later separately authorized
local replacements and the 310-second stream/cancellation checks are recorded in
that evidence. The first slow-stream fixture omitted the generated provider's
300-second HTTP timeout and missed a real installed-config failure; the record
includes its correction and the actual-rendered-config before/after test. The
latest real 8B greeting is still blocked by the pre-load memory estimate. The r3
Qwen greeting passed the old five-minute boundary but failed its ten-minute run
deadline after about 2270 generated tokens without a complete validated answer.
Neither is a successful real-model workflow. Task 5.4 remains open despite the
verified r3 container/CLI replacement. Publication remains outside this change.

The maintained record now includes the r4 four-model matrix and the r5
container/WSL CLI replacement. Shared preparation made the installed Hermes
Meta-Llama 8B smoke pass in 147.25 seconds, but its isolated file-copy result
was wrong (`read_file` rather than the source's Unicode text). The OpenClaw
Qwen 3B isolated workflow also failed. These remain explicit real-model
qualification failures; passing transport and fixtures do not close the
user-visible workflow defect. The latest private paths and exact checks are in
the maintained evidence document above.

The 2026-10-23 user run further showed a Meta-Llama 8B/OpenClaw greeting
still in CPU-only prompt prefill after about four minutes, until user
cancellation, and an unrelated Hermes `--version` preflight deadline. A scoped
version-probe fix passed Go and race tests and a separately built WSL CLI
returned `preflight_ready` against the real Hermes configuration in 1.89
seconds. That CLI is not installed and no new service replacement or real
workflow success is claimed. Task 5.4 remains open.

In the separately authorized follow-up, the WSL CLI was backed up and
atomically replaced. Installed Hermes status and reply smoke passed; OpenClaw
status and direct-provider reply smoke passed. The r5 service and desktop were not replaced, and no
external-agent file workflow passed independent verification. The maintained
evidence document records hashes and the recovery path. Task 5.4 stays open.

A subsequent isolated OpenClaw/Qwen 7B file run failed after a 600-second
budget: the first full-agent request spent about 264 seconds on CPU prefill
and produced no verified destination file. The running container has no GPU
device; the host GPU is visible to Windows inventory but unavailable to the
current WSL/Docker runtime. The maintained evidence records the private test
path and distinct transport, model-quality and hardware findings. Task 5.4
remains open.

A network-isolated cached CUDA diagnostic could not start: the NVIDIA runtime
prestart hook hung. Its disposable container was removed without touching the
healthy CPU service. GPU deployment is unqualified and not part of this change.
