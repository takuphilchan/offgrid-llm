# Attached-browser framing foundation

Date: 2026-11-20. Status: partial tasks 1.2/2.1 of
[browser-selected-tab-attachment](../../../openspec/changes/browser-selected-tab-attachment/proposal.md).
The [network feasibility failures](selected-browser-research-2026-11-20.md)
remain blocking; no attached automation, native-host registration or service
capability was enabled.

## Implemented boundary

- `apps/computer/native-message-framing.mjs`: native-endian four-byte framing,
  64 KiB application cap before payload allocation, bounded JSON depth/value/size
  validation, strict UTF-8, cancellation/EOF handling and fixed error codes.
- `apps/computer/native-json-keys.mjs`: raw bounded JSON scan before semantic
  parsing; rejects duplicate names, including escaped-equivalent nested names.
- `apps/computer/attachment-protocol.mjs`: exact direction/field/version checks and
  detached immutable metadata snapshots. Saved run/input syntax matches current
  desktop admission. No action, evaluation or CDP message exists.
- [Protocol contract](../../../openspec/changes/browser-selected-tab-attachment/protocol.md):
  exact field names, scope identities, nonce limits, reserved future capability
  names, and the distinction between metadata validation and authority.

The real subprocess fixture is `test/fixtures/attachment-framing-child.mjs`. It
reads actual stdin frames, returns bounded metadata on stdout and exits on EOF.
It is not installed or registered with a browser. The subprocess's static test
nonce is explicitly fixture data and does not attest to authentication.

## Verification and retained failures

Environment: Windows, Node 24.18.0, existing dependencies only. No browser launch
was needed for these transport tests.

```powershell
node --test --test-concurrency=1 apps/computer/test/native-message-framing.test.mjs apps/computer/test/attachment-protocol.test.mjs
```

- Initial suite: **FAIL, 9 passed / 1 failed, 0.365 s**. A failed Writable callback
  removed its error listener before Node emitted the error, allowing the raw sink
  error to escape. Corrected listener lifetime; the test remains.
- Independent root review identified duplicate-member acceptance through
  `JSON.parse`. Actual subprocess baseline: **FAIL, 0.143 s**; a duplicate root
  type was accepted and the child exited zero. Added the raw member scanner and
  root/nested/escaped-equivalent subprocess negatives.
- Final focused suite after those corrections: **PASS, 13/13, 0.850 s**.
  Coverage includes bytewise fragmentation, Unicode/multiple frames, exact byte
  limits, oversize headers before bodies, partial EOF, malformed private payload
  redaction, non-JSON/getter input, read/write cancellation, actual sink completion,
  wrong direction/unknown fields, unsafe origins and immutable saved-task scope.
- Final hygiene: strict OpenSpec validation PASS; documentation checker PASS,
  472 maintained files / zero problems; source-size ratchet PASS. Scoped
  `git diff --check` exited zero but does not cover newly untracked files.

## Remaining work

All 12 change tasks remain unchecked. The source is not integrated into desktop
IPC, service pairing or an extension dispatcher. Peer/process authentication,
consent, ordered state/epoch correlation, duplicate operation receipts, worker
lifecycle, queue scheduling, capture chunks, installation and platform/browser
qualification remain open. A valid `bind` or `released` message alone proves none
of those behaviors. No successful framing test changes the failed egress result.

No full companion/desktop/Go/API/renderer gate was run for this isolated codec
increment. The aggregate gate must include these files when the combined source
is stable; focused results do not establish merge readiness or installed support.
