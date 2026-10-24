# External integration foundation — 2026-10-22

## Scope and observed defects

### Hermes copy recovery and deployment, 2026-10-24

The captured Hermes/Meta-Llama write contained `1|` before the actual source text.
The file tool's display result was copied literally. A second adapter defect
classified an explicit read limit as incomplete even when it covered the entire
file. The Hermes hook now accepts complete bounded reads using the provider's
pagination/truncation metadata and contributes invocation-scoped guidance through
Hermes's supported `pre_llm_call` hook, including examples that distinguish an
outer display marker from a genuine source prefix. Existing user text and tool
results remain intact. The verifier can diagnose an extra numeric display prefix
only when the remaining proposal matches the immutable source digest exactly;
it never repairs or executes the proposed write itself.

Source candidate v5 SHA-256:
`54c1576930937ac44b647dbb8b2bc0613b88b8a1e42ce7f0563cd6f35df5beec`.
Two fresh real Hermes/Meta-Llama greeting and exact Unicode copy runs passed the
contract and independent byte/workspace oracle:
`/tmp/offgrid-verification-launch-nw729t3m` and
`/tmp/offgrid-verification-launch-0pcv4hb9`. In the first, the model corrected a
rejected write before read-back. Previous failed candidates remain recorded.
This establishes these narrow contracted cases, not arbitrary model competence
or ordinary uncontracted copying.

The authorized deployment replaced the container with
`offgrid-llm:local-contract-recovery-20261024`, updated the WSL CLI at
`/home/phil/go/bin/offgrid`, and installed matching first-party Hermes/OpenClaw
provider bundles and the Hermes verifier. The verifier was enabled without the
optional built-in-tool override grant. The previous container is retained as
`offgrid-before-computer-1792846272`; matched data backup:
`/home/phil/offgrid-computer-backup-29aVXxDA`; CLI/plugin/config backup:
`/home/phil/offgrid-contract-install-ps8djsym`. The archive checksum and clone
health/tasks/capabilities/UI checks passed; all five model IDs were preserved.
The container is healthy. Startup probes briefly saw connection resets while it
started; subsequent checks passed. The desktop app was not replaced.

Post-deployment installed CLI/service test:
`/tmp/offgrid-verification-launch-ujdkxggw` passed the real Hermes/Meta-Llama
greeting and exact Unicode read/write/read-back with independent verification.
The copy session reported 20 seconds. The installed CLI digest matches the
candidate above. Container logs showed healthy requests and GPU model loading;
the internal loopback llama worker still emits its existing unauthenticated CORS
warning. This is retained as a warning, not described as an error-free platform.

Validation: full Windows Go tests, WSL integration/CLI race tests, 9 Node hook/
provider tests and 23 Python hook/provider/oracle/fixture tests passed. Prefix
recovery tests preserve genuine prefixes, Unicode, CRLF and empty content and
reject guesses when removing a prefix would change genuine source bytes. Both
installed-runtime deterministic launch suites passed greeting/copy, fabricated
completion rejection, cancellation (2.03 seconds), and configuration preservation:
Hermes `/tmp/offgrid-verification-launch-ozwhny9h`, OpenClaw
`/tmp/offgrid-verification-launch-2fqvfamc`. An intermediate test caught the empty
content boundary in the diagnostic loop; it was corrected before deployment.
Broad platform/workflow qualification remains incomplete.

The post-deployment OpenClaw/Qwen greeting and exact-copy run also passed:
`/tmp/offgrid-verification-launch-bltakpt_`, using the installed CLI and updated
container. Task 11.3 is complete for these explicitly contracted workflows.
Older tasks 5.4, 9.3 and 10.3 remain open for their broader installed/native and
uncontracted workflow requirements; these successes do not qualify all models.
The source verifier and Hermes hook were committed separately as `706b676` and
`120c0ba`. Other pre-existing integration changes remain in the working tree.

### Explicit file-contract hooks and contracted OpenClaw launch, 2026-10-24

The source implementation adds `--verify-contract` to managed runs, an authenticated
invocation-local loopback transport, pinned Hermes/OpenClaw hooks, independent
final receipts and nonzero failure/unverified outcomes. Ordinary commands are
unchanged. Hermes requires a distinct explicitly enabled hook plugin; OpenClaw
requires explicit conversation-access permission. No permission is enabled by
production run. Hook results/arguments and model messages are not rewritten.

Actual installed OpenClaw `agent exec` omitted the lifecycle hooks even though its
public dispatcher fixture passed. The failed tests remain under private
`/tmp/offgrid-verification-launch-lazwcrk9` and earlier fixtures. The approved
contracted path now uses documented `agent --local` with a fresh session and
private state. A read-only external `$include` overlay preserves existing config,
nested includes, tools and permissions while pinning workspace/agent/session paths
and suppressing bootstrap creation. It does not copy personal auth/session stores.
Ambiguous agent selection and a changed inspected config reject before launch.
State is retained privately for inspection, not removed automatically.

Installed runtimes: Hermes 0.21.2/f364c197 and OpenClaw 2026.10.4/383c124. Source-built
WSL CLI only; the installed CLI, container, desktop and personal plugins/config
were not replaced. The existing container reports
`offgrid-llm:local-external-gpu-20261024-seq2`. No commit or publication occurred.

Deterministic managed-launch evidence (actual installed runtimes, fake local
inference, not model qualification):

- Hermes `/tmp/offgrid-verification-launch-6cm_hy7m`: greeting and exact Unicode
  read/write/read-back passed; a fabricated success response exited nonzero;
  cancellation stopped/disconnected in 2.02 seconds; saved config/workspace
  checks passed.
- OpenClaw `/tmp/offgrid-verification-launch-s9nmve47`: the same cases passed;
  cancellation stopped/disconnected in 2.03 seconds. The fixture uses nested
  config includes, a non-default named agent with original workspace/state paths,
  and independent checks that those paths were not touched.
- The narrower installed-hook fixtures also pass: actual Hermes discovery/native
  file tools and OpenClaw public hook dispatcher, including a rejected wrong write.
  These alone had not established OpenClaw launch compatibility.

Initial real-model evidence, using fresh disposable homes and the existing idle
service, not changing installed settings:

- Qwen2.5-7B/OpenClaw `/tmp/offgrid-verification-launch-7e9su54_`: one normal greeting
  and exact Unicode copy/read-back passed both file contracts and the independent
  byte/workspace oracle. This is a single narrow result, not all-model qualification.
- Meta-Llama3.1-8B/Hermes `/tmp/offgrid-verification-launch-wk5y9vy1`: greeting passed;
  copy proposed incorrect bytes and was vetoed (`content_mismatch`). No copy was
  created; final receipt was unverified with nonzero exit. The hook's generic
  absolute-path advice was misleading, so shared code-specific rejection guidance
  now describes content mismatch without supplying replacement bytes or weakening
  validation. That failed evidence is retained; it is not relabeled as success.

OpenClaw still reports an isolated plugin-provenance warning (its fresh state has
no upstream install record) and intermittent SQLite reclamation warnings during
shutdown. These are not hidden or claimed fixed; the tests require the trusted
OffGrid bundle and independent outcomes despite them. Hermes reports its optional
tirith scanner unavailable. No claim of warning-free upstream runtime is made.

Checks run for this extension include full Windows Go tests, WSL integration/CLI
race tests, 9 Node provider/hook tests, 20 Python provider/oracle/hook tests,
installed hook and managed-launch fixtures, 21 documentation/layout/hygiene
checker fixtures and their repository checks, source-size, strict OpenSpec and
`git diff --check`. Final reruns and any further real-model outcomes are recorded
below. Native Windows/macOS external-runtime launches, broad real-model workflow
qualification and UI/desktop end-to-end checks were not performed for this CLI
extension. Earlier live-workflow and commit/release gates remain open.

Final reruns with code-specific rejection guidance:

- `build/verification-contract-cli-v2` SHA-256:
  `7c983b5f451c9ed464a3a4b37270ac8cf74ff18f096163aa428ad17b9451f75f`.
  The earlier source CLI used for the Qwen success was
  `312316c9a824903cf31cfdf883b4aaa236f2f7d2addaff0564f7577ca8201a67`.
- Deterministic Hermes `/tmp/offgrid-verification-launch-iw52asme` and OpenClaw
  `/tmp/offgrid-verification-launch-q51sc7q8`: all launch/oracle/false-completion/
  cancellation/preservation checks passed; each cancelled connection closed in
  2.03 seconds. One earlier Hermes rerun completed those behavioral checks but
  hit concurrent JSON evidence writes from auxiliary requests; the test fixture
  now locks and atomically publishes its evidence, with a concurrency regression.
- Real Hermes/Meta-Llama `/tmp/offgrid-verification-launch-5f0r418r`: greeting
  passed again, but the copy still proposed incorrect bytes and was blocked.
  Its explanation also confused display line numbers with source content. The
  new error describes the actual mismatch; it does not fix model reasoning.
  Task 11.3 and the older workflow/commit gates remain unchecked. No repeated
  effects or fabricated passing receipt were used to make the test succeed.

Commands (from the repository root; Go installed-hook/race and Python launch
commands ran in WSL, source binaries were not installed):

```sh
go test ./...
go test -race ./internal/integrations/... ./internal/cli ./cmd/offgrid -count=1
OFFGRID_HERMES_TEST_ROOT=/home/phil/.hermes/hermes-agent OFFGRID_OPENCLAW_TEST_ROOT=/home/phil/.npm-global/lib/node_modules/openclaw go test ./internal/integrations -run TestInstalledVerification -count=1 -v
node --test scripts/test/test-openclaw-provider.mjs scripts/test/test-verification-hooks.mjs
python3 -B -m unittest scripts/test/test_verification_hooks.py scripts/test/test_hermes_provider.py scripts/test/test_external_agent_oracle.py scripts/test/test_external_agent_fixture.py scripts/test/test_external_model_matrix.py scripts/test/test_tool_result_fidelity.py
python3 -B scripts/test/test-verification-launch.py build/verification-contract-cli-v2 --agent hermes
python3 -B scripts/test/test-verification-launch.py build/verification-contract-cli-v2 --agent openclaw
python3 -B scripts/test/test-verification-launch.py build/verification-contract-cli-v2 --agent hermes --live --server http://127.0.0.1:11611 --model Meta-Llama-3.1-8B-Instruct-Q4_K_L-f1e3673a9589
python3 -B scripts/test/test-verification-launch.py build/verification-contract-cli --agent openclaw --live --server http://127.0.0.1:11611 --model Qwen2.5-7B-Instruct-Q4_K_M
node --test scripts/maintenance/check-docs.test.mjs scripts/maintenance/check-repository-hygiene.test.mjs scripts/maintenance/check-application-layout.test.mjs scripts/maintenance/check-developer-layout.test.mjs scripts/maintenance/check-source-size.test.mjs
node scripts/maintenance/check-docs.mjs
node scripts/maintenance/check-application-layout.mjs
node scripts/maintenance/check-developer-layout.mjs
node scripts/maintenance/check-repository-hygiene.mjs
node scripts/maintenance/check-source-size.mjs
openspec validate external-agent-integration-foundation --strict --no-interactive
git diff --check
```

The new Hermes hook directory initially matched the broad `offgrid-*` binary
ignore rule. An explicit source-directory exception now exposes its three source
files; Python bytecode remains ignored and is not embedded. Earlier incidental
bytecode was recoverably moved to a private temporary directory, not committed.

### Host outcome-verification core, 2026-10-24 (not activated)

After the rejected v2 experiment, the approved extension adds read-only host
verification under `internal/integrations/verification`. This is a tested core,
**not a deployed or end-to-end fix**. Normal managed commands, first-party
plugins, model settings, the running seq2/v1 container/WSL CLI and desktop remain
unchanged in this pass. No staging, commit or publication occurred.

Versioned contracts cover exact file copying or no changes to a bounded workspace.
The verifier retains digests/metadata rather than file contents. Exact copying
requires a complete source-read event before an exact-byte write proposal, a
correlated write completion, destination read-back and independent final checks.
It rejects copied display prefixes, read-command JSON, Unicode normalization
changes and truncation without repairing arguments. Host permission rejection
before dispatch is distinct from failed/unknown write effects; neither triggers
an automatic retry. Missing/replayed/cross-session events and unexpected final
workspace changes cannot produce a passing receipt. External zero exit is not
proof of completion.

Filesystem checks use bounded traversal through `os.Root`, regular-file and
hard-link checks, root identity checks and independent content hashes. Tests
cover Unicode, genuine numeric prefixes, whitespace, empty files, existing
destinations, symlinks, hard links, special files, source/permission changes,
path escape, missing/contradictory completions, cancellation and concurrent
calls. Windows prevents the attempted root rename with its open directory
handle; Linux exercises replacement detection. A Linux permissions fixture
initially chose an already-current mode; it now toggles a measured bit.

Verification is not a sandbox, an atomic filesystem snapshot, generic semantic
task verification or proof that unseen transient/out-of-workspace effects never
occurred. Real hook integration and an agreed explicit invocation interface are
still pending. The audit confirmed that Hermes model-provider plugins cannot
silently act as enabled general hook plugins, and both runtimes have hook
failure paths that continue execution. Final verification must not rely solely
on a hook veto. No installed upstream code or personal configuration was edited.

Actual checks in this pass:

- Windows `go test ./...`: pass (unchanged package results may be cached).
- Windows `go test ./internal/integrations/... -count=1`: pass.
- WSL `go test -race ./internal/integrations/... ./internal/cli ./cmd/offgrid -count=1`: pass; the verification/integration subtree was rerun after the final permission-rejection case and passed.
- `node scripts/maintenance/check-docs.mjs`: 251 maintained files, no problems; external URLs/anchors not checked.
- Repository-hygiene and source-size scripts: pass. CI race coverage now includes the verification subpackage.
- `openspec validate external-agent-integration-foundation --strict` and `git diff --check`: pass.
- No new live Hermes/OpenClaw run, native macOS qualification, deployment, UI/desktop test or publication was performed. Existing failed-model evidence and commit gates remain open.

### Result-fidelity v2 activation and rejection, 2026-10-24

The authorized opt-in `llama31-result-fidelity-v2` candidate was activated and
tested, then **rolled back after failing acceptance**. It preserved native
messages, tool schemas/results and user/system content; only pinned,
adapter-owned interpretation guidance changed. No results were stripped,
generated writes corrected, models switched implicitly, or success fabricated.
The loaded worker reported `b1-b95502b-offgrid-seq2`, context 65536 and template
SHA-256 `2df5298901b69af9c7d8a405c9499031df23f92b5785ce6f7c57c8a14e367c13`.
An existing startup log incorrectly printed the v1 constant; source now logs the
actual selected template filename. The independent `/props` digest, not that
misleading log, identified the tested revision.

Frozen synthetic cases use the same installed Meta-Llama 8B, temperature zero,
256 output tokens, schemas and input histories; returned calls are never
executed. Baseline seq2/v1 passed **3/7**; v2 passed **2/7**. Both failed plain
Unicode, genuine numbered text and multiline copying by including display
prefixes, and falsely answered MATCH for contradictory read-back. Both passed
greeting abstention and truncated-read continuation. v2 additionally failed the
exact-receipt case's required one-word format (its prose correctly said MATCH;
this is a format failure, not another incorrect comparison). Private evidence:
`/tmp/offgrid-result-fidelity-smj6sfer` (baseline) and
`/tmp/offgrid-result-fidelity-_zxzdrlj` (v2). No fixture expectation was relaxed.

Fresh installed tests used disposable homes/workspaces and the same file-only
tool scopes as the earlier comparisons, not personal files or the complete
user tool catalog. Greeting logs and dependent tool traces were reviewed in
addition to the independent byte/workspace oracle:

| Runtime / model | Greeting | Exact-file workflow | Wall time, greeting / workflow |
| --- | --- | --- | --- |
| Hermes / Meta-Llama 8B | Pass | **Fail** | 8.80 / 21.45 s |
| OpenClaw / Meta-Llama 8B | Pass | Pass, one run | 21.34 / 29.84 s |
| OpenClaw / Qwen2.5 7B | Pass | Pass, one run | 16.92 / 17.38 s |

Hermes wrote a JSON representation of a read command into the destination
without reading the source, then read its own 132-byte output and falsely
claimed an exact match with the 38-byte source. The source and unrelated files
were preserved. This is a dependency/evidence failure despite valid sequential
schemas. OpenClaw/Meta-Llama instead read the source, wrote its exact 38 bytes,
read the destination and returned a correct comparison. Qwen uses its unchanged
native template and passed the compatibility check. One successful run does not
qualify either model/runtime generally. OpenClaw's upstream SQLite shutdown
maintenance warning remains visible and unresolved.

Private workflow/wire evidence, respectively:

- Hermes: `/tmp/offgrid-hermes-qualification-pct502t4`,
  `/tmp/offgrid-private-wire-0yzfs391`.
- OpenClaw/Meta-Llama: `/tmp/offgrid-openclaw-qualification-fg0wkxmp`,
  `/tmp/offgrid-private-wire-hr5xt3ef`.
- OpenClaw/Qwen: `/tmp/offgrid-openclaw-qualification-as_4rd90`,
  `/tmp/offgrid-private-wire-jaz07oj1`.

The candidate was `offgrid-llm:local-external-gpu-20261024-fidelity2`, image
manifest-list `a141f038dcd59daf63ab1db1d52de341c211ec566e76f7b07feab7667e9a1855`,
config `4de375a982291859fa7fa864c4bbb99f4155e1f25576c76c9ad92f805d74e015`,
service/CLI SHA-256
`a364a9875dc74873fb81a01ce8e1afb01e625e49d2e46ee7e8586cdc9a547332` and local
build identity `0.4.19-dev.fidelity2` (not a release/version-file change).

The first replacement attempt safely stopped at clone restoration: the previous
local service's `dev` build identity intentionally cannot restore through the
standard matching-version guard. It restarted the old service unchanged; its
private verified archive is retained at
`/home/phil/offgrid-result-fidelity-20261024-e_u6vgct`. The next attempt kept an
original old-build archive and took a second snapshot using the explicitly
versioned candidate's maintenance command. Every file digest/size and total size
matched before the normal candidate-version restore and SQLite checks created
the new clone. No restore guard was weakened or live workspace overwritten.

Candidate activation and rollback both preserved 5 models, 17 conversations and
28 tasks. Recovery evidence is retained at
`/home/phil/offgrid-result-fidelity-20261024-bqkxm762`; rejected candidate container
`offgrid-rejected-fidelity2-1792779990` remains stopped. The running service is
again **seq2/v1**, with its original matching WSL CLI
`b58b2e9bb0e47571aa85c9a58bb0c719811e4d6447e730ae26d7774cdbc734aa`.
Rollback did not restore old data over newer data. External configuration hashes
still match the provider-update baseline. Desktop and installed provider plugins
were unchanged by this candidate pass. No commits or publication occurred.

Also recorded: before v2 activation, seq2/v1 Qwen/OpenClaw passed another
greeting/exact-file check (19.03 / 17.08 s), with private evidence
`/tmp/offgrid-openclaw-qualification-n74a63zw` and
`/tmp/offgrid-private-wire-nojvxvou`. This does not qualify Hermes or Meta-Llama.
Tasks 5.4, 9.3 and 10.3 remain open: prompt framing is not a deterministic
verification boundary, and the known semantic failures are not fixed.

Final source checks in this pass passed:

- Windows `go test ./...` and targeted inference/server/integrations/CLI/cmd
  tests.
- WSL `OFFGRID_TEST_PINNED_TEMPLATE=.../build/linux/pinned-llama31.jinja go test
  -race ./internal/inference ./internal/server ./internal/integrations
  ./internal/cli ./cmd/offgrid`, including the actual pinned-template fixture.
- `node --test scripts/test/test-openclaw-provider.mjs` (2 tests).
- `python3 -m unittest scripts/test/test_hermes_provider.py
  scripts/test/test_external_model_matrix.py
  scripts/test/test_external_agent_oracle.py
  scripts/test/test_tool_result_fidelity.py scripts/test/test_llama_patch.py`
  (18 tests).
- `node scripts/maintenance/check-docs.mjs` (251 maintained documents),
  `check-repository-hygiene.mjs`, `check-application-layout.mjs`,
  `check-developer-layout.mjs` and `check-source-size.mjs`.
- `openspec validate external-agent-integration-foundation --strict` and
  `git diff --check`.

Source checks are not a substitute for the failed live cases. UI/desktop suites,
new real Windows/macOS native qualification and the full cross-model workflow
suite were not rerun in this adapter-only follow-up. The prior actionlint
download failure remains unverified; no merge/release readiness claim is made.

### Revision-2 installed checks, 2026-10-24 — workflow gate still open

The CPU and CUDA-built parser suites each pass **117 tests / 583 assertions**,
including native-envelope trigger slicing, schema range/cardinality checks,
ordinary prose exclusions and required/optional modes. Trigger slicing tests
exercise the runtime regex utility and grammar separately, not model sampling.
Restoring the seq1 generator with the new tests reproduces two missing-grammar
failures. Patch admission tests (3), provider Node/Python tests (2/2), targeted
Windows Go tests and WSL race tests (inference/server/integrations/CLI/cmd), docs,
layout/hygiene/source-size, strict OpenSpec and diff checks pass.

After idle checks and verified backup/clone comparison, the authorized update
installed `offgrid-llm:local-external-gpu-20261024-seq2` (image config
`9ab726cd4c0e190187e6e1966883e7cba8ad0db8fcdeb9d55863b2c5c23c3bbb`) and matching
WSL CLI (`b58b2e9bb0e47571aa85c9a58bb0c719811e4d6447e730ae26d7774cdbc734aa`).
Both workers report `b95502b-offgrid-seq2`. Recovery:
`/home/phil/offgrid-native-sequencing-20261024-n3qm68nq`; previous stopped
container `offgrid-before-seq2-1792777584`. Five models, 17 sessions and 28 tasks
were preserved. Installed Hermes/OpenClaw statuses both return preflight ready;
that is not workflow qualification. Desktop and external settings unchanged.

Meta-Llama 8B/Hermes now performs valid, sequential read/write/read-back calls.
Its greeting passed (9.40 s), but the exact-file workflow **failed** (18.63 s):
the model wrote the tool's `1|` line-display prefix into the destination, then
claimed an exact match despite read-back showing `1|1|...` and a 40-byte result
for a 38-byte source. Source and unrelated files were preserved. Hermes's tool
description already documents `LINE_NUM|CONTENT`; its upstream write guard
deliberately permits a single line of that form because it can be genuine file
content. Do not silently strip prefixes or confuse schema compliance with
semantic correctness. Private evidence: `/tmp/offgrid-hermes-qualification-mf0hlf6d`
and `/tmp/offgrid-private-wire-k21zr2at`. Cold preparation took 22.11 s in this
run; timing differs with cache/load state and is not a performance guarantee.

Meta-Llama 8B/OpenClaw **failed the greeting preservation gate** (48.91 s):
it used valid sequential write calls to create `message.log` and `AGENTS.md`
inside the disposable test workspace before answering. The harness stopped
before the file test. No personal workspace was used. This is unnecessary tool
use/model behavior, not a parser success or acceptable greeting. Evidence:
`/tmp/offgrid-openclaw-qualification-3savfn30` and
`/tmp/offgrid-private-wire-s03wu6gr`. The upstream SQLite shutdown maintenance
warning also persists. These failures keep tasks 5.4/9.3 and the commit gate open.

### Installed seq1 result and bare-JSON follow-up, 2026-10-24

The separately approved replacement installed
`offgrid-llm:local-external-gpu-20261023-seq1` and matching WSL CLI (SHA-256
`eb65ac3d85288d506a2dee05477262186f7205236f833071919a8b4783fc4ab6`). Both
CPU and CUDA workers report `b95502b-offgrid-seq1`. Verified backup/clone/live
inventories preserved five models, 17 sessions and 28 tasks. Recovery is retained
privately at `/home/phil/offgrid-native-sequencing-20261023-lxtttujv`, with the
previous container stopped as `offgrid-before-seq1-1792767251`. Desktop unchanged.
The user also approved updating only the first-party provider plugins. Their
backup is `/home/phil/offgrid-provider-seq1-sn4jx_gs`; external configuration
hashes before and after matched. No settings, credentials or personal workspaces
were edited.

One real Qwen2.5-7B/OpenClaw run passed a normal greeting and the independent
Unicode exact-file oracle, with the normal tool-search surface. Four separate
decisions read source, wrote the actual returned text, read destination and then
reported a match. Original source and all unrelated workspace entries were
preserved. Private evidence: `/tmp/offgrid-openclaw-qualification-3n9u3iqe` and
`/tmp/offgrid-private-wire-smuxft7d`. Greeting/workflow wall times were
36.20/34.48 seconds, including external runtime overhead; the corresponding
generation times were about 3.46 seconds and 1.77/1.90/1.12/1.55 seconds. This
single pass is not general model qualification. The upstream SQLite shutdown
maintenance warning remains visible and has not been suppressed.

Meta-Llama-3.1-8B/Hermes passed the greeting but **failed the file workflow**:
all three attempts were rejected before tools ran. A synthetic direct-worker
replay, executing no tools, returned `limit: 1000000` against the tool schema's
maximum of 2000. Another captured response chained calls despite the sequential
flag. Inspection found that optional untagged native JSON has no lazy grammar
trigger in the pinned generator. The revision-2 candidate adds a narrowly
anchored native-envelope trigger; validation remains strict, without clamping
arguments, dropping calls or extracting tool commands from ordinary prose.
Task 9.1 was reopened. Private evidence: `/tmp/offgrid-hermes-qualification-l6jpkezj`
and `/tmp/offgrid-private-wire-nyxl1_a5`. Greeting/workflow wall times were
17.41/45.93 seconds; cold preparation was 74.58 seconds. Prior partial-copy and
false-completion failures remain unresolved until a fresh exact-file test passes.

Additional seq1 checks passed: targeted WSL race tests (inference, server,
integrations, CLI, power and cmd/offgrid), web API/type checks, desktop 71/71,
documentation 251 files and structural checks. Actionlint could not be acquired
because the Go proxy timed out. No commit, release or broad qualification is
claimed; revision-2 results must be recorded separately.

### Approved native sequencing repair, 2026-10-23

The user approved extending the current change with a pinned runtime repair
and first-party provider hooks. The implementation uses revision
`b95502ba9aa0eb73a2f4fc8878d7fbe6a847a0b9-offgrid-seq1`: per-call JSON repetition
is bounded when parallel calls are disabled, and optional calls allow ordinary
text. The inner call parser is required; optionality belongs to the outer
repetition. CPU/CUDA Docker and release builds apply the same checked patch.
Unknown source revisions, dirty patch targets and repeat application fail closed.
No returned call is silently dropped, and final OffGrid validation is unchanged.

Both the CPU and CUDA-built upstream parser suites passed **116 tests / 552
assertions**, including 28 new assertions. Reversing only the generator repair
in a disposable container produced **six failures** with the same tests. These
check parsing and generated grammar without executing a model or tool. A test
filter initially skipped nested assertions; that run was discarded and replaced
by the complete unfiltered suites. Native Windows/macOS builds remain unverified.

Hermes's supported provider kwargs hook and OpenClaw's `prepareExtraParams`
default to sequential calls only when not explicitly overridden. Unit tests
cover true/false/null preservation, aliases, callbacks and unmodified inputs.
Installed-runtime deterministic tests passed repeated setup, smoke, greeting,
independent Unicode file read-back and cancellation. Private evidence:
`/tmp/offgrid-hermes-qualification-jrf9pgns` and
`/tmp/offgrid-openclaw-qualification-vq2sbode`. Each captured five tool-enabled
requests with the expected false flag. Cancellation exited in 2.02/2.07 seconds
respectively and the fixture observed inference-connection closure. These are
fixture results, **not real-model workflow qualification**.

Windows `go test ./...`, provider Node tests (2), Python provider/matrix/oracle/
patch tests (14), documentation/layout/hygiene/source-size checks and strict
OpenSpec validation passed. WSL actionlint acquisition timed out at the Go proxy;
that attempt is not a validation pass. Additional race/build and real-model
results are recorded below when completed. The user separately approved replacing
the idle container and matching WSL CLI for isolated real tests. No commit or
publication is authorized while known workflow failures remain.

### Commit hold and native sequencing diagnosis, 2026-10-23

The user withdrew the proposal to publish an RC with known failures: fix the
workflows before committing. No staging, commit, version bump, publication,
deployment or installed-agent configuration change occurred in this follow-up.
Task 5.4 remains open; the running service is still r8.

The disposable-workspace oracle now compares both source and destination with
the immutable original bytes, rejects links in place of a destination file, and
reports unexpected created, changed or removed workspace entries. Previously,
comparing the two files alone could pass if both had been changed. The greeting
check also records workspace preservation. A nonzero agent exit no longer
prevents independent outcome inspection. Semantic greeting quality still needs
review; neither exit 0 nor unchanged files proves a sensible reply.

A new private wire capture used OpenClaw 2026.10.4 and the explicitly selected
Qwen2.5 7B in a disposable home, with the same permitted file tools. Evidence:
`/tmp/offgrid-private-wire-8insme18` and
`/tmp/offgrid-openclaw-qualification-ldx916r5`. Setup twice took 67.07/65.23 s;
status 20.76 s; smoke 42.95 s; post-smoke status 13.85 s; greeting 56.80 s;
file workflow 61.98 s. These include external CLI startup, not just inference.
The greeting was normal. The file workflow **failed**: one response contained
read-source, write-placeholder and read-destination calls before any source
result was available. The subsequent request contained the correct Unicode
source and the incorrect destination, but the model still claimed success.
The captured request omitted `parallel_tool_calls`. The harness was already
running when the stronger oracle was edited; the new oracle is not claimed to
have executed in that run.

A bounded replay of that synthetic request with `parallel_tool_calls: false`
was rejected by OffGrid's complete-decision validator. A diagnostic request to
the same idle worker, with no returned tool executed, confirmed the raw native
response still contained three calls. The pinned upstream
[parser generator](https://github.com/ggml-org/llama.cpp/blob/b95502ba9aa0eb73a2f4fc8878d7fbe6a847a0b9/common/chat-auto-parser-generator.cpp#L192-L216)
repeats per-call JSON wrappers without conditioning that repetition on the
parallel-call flag. This explains the observed cardinality violation. Simply
setting the flag in a provider hook would reject the batch, not repair the
workflow. Do not drop later calls, infer dependencies from tool names, or weaken
validation to make this appear successful. A pinned runtime repair and
provider-level sequential policy need an explicit extension of the current
template-adapter scope. Hermes's partial Unicode copy and false completion are
separate, still-open semantic failures; the parser finding does not qualify them.

Checks passed in this follow-up: Windows `go test ./...`; the CI-listed WSL
`CGO_ENABLED=1 go test -race` packages (sessions, rag, server, users, cache,
agents, runs, storage, serviceclient, models, audio, computer, inference,
integrations, cli, power, degradation and cmd/offgrid); web `api:check`, `check`
and `build`; desktop tests (71/71); documentation (250 maintained files),
repository hygiene/layout/developer layout/source-size checks; strict OpenSpec
validation; and diff whitespace checks. The web build retained its large-chunk
warning. Windows race testing was unavailable with CGO disabled and was run in
WSL instead. Python provider/matrix/oracle tests passed 10/10 in WSL. These are
regression results, not live-workflow acceptance or native desktop/platform
qualification. No release readiness is claimed.

### GPU/cache and admission follow-up, 2026-10-23

The RTX 5060 is now visible to Windows, WSL and a cached CUDA Docker probe.
The authorized replacement uses `offgrid-llm:local-external-gpu-20261023-r4`,
the same pinned b95502b worker, the existing five-model and workspace volumes,
one loopback service on port 11611, and `q8_0` KV caching. The registry download
for the normal GPU build timed out; the local image was assembled offline from
the cached pinned CUDA runtime, current Go binary and previous unchanged UI.
This is a local deployment, not a published release artifact.

An isolated, same-model CUDA comparison reproduced corrupted Qwen 7B answers
with `q4_0`: all three identical, temperature-zero synthetic greeting requests
failed. With `q8_0`, all three returned the exact requested marker. The two
workers had the same context, tool schema, model weights, CUDA image and flags
apart from cache precision. This establishes a failure of this tested q4 cache
configuration, not that every q4 weight file or every GPU is defective. The
template-escaping hypothesis was ruled out by byte-level rendered-prompt checks.
The Docker guide now recommends the existing q8 default for agent testing.

Load admission now accounts for automatic CPU/GPU layer splitting with host,
driver and transfer reserves. Explicit placement retains separate-pool checks.
Zero reported free VRAM is never replaced by total VRAM. After eviction,
capacity observation retries for at most three seconds to allow driver memory
release, respecting cancellation and never replaying generation. The worker
still confirms the requested context. Meta-Llama 8B loaded with an independently
reported 65,536-token slot using q8, and the service reports the same allocation.
Malformed native tool streams now return a redacted, classified error rather
than the generic incomplete-answer message. Tool validation remains intact.

**End-to-end qualification still fails.** On q8, the real runs reach responses
and some tool work, but independent file checks reject their completion claims;
the Meta/OpenClaw run also logged a runtime-stream error before its own retry:

- Qwen 7B/OpenClaw: normal discovery file run 21.71 s; direct-tool run 18.68 s.
  Neither produced the exact destination file. Evidence is private at
  `/tmp/offgrid-openclaw-qualification-f39ae8rs` and
  `/tmp/offgrid-openclaw-qualification-jygp3hiv`.
- Meta-Llama 8B/Hermes: smoke 15.57 s including cold preparation; file run
  28.54 s. It wrote a placeholder describing a read result instead of the source
  bytes, then claimed success. Evidence:
  `/tmp/offgrid-hermes-qualification-6o99g8nz`.
- Meta-Llama 8B/OpenClaw: smoke 13.41 s; file run 54.25 s; independent file
  verification failed. Evidence:
  `/tmp/offgrid-openclaw-qualification-fofpmqwl`.

These are external model/tool-planning failures remaining after the runtime
fixes. Schema validation cannot prove that a valid string argument contains the
right file contents. No model/default/tool list was changed in installed agent
configuration. The user's home-directory bootstrap write failures were not
replayed. The explicit test workspaces contain all test writes. The earlier
Qwen probe overlapped a user Meta request; that run is not used as clean latency
or model-switch qualification evidence.

Validation passed: Windows `go test ./...`; WSL
`go test -race ./internal/inference ./internal/server ./internal/resource` and
the subsequent focused race checks for the final hybrid-admission change;
documentation, source-size, repository-hygiene, OpenSpec and diff-whitespace
checks. UI/native-desktop checks were not rerun for these server-only changes.
Hardware qualification beyond this one Windows/WSL/RTX setup remains unverified.

The installed WSL CLI and container application share SHA-256
`08169f1455850ab2a2fb7742abdeb109dad7825fcb67dd55ad8d95c24cf935f1`.
The CLI backup is `/home/phil/.offgrid/backup/offgrid-cli-before-kvfix-20261023`.
The original CPU and intermediate GPU containers remain stopped for recovery;
the pre-switch workspace backup is
`/home/phil/.offgrid/backup/gpu-switch-20261023/workspace.zip`.
No desktop replacement, commit or publication occurred in this follow-up.
Task 5.4 remains unchecked because full file workflows are not accepted.

### Template follow-up, 2026-10-23

The user's Qwen/OpenClaw greeting returned a normal answer. Its later
`maintenance-age` SQLite warning originates in installed OpenClaw's maintenance
worker after database execution admission closes, not in OffGrid inference.
Installed source has the throw at `openclaw-agent-execution-CaPKglUL.mjs:1282`
and the worker warning in `session-accessor.sqlite-reclamation-worker-diagnostics-hJYsjFvr.mjs`.
The ordering suggests shutdown/maintenance coordination; it does not prove
database corruption or establish a completed file workflow. Upstream code and
personal databases were not altered, and warnings were not suppressed.

The user's Hermes/Meta greeting instead returned commentary about not needing
a function. Three short scheduled synthetic requests against the same resident
Meta/q8/65,536 runtime reproduced the distinction: plain chat answered normally
(2.03 s), native optional tools called `lookup_fixture` for `hello` (1.41 s), and
explicit abstention instructions still called that synthetic tool (1.42 s).
No function was executed. The embedded template prepends a function-call-only
directive to the first user message; this is a concrete compatibility suspect,
not evidence that every failure is intrinsic model quality.

The user approved a pinned, opt-in template adapter and isolated installed tests.
Its source template hash is
`93c0e9aa3629bbd77e68dbc0f5621f6e6b23aa8d74b932595cdb8d64684526d7`;
runtime is `b1-b95502b`. Qualification remains open until the adapted workflow
checks run. Existing failed results remain above.

### Installed experimental template adapter, 2026-10-23

The final local candidate is `offgrid-llm:local-external-gpu-20261023-r8`.
It explicitly enables `OFFGRID_TOOL_TEMPLATE_ADAPTER=llama31-optional-tools-v1`.
The container application and WSL CLI SHA-256 are both
`0f600c4b384cb4fd196bc8079998bb503b0759512bc6b05e88a41dc873d0cc29`.
The metadata-selected adapter matches the architecture, exact reviewed template
and pinned runtime, not a model display name. Its adapted template digest is
`16097d6b9fcdeb4f0feccd05d1cfbe2104014e2942aec584309301b37a45c8c4`.
The raw GGUF representation has one trailing newline and digest
`e10ca381b1ccc5cf9db52e371f3b6651576caee0a630b452e2816b2d404d4b65`;
only that positively matched representation is normalized. Unknown identities
remain on the native path. Model files, external defaults and user instructions
are unchanged. Native result serialization and tool validation remain intact.

Safe replacement verified idle inference, backed up and verified the workspace,
restored a disposable clone, and compared the five-model, 17-session and 28-task
inventories before replacing the service and CLI. Recovery files are private at
`/home/phil/offgrid-template-adapter-20261023-9uz5iskx`; the previous stopped
container is `offgrid-before-adapter-1792755353`. Earlier adapter test backups
remain available. There is one running OffGrid service, bound to loopback.
The desktop app and installed Hermes/OpenClaw configuration were not replaced.

The r6 candidate improved optional-tool greetings, but returned extra quotes in
the synthetic tool-result echo and failed Hermes's independently checked file
copy (`/tmp/offgrid-model-matrix-a1bdiz75` and
`/tmp/offgrid-hermes-qualification-jqlctjl0`). An r7 experiment changed plain
string result serialization. An actual rendered-prompt check confirmed exact
Unicode reached the model, but model continuation failed with invalid PEG-native
output. The serialization experiment was **removed**, not accepted as a fix.
r8 restores the native serialization and is binary-identical to r6. r7 evidence
is retained at `/tmp/offgrid-model-matrix-ybkqwj04`,
`/tmp/offgrid-hermes-qualification-t582jkkd` and
`/tmp/offgrid-openclaw-qualification-5s7wze86`.

The r7 comparison also tested Qwen2.5 7B, whose unknown template correctly kept
its native path: marker greetings, streaming, a required synthetic call, ASCII
and Unicode result continuations, and cancellation passed. Meta-Llama 8B passed
greeting/call/cancellation but failed both result continuations. These are narrow
synthetic cases, not universal or external-workflow qualification. Qwen's actual
32,768 context remains below the Hermes requirement; no context was inflated.

Final r8 matrix evidence: `/tmp/offgrid-model-matrix-mht5q_js`. Preparation
took 8.529 s; the two marker replies took 0.695/0.329 s, streaming 0.334 s,
required synthetic call 1.868 s and cancellation case 0.325 s. These passed.
Unicode result continuation passed (0.966 s), but the ASCII result echo added
quotes and failed exact comparison (0.717 s). No synthetic tool was executed.
This final run supersedes the removed serialization experiment, not its history.

Final r8 Hermes evidence: `/tmp/offgrid-hermes-qualification-03syw58e`.
Setup twice, status and smoke passed. The actual greeting with file tools present
returned a normal answer and no tool calls in 8.39 s. The file workflow took
57.01 s and exited 0 but **failed independent read-back**. Read-only inspection
of this disposable session's tool messages confirmed the complete 38-byte
Unicode source reached the agent, with `truncated: false`. Its write call instead
contained the line-number prefix and only part of the text, producing 25 bytes.
It read that incorrect destination and still claimed an exact match. This is a
concrete content-selection/verification failure, not missing source bytes or an
authorization rejection. The external tool's `verified: true` reports its write,
not equivalence to the user's requested source. Neither that flag nor process
exit is accepted as outcome verification.

Final r8 OpenClaw evidence: `/tmp/offgrid-openclaw-qualification-3e4__yhk`.
Setup twice, status and direct-provider smoke passed. Its actual `hello` run
exited 0 after 37.87 s but answered with an unrelated claim about `SOUL.md`, so
greeting quality **failed**. The disposable workspace contained a new two-byte
`SOUL.md`; this was unwanted file work for a greeting, not merely odd prose.
No personal workspace or installed external-agent configuration was used.
The isolated file workflow exited 1 after 64.23 s
with invalid tool/runtime responses and the external runtime's own bounded
retries. An independent check found no destination file. The upstream SQLite
shutdown warning remains visible. The r7 normal greeting was not repeatable
qualification and is not substituted for this final failure.

All final tests ended with healthy idle service state: active 0, queued 0,
switching false, one service and the confirmed 65,536-token Meta allocation.
Installed build hashes match. **Task 5.4 remains open: neither external agent is
qualified for the tested file workflow, and OpenClaw/Meta greeting quality is
still unreliable.** The runtime can now produce replies quickly in synthetic
cases, but removing the old template's mandatory-call framing did not solve
every tool-planning or content-fidelity failure. Do not use this as an accepted
external-agent demo or release claim. Another model is not selected implicitly.

Validation for the final candidate passed: Windows `go test ./...`; WSL
`CGO_ENABLED=1 go test -race ./internal/models ./internal/inference ./internal/server`;
`python3 -m unittest scripts/test/test_hermes_provider.py scripts/test/test_external_model_matrix.py`;
`node scripts/maintenance/check-docs.mjs`, `check-repository-hygiene.mjs` and
`check-source-size.mjs`; and strict OpenSpec validation. The Windows Python
launcher was unavailable, so Python checks ran in WSL. A temporary Go inspection
helper initially triggered the hygiene check; that helper was removed and the
check passed. UI, native desktop and other hardware/platform qualification were
not rerun for this server/template-only change. No commit, push or publication.

Implementation lives in the active
[external-agent-integration-foundation change](../../../openspec/changes/external-agent-integration-foundation/proposal.md).
The implementation evidence below is not release qualification. The separately
authorized local deployment is recorded in the final section.

- The installed container logged a battery transition at **98%** charge at
  05:34:45 UTC; subsequent integration and model endpoints advertised 4096
  tokens. Default battery policy now reduces concurrency without the old 4096/
  2048 context caps. Explicit policy and model/resource limits remain enforced.
- Source inspection found another false-pressure path: Go `Alloc/Sys` was
  treated as host RAM utilization. Degradation now uses measured OS available/
  total RAM, retaining the last pressure observation if measurement fails.
- Separate CLI paths selected different models and compared contexts for exact
  equality. Shared selection is explicit override, scoped managed connection,
  external configuration, then live service catalog. Safe conservative context
  allocations remain valid when service capacity grows.
- OpenClaw setup preserves unrelated models, their context limits and custom
  fields, other providers and the global model default. Hermes secrets are not
  sent in process arguments. Configuration has private snapshots and a journal,
  with no blind rollback over concurrent changes.

## Automated evidence

Commands run from the repository root unless noted:

- `go test ./...` — passed on Windows, including server, CLI, storage and
  integration regression tests. Targeted packages were rerun after final edits.
- `go test -race ./internal/integrations ./internal/cli ./internal/power ./internal/degradation ./internal/server ./internal/storage ./internal/serviceclient ./cmd/offgrid`
  — passed under WSL with `CGO_ENABLED=1`. Native Windows race execution was
  unavailable with the configured `CGO_ENABLED=0`; it was not called a pass.
- The complete expanded CI race command also passed under WSL with
  `CGO_ENABLED=1 go test -race -p 2 ./internal/sessions ./internal/rag ./internal/server ./internal/users ./internal/cache ./internal/agents ./internal/runs ./internal/storage ./internal/serviceclient ./internal/models ./internal/audio ./internal/computer ./internal/inference ./internal/integrations ./internal/cli ./internal/power ./internal/degradation ./cmd/offgrid`.
  Final integration/CLI edits were subsequently rerun under the race detector.
- `npm --prefix apps/web run api:check`, `run check`, `run build` — passed.
  Build retains the existing large-bundle warning; no performance claim added.
- `npm --prefix apps/web run test:e2e -- external-integrations.spec.ts functional-pages.spec.ts --workers=1`
  — 5 passed. Includes a third registry ID with generic setup/copy behavior,
  service/host/workflow separation and label availability in all nine locales.
- `npm --prefix apps/desktop test` — 71 passed.
- `python3 -m unittest scripts/test/test_hermes_provider.py` — passed in WSL;
  verifies dynamic chat-only discovery and authentication without network.
  The Windows `python` app-execution alias was unavailable.
- Documentation, application/developer layout, hygiene and source-size checks
  passed; their 21 unit tests passed. `node scripts/release/test-release-ci.mjs`
  passed. `git diff --check` passed.
- `openspec validate external-agent-integration-foundation --strict --no-interactive`
  passed; this validates artifacts, not runtime behavior.

## Installed-runtime compatibility, deterministic inference

Each invocation of `scripts/test/test-external-agents.py` used a fresh private
home, managed-state directory and disposable workspace. No personal agent
configuration was imported. Only file tools were enabled for the Unicode test.
The fixture supplies known OpenAI-compatible responses; these results do not
measure model intelligence or inference quality.

| Runtime | Repeat setup | Model reply | Unicode file/read-back | Cancellation after generation began |
| --- | --- | --- | --- | --- |
| Hermes v0.21.2 (2026.9.11), upstream f364c197, Python 3.11.16 | passed twice | passed | passed, independently byte-compared | exit 130, 0.03 s |
| OpenClaw 2026.10.4 (383c124) | passed twice | passed | passed, independently byte-compared | exit 130, 0.02 s |

Hermes fixture evidence: `/tmp/offgrid-hermes-qualification-uv0qjf80` in WSL.
OpenClaw fixture evidence: `/tmp/offgrid-openclaw-qualification-f7d5xrvy` in WSL.
Private logs/snapshots remain outside the repository; they are not release assets.
Both status checks retained the configured model, and smoke evidence remained
separate from workflow qualification. CLI helper cancellation also passed on
Windows; this does not qualify installed Windows external runtimes.

## Real-model evidence and limitations

An isolated temporary service used the locally available container image plus
the newly built service binary, the existing model volume **read-only**, private
data and loopback port 11612. The installed service at 11611 was not replaced.

OpenClaw with `qwen2.5-3b-instruct-q4_k_m`, CPU, admitted 32768-token context:

- First configuration, repeat configuration and status passed.
- Real model-reply smoke passed in 57.65 s including host inspection/startup.
- File task process exited normally in 216.31 s but returned prose and did not
  create the requested file. Independent verification **failed**. No successful
  tool-workflow qualification was recorded. Transport success cannot certify
  this model for a demo.

Evidence: `/tmp/offgrid-openclaw-qualification-ioyf28rp` in WSL.
Hermes's real-model test was blocked at preflight by the installed service's
4096-token allocation. The isolated service could not admit the resident 8B
model's required 64000 context alongside the existing service within available
WSL RAM; limits were not bypassed. Evidence of the original block is under
`/tmp/offgrid-hermes-qualification-l6hrjfzl`.

The temporary service container was stopped and removed after testing; its
ephemeral data was disposable. The existing service, model volume, personal
agent configuration and desktop installation remain unchanged.

## Qualification still required

Real-model tool workflows for both integrations remain unqualified. Native
Windows/macOS installed-runtime checks, user acceptance and independent locale
speaker review were not performed. Fresh network installation/update of external
runtimes was not exercised; tests used already installed runtimes, and fixture
tests cover the installer boundaries. No deployment, commit, push or release
was performed during implementation. The subsequent authorized deployment is
recorded below; it does not change the qualification limits above.

## Authorized local deployment — 2026-10-22

The user subsequently requested container and desktop replacement and explicitly
approved updating the WSL CLI. No Git publication or release was performed.

- Built the application with `docker/Dockerfile --target application-artifacts`
  and reused the installed native runtime through `docker/Dockerfile.local-runtime`.
  Image: `offgrid-llm:local-integrations-20261022`, version `v0.4.19`, revision
  `local-external-integrations-20261022`. This is a local dirty-checkout build,
  not the published release image.
- Preserved CPU mode, q4_0 KV cache, adaptive 65536 maximum context, the existing
  model/data volumes, loopback port 11611 and container security/restart settings.
  Stopped the original service, then used its exact image for workspace backup,
  verification and restore into a separate test directory. Clone and live checks
  retained the workspace ID, 5 model IDs, 17 conversation names and 28 task IDs.
  No saved task was resumed or reconciled.
- Private backup: `/home/phil/offgrid-deployment-20261022-vTUjHD` in WSL. It
  contains the verified workspace ZIP, container configuration, clone, prior WSL
  CLI and prior Hermes provider. Retained stopped prior container:
  `offgrid-before-integrations-20261022-1792650603`. Backup material may contain
  credentials and is not an ordinary export.
- Rebuilt Windows NSIS package with `electron-builder --win nsis --x64 --publish
  never`. The isolated packaged startup, browser-companion, light/dark/system
  appearance, 200% zoom and explicit-access checks all passed. Installer exit
  code was 0; Windows registration now reports 0.4.19. Installed `app.asar` and
  native companion digests match the tested unpacked package. This local build
  is unsigned, not signing-qualified.
- Retained previous desktop files in
  `D:\offgrid-backups\desktop-before-integrations-20261022-1432`.
  Relaunched the installed app; its processes connected to the container on
  11611, with no separate native OffGrid service. Both share UI build
  `b410e735b9799fa43c6258cc20966602a34c0141967a68c219d85b3c901fe297`.
- Updated `/home/phil/go/bin/offgrid` to the exact container application binary
  and refreshed only the already installed OffGrid-owned Hermes provider bundle.
  External runtimes, model choices and agent configuration were not replaced.

### Live inference and remaining blocker

At idle, the updated service advertised 65536 tokens for the selected Meta-Llama
8B model and 32768 for the other language models, including while on battery.
Hermes recorded a successful real-model smoke check. This does not verify its
tool workflow.

Two subsequent OpenClaw smoke attempts each received an HTTP 200 model reply,
but failed the post-generation stability check: the service temporarily
advertised only 4096 tokens after model loading. Status also temporarily blocked
Hermes. Source tracing identifies the remaining fixed reduced-pressure cap in
`internal/degradation/manager.go:MaxContextSize`, consumed by
`internal/server/runtime_context.go:modelContextWindow`. This is separate from
the corrected battery caps and requires further context-admission work; no
resource protections or integration minimums were disabled to conceal it.

Container health passed, with one active OffGrid container and no OOM flag.
Logs still include six existing conversation-recovery warnings (left untouched),
the expected container-internal non-loopback authentication warning (host port
remains loopback-only), and an unsupported `response_format` request plus a
cancelled model load from intervening activity. Logs are therefore not claimed
error-free. Deployment succeeded; reliable external-agent demo qualification
remains blocked by context instability and the unverified tool workflows.

## Launch/context regression follow-up and replacement — 2026-10-22

This section supersedes the earlier context-instability finding, not the failed
real-model tool-workflow qualification. No commit, publication or desktop
replacement was performed in this follow-up.

### Implemented and automatically tested

- Battery state and transient queue/memory pressure no longer rewrite context.
  New loads check measured available RAM (including Docker cgroup limits),
  model size and KV allocation; insufficient capacity returns an operational
  error without silently changing configuration. Larger resident allocations
  satisfy smaller requests without reloading. The llama worker's reported
  context must cover the requested allocation before activation succeeds.
- Default power policy is informational, including critical/unknown battery
  state. Windows uses GetSystemPowerStatus; macOS uses bounded pmset; Linux
  aggregates adapters and charging evidence. Platform parsers have fixtures;
  macOS native hardware behavior is **not** qualified by cross-compilation.
- Unix child process groups now receive the foreground terminal and restore its
  ownership/settings on exit. Six real PTY cases cover input, Ctrl+C, EOF, raw
  terminal exit, failed start and context cancellation. Launch progress precedes
  waiting; cancelled/exited processes no longer report running. Cancellation is
  normalized through the regular CLI error reporter.
- Chat supports validated text/json_object/local json_schema response formats
  through the llama HTTP path. Unsupported providers and constrained-format plus
  tool combinations reject before dispatch. Malformed/truncated tool protection
  remains unchanged. Legacy completions still reject response_format.

Passing checks on the final application source:

```text
go test ./...                                      # Windows and WSL/Linux
go test -race ./internal/power ./internal/degradation ./internal/resource ./internal/integrations ./internal/inference ./internal/server ./cmd/offgrid  # WSL
npm --prefix apps/web run api:check
npm --prefix apps/web run check
npm --prefix apps/web run build
npm --prefix apps/desktop test                      # 71 passing
python3 -m unittest scripts/test/test_hermes_provider.py
node scripts/maintenance/check-source-size.mjs
node scripts/maintenance/check-docs.mjs              # 250 files, zero problems
node scripts/maintenance/check-repository-hygiene.mjs
openspec validate external-agent-integration-foundation --strict
GOOS=darwin GOARCH=arm64 go build ./cmd/offgrid       # compile only, artifact moved under build/
```

The web build retains the existing large-chunk warning. Native desktop-control,
GPU performance, installed macOS runtime and human usability qualification were
not performed. No claim of complete release CI is made.

### Deployed state and preserved data

- Image `offgrid-llm:local-launch-fix-20261022`, application revision
  `local-integration-launch-20261022`, version v0.4.19. Built application artifacts
  with docker/Dockerfile and reused the unchanged installed native runtime via
  docker/Dockerfile.local-runtime; CPU, q4_0 and existing limits were preserved.
- Stopped the idle service, made and verified a workspace archive with its old
  image, restored to a private clone and tested the new image there before live
  replacement. All 5 model IDs, 17 conversation names and 28 task IDs/statuses
  matched before, clone and live inventories. No task was resumed/reconciled.
- Private recovery directory in WSL:
  `/home/phil/offgrid-launch-fix-20261022-tku7694n`; prior stopped container:
  `offgrid-before-launch-1792664484`. Contains credentials/private data: not an
  ordinary export. Model and data volumes were retained unchanged.
- Matching WSL CLI at `/home/phil/go/bin/offgrid`, SHA-256
  `993ddd4a3f65fc1bade30547166589661136162decb1a30a9458a12a2a6f5b65`.
  Previous CLI is retained in that recovery directory. External runtime/model
  selections and personal agent configurations were not replaced.

### Installed-runtime and live evidence

- Hermes deterministic fixture: configure twice, status, smoke, disposable-file
  exact read-back and cancellation passed; evidence
  `/tmp/offgrid-hermes-qualification-69mpok70`.
- OpenClaw deterministic fixture: same sequence and independent file read-back
  passed; evidence `/tmp/offgrid-openclaw-qualification-_6bmlifp`. Its upstream
  temporary agent-exec child continued transient retries after the fixture was
  cancelled and then exited; wrapper cancellation alone is **not** proof that
  every upstream detached child stops immediately.
- Installed Hermes interactive PTY: prompt accepted, fixture reply visible,
  `/exit` returned code 0. Evidence `/tmp/offgrid-interactive-7r8nlr7k`.
  An earlier probe submitted text and Enter in one burst, triggering upstream
  paste/newline behavior rather than submission; the corrected probe separates
  typing from Enter and waits for the actual input prompt.
- The real Meta-Llama 8B returned valid JSON using Hermes's exact title schema;
  cold request 96.58 s, of which model load was 80.9 s. Runtime and catalog
  remained at 65536 tokens on battery. This proves the wire/schema path, not a
  30-second cold title deadline or tool-workflow quality.
- Further real Hermes/OpenClaw smoke attempts overlapped the user's OpenClaw
  run. Hermes timed out; the OpenClaw check was explicitly stopped to remove
  competition. These attempts are **not passes**. No user process was stopped.

### Newly reproduced remaining blocker: long-prefill idle timeout

The user's `offgrid openclaw run "hello"` at 18:24:33 local time sent an agent
prompt of approximately 8400 tokens, including instructions/tool context. On
the CPU-only 8B model, the log showed 4096 tokens processed at 49% after 291.61 s.
At 18:29:34 OpenClaw logged `LLM idle timeout (300s): no response from model`
and started a same-model retry. This is not the old battery/context reduction:
the actual allocation remained 65536 throughout. Concurrent verification
processes added resource pressure and were stopped; they must not be treated
as an uncontended performance baseline.

Source inspection also shows that the public stream sets headers but does not
flush an initial event before waiting for backend output. The upstream local
first-event watchdog and OffGrid's lack of visible prefill/queue progress can
therefore treat ongoing CPU work as a stalled connection. Sequential tool
decisions must still be complete/validated before exposure; progress must not
be fabricated as an answer or partial executable call.

Additional observed 30-second auxiliary cancellations are misleadingly logged
as `failed to switch model` / HTTP 500 while queued behind ongoing generation.
The 1.14-second SQLite maintenance warning was resolved and is not the main
multi-minute stall. Live operational readiness remains incomplete pending the
long-prefill/progress/cancellation follow-up; task 5.4 stays unchecked despite
successful container replacement. Current evidence does not qualify the demo.

## Slow-prefill follow-up (later on 2026-10-22)

Implemented the approved extension in the same change:

- A validated public chat stream emits an assistant-role start event before
  queue/load admission. Serialized SSE comments describe observed phases/elapsed
  time, not invented tokens, percentages or completion. A 15-minute service budget
  remains independent of keep-alives; disconnects cancel the owned request.
- Sequential tool output still requires a complete validated decision. Native
  text requests validate adapter support before accepting the stream. Error paths
  do not emit `[DONE]`; malformed/truncated streams retain their rejection tests.
- Chat context cancellation/deadlines are operational `request_cancelled` /
  `inference_timeout` outcomes, not `failed to switch model`. Other service-error
  contracts remain intact. An initially broad error-constructor change broke a
  privacy/HTTP-status fixture; classification was narrowed to inference and the
  test then passed.
- OpenClaw retains supported `--local-model-lean --code-mode direct` controls,
  instructions, configured tools and saved defaults. No additional prompt-size
  reduction is claimed. An explicit `--timeout <seconds>` changes only that run;
  absence preserves the upstream default. Unlimited runs are rejected.
- Unix cancellation gives the owned runtime two seconds to abort its requests
  before forced termination. Logging middleware forwards response-controller
  deadlines/flush errors, preserving bounded writes through the real HTTP stack.

### Passing checks and observed limitations

```text
go test ./...                                      # Windows and WSL/Linux
go test -race ./internal/server ./internal/inference ./internal/integrations ./cmd/offgrid  # WSL
OFFGRID_TEST_INSTALLED_OPENCLAW_STREAM=1 go test ./internal/server -run '^TestInstalledOpenClawSlowStream$' -count=1 -v -timeout 7m
npm --prefix apps/web run api:check
npm --prefix apps/web run check
npm --prefix apps/desktop test                      # 71 passing
python3 -m unittest scripts/test/test_hermes_provider.py
node scripts/maintenance/check-docs.mjs              # 250 files, zero problems
node scripts/maintenance/check-source-size.mjs
node scripts/maintenance/check-repository-hygiene.mjs
openspec validate external-agent-integration-foundation --strict --no-interactive
git diff --check
```

The installed OpenClaw test used the actual OffGrid HTTP stream with an isolated
delayed engine: **310 seconds before answer, one request, no retry**, completed
in 334.41 seconds. It crosses the previously failing 300-second first-event
boundary. This is transport/runtime interoperability, not real-model performance.

Installed runtime fixtures (private homes, disposable files, no personal config):

- OpenClaw `/tmp/offgrid-openclaw-qualification-2m9doujm`: repeated configuration,
  status, smoke and independent exact file read-back passed. Cancellation returned
  130 in 2.07 seconds and the inference connection was independently seen closed.
- Hermes `/tmp/offgrid-hermes-qualification-v_ethknu`: same checks passed;
  cancellation returned 130 in 2.02 seconds and closed the inference connection.
- The prior OpenClaw worker-retry observation remains in the historical section.
  These new connection checks do not prove arbitrary third-party detached child
  cleanup or native Windows/macOS process-tree behavior.

Full frozen real-model workflows, native platforms, GPU performance and human
usability remain unqualified. This pass changes no desktop UI or tool authority.

### Authorized replacement and failed real-model check

The user separately approved the container and WSL CLI replacement with the
desktop unchanged. The first stream build was `local-integration-stream-20261022`,
image `offgrid-llm:local-stream-fix-20261022`. Recovery directory:
`/home/phil/offgrid-stream-fix-20261022-csnc47k4`; stopped previous container:
`offgrid-before-stream-1792667402`. Backup verification and restored-clone/live
inventories matched **5 models, 17 conversations, 28 task IDs/statuses**. The
installed CLI SHA-256 was
`e64c3152f7e7e91a98919471ae6589dc79ae953553419687a2e79d9ce04a8e70`.
No saved work was resumed, no personal external-agent config changed, and nothing
was committed/published.

One real isolated OpenClaw greeting with the installed Meta-Llama 8B and a
900-second explicit agent budget **failed**. Private evidence:
`/tmp/offgrid-real-stream-20261022-8vte_44w`. Configuration took 62.49 seconds;
the run exited 1 after 36.38 seconds with its configuration unchanged. The actual
message was `insufficient_runtime_capacity`: the pre-load memory estimator
admitted 32768 rather than the requested 65536 tokens while the external runtime
was running. The service did not shrink context, change the model or start that
model. OpenClaw made its own three empty-error retries, all rejected before
inference; no file tools ran.

This is **not** a reproduced first-event timeout or proof of an allocator OOM.
WSL reported about 9945 MiB total RAM; once OpenClaw exited, about 8824 MiB was
available and an embedding worker remained resident. The load guard includes
model weights, KV memory and a reserve. The next unresolved issue is validating
that memory-admission estimate under the external runtime's footprint, not
disabling safety or silently changing the user's 65k configuration. Task 5.4
remains open for successful real-model operational verification. The failed
check is not relabelled as a smoke pass or demo qualification.

### Final transport hardening and active-work hold

The final source also exposes `ResponseController` through the logging middleware
so write deadlines and flush failures reach the underlying socket. Additional
`go test ./internal/server` runs passed on Windows and WSL, and
`go test -race ./internal/server -run TestChatStream -count=1` passed on WSL.
Application/runtime-reuse image `offgrid-llm:local-stream-fix-20261022-r2` built
successfully, including the renderer build and desktop compatibility check.

Its replacement attempt stopped at the **read-only active-inference guard**:
Qwen was running, so no container was stopped and no CLI was overwritten. The
live image/CLI therefore remain the first stream build listed above until the
active work is released. Inspection-only recovery directory for this refused
attempt: `/home/phil/offgrid-stream-fix-20261022-vl_s6n24` (private metadata,
not a completed backup). The desktop remains unchanged.

Harness invocation errors (Windows Docker not on PATH, direct WSL process missing
the login PATH, and nested-shell regex quoting) were corrected before the actual
checks; they are not counted as successful tests. No model or external runtime
downloads, Git staging, commits or publication occurred.

## Installed provider-timeout correction (20:00–20:20 local, 2026-10-22)

After the user released active inference, the r2 replacement succeeded. Recovery
directory: `/home/phil/offgrid-stream-fix-20261022-910q2fai`; previous stopped
container: `offgrid-before-stream-1792668712`. Restored-clone/live inventories
again matched 5 models, 17 conversations and 28 task IDs/statuses. Installed CLI
SHA-256: `96f5e371474787af04f94ac4b47845456eb8805c0017a8f4a6bb5c7dbdf90682`.
This supersedes the earlier active-work hold, not the failed model checks.

### Root cause and correction of earlier test evidence

The failed user's Qwen trace recorded `idleTimedOut=true`,
`timedOutByRunBudget=false`, `externalAbort=false`, and a redacted runtime config
with `models.providers.offgrid.timeoutSeconds: 300`. The OffGrid setup renderer
in `internal/integrations/registry.go` generated that value. It is a separate
provider HTTP deadline, not merely the model idle watchdog or the agent run
budget. SSE startup/heartbeats and a longer per-run timeout cannot override it.
Upstream documents this separate HTTP timeout in its
[custom-provider reference](https://docs.openclaw.ai/gateway/config-tools/custom-providers).

The earlier 334.41-second installed fixture used a hand-authored provider without
the generated field. A repeat without an explicit agent timeout also passed
(333.64 seconds), but still omitted the problematic provider setting. Neither
proved that OffGrid's generated installed setup worked. That coverage gap is
preserved here explicitly.

The test now uses `integrations.Adapter.Render` and the public logging middleware:

- **Before:** actual generated `timeoutSeconds: 300` cancelled the request at
  300.019 seconds and the upstream runtime retried. The test launcher then hung
  waiting on inherited child pipes and hit its seven-minute test timeout. This
  was a failing reproduction, not a pass. The opt-in Unix test now bounds its
  owned process-group and pipe cleanup on deadline as well.
- **After:** renderer no longer imposes the fixed provider timeout. The same
  installed OpenClaw 2026.10.4 test, with no explicit `--timeout`, received the
  answer after 310 seconds and finished in **319.13 seconds**, **one request,
  no retry**. The delayed engine is still a deterministic fixture, not a real
  model or performance qualification.
- Regression checks assert both that setup omits the timeout and that managed
  merge patches do not overwrite/remove a user's existing timeout. The finite
  upstream agent and 15-minute service budgets remain. No upstream source was
  patched and no tools/instructions were removed as a performance workaround.

With explicit user approval, `openclaw config unset
models.providers.offgrid.timeoutSeconds` removed the old generated field from
the installed config after a validated dry run and private backup at
`/home/phil/offgrid-openclaw-timeout-backup-og6iCbaR/openclaw.json` (0600, inside
a private directory). A semantic before/after comparison found **exactly one
changed field**: that timeout. Models, credentials, tools and other settings
were unchanged. Generic future configuration does not erase deliberate limits.

### Latency is a separate remaining concern

The Qwen log showed roughly 7300 prompt tokens and 4096 prompt tokens processed
in 245.90 seconds on CPU before the old timeout. Loading also costs time.
These are observations from affected runs, not an uncontended benchmark or a
claim that all local models have the same speed.

Both host adapters currently execute five inspection commands before launch.
One read-only observation measured OpenClaw at about 7.00 seconds total
(version 0.171, config location 0.873, plugin inspect 2.953, provider config 1.535,
optional global default 1.466) versus Hermes at about 1.63 seconds. The optional
OpenClaw global-default query returned 1 because that default was unset, which
the adapter permits. These measurements include CLI/WSL startup and were not a
controlled performance benchmark. They explain some startup overhead, not the
multi-minute CPU prefill. Consolidated inspection/cache optimization remains a
separate improvement; no readiness or bundle-integrity checks were bypassed.

Windows and WSL `go test ./internal/integrations ./cmd/offgrid ./internal/server`
passed after the change. Documentation (250 files), source-size, hygiene,
OpenSpec strict validation and `git diff --check` passed. r3 application and
runtime-reuse images built successfully. Final race/deployment/live-check results
are recorded below when available; the previous real Meta-Llama 65k memory
admission failure remains unresolved, not silently waived.

### r3 replacement verification

`go test -race ./internal/server ./internal/integrations ./cmd/offgrid` passed
on WSL (server 76.418s, integrations 1.063s, CLI 37.953s). An earlier attempt
with a shell-quoted `-run` alternation failed due to argument quoting; it is not
counted as a passing race run. The complete package command above replaced it.

After confirming no active/queued inference, the authorized replacement installed
`offgrid-llm:local-stream-fix-20261022-r3`, revision
`local-integration-stream-20261022-r3`. Recovery directory:
`/home/phil/offgrid-stream-fix-20261022-f5b5ke2y`; previous stopped container:
`offgrid-before-stream-1792671919`. Verified backup, restored-clone and live
inventories matched **5 models, 17 conversations, 28 task IDs/statuses**.
Matching WSL CLI SHA-256:
`007f1ed9fa9edf18c1aefdd38ae16ae0d31e2298c94f7a3007e25e21d027dc5f`.
One OffGrid service container is running and healthy. `/api/v2/system` reports
the expected revision; the live generated OpenClaw setup no longer contains the
provider timeout. The installed personal config remains without the old field.
Desktop, saved tasks and model defaults were not changed. Private backups and
stopped previous containers are retained. No Git staging, commit or publication.

### Real Qwen greeting: still failed, not release-qualified

After r3, one isolated OpenClaw `hello` used installed
`Qwen2.5-7B-Instruct-Q4_K_M`, private state/workspace, only file tools, the actual
managed setup, and the **default** finite agent budget (no `--timeout` override).
Private evidence: `/tmp/offgrid-real-stream-20261022-a2gi1jdi`.

- Configuration succeeded in 25.10 seconds; the generated provider had no fixed
  timeout. The greeting invocation preserved that configuration byte-for-byte.
- The model loaded in about 30 seconds, with 8 CPU threads and a confirmed
  32768-token allocation. Logs explicitly reported no usable GPU. About 4600
  prompt tokens took approximately 169 seconds to process; the last periodic
  prompt counter was 4608 tokens at 166.03 seconds.
- The model continued generating past 300 seconds, with **one inference request**
  and no five-minute abort/retry. It generated at roughly 5.7 tokens/second and
  reached approximately **2270 generated tokens**, but delivered no completed
  decision/answer before the upstream run deadline (about 600 seconds).
- The command exited 1 after **615.31 seconds**. OpenClaw reported `embedded run
  timeout`; the service observed cancellation at 599.066 seconds and cancelled
  the owned worker task. Health then showed zero active and queued inference.
  No file/tool result was claimed and no personal workspace was used.

This exposes a remaining real-model/structured-completion problem, not another
provider-connectivity failure or proof that increasing deadlines solves it.
`parallel_tool_calls=false` with tools uses OffGrid's private single-step JSON
adapter, which deliberately buffers output until a complete validated decision.
The counters do not establish what incomplete content the model was generating;
no raw partial output or hidden reasoning was retained. A controlled comparison
of supported model/template/tool adapters and completion behavior is still needed.
Do not bypass tool validation, silently remove capabilities, or claim a successful
greeting based on token generation. Task 5.4 remains unchecked.

The recent Docker log review also found the existing unauthenticated `0.0.0.0`
listener warning inside the container; Docker publication was verified as
`127.0.0.1:11611` only. This is not a claim of authenticated/network-isolated
deployment. CPU/GPU warnings and the expected cancelled-request log are recorded,
not hidden under a blanket "no errors" claim. Native/GPU/tool-workflow and fair
same-model Hermes-versus-OpenClaw performance qualification remain unverified.

### Narrow response-path isolation

Two short, read-only synthetic calls then used the same resident Qwen model,
greeting, system instruction, one non-executable placeholder tool, temperature 0,
and 128-token limit. Only `parallel_tool_calls` differed. No tool implementation
was attached or executed. Private evidence:
`/tmp/offgrid-qwen-contract-hmlb8pvn/results.json`.

- Native tool-template path (`true`): HTTP 200 in **9.175 seconds**, stop reason
  `stop`, four completion tokens. The short output was `Hello!Here`; this is
  transport/completion evidence, not a quality pass or full tool qualification.
- OffGrid constrained single-step path (`false`): HTTP 500 in **23.496 seconds**,
  exhausting 128 completion tokens. The internal error was `sequential model
  response did not finish cleanly`; no incomplete call was emitted.

This reproduces a completion failure in the response-adaptation path even with a
tiny synthetic prompt, separately from OpenClaw's thousands of context tokens.
It does not yet establish which grammar/template/model interaction causes the
non-terminating output, nor qualify the native path as a drop-in replacement.
The next substantive fix requires that adapter investigation and corresponding
model checks, preserving strict validation of complete tool calls. It is not
permission to disable validation or merely raise another timeout.

### Shared native contract and loading follow-up

The r3 observation above predates the r4 fix. Public sequential tool calls now
keep the model's native tool template. The private governed one-step adapter is
selected only by in-process agent calls; its selector cannot be set by public
JSON. Nonstream and stream responses pass registered-tool, schema, finish and
cardinality checks. Tool fragments remain private until a complete `[DONE]` and
validated decision. Tests cover malformed and truncated calls, duplicates,
multiple calls where sequential mode was requested, text streaming, cancellation
and prompt-token request parity across four arbitrary model IDs. This is contract
evidence, not proof that every installed model makes good tool decisions.

The provider scheduler now shares a bounded model preparation among compatible
requests. Cancellation of one auxiliary request no longer tears down a load
needed by another waiter. Tests cover same-model demand, all-waiter cancellation,
capacity, exclusive switching and shutdown. `POST /v1/inference/prepare` uses
ordinary chat authority and the selected installed GGUF model; no generation or
default-model change occurs. Managed external runs prepare before launch, while
older services retain their previous launch behavior with an explicit notice.

The OpenClaw launcher no longer forces `--local-model-lean`. Inspection of the
installed OpenClaw code found that lean mode hides browser, automation, image,
message, PDF and speech tools, contradicting the older claim at line 301 above.
The installed runtime's normal compact tool search remains available. The
isolated Hermes test home disables only its own update check: installed Hermes
`--version` can synchronously check GitHub, causing a 30-second inspection
timeout unrelated to OffGrid inference. User Hermes configuration was untouched.

Checks for this follow-up: Windows `go test ./...` passed; WSL `go test -race
./internal/inference ./internal/server ./internal/integrations ./cmd/offgrid`
passed. The first race pass found a 100 ms image-test deadline insufficient
under instrumentation; a 1.5-second queue and one-second post-admission budget
passed five repetitions and the full affected race suite without changing the
production budget. Web `api:check`, `check` and `build`, four Python harness
unit tests, documentation check (250 files), source-size/hygiene checks, and
OpenSpec strict validation passed. The web build still warns of a bundle above
500 kB. Installed OpenClaw and Hermes deterministic fixtures passed with
disposable homes, reply and file read-back; those fixture models do not certify
real model quality. The first concurrent Hermes fixture timed out inspecting
`--version`; the rerun with the isolated update check disabled passed.

With separate authorization, r4 replaced the service image and matching WSL CLI.
Image `offgrid-llm:local-native-contract-20261022-r4`, revision
`local-native-contract-20261022-r4`; private backup
`/home/phil/offgrid-native-contract-20261022-re146qs6`, previous stopped
container `offgrid-before-stream-1792680116`, CLI SHA-256
`f92c532c30c66e3f4431d09c3225f9957d28e1f144ffa847c5391321db7fa697`.
The workspace backup verified and restored to a check container. Before,
restored-copy and live inventories matched: five models, 17 conversations,
28 task IDs and statuses. One `offgrid` service runs with the expected revision;
desktop app was unchanged. The old r3 backup remains recoverable.

Live r4 matrix evidence: `/tmp/offgrid-model-matrix-o0lg3fpb/results.json`
with four installed chat models. Each was loaded, gave two greeting attempts,
streamed, received a synthetic unexecuted tool offer, and was disconnected
during a separate stream. All four returned to idle after disconnect. Qwen 3B
and 7B returned the exact greeting in the warm calls and stream; the Qwen 7B
warm repeated answer and stream took about 0.8 seconds after a 67.6-second cold
load. Meta 8B's synthetic tool call was valid, but it incorrectly called that
tool for a greeting and quoted the follow-up marker. Qwen 7B exhausted 192
tokens on a required synthetic tool call; no action was dispatched. Phi failed
the exact greeting and synthetic call; Qwen 3B passed the call but failed its
follow-up exact answer. Model-specific tool quality is therefore **not**
qualified by the transport fix. The test environment is WSL/CPU only; native
Windows/macOS, CUDA/Metal and longer real tasks remain unverified here.

An installed OpenClaw 2026.10.4 run using the 3B Qwen model and a private
workspace passed setup, status and greeting smoke checks, then failed the
disposable Unicode file-copy/read-back task after 614.19 seconds (600-second
external budget). The first model request processed about 4,600 prompt tokens
and kept generating beyond 1,200 tokens without a complete tool decision.
Private evidence: `/tmp/offgrid-openclaw-qualification-g7fqcvv8`. Neither a
file write nor a successful outcome was observed. This is a live workflow
failure despite successful native transport and short synthetic calls.

One isolated OpenClaw comparison disabled Tool Search only in the disposable
configuration, retaining the same two allowed file tools and Qwen 3B. Setup and
smoke passed. The file run exited after 160.47 seconds, but the model merely
described using write/read and produced no file. Independent read-back failed.
Private evidence: `/tmp/offgrid-openclaw-qualification-5v04jaec`. This suggests
Tool Search adds cost on this local model, but direct schemas alone do not make
it a reliable tool user. The normal installed OpenClaw configuration was not
changed.

### r5 shared preparation and real Hermes retry

The first installed Hermes 8B test on r4 timed out during its 180-second smoke:
`Manager.Test` skipped the model preparation used by `Manager.Run`, and several
30-second auxiliary requests were cancelled while the model was loading. The
shared host manager now prepares before either path. A regression fixture
asserts that preparation precedes the auxiliary inference. Windows `go test
./...`, WSL `go test -race ./internal/integrations ./cmd/offgrid`, the focused
Go integration/CLI tests, documentation/source-size checks and strict OpenSpec
validation passed after the correction. This verifies the code path, not model
quality.

The separately authorized r5 replacement installed
`offgrid-llm:local-native-contract-20261022-r5` and the matching WSL CLI
(SHA-256 `74bf085084501bc974ee437c8f7502a5801b1a27e97f01f309cb622240051cb0`).
The private recovery directory is
`/home/phil/offgrid-native-contract-20261022-pdz1rsai`; the previous stopped
container is `offgrid-before-stream-1792682927`. The verified workspace
backup, restored check container and live service retained the same five
models, 17 conversations and 28 task IDs/statuses. The desktop app, user
Hermes/OpenClaw configuration and saved work were not replaced.

With the installed Hermes Agent v0.21.2 and Meta-Llama 8B, the isolated setup,
status and real smoke passed. The smoke took **147.25 seconds** including cold
preparation, where r4 had failed at the limit. Private evidence:
`/tmp/offgrid-hermes-qualification-8xpudmho`. The following disposable Unicode
file workflow exited 0 after **224.85 seconds** but **failed independent
read-back**: the agent wrote literal `read_file` to the target instead of
reading the source text (`OffGrid test — café 日本語 12345`) first. The source
remained intact. This is a wrong tool sequence/content decision by the actual
model; a successful process exit must not be reported as successful work.
During the workflow the service also logged cancelled 30-second auxiliary
requests (HTTP 499), and Hermes reported an auxiliary-title timeout. The
primary smoke succeeded, but auxiliary latency remains a UX issue.

The dynamic four-model reply/tool/cancellation matrix, both installed external
agent file workflows, and cold/warm timings have now been exercised. **Neither
external agent is qualified for reliable file work on the models tested.**
OpenClaw Qwen 3B failed to create a verified file; Hermes Meta-Llama 8B
created the wrong content. No Windows-native/macOS, GPU, or broad model/workflow
qualification was run. The r5 service is a controlled local test replacement,
not release-readiness evidence. A focused checkpoint commit `10f2e6af` covers
the separate battery-detection/context policy fix; the remaining integration
implementation is not committed or published.

A final r5 installed OpenClaw 2026.10.4 check in a fresh disposable home used
Qwen 3B and passed setup twice (57.51/41.61 seconds), status (11.52 seconds),
real greeting smoke (51.46 seconds) and post-smoke status (11.45 seconds).
Private evidence: `/tmp/offgrid-openclaw-qualification-6a7yjq4k`. These times
include WSL/external CLI startup and are not a controlled inference benchmark.
The earlier r4 file-workflow failure remains the last independently checked
OpenClaw file result; this r5 smoke did not repeat or repair that workflow.

### 2026-10-23 user run: CPU prefill and Hermes version probe

The user's installed OpenClaw 2026.10.4 run with Meta-Llama 8B cataloged 41
tools, then produced no answer before the user cancelled at 23:54:40, about
four minutes after launch. The r5 service was healthy afterward with no active
or queued inference. Its logs showed the Meta model running CPU-only (`-ngl 0`,
eight threads) and still pre-filling the large OpenClaw prompt; roughly 4,096
prompt tokens had been processed when cancellation propagated. A started HTTP
stream and a catalog message were not an assistant answer. This is a real
latency/UX failure, not evidence that the service was dead or that the task
completed. The current code has not removed OpenClaw's tool catalog or silently
changed the selected model.

The subsequent installed `offgrid hermes run "hello"` failed before inference:
Hermes version inspection exceeded its deadline. Installed Hermes v0.21.2
performs an optional update check in its `--version` startup path. A focused
host fix now runs only that version probe with a disposable Hermes home whose
configuration disables the optional update check; the real user's provider,
model and endpoint configuration is still read from the normal home. A
regression test checks this isolation. Windows `go test ./...` and WSL
`go test -race ./cmd/offgrid ./internal/integrations` passed. A separately
built, **not installed** WSL CLI reported the real Hermes configuration
`preflight_ready` in 1.89 seconds. The installed CLI and r5 container were not
replaced in this diagnostic pass; this does not qualify a real Hermes reply or
repair either agent's failed file workflow.

In the subsequent user-authorized install pass, the matching WSL CLI was built
from the workspace and installed at `/home/phil/go/bin/offgrid` (SHA-256
`b17c309af9f05dbf27607b68e726651174ca524ef9422c82c3a08d10d39a6892`).
The previous CLI (SHA-256
`74bf085084501bc974ee437c8f7502a5801b1a27e97f01f309cb622240051cb0`)
was preserved at
`/home/phil/.offgrid/backup/offgrid-cli-before-hermes-version-20261023`.
Installed `offgrid hermes status` returned `preflight_ready`, and installed
`offgrid hermes test` returned `smoke_passed` with the selected Meta-Llama 8B.
Installed `offgrid openclaw status` returned `preflight_ready`, and its direct
provider smoke returned `smoke_passed` in 25.23 seconds with the same model;
no full OpenClaw agent run was repeated here. Windows `go test ./...` passed again before
installation, and the service stayed healthy with the same five installed
models. The r5 container, desktop app, model selection and external-agent
configuration were unchanged. This confirms both providers can obtain a real
model reply, **not** quick full-agent responses or verified file-tool workflows.

### 2026-10-23 isolated OpenClaw 7B workflow and hardware boundary

An isolated OpenClaw 2026.10.4 run used Qwen2.5-7B-Instruct-Q4_K_M and a
disposable workspace/configuration with only approved read/write file tools.
Two configuration passes, status and a direct model smoke passed. The actual
file task failed: its 600-second agent budget ended after 614.57 seconds of
process time, with no destination file. The first full-agent request cataloged
four tools; llama-server took about 264 seconds to process 4,608 of roughly
4,700 prompt tokens, then emitted about 1,588 tokens at roughly 5.16 tokens/s
without a complete usable tool action before cancellation. The service returned
to healthy idle. Private evidence:
`/tmp/offgrid-openclaw-qualification-0ok4yk9s`.

This second real-model failure demonstrates that a passing `/v1` transport or
provider smoke is not sufficient for a working OpenClaw task. The live Docker
service has no assigned GPU device and its llama worker runs with zero GPU
layers. Windows enumerates an NVIDIA RTX 5060 Laptop GPU, but `nvidia-smi`
reported insufficient permission there and NVML was unavailable in WSL; GPU
acceleration is not installed or qualified. The container was not replaced and
the user's OpenClaw model/tool configuration was not changed. The full-tool
Meta-Llama run and the smaller-catalog Qwen run are CPU-limited; reducing tools
or silently switching models would violate the approved capability and selection
contract. Faster end-to-end work needs an explicitly chosen model/capability
tradeoff or a separately qualified GPU/device path, followed by independent
workflow checks. No general OpenClaw qualification is claimed.

A cached, network-isolated CUDA diagnostic container was also attempted with
`--gpus all --pull never`. It remained in Docker's `Created` state while the
NVIDIA runtime prestart hook ran for more than six minutes; it never reached
`nvidia-smi`. The diagnostic hook was stopped and Docker removed only that
disposable probe container. The existing OffGrid CPU container remained healthy.
This is a GPU-passthrough/runtime blocker, not evidence of working CUDA or a
reason to replace the running service. Driver/runtime repair and an actual
GPU-qualified model run require a separate decision and verification.
