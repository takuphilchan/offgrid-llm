## Native document provider implementation and test deployment (2026-10-12)

This pass adds native provider code and a local testing build. It does **not**
finish the program: OpenSpec remains **21/32**. The earlier entries below are
historical evidence, not the current deployment or capability state.

- Windows now negotiates the document contract. Bounded RawView UIA discovery
  includes template containers. An exact Notepad package/AUMID profile binds the
  selected tab, content presenter and observed AddButton; package revision and
  layout are revalidated before its single invocation. Creation succeeds only
  after distinct blank-document evidence. Tests cover wrong versions, unrelated
  apps, ambiguous controls, changed layouts and protected content. There has
  been **no installed Notepad creation/input test**; this is not qualification.
- AX/AT-SPI providers retain native references and separate document, editor,
  generation and text revisions. Native range mapping and preimage checks guard
  selection, insertion and replacement. Multi-call AT-SPI replacement never
  retries/restores after deletion; it rechecks the document before insertion.
  Missing marked-text evidence blocks edits. Generic AT-SPI composition remains
  unknown; its document-edit path is consequently unavailable. Unix and browser
  creation profiles are still missing, not merely awaiting tests. The AX code
  has not been compiled or run on macOS.
- The real owned GTK fixture exposed libatspi process-global shutdown/restart
  cache failure and an Xvfb/WSLg environment mismatch. Initialization now lasts
  for the owned worker process; closing an adapter still releases its references.
  The isolated runner explicitly selects X11 and updates its private D-Bus
  activation environment. Its legacy Save assertion polls for the independently
  observed result after **one** shortcut, never resending it.
- The final isolated Linux native suite passes with AT-SPI 2.60.4, GTK3, Xvfb
  and an owned fixture. It verifies real document inventories/read-back across
  tabs with a reused editor and Unicode, plus legacy control input/read-back,
  checkbox and stale-observation tests. Evidence:
  `/home/phil/ogn-kaE6JLgp/offgrid-native-linux.dC3CcX`. It does **not** test new
  document edits through a stable composition provider, installed editors,
  real Wayland or macOS. No personal applications/documents were controlled.
- Windows `go test ./...`, companion **73/73**, desktop **65/65**, API drift,
  TypeScript and production UI build pass. The existing large-chunk warning
  remains. The complete CI race set plus the host command and Linux-native-tagged
  computer/host packages pass in WSL, including the final provider refinements.
- The new isolated service passes **21/21** workflow/layout/streaming/replay
  browser tests (`offgrid-web-qualification-9HSQMT`). The matching Windows package
  passes startup/recovery (`22YpQ2`: first window 930 ms, recovery 1086 ms), owned
  packaged browser (`DAcpO4`), access UI (`JAu7Ns`), and theme/zoom/locales
  (`CQZ33R`). These are disposable test profiles, not native application or
  model-accuracy qualification. Six harness and six documentation unit tests pass.

Remaining implementation includes reviewed Unix/browser creation semantics,
usable Unix composition profiles, and complete native/model workflow harnesses.
Installed Windows/another-editor/browser workflows, macOS Intel/Apple Silicon,
real Linux X11/Wayland, model latency/completion, signing and release gates remain
unverified. Do not mark 32/32 or describe these gaps as only testing.

### Local replacement receipt

The user authorized replacing the existing service and updating the desktop.
Both operations completed; they are a testing deployment, not a new release.

- Exactly one running service: `offgrid`, container `da9fe65955b7`, image
  `offgrid-llm:document-providers-20261012`, image digest
  `sha256:63e02da3fe384bda98a709945b90cf44240c893a039b18e2e9f61da305f7af78`.
  `/health` reports healthy. The service binary hash matches the new build.
  The previous container is stopped with restart disabled as
  `offgrid-before-document-providers-1791793445`; it is not a second service.
- After confirming no running work, inference or computer sessions, the stopped
  service data was copied to the private directory
  `/home/phil/offgrid-document-provider-backup-0silL2fp`. Its workspace archive
  passed the workspace verifier and SHA-256 check. Existing model volumes were
  retained without alteration; this operation did not make another model-weight
  backup or download/replace a model.
- The workspace identity, current UI build, all saved task IDs/statuses,
  conversation names, language/speech model IDs and ASR/TTS availability match
  their pre-replacement values. Uncertain/interrupted tasks were not resumed,
  reconciled, removed or converted to a success.
- The matching unsigned local **0.4.16** Windows package was installed in the
  existing per-user directory. The previous application and desktop/profile
  state are in the ACL-protected
  `D:\offgrid-llm-backups\desktop-document-providers-20261012` directory.
  All nine checked application/service/UI/companion files match the new package.
  This is a working-tree build, not a signed or published release.
- A fresh isolated profile launched the installed executable against the existing
  external service and passed pack/UI integrity, current composer, protocol-v3,
  policy popover, 200% zoom/scroll, Escape/focus and legacy-layout absence checks.
  No task was submitted and no computer-control consent was granted. Evidence:
  `offgrid-installed-document-providers-6nSbhV`. The temporary test app closed;
  the external service remained healthy.
- Final Windows computer/host tests, all twelve harness/docs unit tests,
  125 maintained-document link checks, strict change validation and whitespace
  checks pass. No commit, push, tag or publication was performed.

## Native document service/workflow integration (2026-10-12)

This continuation completes stage-relevant service integration and measurement
infrastructure, not the complete native-document program. Installed native
document workflows remain unavailable; their remaining work includes code as
well as platform testing. The deployment described in later historical entries
has not been replaced by this pass.

OpenSpec is **21/32 complete** after tasks 6.3, 8.3, 9.2 and 9.5. The eleven open
tasks remain 3.4, 4.1–4.4, 5.2–5.4, 8.2, 9.3 and 9.4. Review/check completion is
not native workflow completion. In particular, missing reviewed creation
semantics cannot be replaced with guessed shortcuts or title-based targeting.

- New task-first jobs use workflow-evidence v2 without upgrading existing jobs.
  Document tool discovery and execution both require an actor-owned negotiated
  `native_documents:1` host. The service, authenticated hub, existing native
  executor/supervisor and dispatch journal now connect end to end. Strict input
  validation covers duplicate/unknown/null fields, exact Unicode, bounded escaped
  payloads and unnegotiated/foreign sessions. Reads do not consume mutation budget.
- Document schemas follow actual inventory, selection and read-back state.
  Authorized MCP tools remain composed. A generic observation restores ordinary
  control tools, including the original `computer_replace_text` shape. Only one
  shape is offered at a time. Model check summaries omit redundant canonical
  digests/bindings without deleting original instructions or saved history.
- The full synthetic create–declare–edit–read-back test exposed an 8192-token
  overflow and loss of verified blankness after the intended subsequent edit.
  Both were fixed. Creation/edit milestones now survive only a continuous
  independently verified same-document edit chain. Unknown input, another tab,
  changed preimages/revisions, omitted initial verification or lost results
  cannot extend that chain. Current-text predicates still check current state.
- `TestDocumentWorkflowRunnerHostCreateEditReadback` passes all three cases:
  normal completion, partial input and lost acknowledgment. Each attempts one
  creation and one edit; uncertain outcomes stop rather than repeat input.
  The test uses an in-memory provider behind the real runner/hub/journal, not
  native applications. Existing persistence/restart/Stop and repair-budget
  regressions also pass. This is more than the old declaration-only model
  fixture, but does **not** complete the OS-backed sequences in task 8.2.
- An opt-in installed-Qwen synthetic workflow was attempted through literal
  loopback `/v1/chat/completions`. It failed before a document action because
  inference did not return within the request/test deadlines. No successful
  model run or usable latency result is claimed. The harness now stops after
  the first failed repetition, records attempted model calls even on failure,
  and leaves unavailable actual token counts null. No model was downloaded,
  no desktop input occurred, and the user's saved task was not resumed.
- Verification passed on Windows: `go test ./...`, all **73** companion tests,
  all **65** desktop unit tests, API drift, TypeScript checking and production UI
  build. The existing large-chunk build warning remains. The full CI race set
  plus `cmd/offgrid-computer` passed in WSL with fresh private ext4 temporary
  state (`/tmp` was full; no pre-existing files were removed). WSL Python 3.14.4
  passed all **20** client tests. The six harness tests and six documentation
  tests pass; the maintained-doc check covers 125 files. Strict change validation
  and `git diff --check` pass. Native opt-in cases remain unrun, not passes.
- A fresh isolated Windows service passed **21/21** requested workflow/layout/
  streaming/replay checks (`offgrid-web-qualification-7MOjaG`). A separate
  test-only desktop package was built at
  `build/document-desktop-verification-20261012/win-unpacked`; it was not installed.
  Startup/recovery, packaged browser, theme/200%-zoom and desktop access UI
  checks all pass. Their temporary evidence suffixes are `iHON0y`, `lehSEs`,
  `8NjE8q`, and `qriteX` respectively. Startup observed its first window at
  1371 ms and recovery at 1561 ms; these are one fixture run, not latency
  qualification or signing evidence. Package creation was retried from the
  correct desktop working directory after its hook rejected the root invocation.
- A separate isolated service run passed **24/24** typed-chat/voice UI checks
  (`offgrid-web-qualification-7jmsFk`): streaming, cancellation, model selection,
  synthetic microphone lifecycle, bounded speech prefetch, navigation and
  keyboard/popover behavior. No microphone recording was made. Final Windows
  `go test ./...` and WSL race reruns for agents, server, computer and the host
  command also passed after the measurement and typed-error refinements.

### Document measurement record

The owned fixture collector and harness retain this closed numeric schema.
No private text, titles, paths, target IDs, tool arguments or raw errors are
accepted. Unknown/duplicate records are rejected. An absent metric is null,
not a fabricated zero. Model repetition stops on failure; there is no automatic
model download or substitution.

| Results column | Meaning |
|---|---|
| Platform/profile, repetition, status | Harness identity and outcome; keep native, synthetic and installed-model runs separate |
| Discovery calls/ms | Application catalog refresh and identity checks |
| Readiness calls/ms | Bounded read-only application readiness |
| Observation calls/ms | Independent document inventory/snapshot reads |
| Preparation calls/ms | Preparation including fresh precondition reads |
| Input calls/ms | The attempted provider operation, excluding subsequent verification |
| Verification calls/ms | Postcondition polling and separate read-back |
| Model calls/ms | Model request time; scripted calls are not inference time |
| Actual prompt tokens | Sum of backend-reported usage, null if any usage is unavailable |
| Total calls/ms | Whole fixture run; not the sum of potentially nested stages |

The synthetic editor starts already selected, so discovery/readiness are null
there. Separate controller fixtures exercise those timers. Actual installed
profile timings, p95 claims, real-model completion percentages, soak and pilot
gates remain unfulfilled. Instrumentation does not establish those results.

### Acceptance reconciliation

| Approved scenario | Current evidence / remaining work |
|---|---|
| Packaged Notepad opens and is recognized | Identity/activation contracts exist; isolated installed-app acceptance unverified |
| Fresh blank document, exact multilingual edit, other tabs unchanged, no save | Synthetic full sequence passes; Windows creation profiles, AX/AT-SPI providers and real application sequence still required |
| Reused editor, tab/focus/revision change and lost acknowledgment | Contract, journal, runner and synthetic fault tests pass; native provider fault coverage remains unverified |
| Failed observation is not an invented mutation | Typed operation-local recovery and persistence/restart regressions pass |
| Equivalent UIA/AX/AT-SPI workflows | Windows backend is staged, not advertised; AX/AT-SPI document providers are incomplete; no macOS/X11/Wayland qualification |

No container replacement, desktop installation, commit, push or release occurred
in this continuation. The user explicitly declined native testing against
personal documents; no such tests were run. This evidence does not authorize
checking off incomplete providers or claiming all remaining work is testing.

## Native document workflows: read recovery (2026-10-11)

- Implemented a versioned native operation-effects table shared by the service,
  durable runner and desktop action budget. Classification requires the reviewed
  provider, namespace and descriptor revision; remote tool annotations and names
  cannot establish a read-only checkpoint.
- Missing target/readiness now differs from revoked authority. Native read repair
  shares the three-repair budget and unrelated catalog reads cannot reset it.
  Failed reads describe only their own effects, retaining earlier launch evidence.
  Old uncertain records are not reclassified or replayed.
- Windows Go checks passed for `internal/capabilities`, `internal/agents`,
  `internal/server`, `internal/computer`, `cmd/offgrid-computer` and
  `internal/storage`. Coverage includes cancellation, Stop, restart, failed
  checkpoint persistence, forged native descriptors and preceding launches.
  All 58 companion tests passed. These checks used synthetic/owned state and
  did not launch applications, record a user's task outcome or modify documents.
- The first WSL race attempt could not link because `/tmp` was full. A D:-backed
  retry was unsuitable for the storage restore tests and overlapped ongoing
  source changes. A clean retry with fresh temporary state on the WSL ext4
  filesystem passed `-race` for agents, server, computer, offgrid-computer,
  capabilities and storage. No pre-existing temporary data was deleted.
- Added closed executable/packaged catalog identities, OS-registered package
  activation with no arguments, registration revalidation before dispatch, and
  process AUMID/full-package matching in addition to session/generation and
  executable identity. Removed the unconditional System32 Notepad redirector.
  Application readiness now omits missing targets and carries a bounded reason.
- Windows focused Go suites passed again after these changes; all 59 companion
  tests passed, including malformed readiness and old-host compatibility.
  Read-only OS discovery validated 51 registrations including four packaged GUI
  apps; it did not activate them. Pure fixtures cover mismatched package entries,
  replacement revisions, canceled/uncertain activation and existing-process
  readiness without relaunch. Six documentation tests and the maintained-doc
  link check passed. An isolated-account real package activation test was added
  but not enabled in this personal Windows session.
- Document/tab lifecycle and exact text editing remain required implementation
  stages. No real installed application workflow or platform qualification is
  implied. No container or installed desktop was replaced.
- Added shared document inventory/binding/snapshot types and exact Unicode range
  conversion using the existing pinned Unicode-17 segmentation dependency.
  Focused tests pass for reused editors, duplicate titles, tab reordering,
  incomplete discovery, changed revisions, IME uncertainty, byte limits,
  combining text, emoji, RTL and CRLF. The creation-evidence helper refuses old
  tabs, missing earlier documents, nonblank or truncated results. These are
  contract tests, not proof that native creation/input has been implemented.
- Added closed `native_documents:1` negotiation across verified pack metadata,
  worker startup, companion enrollment and actor-owned service sessions. Legacy
  pairs retain their old capabilities; unknown versions/fields and mismatched
  acknowledgments fail closed. The existing native providers still advertise no
  document capability until their implementations are present. Focused Windows
  Go tests, 62 companion tests and 65 desktop tests passed; negotiation never
  widens a selected-window grant or enables unimplemented document tools.
- Added an opt-in owned Windows multi-tab editor fixture with duplicate titles,
  one reused RichEdit control, Unicode, selection, delayed creation and
  input/focus fault injection. Its scope validator checks the exact child PID,
  process generation/session, executable identity and parent/child HWNDs before
  each fixture command. The fixture compiles and ordinary ownership-refusal tests
  pass. The interactive fixture has not run: it requires a disposable
  `OffGridTest-*` account and explicit native/isolation opt-ins. No real Notepad
  document input, tab creation or installed application qualification is claimed.
- Final recovery review tied a missing-selection observation to the single
  pending launch in durable history. Readiness of a different application cannot
  reset that repair budget. The initial new regression fixture omitted the
  running-state setup and hit the persistence cancellation guard; after fixing
  that test setup, the targeted recovery tests and Windows `go test ./...` pass.
  The final WSL `-race` rerun also passes for agents, server, computer,
  offgrid-computer, capabilities and storage using fresh ext4 temporary state.
  All 62 companion and 65 desktop tests pass. Documentation checks and strict
  OpenSpec validation also pass. At that checkpoint, the complete CI race matrix,
  renderer/Python suites and installed/native qualification gates had not run.
- Next-stage environment gap: this checkout has Windows and WSL, but no available
  macOS test host. WSL currently lacks the AT-SPI/GTK/json-glib development
  packages, Xvfb and Openbox required by the existing native Linux runner.
  Cross-platform fixture/native-package checks and document-provider editing
  remain incomplete. Ordinary Go/JavaScript passes do not replace these checks;
  the full Notepad create–edit–read-back workflow is not yet ready for deployment.
- Added host-owned exact text preparation and immutable, revision-checked 4 KiB
  paging. Tests cover insertion versus selection/whole-value replacement,
  unchanged prefix/suffix, stale bindings/selections/capabilities, composition,
  Unicode boundaries, oversized graphemes and private-preimage serialization.
  Windows `go test ./internal/computer` passed (6.207 seconds); the maintained-doc
  check passed for 124 files. These helpers do not dispatch input or enable a
  document provider. Task 5.1 is complete; provider and native acceptance work
  remains required.
- Added owned GTK/AppKit document fixtures and isolated Unix runner tests. Pure
  ownership/range contracts pass on Windows and all 62 companion tests pass.
  The first contract run exposed an incorrect macOS driver identifier in the
  test validator; this was corrected to the actual provider identity and rerun.
  Native Linux/macOS compilation and execution remain unrun for the environment
  reasons above. Task 3.4 remains unchecked, not counted as a platform pass.
- Added explicit workflow-evidence v2 persistence/readers and independent
  document blankness, exact-text and expected-edit evaluators. V0/v1 history is
  preserved; v1 refuses v2 fields and unknown future versions remain opaque and
  non-executable. Host projections contain bound identities and content digests,
  not full private preimages. Tests reject another tab with identical text,
  replaced generations/editors, stale revisions, input echoes, uncertain input,
  incorrect action/preimage identity and contradictory newer reads.
  `go test ./internal/agents ./internal/server ./internal/storage ./internal/computer`
  passed on Windows, along with six documentation tests and the 124-file link
  check. Task 6.1 is complete. V2 is not automatically enabled on public tasks;
  document provider dispatch, pre-edit integration and native acceptance remain
  required, and no new editing capability is advertised by this change alone.
- The subsequent compatibility run passed Windows `go test ./...`, all 62
  companion and 65 desktop tests, OpenAPI drift, TypeScript checking and the
  production renderer build. The build retains a large-chunk warning. The full
  CI race package set plus `cmd/offgrid-computer` passed in WSL using private
  ext4 temporary directories. Windows' `python` launcher was unavailable; the
  existing WSL Python 3.14.4 environment passed all 20 client tests. Both native
  shell runners passed syntax checks; this is not native execution evidence.
  The remaining provider implementations and native/installed qualification
  are still incomplete, regardless of these compatibility passes.
- A fresh test-only Windows service build passed all 20 requested real-service
  browser checks (`workflow-presentation`, `agent-streaming`,
  `agent-replay-scope`, `agent-layout`; one worker, 37.2 seconds). The harness
  used disposable state at `offgrid-web-qualification-pKQ2Lz` under the Windows
  temporary directory and stopped its owned service afterward. The executable
  was built in a fresh `build/native-document-check-*` directory, leaving the
  existing installed/runtime binaries unchanged. Packaged startup/compatibility
  jobs and native application workflows remain unrun; task 9.2 is still open.
- Final review added a regression for broken canonical step ordering after an
  apparent successful read-back; the assessment now clears that stale success.
  Windows `go test ./...` and the affected WSL server race suite passed again
  (107.784 seconds for the latter). Task 9.1's command set has passed on the
  current tree. It must be rerun after subsequent provider/client changes and
  does not substitute for the unchecked installed/native gates. The checklist
  was 12/32 at that checkpoint, not release-ready; remaining implementation
  was not merely testing.
- The next implementation pass completed the document pre-edit guard and
  canonical result binding, API/SDK projections, current task presentation,
  user-facing support matrix and isolated-runner infrastructure (tasks 6.2,
  7.1–7.3 and 8.1). The checklist is now **17/32**, not release-ready. Native
  lifecycle providers, edit dispatch, full-sequence model integration,
  instrumentation and installed/native qualification remain open. The user
  confirmed there are no disposable platform test machines available and asked
  that unavailable native tests remain explicitly unverified; that does not
  convert unfinished provider code into a completed task.
- Guard regressions cover all four creation/edit modes, missing/foreign/stale
  bindings, malformed/ambiguous arguments, canonical ordering, input echoes,
  three-repair exhaustion across reads and restarts, and failed persistence
  before/after dispatch. A newly identified assessment bug discarded the
  recorded mutation when the normal post-input document inventory was read;
  the evaluator now preserves that action only for the same document/revision.
  A changed active document invalidates the evidence. Independent read-back is
  still mandatory; successful input is not success of the overall task.
- Windows checks passed for `internal/agents`, `internal/server`,
  `internal/computer`, `cmd/offgrid`, `internal/serviceclient` and
  `cmd/offgrid-computer`. TypeScript, OpenAPI generation/drift and production UI
  build passed (the existing large-chunk warning remains). The fresh isolated
  service browser run passed **47/47** checks in 39.4 seconds, including all
  nine locales, narrow/RTL layouts, v0/v1/v2/unknown evidence presentation,
  explicit same-job read recovery and navigation/reload without replay.
  Disposable browser evidence is under `offgrid-web-qualification-RhdTYM` in
  the Windows temporary directory, not the user's running workspace.
- WSL race checks passed for `internal/agents` (23.708s), `internal/server`
  (51.033s), `internal/computer` (5.274s), and `cmd/offgrid-computer` (cached).
  The first attempt failed because the `/tmp` tmpfs was full and the intended
  temporary-directory assignment was not preserved through the shell. The
  rerun passed with explicit `TMPDIR`/`GOTMPDIR` arguments pointing to a private
  `/var/tmp/offgrid-document-race.*` directory. No user files were removed.
  WSL Python 3.14.4 passed all **20** client tests in 0.36 seconds.
- The new document-runner's five pure tests and six documentation-check tests
  passed; all 124 maintained Markdown files passed link/metadata checking.
  A direct runner invocation in the personal account refused with
  `isolated_account_required` before compilation or native input. Missing
  installed/model entry points produce `required_native_test_missing`; skipped
  native tests cannot become passes. The runner creates fresh service/task
  state, verifies that service's workspace identity, preserves redacted test
  metadata, and never saves/reconciles the user's current task or downloads a
  model. This is infrastructure evidence, not a completed native workflow.
- Bounded Windows TextPattern read/selection helpers and an owned-fixture test
  are present, including exact UTF-16 decoding and re-acquiring the document
  extent to detect concurrent appends/selection changes. These helpers do not
  yet enable document editing or a qualified provider. The document inventory
  model projection now bounds rows and prioritizes the active tab without
  changing saved evidence; task 6.3 remains open for full schema/token/model
  continuation coverage. No native input, container replacement, desktop
  installation, commit, push or release occurred in this pass.
- After the inventory projection was added, Windows `go test ./...` passed
  again, along with the complete companion/desktop suites and all 11 combined
  harness/documentation tests. The maintained-doc check still passes for 124
  files, and strict OpenSpec validation passes. These are automated compatibility
  results, not native document creation/editing or installed-package acceptance.

### Native document host/transport integration (2026-10-12)

The change remains **17/32 complete**. This continuation adds implementation
inside the open provider/transport tasks; it does not mark those tasks complete
or describe their remaining work as testing only.

- The shared executor now sits behind the existing native supervisor/journal in
  both selected-window and task-desktop workers. It consumes preparations before
  dispatch, performs fresh precondition checks, and verifies independent provider
  read-back. Selection, insertion, selected-range replacement, whole-value
  replacement, and creation remain distinct operations. Partial dispatch and
  lost acknowledgments never trigger fallback input or another creation.
- The companion transports share one document adapter and preserve their
  existing grants, input lease and Stop lifecycle. Bounded read pages retain
  snapshot/revision identity. New preparations re-read unchanged state after
  model latency rather than failing merely because the discovery observation
  aged; they cannot extend old approvals or accept changed content/selection.
  Invalid or foreign document evidence is rejected before journal completion and
  again on recovery. A generic successful receipt cannot certify a text edit.
- A staged Windows backend now walks a bounded UIA tree, verifies numeric
  TabItem/SelectionItem relationships and child-pane/ControllerFor links, and
  binds the selected tab separately from its editor. It supports internal reads,
  tab selection and exact text/selection dispatch. Unicode insertion is bounded
  by grapheme-safe batches with focus, identity, composition and read-back checks;
  it does not use the clipboard or a second input method. Protected descendants
  cannot be read through an outer document TextPattern. Reserved UIA property
  values are interface-checked before any array vtable call.
- Regressions cover duplicate captions, reordered tabs, reused editors, ambiguous
  pane links, missing/partial trees, protected controls, stale/foreign snapshots,
  model/approval latency, invalid result provenance, lost acknowledgments and
  Stop. Windows COM contract tests use fake objects, not personal applications.
  The current desktop payload now has a shared source-file inventory and an
  import-coverage test so a new companion helper cannot be omitted by the package
  copy loop. Legacy manifests retain their own compatible payload requirements.
- Windows `go test ./...` passed. WSL race checks passed for `internal/computer`
  (10.900s), `cmd/offgrid-computer` (1.163s), `internal/agents` (cached), and
  `internal/server` (108.369s). The complete companion suite, including the
  package-import regression, subsequently passed **73/73**. Desktop tests passed
  **65/65**, documentation tests **6/6**,
  and strict OpenSpec validation passed. Maintained documentation checking covered
  125 Markdown files without link/metadata problems. These are automated
  compatibility results, not real native-input, model-accuracy or latency claims.

Still unfinished: reviewed creation/browser profiles, coordinated public
service/tool activation, complete model-stage schemas and full-sequence model
tests, AX/AT-SPI document providers, metrics and installed/native qualification.
The Windows backend intentionally does **not** advertise `native_documents` yet.
No installed editor/version has been qualified. The user has no disposable
platform test environments available, so actual native tests remain **unverified**.
UI/API/SDK checks from the preceding evidence entry were not rerun for this
host-only continuation. No personal native input, task replay/reconciliation,
container replacement or desktop installation occurred. The earlier focused
Unicode/JSON commit `2e8a8e4` does not include this uncommitted continuation; nothing
was pushed or released.

### Coordinated checkpoint verification (2026-10-12)

The operator requested a local commit of the accumulated changes and a matching
test deployment. This checkpoint is **not** completion of the native-document
program: its task count remains 17/32, public document tool activation and the
remaining providers are unfinished, and native application qualification remains
unverified. The document feature gate stays closed. No uncertain task was replayed
or reconciled and no personal native-input test was run.

Checks rerun on the coordinated working tree:

- Windows `go test ./...` passed; WSL race checks passed for the complete CI list
  covering sessions, RAG, server, users, cache, agents, runs, storage, serviceclient,
  models, audio, computer and inference. Some package results used Go's cache.
- API generation drift, TypeScript checking and production UI build passed.
  The UI build still warns about a bundle larger than 500 kB.
- The disposable real-service browser wrapper ran 324 tests: **314 passed,
  10 skipped**. It did not target the installed workspace.
- Companion contracts passed **73/73**, desktop unit tests **65/65**, isolated
  native-harness guards **5/5**, documentation tests **6/6**, maintained Markdown
  checks **125 files / 0 problems**, and strict OpenSpec validation **12/12**.
- Python client tests passed **20/20**. An initial plain-pytest invocation also
  collected the manual live-service script and produced ten missing-fixture
  errors. That script is now explicitly excluded from pytest collection without
  changing its opt-in CLI behavior; plain `python3 -m pytest -q` then passed all
  20 isolated tests. No live Python test contacted the user's service.

These checks support a development checkpoint, not release, real-model document
accuracy, native app compatibility or performance claims. Container replacement
and installed-desktop verification are recorded separately only when completed.

### Coordinated local test deployment (2026-10-12)

The authorized replacement deployed implementation revision `dfcea08` as
`offgrid-llm:document-checkpoint-20261012` and installed the matching unsigned
Windows desktop 0.4.16 development package. This is not a new published version.
The document feature remains gated and the change remains **17/32**; remaining
work includes implementation as well as native/platform qualification.

- The desktop exited normally without force termination. Its previous application,
  local state and Electron profile are preserved in the access-restricted backup
  `D:\offgrid-llm-backups\desktop-documents-20261012`.
- The service was stopped for a consistent workspace backup. The archive passed
  verification and restoration to isolated state before activation. Backup and
  comparison evidence is retained at
  `/home/phil/offgrid-document-checkpoint-backup-20261012-KIwnadME`.
  Every current model file matched the independently verified snapshot at
  `/home/phil/offgrid-consent-edge-backup-20261010-Q4mOPmBn`; model files were not
  replaced or downloaded.
- Validation used the restored workspace and a read-only model volume while the
  original service was stopped. A short Qwen text response passed. Before/after
  checks preserved workspace identity, conversation names, task IDs/statuses,
  installed model IDs and ASR/TTS readiness. No uncertain task was resumed,
  reconciled or submitted again.
- Only `offgrid` runs, on loopback port 11611. The previous container remains
  stopped as `offgrid-before-document-checkpoint-20261012-1791742632`.
  Temporary validation containers were removed without deleting data volumes.
- Packaged startup/recovery, saved/live appearance, 200% zoom, access-panel and
  managed-browser workflow tests passed. The browser test was corrected to use
  the current saved-task access flow instead of the removed legacy page. It
  verified form text, selection, checkbox state, local Stop and a second
  connection using only its owned local fixture. This is not native-app or
  model-driven workflow qualification.
- Installed executable, app bundle, backend, renderer and computer worker/helper
  hashes match the staged package. Installed read-only Electron verification
  passed, using an isolated profile and blocking mutating service requests.
  Installed and service UI identities both equal
  `d586cd9823a7b87537c2ebea1a5cd944bbe067f430b5f3fbe90fd00da33d8795`.
  Evidence directories under the Windows temporary directory are
  `offgrid-desktop-startup-fNv6ZI`, `offgrid-desktop-theme-eFiFYj`,
  `offgrid-desktop-access-YT7qNj`, `offgrid-packaged-browser-U08WdL` and
  `offgrid-installed-document-checkpoint-UgJqsD`.

Source and verification changes were committed locally. No push, tag, release,
personal native input, microphone activation or new desktop access grant occurred.
The deployment does not complete the new-document creation/editing workflow.

## Qualification-gate continuation (2026-10-21)

The isolated native-workflow harness now refuses an installed or real-model
suite that merely exits successfully. The owned test must emit one bounded,
redacted outcome record covering distinct blank-document creation, exact Unicode
read-back, unchanged other documents, no save, verified browser blank tab and
single dispatch; each suite must cover at least two application families.
Harness unit tests passed **7/7** on Windows. The opt-in macOS fixture test now
calls the actual AX document reader, as the Linux fixture already calls AT-SPI;
its private fixture oracle alone cannot qualify the provider.

This is a gate and fixture-test improvement, not a native workflow pass.
The installed/model Go entry points do not yet exist, so the harness still
reports `required_native_test_missing` before input. No disposable macOS,
Linux or Windows native-app environment was provided for this continuation;
those OS-backed checks and all native/model completion claims remain
**unverified**. No installed application, desktop grant or saved task was
controlled, resumed or reconciled.
