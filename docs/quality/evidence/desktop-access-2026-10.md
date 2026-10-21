## Bounded desktop context, progress and registered applications (2026-10-10)

### Declaration and pre-edit repair (2026-10-11)

The latest saved Unicode editor test failed before application launch: it proposed
a control-text expectation without an observed control, then repeatedly retried
the blocked launch. The recorded context was below the configured 8,192 tokens;
this was a declaration/recovery defect, not demonstrated context exhaustion.
No rejected launch or edit was dispatched, and the saved task remains interrupted.

The repair aligns the initial sequence and saved-task guidance, reports invalid
fields, and counts invalid declarations in the existing three-repair budget.
Version-1 text/checkbox edits now require a matching declared value and canonical
session/target/control binding before host preparation, including Full access.
Unrelated reads or declarations cannot reset an outstanding edit repair. The
repair tool exposes a simple, fully required six-field schema for the specific
rejected edit; the model must still supply and submit it. No auto-filled
expectations, new permissions, native input operations or UI changes were added.

Actual installed `Qwen2.5-7B-Instruct-Q4_K_M` inference used only synthetic
application/control evidence and disposable task storage; the provider refuses
all desktop mutations and cannot contact the host hub. Earlier attempts exposed
omitted control IDs, misleading return-to-discovery guidance, and poor conditional
schema handling. Those failed checks were not treated as successes. One run also
exceeded its test timeout. Final flat-schema checks passed:

- Application declaration: one model call, 3,081 prompt tokens, 28.96 seconds.
- Text declaration: four model calls, 3,258–3,445 prompt tokens, 119.30 seconds.
  This included two inspections, a rejected unchecked edit, then a correct
  declaration of the exact Unicode text and canonical window/control IDs.
- Shared UIA/AX/AT-SPI fixtures cover matching text and false checkbox values,
  wrong/missing checks, mismatched/replaced/ambiguous identities, unavailable
  control operations, fresh references, old-host observation-local bindings,
  session revocation, legacy tasks, and unknown versions. Authorization and
  dispatch-boundary tests assert no host prepare or input for rejected edits.
- Deterministic runner tests cover malformed declarations, unrelated reads and
  metadata, matching correction, persisted repair limits and expired authority
  after restart. Full `go test ./...`, API drift, TypeScript, and documentation
  checks passed after the final code changes.
- WSL race checks passed for agents (16.949 s), server (48.190 s), computer
  (4.511 s), companion, storage and service-client packages using private
  temporary state. Windows race execution remains unavailable without cgo.
- Rebuilt-service browser evidence tests: 15 passed, covering all nine locales,
  narrow screens, verification status and reload without replay. Strict OpenSpec
  validation passed. Desktop/companion and full browser suites from the foundation
  record below were not rerun for this service-only repair; no native or renderer
  source changed in this repair.

These are contract and installed-model declaration checks, not a completed real
Notepad workflow, model reliability percentage, cross-platform qualification or
interactive latency claim. No personal application was controlled for this repair.

Authorized service-only deployment replaced the existing container with
`offgrid-llm:workflow-repair-20261011`. The workspace backup at
`/home/phil/offgrid-workflow-repair-backup-20261011-FCCg0OrM/workspace.zip`
passed verification and restored-workspace checks before activation. The separate
model snapshot and current model files matched SHA-256 manifests. Model IDs,
speech readiness, conversation names, workspace identity and all saved task
IDs/statuses matched before and after replacement; the interrupted task was not
resumed. A short installed-model inference passed against restored data.
Only `offgrid` runs on the existing loopback port; the old container remains
stopped as `offgrid-before-workflow-repair-20261011-1791698602` for recovery.
The live binary digest matches the built repair. UI build identity remains
`c733833a0d4d7dfa15ff75182fb51e2598a0c216f0eb11ccf496c6f08d799b16`;
desktop/companion files and installed model weights were unchanged. This local
deployment is not a commit, release, native workflow qualification or permission
to resume saved work.

### Workflow-evidence foundation evidence (2026-10-11)

`desktop-workflow-foundation` adds immutable, versioned outcome declarations,
service-assessed application/text/checkbox checks, three completion-repair
opportunities and owner-scoped saved-control discovery. New task-first native
tasks activate this contract; existing tasks are not retroactively certified.
Unsupported research/file/submission checks remain unverified. Host-issued
cross-observation identities connect fresh input references without guessing by
label or text. They reset on control/target replacement; older hosts without the
metadata retain limited verification, not expanded authority.

Scenario coverage includes compound and omitted expectations, caller/model
provenance, steering revisions, lost writes, unknown versions, immutable checks,
independent read-back, mismatched arguments/targets, ambiguous identities,
invalidated evidence, bounded repair, uncertain dispatch without replay, and
legacy status preservation. Snapshot, event and export reads remain owner-only.
Equivalent UIA/AX/AT-SPI format fixtures exercise 400 controls, Unicode literal
matching, source truncation and observation/query-bound pagination. Discovery
makes zero host prepare/focus/dispatch calls and never extends input freshness.

The composed-schema 8,192-token fixtures retain MCP permissions and control
search through compaction. The deterministic tokenizer double reports 5,200
input tokens (not a real model measurement); allocation stays 8,192. Open-only
fixtures use 6 model calls, duplicate-launch repair 8, compound control fixtures
16, and compound plus rejected relaunch 17. The compound fixtures find the
document omitted from the initial 24-row view with one search and one inspection
round trip instead of paging raw JSON. Results are capped at 4,096 encoded bytes
and 16 controls. This demonstrates bounded transport/context behavior, not
small-model accuracy, inference latency or universal application reliability.
The 400-control Unicode fixture uses a 43-byte query and returns a 351-byte
matching page from 49,061-byte UIA/AT-SPI or 49,069-byte AX canonical evidence.
Canonical bytes are retained unchanged. The same query adds one model/tool round
trip; inspection is a separate explicit read and neither refreshes the capture.

Recorded checks:

- `go test ./...` passed; focused agent/server/computer/CLI/service-client suites
  and additive projection tests passed. A concurrent run exceeded an existing
  three-second chat startup deadline; the isolated rerun passed without changing
  that deadline. A newly added failure test was corrected to expect the existing
  uncertain-result error, rather than incorrectly expecting a nil error.
- API generation/drift, TypeScript checking and production UI build passed.
  Vite still reports the existing large main-chunk warning.
- Disposable real-service browser suite: 302 passed, 10 gated tests skipped;
  new evidence suite: 15 passed, including all nine locales, narrow/RTL layouts,
  keyboard disclosure and reload without resubmission. Installed-deployment,
  restart-model and separately seeded job scenarios remain gated, not certified.
- Desktop tests: 65 passed. Companion tests: 57 passed. Python client transport
  tests: 20 passed in WSL (`python3 -m pytest python/tests/test_client.py`);
  Windows has no usable `python` executable.
- Opt-in `TestNativeWindowsLiveUIA` passed using only its owned fixture window:
  real UIA, protected-field exclusion, Unicode edits, independent Win32 read-back,
  fresh input references with stable OS evidence identity, stale/replay rejection.
- Documentation checker: 6 tests, 118 maintained files, zero link problems;
  strict OpenSpec validation passed. Final scenario audit is recorded below.

Windows race execution is unavailable because this environment lacks cgo.
The first WSL attempt exhausted its pre-existing 4.9 GB `/tmp` tmpfs. Tests were
redirected to a new private directory under `/home/phil` without deleting user
files. A concurrent race run exceeded the existing five-second proposal-fixture
deadline. The quiet rerun of `go test -race ./internal/agents ./internal/server
./internal/computer ./cmd/offgrid-computer ./internal/storage
./internal/serviceclient` passed all six packages (agents 29.694 s, server
101.932 s, computer 6.440 s, storage 4.466 s; companion/service-client cached).
The final rerun after the audit fixes also passed all six packages (agents
30.327 s, server 103.117 s, computer 7.740 s, storage 4.476 s).
This is Linux/WSL race evidence, not a Windows race or macOS runtime claim.

Final scenario audit against both foundation delta specifications:

| Scenario group | Implementation and regression evidence |
| --- | --- |
| Compound requests, missing declarations, immutable outcomes and user corrections | `workflow_evidence_test.go` covers durable records, first-mutation rejection, additive declarations, caller/model provenance and authenticated safe-boundary steering. Composed desktop continuation fixtures retain expectations through compaction. |
| Narrow proof, foreign/failed evidence, changed controls and older hosts | `workflow_evaluators_test.go` covers application versus research/file outcomes, exact text/checkbox predicates, canonical step arguments, target/session binding, identity replacement/ambiguity and unavailable old-host evidence. The owned Windows UIA fixture checks actual cross-observation identity continuity. |
| Early completion, exhausted repair and uncertain dispatch | `workflow_completion_test.go` covers three corrective opportunities, restart, progress-sensitive budgets, explicit unsupported/partial results and no retry of uncertain mutations. Unsupported research is reported as unverified; it is not promoted to proof or sent through an endless repair loop. |
| Persistence failure, legacy records, owner isolation and observation-only clients | Agent transaction-failure tests, `workflow_contract_test.go`, CLI/Python transport tests and `task-evidence.spec.ts` cover rollback before publication, unchanged status/request hashes, additive projections, unknown versions, nine locales, provenance, keyboard/RTL layouts and reload without submission. |
| Omitted controls, Unicode, ambiguity, bounded pages and incomplete sources | `native_discovery_test.go` in computer/server packages covers 400-control fixtures, exact identities, literal filters, source versus model limits, text pagination and observation/query-bound cursors. Canonical evidence is unchanged. |
| Expiry, switched/revoked/foreign sessions and unknown evidence | Server discovery authorization tests reject invalid ownership, session and target bindings before disclosure, preserve historical age and make no host focus/prepare/dispatch calls. Unknown security fields and malformed/null filter values fail closed. |
| Portable contracts, MCP composition and context limits | Equivalent UIA/AX/AT-SPI fixtures and the composed 8,192-token desktop tests preserve selected-window restrictions, independent MCP authorization and the existing context allocation. No native availability or qualification flag is expanded. |

The audit found and fixed a boundary issue: the runner adds `offgrid_step_id`
after discovery serializes its response. Discovery now reserves 64 bytes within
the 4,096-byte limit for that metadata. A maximum-width step-ID regression checks
both search and inspection with large, JSON-escaped text. This keeps the complete
model-facing reply bounded rather than only its pre-runner payload. Malformed
optional JSON `null` filters are also rejected instead of silently widening a
query. A separate multi-control regression reproduced an evaluator bug where a
later read-back erased an unrelated passing check. Verification now invalidates
only the same bound control/predicate; the test verifies two text fields and a
checkbox together, then confirms a failed re-check clears only its own outcome.
The regression failed before the fix. No unresolved implementation finding
remains within this foundation's approved scope; the platform and larger-program
gaps below remain open.

Check details also display the recorded verification time in the current locale,
so historical read-back is not presented as a fresh live observation. The browser
regression checks the timestamp alongside provenance and the existing scope note.

During foundation implementation, no production container, installed desktop,
personal application, model, release or commit was changed. macOS installed AX behavior, Linux desktop/Wayland input,
cross-platform launch, real-model accuracy/latency, soak and pilot qualification
remain unmeasured here. The remaining program stages—richer typed native actions,
verified files/transfers, local vision, wider host support, Chat/job linkage and
connection lifecycle—remain separate work; finishing this foundation does not
complete or qualify that larger program.

Subsequent user-authorized local deployment (2026-10-11): replaced the existing
`offgrid` service with `offgrid-llm:workflow-foundation-20261011` and reinstalled
the matching Windows desktop, service binary, UI and native worker. This remains
an unsigned local 0.4.16 test build, not a published release. UI identity is
`c733833a0d4d7dfa15ff75182fb51e2598a0c216f0eb11ccf496c6f08d799b16`.

The old service was stopped before maintenance or restored-workspace validation;
only the replacement service was left running on `127.0.0.1:11611`. Workspace
backup/verification/restore passed, and the separate model snapshot and current
model store matched by SHA-256. The existing workspace identity, model IDs, 21
saved task IDs/statuses and speech selections were preserved. A short Qwen2.5-7B
inference check on the restored workspace returned `OK`. Speech readiness was
checked without recording or generating private speech.

Packaged Windows startup/recovery and access-panel checks passed, including 100%
and 200% zoom. Installed application/UI/service/worker hashes matched the built
package; the installed desktop attached to the external service without starting
another workspace or submitting a task. Private desktop/workspace backups and the
stopped former container were retained. No task, desktop grant or uncertain
action was automatically resumed. No commit, push or publication was performed.

The later saved run `run-e690be325a606dfe94f5e040b0c5d4eb` verified Edge but
repeated launching it and then exceeded its 8,192-token allocation (9,721-token
preflight). No search control action was dispatched. Its 45-control observation
included two roughly 2 KB high-entropy URLs. The previous lossless projection
was insufficient: it retained the very text that dominated the prompt.

The approved follow-up now bounds desktop model views to 24 control rows and
4,000 serialized row bytes, prioritizing editable controls and marking every
omission/truncation. Full host evidence remains unchanged in task steps and can
be read through task-local 1,024-byte Unicode-safe `task_history` pages. Short
references resolve to canonical host IDs before authorization and persistence;
they never rewrite user-entered text or MCP arguments. Legacy window views and
unknown-protocol fallback remain compatible. These bounds are not a substitute
for the runtime's complete-prompt budget check.

Verified application progress survives context compaction and restart. A repeat
launch is rejected before host dispatch; successful follow-on control progress
resolves that repair state. A regression exposed and fixed the case where a
correctly blocked duplicate launch left the task permanently unresolved despite
successful later controls. Readiness observations can supply the initial control
view without a redundant observation just to satisfy the first-action guard.

Windows discovery now reads user/machine Start Menu links and App Paths
registrations, instead of limiting discovery to three product names. Only
validated local, zero-argument GUI executables enter the host-issued catalog;
the host still rechecks file identity and uses its existing durable dispatch
journal. Read-only inspection found 48 validated applications on this machine
and launched none. Argument-bearing shortcuts, packaged activation, console
applications and known installer/interpreter helpers are excluded. This is not
universal application support or a general-purpose security sandbox.

Verification so far: Windows Go suite, Linux race checks for agents/server/
inference/computer and the companion command, API drift, TypeScript and current
UI build, desktop/companion unit suites, maintained-doc checks and strict OpenSpec
validation. Owned native launch/readiness/replay checks passed without opening
personal applications. With the installed Qwen tokenizer, synthetic Edge, Writer
and non-English application continuations fit an 8,192-token allocation; final
prompt counts were 4,984, 4,985 and 4,987. The duplicate-launch repair continuation
also passed (4,985 tokens). Model decisions and host actions in those durable
fixtures were controlled doubles; separate real-model checks do not dispatch
their proposed actions.

The installed Qwen2.5-7B Q4_K_M runtime returned the expected structured text-entry
call for both synthetic Edge and Writer views using the new short references.
Their full observations were 11,942/11,946 bytes; model projections were 2,858
bytes. Preflight and generation counts matched exactly at 2,674/2,676 tokens.
Tokenizer checks took 22/28 ms, but total CPU response times were 128.54/56.26 s.
These checks pass correctness for that next step, not interactive latency or
end-to-end public search. Nothing returned by either model test was dispatched.
Packaged Windows startup/recovery and current access UI checks also passed,
including 200% zoom scrolling, Escape/focus behavior and no automatic consent.
Evidence: `offgrid-desktop-startup-ud1ifg` and `offgrid-desktop-access-garDeH`
under the Windows temporary directory. Cross-platform installed suites and the
full browser regression suite were not rerun for this backend-only increment.

Hardware assessment: this machine has an RTX 5060 Laptop GPU with about 8 GB
VRAM and about 16 GB system RAM, but the existing container requests no GPU and
its current llama runtime uses CPU inference. Device presence is not evidence
of accelerated performance. This update preserves that runtime and installed
weights; no model download/replacement or GPU qualification is implied.

This increment does not complete the wider program: durable tool-enabled Chat
linkage, OAuth connection lifecycle, packaged-app activation, native visual
fallback, broader structured controls/files, cross-platform providers and
real-model workflow/latency qualification remain required. No public tourism
search, personal-profile selection, soak/pilot, signing or release claim is
established by these synthetic and owned-fixture checks.

Authorized local deployment completed: `offgrid-llm:progressive-20261010`,
container `7c475671eb6a`, replaces the prior service on loopback port 11611.
Only `offgrid` is running; `offgrid-before-progressive-20261010-1791638987`
is stopped with restart disabled for rollback. Workspace backup/verify/restore
and restored-workspace validation passed before activation. Before/after checks
preserved workspace identity, conversations, model IDs and task IDs/statuses;
full model-file digests matched the independent retained snapshot. No personal
task was resumed and no desktop authority was granted during these checks.

Service backup: `/home/phil/offgrid-progressive-backup-20261010-658qZJEh`.
Desktop application/profile backup:
`D:\offgrid-llm-backups\desktop-progressive-20261010-212701`.
The existing model snapshot remains at
`/home/phil/offgrid-consent-edge-backup-20261010-Q4mOPmBn` and was reverified.
Do not roll back a companion execution journal when restoring a workspace.

The matching unsigned local Windows 0.4.16 installer was applied to the existing
installation. Application, bundled service, UI, manifest and worker hashes match
the staged package. The current UI identity is unchanged:
`af59d55f901ed45e31a70577615d7075f39351b9aa10a33052e09aae61321e18`.
Linux service SHA-256:
`9d69aece9cc7084d069c2e829361b5948e8f8107c8f747d0a0c831a76528c27f`.
Native worker SHA-256:
`0738a1d13e4d676e7899731290ef50da7c7a3c119960a2f7ff17910d0e42ff02`.
Installer SHA-256:
`f34cf95fe5afc5ce806b180c1f8fd735396e1c2f106a3006fd4d7f585241d31e`.
Installed checks confirmed external-workspace attachment, current Agents UI,
zoomed scrolling, no task submission/automatic consent, unconsented worker
discovery denial and Stop acknowledgment after worker exit. Evidence directories:
`offgrid-installed-consent-edge-qX4ZOj` and `offgrid-installed-edge-worker-L3DRq7`
under the Windows temporary directory. This is a local test deployment, not a
commit, publication, signed release or general computer-use qualification.

## Post-launch Edge context and slow-inference repair (2026-10-10)

This is a different failure from the earlier missing-Edge/repeated-Chrome report.
The latest saved task, `run-f5916a119bd5700892381539b8a0c20d`, created at
08:11:30 UTC and failed at 08:15:43 UTC, discovered Edge, dispatched its launch
once and independently verified a usable Edge surface. Its 46-control observation
was 8,150 bytes. No search action was dispatched. OffGrid's conservative preflight
reported 8,449 estimated tokens against an 8,192-token allocation; this was a local
guard failure, not evidence of a llama-server context rejection. More Full access
or a browser-name substitution cannot repair it.

The source repair keeps canonical observations unchanged but projects known
native views losslessly for model input, measures full prompts through the
installed runtime tokenizer where supported, and stops counting outer HTTP JSON
escaping as model text. Unsupported older token-count endpoints retain the
conservative fallback; other failures stop safely. Short task-local archive
aliases replace an ever-growing list of digest strings. Original evidence,
legacy references, user instructions, current controls and pending launch facts
remain available after compaction/restart. Generic application guidance replaces
Chrome-specific prompt examples. No model-specific token ratio, larger automatic
context allocation, hardcoded tourism workflow or model download was introduced.

Real-model tests also exposed the separate latency/freshness boundary. The
installed Qwen2.5-7B CPU runtime produced a correct synthetic Edge next-control
call in 44.25 seconds on retry; preflight and generation both counted 4,195 input
tokens (the count itself took 40 ms). An earlier Edge attempt hit the test's
three-minute deadline; a synthetic Writer turn succeeded in 176.38 seconds with
matching 4,199-token counts. These observations are not latency qualification.
No returned model call was dispatched. The opt-in test now uses the existing
five-minute production step budget, not an unlimited timeout.

Windows protocol-v3 hosts now freshly re-observe retained structured controls
before a delayed preparation/readback and proceed only for an unchanged complete
view, runtime/state digest and exact target. Fresh capture metadata is used;
old preparations are cleared. Changed, incomplete or forged observations remain
blocked. V2 expiry, local consent, dispatch validation and replay protection are
unchanged. Other adapters retain strict expiry; pointer/vision revalidation is
not implemented by this change.

Verification: `go test ./...` passed on Windows. The complete configured Linux
race suite passed, with inference now included in CI, plus the companion worker.
Desktop and companion suites passed (65 and 57 tests respectively). The
11-call durable fixture completed at an 8,192-token allocation using the installed
tokenizer for Edge, Writer and a non-English application name; final prompt counts
were 3,846, 3,847 and 3,849. Model choices and host actions in that fixture were
controlled test doubles. Lossless projection tests cover all native driver tags,
Unicode, every field/flag and unknown-protocol fallback. Counter tests cover
runtime leases, cancellation, unavailable endpoints and private-error redaction.
The owned real Windows UIA fixture passed delayed edit/readback, forged-time and
manually changed-state rejection, independent Win32 verification and replay
checks. It touched only its disposable test window. API drift, maintained-doc
checks and strict OpenSpec validation passed.

The owner-authorized local deployment replaced the service with
`offgrid-llm:context-native-20261010` (container `a42be4ec1f28`) and installed the
matching unsigned Windows 0.4.16 desktop build. Only `offgrid` is running, on
loopback port 11611; the prior service is stopped for rollback. The Linux service
SHA-256 is `22a338a74337305fc357f58911f8ef417fdb17e22863edf4de49c581ac993831`.
Installed application, bundled service, current React UI, computer manifest and
native worker hashes match the rebuilt package. The installer SHA-256 is
`ca337c1445f3231706561fbfc8d38d0facf4f281395d44fa81c8f51d2d1af991`.

Workspace backup/restore verification and before/after comparisons passed for
workspace identity, conversation names, model IDs and task IDs/statuses. The
private service backup is under
`/home/phil/offgrid-context-native-backup-20261010-l4dvEyrV`; the independently
retained model snapshot under
`/home/phil/offgrid-consent-edge-backup-20261010-Q4mOPmBn` was revalidated against
all source-file SHA-256 digests. Desktop application and profile backups are under
`D:\offgrid-llm-backups\desktop-context-native-20261010-182013`.

The first isolated validation attempt exceeded the deployment script's 45-second
startup allowance and automatically restored the old service. The old service
itself took about 75 seconds to start while a configured MCP connection timed out.
A bounded 180-attempt readiness check passed on retry. Temporary backup helpers
and the restored-workspace validation container were removed; helper containers
now have explicit operation names instead of Docker-generated names.

Packaged startup/recovery and consent-layout tests passed. The installed desktop
connected to the external service with protocol-v3 support, current Agents layout,
scrolling at 200% zoom and Escape/focus behavior; no task was submitted or desktop
access granted by those checks. The installed worker rejected unconsented discovery
and exited on Stop. An unsaved real-Qwen text request returned the expected answer
in 11.58 seconds. The normal installed desktop was reopened after verification.
The failed task was not replayed; no personal browser profile, model installation,
commit or release was changed. A full public tourism search with the real model
remains unqualified, and CPU latency remains a known limitation rather than a
solved performance claim.

## Compact consent and Edge continuation follow-up (2026-10-10)

The reported “open edge and search tourism” task was repeatedly opening Chrome,
not completing an Edge search. Its saved catalog contained Chrome and Notepad
but no Edge; a fresh read-only Windows controller found the installed Edge binary.
The exact cause of the initial miss is not established. The long-lived controller
held its catalog from startup, and compaction discarded the previous launch from
the model view. Each changing archive index also rewrote the first system message:
service logs showed about 181 seconds of prompt processing for repeated 3,400-token
prompts. These are separate catalog, continuation and prompt-cache issues, not proof
that another model or more permissions would fix the task. The owner authorized
cancellation; the saved task remains stopped and was not replayed.

The repair refreshes only reviewed installed-app paths, preserves unchanged file
identities and rejects replaced preparations. Durable launch facts survive archival
and restart. A repeat unverified launch is rejected before dispatch and receives
bounded corrective feedback; successful discovery does not reset this repair.
Matching verification permits continuation. The archive index now follows stable
instructions and before the latest complete exchange, keeping the final tool result
available to transient browser-vision attachment. A regression asserts the intact
assistant/tool pair and final tool position. Prompts are shorter without removing permission/verification rules.
Missing named apps must be reported rather than silently substituted (model guidance,
not a claim of semantic enforcement for every user request).

The native session confirmation is now 440 × 330 logical pixels by default, with
full exclusions and technical identifiers under Details. Session scope, limits,
restricted-action summary and revocation remain visible. Explicit Segoe UI/Segoe UI
Semibold faces fix the observed stock-font appearance; Cancel has a single solid
rounded keyboard-focus outline instead of a dotted inset. Cancel remains the safe
default, and native host authority is unchanged.

Current source verification: Windows Go agent/server/computer/worker suites and
Linux race checks passed; desktop 65 and companion 57 tests passed. Regressions
include 8K-context composed MCP/desktop work, a duplicate-launch proposal followed
by discovery and verification with exactly one launch dispatched, preserved
archive evidence after restart, and catalog identity replacement. All 12 owned
native consent cases passed, including Details, short-screen scrolling, 200% DPI,
keyboard cancellation and default denial. Screenshots of the real fixture dialog
were inspected in `build/consent-compact-review/`. Windows computer/worker suites
passed sequentially after normal host shutdown, including consent reuse, stop and
UIA. The installed Edge executable also passed the owned temporary-profile/local
HTTP search fixture, with independent verification of the submitted query. Earlier
attempts while the real host held its emergency shortcut correctly failed; the
ownership rule was not weakened. This is not a personal-profile or real-model
tourism end-to-end qualification. Native consent remains English and follows the
Windows app theme; screen-reader/high-contrast user testing and real monitor
migration remain unqualified. No production-readiness claim follows from these tests.

Requested local deployment (supersedes the earlier same-day builds below):

- Running container: `3dfebd9cee4b`, image `offgrid-llm:consent-edge-final-20261010`
  (`sha256:6978244ab66d3ede2a5bfe431235ad2b0ac795500fcffe76957f8e165d431bdd`).
  Only `offgrid` runs; the preceding container is stopped with restart disabled.
  Existing loopback port, workspace identity, volumes, conversations, task states
  and model inventory were preserved. Restore validation used one temporary
  service only while the original was stopped, removed before final activation.
- Workspace backup: `/home/phil/offgrid-consent-edge-final-backup-20261010-kVTngeI0`.
  The independent 11 GB model snapshot at
  `/home/phil/offgrid-consent-edge-backup-20261010-Q4mOPmBn` was rehashed against
  the unchanged source volume before the final replacement.
- Windows installer: `build/desktop-consent-edge-final-20261010/OffGrid LLM Desktop-Setup-0.4.16.exe`;
  SHA-256 `fee89c6e337dfe6724466337448ff7e07eb894128f541ec150f93f4a6363ae5c`.
  Normal quit preceded installation. Installed application, bundled service,
  current UI and native pack match the package. Desktop backup:
  `D:/offgrid-llm-backups/desktop-consent-edge-final-20261010-160648`;
  the pre-update backup `desktop-consent-edge-20261010-154642` additionally passed
  all 609 source-file SHA-256 comparisons. This is an unsigned local 0.4.16 build,
  not a published release. The ordinary installed app was subsequently observed
  running from its expected installation path, launched by Explorer (main PID 3648),
  separately from the isolated verification profiles.
- Packaged access checks passed at 100%/200% zoom without granting consent,
  evidence `%TEMP%/offgrid-desktop-access-0A3IlS`. One preceding fixture run timed
  out finding its access button; the standalone rerun passed. That intermittent
  timeout is not treated as broad performance qualification. The current renderer
  remains `af59d55f901ed45e31a70577615d7075f39351b9aa10a33052e09aae61321e18`;
  no legacy Agents page or additional Stop popup was restored.
- Final installed-app and native-worker checks passed with isolated state:
  `%TEMP%/offgrid-installed-consent-edge-tLbI28` and
  `%TEMP%/offgrid-installed-edge-worker-vHuhyX`. Unconsented discovery was denied,
  Stop awaited worker exit, and no task was submitted. The running Linux binary's
  SHA-256 matches the built artifact. On the final service, installed Qwen2.5-7B
  passed the synthetic two-step tool check in 111.0 seconds, an unsaved text turn
  in 1.7 seconds, and the revised synthetic archived continuation in 11.2 seconds.
  These are distinct smoke workloads, not a claim that cold admission or complete
  desktop workflows meet interactive latency targets. TypeScript/API drift checks
  and strict OpenSpec validation also passed.

No model download, user-task replay, commit, push, release or archive was performed.

## Cold model check and Windows consent presentation (2026-10-10)

The installed service's logs showed a 90.001-second failed access-attachment
request, followed by a successful warm retry in 23.9 seconds. Qwen cold loading
took 30.6 seconds and the first synthetic tool-call test took 40.9 seconds; the
old single 90-second deadline covered queueing, loading and both probe calls.
The generic error incorrectly blamed current inference rather than identifying
which phase failed.

The repair retains lifecycle admission (configured load/queue deadline), then
starts a separate three-minute two-step probe. HTTP clients allow the default
load, probe and optional vision budgets. Transient saved-input failures return
503 and retain the pending task; incorrect tool output remains 422. No model
is silently replaced and no host tool is dispatched by a smoke check.

Windows host consent now uses an owned native monochrome OffGrid dialog with
explicit Enable access/Cancel labels, readable grouped permission text, a fixed
action row and scrollable details. Cancel remains the default. The worker still
owns consent and revalidates OS/session state afterward; neither the renderer
nor the service can set an approval boolean. No extra Stop popup was restored.

Verification for this patch:

- Go server, inference and agent suites passed; race coverage for server,
  inference, agents and computer passed. Regression fixtures cover admission
  longer than the probe budget, cancellation without interrupting existing
  work, probe deadlines, lease release, retryable saved-task errors and no dispatch.
- Windows native computer/worker fixtures passed sequentially, including
  real local consent, session reuse, local Stop and owned UIA operations.
  Nine additional consent fixture cases cover Allow, intentional keyboard
  activation, Cancel, close, Escape, initial Enter, a bare command, short-window
  scrolling and synthetic 200% DPI layout. No personal app or installed grant
  was approved. Fixture-only screenshots were visually checked under
  `build/consent-runtime-review/` (ignored build artifacts).
- Desktop tests: 65 passed. Companion tests: 57 passed. TypeScript, generated
  API drift and production UI build passed. Focused browser regressions:
  40 passed (task access, current task layout and task-first flows, including
  narrow/RTL views). The existing bundle-size warning remains.

The native dialog follows Windows app theme/high-contrast colors, not a manual
renderer theme override. Native consent remains English. Real monitor migration,
screen-reader/high-contrast user testing, macOS/Linux presentation, and full
real-model workflow qualification are not established by these Windows fixtures.
The source-test checkpoint was followed by the separately requested local deployment:

- Container `bd044491ac56`, image `offgrid-llm:consent-runtime-20261010`, is the
  single running OffGrid service on the existing port and data/model volumes.
  `offgrid-before-consent-runtime-20261010-1791614014` is stopped with restart
  disabled. Private backup `/home/phil/offgrid-consent-runtime-backup-20261010-HifUYZJQ`
  passed workspace restore and source/model-copy SHA-256 comparisons. Clone
  validation ran only while the original was stopped and was removed before cutover.
- The Windows installer replaced the app after graceful shutdown and verified
  backup `D:/offgrid-llm-backups/desktop-consent-runtime-20261010-142817`.
  Installed app, service, UI and native-pack hashes match the local package.
  Installer SHA-256: `7b742e6c8fc88614520d5ec375c658993ab572c6ea8add255147f11b729ca4cb`.
- Packaged/installed Electron and installed-worker checks passed with isolated
  state, denied unconsented discovery, confirmed worker exit, scroll/dismiss tests
  and no saved task submissions. Evidence: `%TEMP%/offgrid-desktop-access-l89NWy`,
  `%TEMP%/offgrid-installed-consent-runtime-SBcpj7`, and
  `%TEMP%/offgrid-installed-chrome-worker-tpSzFM`.
- Real installed Qwen2.5-7B passed the synthetic two-step tool check in 89.1 seconds;
  an unsaved text request returned `READY` in 1.6 seconds. Neither test controlled
  the desktop or replayed a task. Final service health was healthy with no active
  or queued inference. The normal installed desktop was subsequently observed
  running from its expected installation path, with no second Windows service.

The local 0.4.16 packages remain unsigned. No model download, commit, push or
publication occurred. These results do not replace platform, soak or pilot gates.
The owner then rejected the consent window's oversized presentation; task 6.2 is
reopened for a compact layout, not counted as aesthetically accepted by these tests.

## Chrome continuation and single-choice access repair (2026-10-10)

The reported task launched Chrome and verified an application surface before
failing the 8,192-token context guard. The old error incorrectly said no tool
had run. Its observation contained only seven region elements, not the profile
choices. Read-only investigation did not replay the task or select an account.

- Context compaction now preserves the latest complete exchange by role rather
  than keeping a fixed four-message tail, while retaining user instructions,
  incomplete calls and exact durable archive evidence. Desktop responses are
  capped at 1,024 output tokens, reserving room for the current controls and MCP
  schemas. Failures now distinguish earlier recorded effects from further work
  that was not dispatched. The composed desktop/MCP workflow passes at 8,192
  tokens with enlarged provider descriptions and observation data.
- A disposable, real Chrome fixture reproduced both cold accessibility trees
  and delayed text read-back. The host re-observes for document readiness and
  polls read-back without re-dispatching a mutation. Empty non-interactive layout
  containers no longer inflate the flat model observation. Three consecutive
  typed local searches passed; the final filtered-view fixture passed again.
  The HTTP server independently verified the exact query. Personal Chrome
  profiles and the user's failed task were not controlled or replayed.
- Full task access is now one explicit menu choice, not a checked policy plus
  an Enable button. It remains unchecked until the host grants access; refusal,
  revocation, navigation and saved defaults cannot claim permission. Narrower
  policies and in-app Stop revoke session access. The menu remains scrollable.

Checks: Windows `go test ./...`; WSL race tests for agents/computer/server;
TypeScript/API drift/build; **65 desktop**, **57 companion** tests; and the
built-renderer/service suite **287 passed, 10 opt-in skipped**, evidence
`%TEMP%/offgrid-web-qualification-nY90qQ`. Native UIA/worker/consent/Stop fixtures
passed sequentially. One native checkbox run during the parallel test load
reported an uncertain outcome; both its isolated rerun and the complete final
sequential native suite passed without relaxing focus validation. Actual physical
lock/suspend, personal-profile chooser workflows, broad public-site search,
non-Windows platforms and model-driven task success rates remain unqualified.

Coordinated local testing deployment completed on **2026-10-10**:

- Only `offgrid` is running: container `87dde92a0f8f`, image
  `offgrid-llm:chrome-access-20261010`, on the existing loopback port and named
  data/model volumes. Previous container
  `offgrid-before-chrome-access-20261010-1791609481` is stopped, restart disabled.
- Private backup `/home/phil/offgrid-chrome-access-backup-20261010-PNCnZIpm`:
  workspace archive verified/restored; separate model copy matched source
  SHA-256 inventories. Restored-workspace checks ran only while the original
  service was stopped; that validation container was removed before activation.
  Workspace identity, conversations, model IDs and task IDs/states matched.
- The installed Windows app was normally closed, backed up to
  `D:/offgrid-llm-backups/desktop-chrome-access-20261010-131311`, then replaced
  by the matching installer. Installed app, service binary, UI and native-pack
  hashes match `build/desktop-chrome-access-20261010/win-unpacked`.
  Installer SHA-256:
  `ddb1a603c066491931550c7c1ccb88bbdeed098b4491773e77fc585725f82764`.
- Packaged and installed Electron checks passed against UI
  `43ed66cee2b8e9a5875159c38f15f11431858961ba402d8869e1b1e34a93a555`.
  Evidence: `%TEMP%/offgrid-desktop-access-RI9i1y` and
  `%TEMP%/offgrid-installed-chrome-access-ZDbndD`. The installed app connected
  to the existing container, not a second bundled service. The real host reported
  access disabled; the single choice and 200% scrolling checks did not grant
  consent, submit a task or change saved work.
- Installed native-worker startup, denied unconsented discovery, and Stop
  confirmed by owned-process exit passed in isolated state:
  `%TEMP%/offgrid-installed-chrome-worker-H2w6sN`. Live read-only Agents/history/
  connection checks passed. Unsaved Qwen2.5-7B text inference returned `READY`
  in 22.1 seconds; that is not an end-to-end computer task or latency qualification.

The packages remain **unsigned local 0.4.16 testing builds**. No commit, push,
publication, automatic task replay or personal-profile selection occurred.
These checks are not release, pilot or broad computer-use qualification.

## Desktop-session access and in-app Stop repair (2026-10-10)

The owner explicitly approved remembering Full access until the desktop session
ends, and removing the additional floating Stop window. This refines
`task-wide-desktop-access`; it is not unrestricted authority or a new task runner.

- The private native worker retains affirmative session consent in memory for
  one service/workspace/actor. Separately enrolled tasks still receive distinct
  60-minute/500-action grants. Release clears task authority and prepared state;
  duplicate-action tombstones survive. No default, renderer flag, navigation,
  saved preference or host restart creates/restores consent.
- Explicit Enable/Revoke and host-owned status are in the current access menu.
  Account/workspace changes, quit, lock/suspend and local Stop revoke it. The
  desktop's in-app banner remains available between tasks and across navigation;
  trusted local Stop/status IPC stays available offline and on recovery pages.
- Desktop-owned Windows workers have no second Stop popup. A hidden independent
  native thread still handles the emergency shortcut and OS-session events.
  Renderer crash/unresponsiveness revokes control. Standalone/non-Windows
  compatibility stop surfaces are not silently removed.
- Internal menu scrolling no longer triggers placement recalculation. Focused
  regions explicitly handle scroll keys without intercepting input controls;
  Escape restores the trigger even after asynchronous policy changes. A real
  packaged Electron check exposed default End-key behavior differing from the
  browser fixture; that defect was fixed and the packaged check then passed.

Checks completed in isolated state:

- Windows `go test ./...`; WSL Go race tests for computer, worker, agents and
  server; TypeScript/API drift and the production renderer build.
- **65 desktop** and **57 companion** tests, including two task sessions on one
  worker, actor/workspace isolation, no default consent, cancellation during
  consent, offline revocation, and pack rejection when desktop transport is absent.
- Built-renderer/service suite: **286 passed, 10 opt-in skipped**, initially
  `%TEMP%/offgrid-web-qualification-LADyOL`; final focused scrolling, access and
  shell rerun **32 passed**, `%TEMP%/offgrid-web-qualification-IRkH0n`.
  The complete final suite passed again (**286/10**) against the final renderer
  and rebuilt service: `%TEMP%/offgrid-web-qualification-vEolK0`.
- Opt-in native fixtures passed one real owned consent dialog followed by two
  separate authority-only tasks without another consent/Stop popup, and confirmed
  private Stop by worker exit. No user application was opened or controlled.
  Hidden-monitor shortcut/lock/suspend tests inject native notification messages;
  they do not establish physical keyboard/lock/sleep qualification.
- Unpacked unsigned Windows build: `build/desktop-session-access-20261010`.
  Its copied UI/native pack passed integrity checks and the real main/preload/UI
  smoke test at **100% and 200% Electron zoom**, with no task submission or consent.
  Evidence: `%TEMP%/offgrid-desktop-access-lULx1b`; UI build
  `338ee260fdbadbc0bba635738896a319cf0e5f55923ec0af30dedb8dcf4b11ec`.
  The package check is added to Windows CI. The ordinary builder download timed
  out; the successful build used the already-cached identical Electron 43.7.9
  archive, without changing dependency versions or proxy/security settings.
  The copied production native worker also passed private-IPC startup, refused
  discovery without consent, and acknowledged Stop by exiting; evidence state:
  `%TEMP%/offgrid-session-pack-Apr9Wh`.

Coordinated local testing deployment completed on **2026-10-10**:

- Replaced the existing container with `offgrid-llm:session-access-20261010`,
  container `a51c40064d8f`, on the same loopback port and named data/model volumes.
  Exactly one OffGrid service is running. The previous container is stopped with
  restart disabled: `offgrid-before-session-access-20261010-1791566739`.
- Private WSL backup: `/home/phil/offgrid-session-access-backup-20261010-3DAA9l70`.
  The stopped-workspace archive passed verification and restore/database checks;
  the separate 11 GiB model copy matched SHA-256 inventories of the original.
  Restored-workspace validation ran only while the original service was stopped;
  the validation container was stopped and removed before live activation.
  Workspace identity, conversation names, model IDs and task IDs/states matched
  before/after. Saved pending/interrupted tasks were not approved or replayed.
- The installed Windows application was normally quit, privately backed up at
  `D:/offgrid-llm-backups/desktop-session-access-20261010-012225`, and replaced
  with the matching NSIS installer. Application, UI, service binary and native
  host pack file hashes match the tested package. Installer SHA-256:
  `7685e7a7a9177834f54bf5f46a13a4ed6632e4bcbaa96f8235a795f9832a442b`.
  This remains an **unsigned local 0.4.16 testing build**, not a published release.
- Live read-only browser checks passed current Agents layout, legacy-layout
  absence, access-menu dismissal/focus restoration, history and connection/tool
  navigation. A synthetic unsaved Qwen2.5-7B text request returned `READY` in
  14.4 seconds; this was not a computer-action workflow or a latency qualification.
- The actual installed Electron app passed native-pack integrity and connected
  to the existing container, not a second bundled service. Its real preload/UI
  reported Full access disabled until explicit consent; the access menu scrolled
  correctly at 200% zoom and Escape restored focus. No task, consent or host
  control was triggered. Evidence: `%TEMP%/offgrid-installed-session-access-QnRyWn`.
  Final installer-package fixture rerun also passed 100%/200% zoom:
  `%TEMP%/offgrid-desktop-access-PzSVDM`.

No commit/push/release was made. Real-model application workflows, physical
lock/suspend, screen-reader use, non-Windows v3, signing and soak/pilot gates were
not qualified by this deployment. The earlier deployed build/evidence below is
historical. Preserve current host dispatch journals if rolling back; never restore
an older journal over newer action records.

## Task-wide Windows desktop authority (2026-10-09)

The `task-wide-desktop-access` change replaces mandatory window selection on
the matching Windows x64 desktop with one explicit task consent. New desktop
tasks are durably held before inference, then attach the authorized host grant
idempotently. Approval policy and optional limited-window scope remain in the
current composer; navigation never grants access or resubmits work. Existing
v2 grants are not expanded. The larger shared Chat/OAuth/visual-control program
is not completed by this host-access slice.

- Protocol 3 separates task authority from observation-bound execution targets.
  The existing host journal and supervisor govern reviewed application launch,
  focus and structured control, with a 60-minute/500-change limit. Local stop,
  session lock/disconnect and suspend revoke authority independently of service
  connectivity. MCP tools remain available alongside computer tools.
- Full Windows `go test ./...` and Linux race checks for computer, agents, server
  and capabilities pass. Desktop tests: **61 passed**. Companion tests:
  **56 passed**. TypeScript, API drift, strict OpenSpec validation and the current
  renderer build pass. The full browser suite passed **282 tests**, with
  **11 opt-in skips**. The six focused access regressions also passed with the
  desktop-scope selector checked in all nine locales and narrow RTL layouts.
  A redundant aborted task read on React remount was fixed
  and the previously failing work-continuity test passed in the full rerun.
- Tests cover held-task persistence/restart, duplicate submission/attachment,
  cross-actor and policy mismatch, v2 downgrade rejection, executable replacement,
  stale targets, timeout, cancellation, process reuse, grant expiry, action budget
  and duplicate dispatch. The durable Chrome-without-URL integration fixture
  composes a real local MCP HTTP fixture with the desktop tool queue; its model
  and companion responses are controlled fixtures, not a real-model benchmark.
- Opt-in Windows fixtures exercised actual UI Automation, local consent, private
  worker IPC, read-back and local stop. A disposable owned Win32 application
  exercised the production launcher, independently observed surface readiness,
  process reuse and journal replay. Only fixture-owned processes/windows were
  controlled; no user's task or application was replayed. Lock/suspend notification
  regressions inject the corresponding messages; they do not establish a full
  physical sleep/wake or locked-session qualification.
- The final packaged Windows service, renderer and companion passed isolated
  startup/recovery checks (`%TEMP%/offgrid-desktop-startup-8veVxh`). A packaged v3
  worker passed private-IPC startup/desktop-identity checks and rejected discovery
  before consent. The installer is **unsigned** and retains local version 0.4.16;
  no new release, signing, commit or publication is implied by these checks.
- Windows structured control remains a preview with reviewed application paths.
  Launcher stubs that redirect to another executable are not broadly qualified.
  Native vision, full host-file workflows, all-application support, real-model
  personal Chrome workflows, macOS/Wayland/X11 v3, screen-reader qualification,
  and the soak/pilot gates remain unqualified. Existing speech profiles were not
  requalified by computer-control tests.

Owner-authorized local testing deployment:

- Replaced the existing container with `offgrid-llm:task-wide-20261009`
  (`e57942f433ad`) on loopback port 11611. Only `offgrid` runs. Its predecessor
  `offgrid-before-task-wide-20261009-1791561340` is stopped with restart disabled.
  The verified workspace archive and separate model copy/checksums are under
  `/home/phil/offgrid-task-wide-backup-20261009-ZeflR4Op`. Restored-workspace
  validation ran while the original service was stopped, before live activation;
  workspace identity, conversation names, task IDs/statuses and model inventory
  match the pre-cutover snapshots.
- Reinstalled the matching Windows application at
  `C:\Users\phil\AppData\Local\Programs\OffGrid LLM Desktop`. The protected
  application/configuration/profile backup is
  `D:\offgrid-llm-backups\desktop-task-wide-20261009-235619`; desktop model
  directories were left untouched. Executable, app archive, backend, renderer
  and native-pack hashes match the final package. A service rollback must not
  restore an older host duplicate-action journal.
- Service and installed renderer share UI build
  `bce29301b7c448d861b4ac1f61358a99d6f3ca40313ef22acd9fe8168a57a66b`.
  The installed Electron smoke check connected to that external workspace,
  verified protocol 3, default whole-desktop scope, upfront policy, keyboard
  dismissal/focus and absence of retired Agents controls. It submitted no tasks
  or approvals. Evidence: `%TEMP%/offgrid-installed-task-wide-d6xTE3`.
  Hidden-window screenshots used Electron's explicit capture API after a
  Playwright screenshot timed out; the subsequent installed-app check passed.
- Live browser read-only checks passed. A synthetic unsaved Qwen 7B text request
  returned the expected response in **50.7 seconds**, including model loading
  and processing; this is not an interactive-latency qualification or a real
  application-control acceptance run. The user's saved tasks were not replayed.
  The application was closed normally after isolated smoke testing; reopen it
  normally to test the new task flow. No WSL/VPN changes, commit, push or release
  were performed.

## Task access intent and false completion repair (2026-10-09)

Recent owner-visible Chrome/search tests exposed two distinct failures: a
text-only model response was shown as completed work, and an invalid access
proposal followed by instructions to the user also became completed. Source
change `task-access-intent` addresses these boundaries without claiming that
the still-window-bound companion supports task-wide desktop authority.

- The current composer offers computer approval policy before Start. New tasks
  persist that intent in the request digest and snapshot; matching local consent
  is still required. The access view no longer selects the policy again for
  those tasks. API and runner both reject changed/mismatched policy at attachment.
- Rejected proposals followed by prose use the same three-repair budget and
  pause rather than claim completion. The unresolved marker survives resume and
  restart; an unrelated successful tool cannot erase missing computer access.
  Explicit user corrections can change the requested outcome. Uncertain actions
  remain outside automatic repair.
- Text-only task-first results expose `answer_only`, shown as **Answered** with
  a no-external-actions notice. This preserves legitimate writing tasks without
  an English keyword classifier or a forced dummy tool call. No new field
  implies universal task verification.
- Full Windows `go test ./...` passes. WSL Go 1.26.6/GCC race checks pass for
  agents, server, computer and capabilities. Focused regressions cover the
  malformed Chrome request followed by user-facing JSON, unrelated reads,
  restart, policy mismatch, request conflicts and legacy omission compatibility.
- TypeScript, API drift and production renderer build pass. The isolated
  built-renderer/service suite passed **281 tests**, with **10 opt-in skips**;
  evidence: `%TEMP%/offgrid-web-qualification-uRUeHZ`. The expanded access-intent
  browser suite passed **5 tests**, including a pre-upgrade pending request.
  Nine-locale narrow-layout checks and inspected English/Arabic screenshots
  preserve the monochrome UI. Desktop unit tests: **58 passed**. Documentation
  tests: **6 passed**; maintained-document link check and strict OpenSpec
  validation pass.
- These checks use isolated state and synthetic tools. No real-model Chrome
  launch/search success, native side effects, installed-package upgrade,
  screen-reader qualification, or broad desktop reliability is claimed.
  Those implementation checks did not change the current container, installed
  host, task history or user models.
- Subsequent owner-authorized testing deployment rebuilt the service/renderer
  and replaced the existing container with `offgrid-llm:task-access-20261009`.
  A verified stopped-workspace archive, separate model copy/checksums, and
  successful restored-workspace checks preceded activation. Exactly one OffGrid
  service runs; the previous container is stopped with restart disabled.
  Workspace identity, saved conversations, task IDs/statuses and model inventory
  were preserved. Live browser checks verified the upfront policy, outside-click
  and Escape dismissal, historical answer-only notice and absence of legacy
  Agents controls without submitting or replaying tasks. An unsaved Qwen 7B
  text-inference smoke check passed (52.8 seconds including loading/processing,
  not a latency qualification). Installed speech profiles still report available;
  speech inference and native control were not requalified. The installed desktop
  host was not changed by that container deployment. This is an uncommitted
  local test build, not a release.
- The owner then explicitly requested Windows desktop replacement. A fresh
  unsigned local 0.4.16 NSIS package was built with the matching renderer,
  Windows service and integrity-checked computer pack. Desktop unit tests
  passed (58), and packaged Windows startup/recovery passed with isolated state
  (`%TEMP%/offgrid-desktop-startup-ANzzrN`). After graceful desktop shutdown,
  the previous application, local configuration/data (excluding unchanged model
  folders) and Electron profile were backed up before same-location per-user
  reinstall. Installed executable, archive, service, UI and native worker hashes
  match the build. An installed-app smoke check verified attachment to the
  existing container, the upfront policy, Escape/focus behavior and absence of
  legacy Agents controls without submitting tasks
  (`%TEMP%/offgrid-installed-task-access-PKHeWM`). This does not add task-wide
  desktop authority, change pending tasks' recorded policies, or establish
  signing/native workflow qualification.

Still required from the approved task-wide program: one upfront local task
consent, protocol-v3 desktop grants, host-issued application discovery and
durable launching, cross-app target switching, host-independent stopping and
installed workflow evidence. The policy menu does **not** finish that program,
and the remaining compatibility app/window picker must not be mistaken for
the intended final workflow.

## Agent access proposal recovery (2026-10-09)

Source-only `agent-access-recovery` implementation on
`81871e396392cb211e59ce942b18da66b23dc501` plus uncommitted changes.
The exact `request_computer_access` proposal for Chrome in browser mode with an
empty URL now receives corrective app/URL guidance, not a fabricated permission
failure. The isolated server/runner regression reaches local access in the same
run without dispatching an action. This is not evidence of Chrome launching.

- Windows `go test ./...` passes, including bounded repair, exhaustion, explicit
  recovery, restart persistence, cross-actor denial, abandoned batches and
  no automatic repair of hard denials or uncertain execution errors.
- `go test -race ./internal/agents ./internal/server ./internal/computer
  ./internal/capabilities` passes in WSL (Go 1.26.6, GCC). Windows lacks a C
  compiler. The first WSL attempt exhausted its already-full `/tmp`; the passing
  run uses `/home/phil/offgrid-access-race.47cC1h`. No unrelated files were removed.
- API drift, documentation link checks and strict OpenSpec validation pass.
- Current renderer and desktop layouts are unchanged. No existing task, model,
  installed app or container was modified; no deployment or publication occurred.

Not run: real-model recovery evaluation or native application workflows. This
stage does not deliver desktop-wide grants, typed application launching, Chat
tool execution, OAuth or cross-platform qualification. These remain required
stages of the approved task-wide computer-use program.

## Composed task capabilities (2026-10-09)

Source-only `agent-capability-composition`, based on
`81871e396392cb211e59ce942b18da66b23dc501` plus uncommitted changes.
Attaching computer access to a task-first job now preserves policy-authorized
registry/MCP tools. Discovery and dispatch recheck policy; computer approval does
not approve MCP actions or grant service/container filesystem access to the host.
Legacy computer-only runs retain their narrower tool set.

- Isolated tests cover revoked policy, actor boundaries, reserved tool identities,
  local execution exclusion, host/service file separation, provider-specific
  observation ordering and approval records. A durable mixed-tool test uses a
  real local HTTP MCP fixture and a controlled companion; it does not control an
  installed application or contact a user's external account.
- Full `go test ./...`, WSL race checks for agents/server/computer/capabilities,
  API drift, documentation checks and strict OpenSpec validation pass.
- Real-app/model workflows, OAuth, task-wide desktop grants and platform input
  qualification remain subsequent program work. No deployment or publication.

## Retired Agents presentation (2026-10-09)

Source-only `retire-legacy-agent-workspace`, on the same base revision above.
Read-only inspection found the live `offgrid` container running version 0.4.16
at that revision, not an older image. Its task store reported
`check SQLite runtime: context deadline exceeded`. Storage health withdrew the
task-first capability, which selected a second, obsolete Agents renderer. This
establishes the UI fallback cause, not the underlying SQLite timeout cause.

- Removed the legacy composer/result page, mode picker, exclusive setup/scroll
  helpers, dead styling/client wrappers and unused syntax-theme asset. Current
  tool/connection management remains in a separate focused component. Current
  task access, approvals, voice, history, compatibility APIs and user data remain.
- Contract support no longer depends on transient storage health. Task operations
  still fail closed on unavailable storage. Browser and desktop-renderer fixtures
  prove cold incompatibility, health loss, failed identity refresh, draft retention
  and recovery without legacy controls, automatic submissions or action replay.
- Windows `go test ./...` and WSL race checks for agents/server/computer/
  capabilities pass. TypeScript, production renderer build and API drift pass.
  The build retains the existing large-chunk warning.
- `node dev/scripts/test-web-workspace.mjs build/windows/offgrid.exe --workers=2
  --reporter=line` passes **277 tests, ten opt-in skips, zero failures** using the
  disposable service/profile `offgrid-web-qualification-EcXguh`. An earlier full
  run exposed three outdated test fixtures; they were corrected before this
  complete rerun. No live container or installed workspace was used as test state.
- Desktop unit tests pass **58 tests**. Documentation checks and strict OpenSpec
  validation pass. This is not installed-package, OS input, independent locale
  review, real-model or cross-platform qualification.

No saved data was removed or migrated. Source deletions are recoverable through
Git. The running container and installed application have not been replaced by
this change; they require rebuilt artifacts before it is live. The live task
store initialization error still needs separate recovery/diagnosis; removing the
legacy UI does not clear that error, recreate its database or retry uncertain work.

Subsequent operator-authorized local deployment (2026-10-09):

- Rebuilt application/UI from the working tree with revision suffix `-dirty`,
  retaining the unchanged local speech/inference runtime. Replacement image:
  `offgrid-llm:agents-current-20261009`, identity
  `sha256:d7f5ca3a7d4590139b404d83c99bd69bd7e603b463e63cc92ed295aef525bd73`.
  This is a local testing build, not a version bump or published release.
- Stopped the original service, used its version-matched workspace backup/verify
  commands, copied separate model storage and recorded checksums. Private WSL
  backup: `/home/phil/offgrid-agents-backup-20261009-IdowR18V`. Restoring the
  archive and starting the new image against that isolated copy passed before
  the live volumes were attached. The temporary validation container was removed.
- `offgrid` is the only running container, still bound to `127.0.0.1:11611` and
  the same data/model volumes. Previous container
  `offgrid-before-agents-20261009-1791547407` remains stopped with restart disabled.
- Live health, task history, saved conversations, model inventory, Knowledge
  status and desktop bridge metadata respond. Workspace identity is unchanged;
  three conversations, five tasks and four legacy-projection models remain.
  UI build ID is `914eab9ff6b2edf10330ca684ed3e47feb6350ec8e94cc889699359250fabcf8`.
- Read-only browser smoke checks pass for the current composer, tools and
  connections, absent legacy controls and no storage-error banner. They submitted
  no work. The SQLite initialization timeout did not recur after replacement;
  this does not establish its original cause or prove it cannot recur.
- Speech status reports the installed Qwen ASR and Piper profiles available.
  The synthetic stateless chat check timed out after 120 seconds while health
  reported another generation active and the check queued. Existing work was not
  canceled. After health confirmed zero active/queued requests, a separate
  stateless check using the already loaded Qwen model returned `OK` with finish
  reason `stop` in 2.03 seconds. This proves one inference request, not latency
  qualification; speech availability is not a new audio-quality qualification.
- No commit, push, publication or installed desktop-package replacement occurred.

## Desktop/service compatibility repair (2026-10-09)

Source-only implementation of `desktop-service-compatibility`, based on
`6a311f40ab721d41d76df9c0e9e3f87b9c70553f` plus this uncommitted change.
The reported desktop 0.4.15 / external service 0.4.14 failure was caused by
release equality being used for both independently managed services and bundled
children. The new policy checks external API/renderer bridge compatibility;
owned children still require exact package version and renderer consistency.

- Renderer builds emit an index-bound, bounded compatibility manifest. The Go
  identity projection distinguishes missing, invalid and ready metadata from the
  actual UI root. Desktop and reused-runtime packaging verify the copied files.
- The reviewed legacy tuple is version 0.4.14 or 0.4.15 and UI fingerprint
  `a7336324e4ea09e8a9f5e25aabf2a53c2f9cb39b81eb990a081ba9a84e66f617`,
  bridge protocol 1. The main/preload/computer-runtime diff between these tags
  is empty. Regression fixtures deliberately use a different new-package UI
  fingerprint. Unknown tuples and explicitly invalid declarations remain denied.
- Explicit reconnect and full-document reload reassess attachment. Observed
  identity changes revoke prior computer authority before accepting another
  workspace; failed revocation keeps the bridge unavailable. A disconnected
  external workspace does not silently become a new bundled workspace on Retry.
- Recovery gives localized, reason-specific guidance without duplicate text.
  Settings distinguishes ownership and compatibility basis. Installer completion
  says application installation, not service readiness or a Docker upgrade.

Executed local checks (Windows x64 build 26200, Electron 43.7.9):

- `go test ./...` passes. Windows race execution cannot run without a C compiler;
  the eleven-package race command in the OpenSpec task list passes in WSL with
  Go 1.26.6 and GCC. Its temporary directory is under `/home/phil` because WSL's
  existing `/tmp` is full; no unrelated temporary files were removed.
- API generation/drift, TypeScript and production renderer build pass.
  `node dev/scripts/test-web-workspace.mjs build/windows/offgrid.exe --workers=2`
  passes **280 tests, ten opt-in skips, zero failures**, using the temporary
  service/profile `offgrid-web-qualification-kEqBQL`, not the live workspace.
- `npm --prefix desktop test` passes **58 tests**, including malformed/hostile
  identities, owned/external separation, legacy fallback, consent/auth/IPC
  boundaries, stop failure, and disconnected-service retries.
- After a fresh `electron-builder --dir --win --x64 --publish never`, packaged
  startup, browser and theme scripts pass. Evidence directories under Windows
  `%TEMP%`: `offgrid-desktop-startup-QRuZHi`,
  `offgrid-packaged-browser-rZtphL`, and `offgrid-desktop-theme-LZie9N`.
  Startup first window was 1,620 ms and incompatible-contract recovery 1,803 ms.
  These are individual fixture measurements, not latency qualification.
  Tests cover legacy relaunch, contract changes on reload, mixed-package refusal,
  external-service survival, all six routes at true 200% zoom, light/dark/system
  themes and nine readable language options. Browser actions use only the
  controlled fixture and owned browser; no user's application is controlled.
- Docker `ui-builder` and `application-artifacts` targets build, and the
  reused-runtime `checked-ui` stage verifies the copied manifest. Local images
  are `offgrid-compat-ui-check:local`, `offgrid-compat-application-check:local`
  and `offgrid-compat-reuse-check:local`. These are build checks, not a live
  container replacement or full GPU-image qualification.
- The bounded Windows installer diagnostic/probe suite passes, including
  deliberately blocked/invalid capture fixtures; their expected warnings are
  not installation failures.
- `electron-builder --config installer-test.cjs --win nsis --x64 --publish never`
  and `test-windows-installer.ps1` pass with the test-only product registration.
  Final evidence: `%TEMP%/offgrid-install-b77e186fffe941dfb386561a1a129435`,
  installed startup: `offgrid-desktop-startup-1MlLuE`. Clean install, silent
  repair, actual Finish launch, silent running-app refusal, consent-driven
  reinstall and uninstall preserve both the desktop fixture and the running
  legacy external service's sentinel digest. Silent refusal leaves the desktop
  process alive too. Finish closes in 79/61 ms with launch checked/unchecked;
  these are fixture observations, not a performance guarantee. Native visible
  controls contain the installation/external-service guidance. The PrintWindow
  image omits some static labels, so it is not the sole oracle for that text.
- An intermediate installer repeat failed because the separately running probe
  suite's deliberately frozen window shared the test-title prefix. The installer
  did not hang. Only that repeat's test app was gracefully closed/uninstalled;
  diagnostics and fixture data remain. The final installer run above is serial
  and passes all assertions. Run the probe and lifecycle suites sequentially.
- Documentation tests (6), link checks (93 maintained files), strict OpenSpec
  validation and `git diff --check` pass. These checks do not replace runtime
  tests or authorize a release.

Limitations: Linux and macOS Intel/Apple Silicon packaged startup tests for this
change have not run on their platform runners. No cross-platform qualification,
publisher signature, notarization, native-control/model qualification, or
soak/pilot completion is claimed. No production registration, installed app,
workspace data, model, live container, VPN or WSL configuration was changed.
There is no commit, push, tag movement or publication in this validation.
Existing installed desktops still require an explicitly approved updated
package; source changes alone do not fix their running binary.
