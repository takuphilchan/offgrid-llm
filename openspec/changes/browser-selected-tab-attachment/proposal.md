# Proposal

## Why

OffGrid's managed browser cannot use a tab where a user is already signed in. Roadmap M2.02 requires explicit Chrome/Edge tab attachment without taking ownership of the user's browser or weakening the existing task, consent and no-replay boundaries.

## What Changes

- Add a selected-tab attachment provider using a Manifest V3 extension and a local native-messaging host. The user selects a tab through the browser's extension action and confirms the exact task and origin scope in trusted OffGrid UI.
- Reuse the current task service, companion dispatch journal, browser tool contracts, observations and manual-control state machine. Bind every attachment to the actor, workspace, task, pending access request, extension connection and tab generation. No raw model CDP or JavaScript execution interface is added.
- Distinguish borrowed tabs from tabs created for the task. Stop, revocation and failure detach borrowed tabs and preserve the browser/profile. Additional borrowed tabs require explicit selection; up to eight consented task tabs remain the cap.
- Add capability/version negotiation, bounded native messages, local Stop, task-bound initial attachment and renewal, and truthful recovery after lost replies, worker suspension, debugger loss or browser replacement.
- Establish a blocking network-boundary feasibility gate before enabling attachment automation. Existing-profile traffic, service workers, established sockets and DNS behavior must not be represented as having managed Chromium's CONNECT enforcement without evidence. An unresolved boundary leaves the affected capability and M2 requirement incomplete.
- Qualify the composed path with disposable profiles and synthetic signed-in sites, then separately record branded-browser, installed-package and explicitly authorized real-profile gates.

## Capabilities

### New Capabilities

- `selected-browser-attachment`: Explicit ownership, authorization, lifecycle and compatibility of selected existing browser tabs through OffGrid's canonical browser execution path.

### Modified Capabilities

None. Existing `task-access-intent` and `desktop-service-compatibility` requirements remain in force. This change composes the active browser reliability, workspace, structured-control and manual-takeover contracts without editing their scope.

## Impact

Planned areas: a small packaged extension/native-host bridge, `apps/computer` driver adapters and shared workspace lifecycle, desktop computer-only IPC/consent, browser hub pairing/capability enforcement, API schemas, localized shared UI, packaging manifests and isolated integration fixtures. A transport is not an additional task runner.

No personal browser access, native-host registration, extension installation, store publication, profile copying, cookie export, global proxy modification or model download is authorized by creating these artifacts. General extension marketplace governance, browser files and real-model/browser completion qualification remain in their existing program scope; attachment must not advertise unsupported tool features as complete. This proposal is ready for design review, not evidence of implemented attachment support.
