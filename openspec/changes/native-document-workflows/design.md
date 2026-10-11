# Design

## Context

See proposal.md for the approved behavior and acceptance cases. Architecture and
release authority remain in `docs/advanced/ARCHITECTURE.md`,
`docs/advanced/production-readiness.md` and the existing reliability plan.

The investigation and source inspection establish these integration points:

- `native_applications_windows.go` discovers executable registrations and includes
  the system Notepad redirector. `native_desktop.go:applicationFor` compares
  executable file identity, which cannot recognize its packaged destination.
  `VerifyApplication` currently returns `verified:false` with zero-value target
  data after its deadline. `Binding` conflates missing selection and bad scope.
- `tool_failure.go:readOnlyBuiltin` recognizes saved-control discovery, but not
  native observation/readiness. `BrowserHub` already has a separate native-read
  list for budget accounting. Their inconsistent classifications produce the
  misleading uncertain state after a read-only scope error.
- `NativeElement` has flat control metadata and evidence identity, but no active
  document, selected tab, range or document revision. The same editor may serve
  several tabs. `NativeWindows` reads/sets ValuePattern only; its activation path
  uses InvokePattern, not SelectionItemPattern. AX and AT-SPI already have separate
  read/set-text paths but no portable document/range contract.
- The service tools expose whole-value replacement and fixed shortcuts. The
  three layers of validators in Go, companion JavaScript and the worker must
  agree; adding a tool description alone cannot implement an operation.
- Workflow v1 pins control identity across observations. Its checks do not bind
  document identity or distinguish new blank documents from existing contents.
  The installed-model fixture intentionally stops before actual input: it cannot
  prove the real Notepad workflow. Native CI uses owned fixtures, not all installed
  application versions.

## Goals / Non-Goals

Goals: implement the approved document workflow through the existing runner and
native supervisor, with useful typed operations, bounded model work, defensible
target matching and testable provider behavior. A host adapter remains an owned
process; the service remains the durable task authority.

Non-goals: unrestricted input, trusting every installed application, executing
downloaded adapters, matching by natural-language labels, expanding a selected
window grant, changing the current UI shell, or qualifying platforms through
compilation. Proposal exclusions also apply.

## Decisions

### 1. Model an installed application as an activation identity

Replace the executable-only assumption inside the host catalog with a closed
identity variant: ordinary executable or registered packaged application. Both
variants retain an opaque task-local catalog ID, display metadata, registration
revision and provider-specific validation data. Neither raw identity nor paths
become model-controlled launch arguments.

For Windows, enumerate supported entries from the OS applications catalog and
package registrations, supplementing the existing App Paths/Start Menu path.
Packaged entries bind the application user model ID (AUMID), package full
identity/revision and registered application entry. Prefer that registration
where it provides a verified identity for a packaged app. Collapse duplicate
entries only when OS identity proves equivalence; a similar display name is not
enough. An unproven redirector remains unavailable for reliable launch, not an
implicit mapping to any process it happens to start.

Dispatch registered packaged activation using the OS activation interface, with
no model-supplied parameters. Revalidate the registration before dispatch. Match
the resulting surface using the process application/package identities, process
generation, OS session and window ownership. A package can contain multiple app
entries: package family alone is insufficient. Keep exact-file validation for
ordinary executables. An activation-returned PID is evidence to inspect, not a
substitute for identity checks; existing-process reuse remains valid.

Microsoft provides [AUMID-based activation](https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nf-shobjidl_core-iapplicationactivationmanager-activateapplication)
and process queries for [application identity](https://learn.microsoft.com/en-us/windows/win32/api/appmodel/nf-appmodel-getapplicationusermodelid)
and [package identity](https://learn.microsoft.com/en-us/windows/win32/api/appmodel/nf-appmodel-getpackagefullname).
These are inputs to OffGrid's validation, not evidence that a requested task
succeeded. Shared/brokered surfaces whose ownership cannot be proven stay
unsupported. Do not use Explorer command strings, executable-name allowlists,
parent-process heuristics or a hardcoded Notepad package path as fallback.

Readiness returns a stable reason and bounded retry hint alongside existing
compatibility fields. New clients do not receive zero targets as usable state.
Retain the current ten-second maximum readiness wait inside the worker request
deadline. Waiting polls/observes only; no launch, focus or typing is hidden in a
readiness retry. Registration updates invalidate preparations, not saved history.

### 2. Separate effect classification, target availability and authority

Introduce one reviewed native operation-effects table used by agent checkpoint
classification, service dispatch and host action-budget accounting. It explicitly
lists supported native reads and verifies their provider/namespace/kind/version.
It does not trust low risk, MCP annotations or a matching tool name. Legacy
browser operations are not reclassified incidentally by this change.

Persist the existing read-only intent marker before a trusted read. A timeout or
provider failure during that read cannot make it an uncertain document mutation.
Preserve a stable code, sanitized message and operation-local `effects:none`;
do not say the entire task made no changes after a successful earlier launch.

Add distinct host errors for `target_not_selected`, `application_not_ready` and
`document_changed`. Missing selection is recoverable only after validating the
task authority. A foreign process/session, lost permission, lock or revoked grant
still fails closed. The hub must not call its authority-revocation path solely
because no window was selected yet. Transport corruption or worker death can
still revoke access while being classified as a failed read at task level.

Feed proven no-input recoverable failures into the existing three-repair budget.
Unrelated catalog reads and unchanged readiness do not reset it. End in a clear
interrupted state after exhaustion. Explicit resume remains necessary following
restart or exhausted recovery; navigation never resumes work. Keep uncertain
mutation paths unchanged, including failpoints after dispatch and before saving
the result. Never downgrade an old uncertain record from a name-only inference.

### 3. Negotiate document support independently from consent scope

Add a versioned `native_documents:1` feature contract carried through verified
pack metadata, worker startup, companion enrollment and service readiness. Its
operation set is validated against the packaged adapter and current surface.
Advertise it only after both endpoints agree; old service/host pairs continue
their existing protocol without receiving unknown operation fields. Keep the
private transport framing, ownership, authentication and one-input lease.

Feature support does not broaden v2 selected-window scope or convert macOS/Linux
hosts into v3 desktop launchers. A newly created window outside a window grant
cannot be adopted; the operation is unavailable unless its provider can keep the
new document in scope. V3 desktop tasks can adopt a verified new surface in the
same authorized application and OS session. No new access popup is added for
operations already within a grant.

The initial typed tool set is:

| Tool | Effect and binding |
| --- | --- |
| `computer_documents` | Read current document/tab inventory and supported commands; bounded references, no activation |
| `computer_create_document` | Invoke one observed, supported blank-document/tab command; journal one mutation |
| `computer_select_document` | Select an exact observed document/tab and verify active identity; journal mutation |
| `computer_read_text` | Read a bounded complete snapshot or pages of it, with revision and completeness |
| `computer_select_text` | Select a validated range within one revision; journal selection change |
| `computer_insert_text` | Insert at an unchanged collapsed selection in the bound document |
| `computer_replace_selection` | Replace an unchanged selected range, preserving the remainder |

Retain `computer_replace_text` for explicitly requested whole-field replacement,
but apply document preconditions when the negotiated document contract is active.
Do not reinterpret existing `tab` keyboard navigation as new-tab creation.
Expose only current-stage relevant schemas, with ordinary MCP capabilities still
composed independently. There is no monolithic model-defined action sequence.

### 4. Bind documents, not just controls

Add provider-issued document references distinct from input element IDs and
existing evidence identities. A binding includes the complete target, active
tab/document identity, generation, editor identity, observation and content
revision. UIA runtime identity, AX retained object identity and AT-SPI object
identity can anchor controls; none alone proves document continuity when an
application reuses one editor. Include the verified active tab/document container
and invalidate on a changed selection, removed container or unknown transition.

Represent roles, selected state, selection-container relationships and supported
actions explicitly rather than parsing localized role names. A provider reports
unknown/incomplete document identity when it cannot prove these relationships.
Duplicate titles, tab reordering and identical text never merge identities.
Only the active document's editor text is collected for ordinary operation;
discovery does not switch through inactive private documents.

Creation preparation records the observed command, pre-creation document set and
active binding. After its single dispatch, a bounded read-only wait must establish
a distinct selected document and complete empty editor. A restored tab, empty
preview, template, picker or truncated text is not that evidence. If the app reuses
a document without distinguishable generation, creation is unavailable rather
than certified by a caption such as Untitled.

Use structured selection/actions where supported. Reviewed provider profiles may
map verified application identity and structured command metadata to typed
creation semantics; they are shipped code, not downloaded scripts or model
macros. Such a profile does not grant authority, cannot override blankness checks,
and must report its supported application versions. Require a generic controlled
fixture and more than one real application family in the acceptance matrix to
avoid a Notepad-only special case. Unknown commands are not silently treated as
new-document commands because their labels contain a word in English.

### 5. Prepare exact text semantics and dispatch once

Text preparation captures the full bounded preimage, document revision, editor,
selection/caret, requested mode and text. The host derives the expected result
from this data. Revalidate all inputs immediately before dispatch; stale user
typing, selection changes, focus loss, a replaced document or a protected control
must stop the edit. New operations cannot fall through to whole-value replacement.

Model-visible ranges use Unicode scalar offsets over the exact read snapshot,
with their snapshot/revision IDs; never bytes or arbitrary live offsets. Providers
convert to their native representation and independently verify the resolved
range text. Reject boundaries that split a grapheme, surrogate, CRLF pair or an
unmappable provider text unit. Define document line endings in the snapshot and
never silently trim or Unicode-normalize text to make verification pass. Input
with active or indeterminate composition is rejected until a stable state can be
established; do not cancel or commit the user's IME session.

Keep input at most 16,000 UTF-8 bytes, as with current typed replacement. Initially
limit complete editable snapshots to 64 KiB UTF-8, with 4 KiB read pages and
explicit oversized/unavailable results. These are supported-workflow limits,
not silent truncation. Model observations remain within their current bounds.
Exact evidence can use the complete host snapshot and service-authored digest
without copying the whole document into every model turn. No model-supplied
digest, truncated preview or input echo can establish read-back success.

Provider implementations:

- Windows: keep ValuePattern for supported whole-value writes; add TextPattern
  reads/selection and SelectionItemPattern tab selection. TextPattern is not an
  insertion API. Use one reviewed, focus-bound Unicode input path for editable
  text controls that require it, with protected-role/integrity checks and bounded
  cancellation between input batches. Check current foreground and focus again
  after selection and input. Partial dispatch remains uncertain; no second input
  method is attempted after failure. No clipboard fallback. Microsoft documents
  [text-pattern limitations](https://learn.microsoft.com/en-us/dotnet/framework/ui-automation/ui-automation-textpattern-overview)
  and [selection-item semantics](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-implementingselectionitem).
- macOS: implement capability-checked AX selected-text/range and value operations
  with retained document/control identities and read-back. Unsupported settable
  attributes are not emulated using scripts or broad key macros. The OS exposes
  [selected text range](https://developer.apple.com/documentation/applicationservices/kaxselectedtextrangeattribute);
  validate each provider's real range mapping on the target OS.
- Linux: use AT-SPI Text, Selection and EditableText for supported controls.
  [EditableText](https://gnome.pages.gitlab.gnome.org/at-spi2-core/libatspi/iface.EditableText.html)
  distinguishes insertion from entire-content replacement. If replacement needs
  multiple native calls, journal it as a multi-stage effect: failure after a
  deletion is partial/uncertain and must not retry or restore content blindly.
  X11-only keyboard support stays X11-only; no Wayland bypass is introduced.

The host cannot make arbitrary applications atomic or perfectly classify their
effects. Restricted contexts, unreliable target/range identity and protected
controls remain unavailable/manual-takeover cases. This design is not a claim of
a universal security sandbox.

### 6. Extend evidence without rewriting existing task history

Use workflow evidence version 2 for new document-aware tasks, retaining explicit
v0/v1 readers and their original behavior. Version 2 adds document generation,
pre/post content revisions and operation binding to canonical evidence. Add
`document_blank`, `document_text_equals` and `document_edit_matches` predicates.
The last verifies the expected edit against its preimage and unchanged surrounding
content; it avoids making the model copy a long document merely to declare a
selection edit. It is distinct from semantic proof of the user's entire outcome.

Keep the pre-edit expectation guard. Declarations identify observed references and
the intended text/mode; service and host bind canonical identities, selections
and expected results. The model cannot change an unresolved check, auto-fill a
new document identity or mark its own result verified. Application-open evidence
does not satisfy document blankness or writing. Creation must record a typed
creation expectation before dispatch and bind its resulting document afterwards,
using the same recorded action identity, not a guessed future tab ID.

No separate conversation/task store is introduced. Save result and verification
through the existing task transaction before publishing completion. If persistence
fails after native dispatch, recover the journal result without repeating input.
Unknown versions remain opaque and refuse execution. Old uncertain tasks remain
unchanged; this change does not reconcile the user's current failed run.

### 7. Keep useful recovery in the current interface

Reuse `TaskResult`, `TaskEvidence` and current progress/event components. Display
the failed stage (application readiness, document selection, observation, input
or read-back), stable reason and permitted next action. Only genuine uncertain
mutations use the reconciliation form. For a recoverable interrupted read, Resume
continues the canonical task after fresh authorization; it does not start another.
Missing capability shows which component/profile is unavailable.

Update generated API contracts, nine locales, CLI JSON and Python projections.
No duplicated controls, legacy page, new picker-first workflow or consent dialog.
Maintain keyboard/RTL/narrow-screen behavior and outside-click/Escape semantics
where existing recovery details use a popover.

### 8. Verification and latency are part of the feature

Layer tests so that each failure has an identifiable owner:

1. Pure contracts: identity variants, document generations, Unicode ranges,
   preconditions, read classification, unsupported features and serialization.
2. Durable service/companion integration: every persistence/dispatch boundary,
   worker crash, lost acknowledgment, Stop, bounded repair and unchanged MCP
   composition. These fixtures can assert zero unintended input.
3. Owned native editors: multiple tabs sharing one editor, renamed/reordered tabs,
   unrelated unsaved content, partial text writes, delayed readiness and IME/focus
   changes. Provider tests must use actual native APIs, not just shared JSON.
4. Isolated installed applications: current packaged Notepad plus another editor;
   browser blank-tab creation/selection with a disposable profile, not personal
   accounts. Use a dedicated Windows account/VM because modern Notepad can restore
   personal tabs even when launched for a test. Record app/package versions.
5. Installed-model continuation: run the full open/create/edit/read-back sequence
   against owned fixtures, not only expectation declaration. Repeat the exact
   multilingual workflow three times per claimed profile, including the installed
   Qwen 7B/8192-token profile and available alternative models. No automatic model
   downloads or per-model task shortcuts.

New fixture targets must prove non-target documents unchanged from known baseline
values. Real tests must not read personal tabs to manufacture that proof. All
native-input suites need explicit isolated-environment opt-in and fail closed if
they cannot verify their fixture scope. Permission tests and missing macOS/Wayland
hardware are reported as unavailable, not passed or silently skipped qualification.

Measure catalog/readiness, observation, prepare, input, read-back, model time and
total duration separately. Host waits are bounded and do not consume a model call
per poll. Record p50/p95 for repeatable fixtures, model call counts and actual prompt
tokens. Retain 8192-token regression tests with all active tool schemas, without
raising allocation. UI Start/Pause/Stop feedback keeps the program's 250 ms target;
local stop-dispatch keeps its one-second target with honest uncertainty when late.
Do not advertise an overall speedup or production readiness from mocks.

## Risks / Trade-offs

- Packaged and brokered processes can share hosts -> require exact registered
  application ownership; unsupported association is explicit, never title matching.
- Editors reuse controls and underreport selection -> combine document generation
  and revision; reject incomplete evidence instead of broadening the target.
- Accessibility providers use different text units -> conversion/read-back tests
  across scripts, emoji, line endings and real OS controls; no guessed offsets.
- Input can partially dispatch -> one durable intent and monotonic stage record;
  stop remaining batches and reconcile, never switch method and retry.
- A richer schema burdens small models -> stage-gated flat tools, task-local
  references and bounded projections; measure complete prompts and full sequences.
- Real app versions may not expose reliable document semantics -> reviewed profile
  ranges and negative capability reporting; do not silently substitute a toy editor
  for the required modern Notepad acceptance test.
- Additional checks can increase latency -> deterministic host readiness/read-back
  rather than speculative model loops; record overhead separately from inference.
- A platform runner is missing -> implementation tests and unavailable qualification
  remain separate; corresponding real-provider tasks stay incomplete.

## Migration Plan

1. Ship compatible readers and native-read recovery without rewriting existing
   uncertain tasks. Add version/feature negotiation before enabling new operations.
2. Package worker, companion transport and desktop together. A new service with
   an old host shows document functions unavailable; it cannot emulate them by
   removing target checks. New host/old service retains its legacy feature set.
3. Activate workflow v2 only for new supported tasks. Preserve old records, aliases,
   consent expiry, model installations and current UI. No permission expansion on
   upgrade, and no automatic dispatch during migration/reconnect.
4. Run the isolated and installed qualification matrix and record evidence before
   distribution claims. Later authorized deployment must replace, not duplicate,
   the service and update the matching desktop host after backing up data.
5. Roll back only after stopping active control, retaining task/journal records and
   verifying schema compatibility. Never restore a host journal or grants from an
   older snapshot to resume native actions. Unsupported downgrade blocks execution
   rather than discarding document bindings.

These migration steps describe delivery requirements, not authorization to deploy,
reinstall the user's app, commit or publish during this change's planning.
