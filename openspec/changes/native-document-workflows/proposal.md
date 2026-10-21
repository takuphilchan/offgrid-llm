# Proposal

## Why

The installed Notepad test launched the application but failed to recognize its
packaged process, then mislabeled a failed observation as an uncertain mutation.
Beyond that defect, opening an application does not provide a safe new-document
workflow: OffGrid needs document/tab identity and verified Unicode editing before
it can reliably write without changing existing work.

## What Changes

- Recognize OS-registered packaged Windows applications through validated package
  and application identities, alongside existing executable identities. Match
  activation to the resulting application surface without title/name guessing,
  arbitrary launch strings, or a Notepad-only redirection exception.
- Return useful bounded readiness states and distinguish no selected document
  from violated desktop authority. Failed trusted native reads must not demand
  invented mutation outcomes; genuinely uncertain mutations remain unreplayable.
- Add a negotiated native document contract: discover/select tabs or documents,
  create a fresh blank document through a supported observed command, and bind
  its editor and active-document identity before writing. Reusing a window or
  editor control must not silently select a different document.
- Separate insertion, selection replacement and whole-field replacement. Add
  bounded full-text read-back, Unicode-safe range semantics, preconditions and
  exactly-once dispatch through the existing host supervisor and task runner.
- Extend existing workflow evidence to bind edits to the document, content
  revision and expected result. A blank-looking truncated view is not proof of
  emptiness; a successful input dispatch is not proof of correct text.
- Implement shared contracts with Windows UIA, macOS Accessibility and Linux
  AT-SPI adapters for the controls each can actually support. Keep existing
  selected-window scope on platforms without task-wide launching; expose absent
  functions as unavailable, not as successful no-ops or an OS-name capability flag.
- Keep recovery and evidence in the current task UI and existing APIs. Do not
  add another access dialog, legacy page, task store or model-specific workflow.
- For newly admitted tasks, derive mechanical outcome checks from validated typed
  action proposals and persist them before dispatch, instead of requiring another
  model turn to duplicate the same action arguments. Preserve independent checks
  and legacy jobs. Detect successful-but-unchanged observation loops explicitly.

### User-visible acceptance

1. On a qualified Windows installation, the current packaged Notepad is found,
   opened and recognized even when an executable registration is a redirector.
2. With existing disposable unsaved documents/tabs present, the requested new
   blank document receives exactly `OffGrid test — café 日本語 12345`. Independent
   read-back matches; existing documents remain unchanged and nothing is saved.
3. Selecting another tab, closing/replacing the document, focus theft or concurrent
   typing between preparation and dispatch prevents stale edits. Lost input
   acknowledgments never cause another creation or text insertion.
4. A failed observation reports an observation/readiness problem and a safe next
   step, not a request to confirm a nonexistent text mutation. It does not erase
   the earlier launch or fabricate successful readiness.
5. Equivalent controlled editors exercise the same document and Unicode contracts
   on UIA, Accessibility and AT-SPI. Real platform/app evidence is recorded
   separately from fixtures and unavailable platform tests remain explicit.
6. A typed launch/create/edit proposal can progress without a separate metadata
   call, but cannot bypass observed identities, authorization or read-back.
   Repeated unchanged reads receive bounded correction, not unbounded rediscovery.
   A document-capable host with an unsupported surface can still expose its
   existing structured-control operations after fresh observation.

### Boundary

This continues the structured-workflow stage identified by
`desktop-workflow-foundation`; it does not reopen its evidence/discovery scope or
expand `voice-model-foundation`. Packaged Windows activation, document/tab
creation and editing are the new behavior. Browser tab selection/new blank tabs
use the same supported native-surface rules; this is not personal-profile CDP
attachment, network policy, browser research qualification or credential access.

Excluded: general file APIs, saving/exporting documents, spreadsheet semantics,
closing or discarding existing documents, arbitrary shortcuts/scripts, general
visual automation, new consent scope, macOS/Linux task-wide launch infrastructure,
and claims that every application is controllable. Existing unsupported hosts
remain usable for their prior capabilities. The complete cross-platform
computer-use program remains open.

Native tests must use an explicitly isolated desktop/account and disposable
documents. No personal-app control, task reconciliation, container replacement,
installed-app update, model download, commit, push or release is authorized by
these planning artifacts.

## Capabilities

### New Capabilities

- `native-application-identity`: OS-backed application identity, activation and
  readiness for executable and packaged applications.
- `native-document-editing`: Negotiated document/tab lifecycle, target-bound
  Unicode edits and independent read-back without overwriting existing work.
- `native-read-recovery`: Trusted read-only failure classification and bounded
  recovery in the existing durable task without replay or invented outcomes.

### Modified Capabilities

None in the maintained spec inventory, which is currently empty. The in-flight
`task-desktop-authority`, `task-outcome-evidence` and `native-control-discovery`
requirements remain controlling; these additions build on them.

## Impact

- `internal/computer`: application catalog/provider identity, native adapters,
  operation validation, observations, evidence, host journal and protocol gating.
- `cmd/offgrid-computer`, `computer`: private worker protocol, negotiated feature
  availability, document prepare/dispatch/read-back and packaged-host tests.
- `internal/agents`, `internal/server`: effect classification, structured tools,
  bounded model context, staged expectations and document-aware verification.
- Existing API projections, generated TypeScript, current task recovery UI and
  nine locales; CLI/Python compatibility and additive result types as needed.
- OS-backed native fixtures, service/companion/browser tests and installed-model
  continuation tests. No additional hosted inference or executable downloads.
- Existing API, driver, desktop and computer-task documentation. Actual results,
  latency breakdowns and qualification gaps belong in
  `docs/advanced/product-reliability-plan.md`; release gates remain in
  `docs/advanced/production-readiness.md`.
