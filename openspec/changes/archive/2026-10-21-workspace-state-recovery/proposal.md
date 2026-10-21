# Proposal

## Why

The approved whole-product redesign starts with truthful interaction states:
several current controls describe an action they do not perform, and recovery
can refresh unrelated data. Fixing these contracts first prevents the new shell
and workspaces from retaining the same confusing behavior behind new styling.

## What Changes

- Distinguish application discovery from granting access to a selected target.
- Bind speech-model discovery/search retry to the failed read operation, keeping
  installation retries separate and guarding cancellation and stale responses.
- Represent microphone readiness, permission, recording, and transcription as
  distinct cancellable phases with accurate accessible labels.
- Distinguish Knowledge checking, ready, unavailable, and failed checks without
  treating a pending check as a service failure or falling back from requested
  Knowledge to ordinary inference.
- Remove unconditional on-device/privacy claims from the shell, empty chat,
  authentication, and speech preparation copy. Identify the selected service
  without guessing its physical location from a loopback address.
- Add regression coverage and document the shared interaction contract.

This is stage 1 of the approved redesign, not the entire redesign. Navigation,
shared controls/layout, work continuity, document-scoped chat, model-to-use flow,
Activity, and Settings follow in independent changes. No runtime qualification,
model downloads, microphone recording, native input, deployment, or release is
included. Existing monochrome styling and authorization are unchanged.

## Capabilities

### New Capabilities

- `workspace-operation-feedback`: Truthful phases, action labels, operation-bound
  recovery, and processing-context messaging in the shared web/desktop renderer.

### Modified Capabilities

None. The existing voice-model-foundation change retains its acquisition and
runtime scope; this change does not mark its outstanding tasks complete.

## Impact

Shared React controls, Chat, model package management, application-access UI,
localized presentation copy, Playwright tests, and workspace documentation.
No public API, storage format, authorization policy, or dependency changes.
