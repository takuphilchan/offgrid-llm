# Proposal

## Why

Roadmap M2.07 requires exact user-approved upload bytes, owned downloads and
inspectable provenance. The current companion checks a source path and later asks
Chromium to reopen it, while downloads use timestamp-based destination names and
report verified staging without durable artifact identity or source metadata.

## What Changes

- Bind upload execution to one bounded, digest-verified byte snapshot of the
  locally selected file; reject changed, special, oversized or revoked inputs
  before browser input. Preserve the existing 512 MiB public limit.
- Retain task-owned downloaded artifacts with exclusive names, bounded transfer
  and staging, verified content/size/digest, source/time/document provenance and
  accessible host-mediated inspection/export. Never open or execute downloads
  automatically or overwrite user files.
- Distinguish native file-input selection from remote receipt and local staging
  from completed user export. Preserve uncertain dispatched effects, lost-reply
  tombstones, explicit recovery and manual-takeover grant invalidation.
- Add real isolated Chromium/HTTP/file-system acceptance and desktop/service
  contract checks; retain model, installed-OS and human qualification as open
  external gates.

## Capabilities

### New Capabilities

- `browser-file-transfers`: User-selected exact-byte uploads, bounded owned
  downloads, artifact provenance and explicit inspection/export lifecycle.

### Modified Capabilities

None. Existing action receipts, approval policy, task/session ownership and
takeover contracts remain authoritative and are reused.

## Impact

Primarily `apps/computer/browser.mjs`, a focused transfer module and existing
companion/session journal; desktop upload selection and artifact inspection;
browser tool descriptions, receipt validation and result projection as needed.
Additive capability negotiation must prevent older components claiming stronger
verification. No new executor, personal-profile access, external upload, model
download, installation, release or automatic artifact publication is authorized
by this increment. Full browser and program qualification remain on the roadmap.
