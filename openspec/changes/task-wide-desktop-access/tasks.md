# Tasks

## 1. Host authority and application lifecycle

- [x] 1.1 Implement explicit v3 task authority and separate execution targets in the Go supervisor/worker, preserving v2 limits and stop/journal behavior; pass `go test ./internal/computer ./cmd/offgrid-computer` with scope, expiry, budget, stale-target and replay tests; document the protocol in computer/README.md.
- [x] 1.2 Implement reviewed host-issued application discovery, journaled launch/focus and independently verified surface readiness, without model paths/commands; pass catalog, executable replacement, process-reuse, launch timeout/cancellation and owned-native-fixture tests; document supported profiles and limitations.

## 2. Service and companion integration

- [x] 2.1 Add v3 pairing, immutable task/actor/workspace session binding and composable desktop tools with application-open completion evidence; pass `go test ./internal/server ./internal/agents ./internal/computer` including Chrome-without-URL and no MCP disappearance; update API documentation/contracts.
- [x] 2.2 Add atomic held desktop task admission and idempotent grant attachment before inference; pass crash/retry/cross-actor/policy-mismatch/v2-downgrade tests, with existing omitted-scope request hashes unchanged.
- [x] 2.3 Connect the trusted Electron/private companion flow to v3 local consent and dynamic targets; pass `npm test --prefix desktop` and `npm test --prefix computer`, including stop-during-consent, disconnect, target switching and protocol-skew tests.

## 3. Current task UI and end-to-end checks

- [x] 3.1 Integrate desktop scope and initial consent into the current task composer and recovery view, without mandatory app selection or navigation-triggered consent; preserve optional limited-window flow, nine locales and popover accessibility; pass typecheck, API drift and focused browser regressions.
- [x] 3.2 Run integrated service/companion tests, applicable race checks, built current UI and packaged host smoke checks using isolated state; record real results and unrun platform/model qualification in docs/advanced/product-reliability-plan.md, and update docs/guides/computer-tasks.md with the actual shipped flow. No container replacement, installer replacement, commit or release is implied by test completion.

## 4. Approved session-access UX repair (2026-10-10)

- [x] 4.1 Keep explicit Full access authorization in the live native host across separately bound tasks, never in stored renderer preferences; test consent reuse, actor/workspace mismatch, task release, revocation and restart with Go and companion suites.
- [x] 4.2 Add explicit enable/revoke and accurate permission status to the current access control; revalidate actor/workspace in trusted Electron and revoke on quit/lock/suspend/account change; pass desktop and UI regressions without automatic approval from defaults.
- [x] 4.3 Fix real popover scrolling and remove the duplicate Windows Stop popup for desktop-owned workers, retaining persistent in-app/offline Stop and the independent emergency/OS-session monitor; test wheel/keyboard, short/zoomed/RTL layout and owned native Stop fixtures. Run applicable type/API/build/race checks and update existing documentation with measured results and deployment status.

## 5. Reported Chrome continuation and consent regression (2026-10-10)

- [x] 5.1 Reproduce the 8192-token launch/observe continuation failure with the actual composed tool schemas; fix bounded context archival without dropping current observations, user instructions or durable evidence, and never claim no tool ran after a recorded dispatch. Run agent/server regression and race tests.
- [x] 5.2 Reproduce Windows Chromium accessibility startup using an owned browser/local search fixture; fix observation readiness without selecting personal profiles, weakening target/foreground validation or adding arbitrary launch arguments. Verify a typed search and independent result in the fixture; record actual native coverage and remaining limitations.
- [x] 5.3 Combine Full access selection and explicit host authorization into one current-UI choice, preserving no consent from defaults, denial/revocation and all locales. Run UI/type/API/build, desktop and companion checks, and update the existing reliability/API/computer documentation. Installation, publication and personal-browser actions remain separate from isolated verification.

## 6. Runtime admission and native consent presentation (2026-10-10)

- [x] 6.1 Separate queued/cold model admission from the bounded two-step runtime probe, preserve cancellation and no dispatch on failure, return actionable transient errors, and align client deadlines. Verify delayed admission, cancellation, probe timeout, retry status and existing admission tests; update API/reliability evidence.
- [x] 6.2 Replace the Windows MessageBox consent with a compact monochrome OffGrid confirmation, explicit Allow/Cancel actions, keyboard-safe defaults and scrollable content for short/high-DPI screens. Keep session scope, limits, revocation and a restrictions summary visible; expose the complete exclusions and workspace identifiers through Details. Validate bounded default dimensions, expand/collapse, consistent fonts, owned consent/denial/close/session-reuse fixtures; preserve native host authority, update desktop documentation and record platform limitations.

## 7. Edge catalog and repeated launch repair (2026-10-10)

- [x] 7.1 Refresh reviewed installed-application discovery when explicitly requested, preserving identities for unchanged binaries and invalidating replaced identities/preparations. Cover initially unavailable and later installed/updated apps without arbitrary paths or widened launch profiles; record read-only installed Edge evidence.
- [x] 7.2 Keep stable system instructions through context compaction, retain launch evidence across archival and refuse repeated launches awaiting verification through the existing bounded pre-dispatch repair path. Test restart/persistence, no duplicate dispatch, prompt-prefix stability and actionable missing-application guidance; run Go/race checks and record limitations.

## 8. Realistic observation context regression (2026-10-10)

- [x] 8.1 Reproduce the post-launch failure with at least 46 controls and an 8 KB native observation at the allocated 8192-token window. Use a lossless model-facing representation and account for prompt content rather than escaped HTTP serialization; preserve complete canonical results, identifiers, instructions, verification and fail-closed budget checks. Test multiple applications, Unicode, protocol evolution, persistence and subsequent control operations with `go test ./internal/agents ./internal/server ./internal/computer` and applicable race checks. Remove application-specific routing examples, document evidence and remaining limits; do not replay the user's failed task or claim public-search completion from a fixture.
- [x] 8.2 Fix the subsequent Windows structured-control expiry loop for slow local inference. Before preparation or verification, protocol-v3 workers may re-observe a retained, exactly matching host-issued observation and proceed only if the complete bounded control view, runtime identities, state digest and target are unchanged. Reject incomplete/changed/foreign observations, retain normal dispatch/consent/replay checks and v2 behavior, and test delayed owned-fixture input and changed-state rejection. Other drivers remain fail-closed until they implement and qualify equivalent revalidation; do not extend a stale timestamp or reuse an expired approval.

## 9. Approved general-runtime follow-up

- [x] 9.1 Implement bounded desktop observations with explicit omissions, scoped paged retrieval and exact short-reference resolution while preserving canonical evidence. Test high-entropy URLs, Unicode, invalid/foreign references, unknown protocols and 8192-token continuation using `go test ./internal/server ./internal/agents`; update API documentation.
- [x] 9.2 Preserve verified application progress across compaction, reject redundant launches until a current read-only check, and accept readiness observations for subsequent controls. Test no duplicate dispatch and checkpoint recovery with `go test ./internal/agents ./internal/server`.
- [x] 9.3 Implement OS-backed Windows application registration discovery with validated no-command launch identities and replacement checks. Test unknown product names, malicious registrations, executable replacement and read-only installed discovery with `go test ./internal/computer ./cmd/offgrid-computer`; document exact coverage and remaining activation providers.
- [x] 9.4 Run applicable Go/race, UI/API, companion and desktop package checks; record results and unresolved full-program stages in the existing reliability plan. Assess GPU readiness without silently downloading/replacing a model or claiming acceleration from device presence.
- [x] 9.5 After validation, back up and replace the current container and update the matching installed desktop application as requested. Confirm one running service, health, installed hashes and preserved state; do not resume personal jobs or grant access during deployment.
