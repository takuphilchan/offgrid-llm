# Canonical CLI source drafts

Date: 2026-11-20. Source: the uncommitted shared worktree in
`D:\offgrid-llm`, continuing `agent-conversation-workflows` 10.1–10.2.
The full conversation/client and model qualification requirements remain open.

## Behavior and authority

`internal/cli/conversation_source_draft.go` reuses the existing
`ConversationFileDraft` and `ChatImageCommand` implementations. Canonical
interactive chat can upload, select, inspect, prepare and remove draft files and
snapshot local PNG/JPEG images. These operations do not submit input. File
mutations require source and session-file capabilities; image selection and
Knowledge enablement require canonical source support. The service remains the
owner of source access, exact revisions, context and model readiness.

The pending canonical input contains detached file references and image values.
Receipt recovery never uploads again and observes the existing accepted input
before considering another POST. Acknowledgment clears files/images only if the
submitted draft version has not changed. Knowledge remains a selected preference.
Source-only ordinary input preserves an empty caption; answers/corrections cannot
silently absorb selected sources.

An unknown source-bearing result blocks replacement text. `/retry` uses its
original request identity and source snapshot; `/discard-retry` explicitly
forgets the local retry, retains draft sources and states that it does not cancel
service work. Existing source-free text replacement behavior remains compatible.
Leaving the CLI still detaches observation rather than stopping the service.

`serviceclient.SubmitSessionInput` checks required source/scope/response-option
capabilities after receipt lookup and before a fresh submission. An accepted
receipt remains recoverable when these capabilities are withdrawn. The input
rejection wrapper preserves the underlying transport error and is specific to
the submit phase, so a failure reading receipts cannot be mistaken for evidence
that an earlier input was rejected.

## Rejection contract

The initial submit POST's 400/413/422 response permits a corrected draft with a
new request identity. A failed required-capability preflight before any POST also
permits correction; this is known from the client's submission phase rather than
inferred from an HTTP status. The relevant current service paths are
`handleSubmitConversationInput`, `validateConversationSources`,
`attachmentServiceError`, `sessionImageReady` and
`Runner.AcceptConversationInput`: invalid envelopes/content, oversized or
unavailable source evidence and unavailable vision prerequisites reject before
input acceptance commits. This is a narrow contract, not a rule that every HTTP
error proves no work occurred. 403/404/409/429, transport/5xx failures and failures
during receipt inspection remain conservative. A later validation rejection
cannot erase uncertainty retained from an earlier invocation.

## Acceptance evidence

| Requirement | Test and observed boundary |
|---|---|
| Source-only file/image input without resubmission | `TestCanonicalSourceDraftSourceOnlyAndNoResubmission`: real authenticated client transport, exact revision/image payload, then a source-free follow-up |
| Immutable retry despite in-place draft edits | `TestCanonicalSourceDraftUnknownRetryIsDetachedFromEdits`: two image snapshots, removal that shifts the editable slice, new file/image, blocked replacement text; both recovered acceptance and explicit same-ID POST retry |
| Receipt recovery before capability checks | Receipt-recovery and missing-receipt capability-withdrawal cases: accepted input recovered without another POST, absent receipt never bypasses withdrawn support |
| No silently dropped uploads or unprepared sources | Failed-upload, retained-source preparation, unsupported file capability and pending-question cases |
| Explicit uncertainty/rejection handling | Initial submit 400/413/422, later submit rejection after 503, receipt-read 400, explicit local discard and unchanged upload count |
| Exact transport phase | `TestConversationInputRejectionRequiresInitialSubmitResponse`: eight statuses at both inspection and submit; wrapper preserves underlying error |
| Actual service, store and migration integration | `TestCanonicalSourceDraftCLIActualServiceReceiptAndRestart`: eligible legacy text import, owned file upload/preview/preparation, image prerequisite, source-only input, deliberately lost accepted response, retry observation, subsequent text, durable evidence preparation/restart and legacy write denial |
| Actual rejection before acceptance | `TestCanonicalSourceDraftCLIActualValidationRejectionAllowsCorrection`: oversized caption receives actual service 400, corrected input accepted once with retained source |
| Shared image/model readiness | `TestCanonicalSourceDraftCLIActualVisionRejectionRetainsDraft`: actual missing projector returns 422, no input or inference is accepted, image remains selected until explicit removal and corrected text is accepted |
| Caption correction and explicit source-only discussion | Missing source-only option capability allows an explicit caption without local discard; explicit `/message` with selected sources preserves empty caption while a clarification is pending |

## Checks

Windows source checkout using the existing Go 1.26.6 toolchain and isolated
temporary files/stores/HTTP servers. No installed service or model weights were
used. A non-model projector fixture exercises the existing readiness contract;
it is not evidence of successful vision inference.

| Command | Actual outcome |
|---|---|
| Focused CLI source/question/discussion checks | Passed: 0.637s |
| Strengthened detached source retry checks | Passed: 0.858s |
| `go test ./internal/cli ./internal/server -run TestCanonicalSourceDraft -count=1` | Passed: CLI 1.045s, server 3.257s |
| Final focused source/transport checks | Passed: CLI 0.952s, server 2.039s, serviceclient 0.339s |
| `go test ./internal/cli ./internal/serviceclient ./internal/server -count=1` | Passed: CLI 3.156s, serviceclient 0.907s, server 205.446s |
| Final full CLI/serviceclient after pre-POST rejection and source-only discussion fixes | Passed: CLI 3.723s, serviceclient 0.890s |
| Final actual server source journeys, including vision rejection | Passed: 2.238s; server production was unchanged |
| WSL `CGO_ENABLED=1 GOPROXY=off /usr/local/go/bin/go test -race ./internal/cli ./internal/server -run TestCanonicalSourceDraft -count=1` | Passed: CLI 1.868s, server 6.006s |
| WSL `CGO_ENABLED=1 GOPROXY=off /usr/local/go/bin/go test -race ./internal/serviceclient -run TestConversationInput -count=1` | Passed: 1.540s |
| Source-size, strict OpenSpec validation and diff whitespace checks | Passed |
| `node scripts/maintenance/check-docs.mjs` | Passed: 455 maintained Markdown files; this does not check external URLs or anchors |

The first Linux race command used a parenthesized alternation through WSL's
shell-forwarded arguments and failed with a bash syntax error before Go started.
The two simple-pattern commands above replaced that invocation; no test or
production behavior changed to obtain their passes.

Independent read-only review by the scheduling agent found no concrete blocking
finding in the sampled draft/loop/client slice. It checked capability alignment,
image detachment, draft-version clearing, receipt-before-capability order and
initial versus prior-uncertain rejection handling; it did not execute tests.
A follow-up read-only review checked the pre-POST capability rejection wrapper
and empty explicit `/message` source-only path. It found no blocking issue;
prior uncertainty, focused-work identity and failed-upload rejection remain.

Real local-model document/vision behavior, accessibility/usability, all client
cutover/migration paths and the remaining 10.1–10.3/14.x qualification are not
completed by these deterministic tests. No roadmap milestone is marked complete.
