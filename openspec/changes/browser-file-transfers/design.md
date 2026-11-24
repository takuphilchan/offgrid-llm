# Design

## Context

See [proposal](proposal.md) and the existing browser workspace/journal owners.
`browser.mjs` currently hashes the selected path before `setInputFiles(path)`;
it saves downloads as `Date.now()` plus a sanitized page label. The desktop picker
hashes another opening of the selected path. Both boundaries need one captured
content identity. Existing 512 MiB admission, exact target/document validation,
manual grant invalidation and no-replay receipts remain authoritative.

## Goals / Non-Goals

Implement the transfer contract inside existing task/session/resource owners.
Do not create a file executor, import personal profiles, bypass consent, execute
downloaded content, or imply remote receipt from a native file-input assignment.
Attached-provider parity remains with its own change; this work supplies a shared
contract and managed implementation, not an exception to attachment protections.

## Decisions

### Snapshot bytes before input

Open the selected source once with regular-file and identity checks, bounded reads
and cancellation. Hash the exact bytes copied to an exclusively created private
snapshot; compare with the locally granted size/digest before dispatch. Seal the
snapshot against the application's other writable paths and retain its ownership
until browser assignment and independent file-content verification finish. Never
publish its path to the service/model or make the source path a later input.
Use the established protected host storage and exact artifact identities; do not
place snapshots in an agent workspace or directory selected by a page. Enforce
the trusted-local-process boundary explicitly: application tools cannot mutate
the snapshot; arbitrary compromise of the user's full OS authority is not solved
by randomized filenames or read-only mode alone.

Small in-memory Playwright payloads are an option, not the universal algorithm.
The installed pinned Playwright 1.62.1 rejects buffer payloads at 50 MiB; reducing
OffGrid's accepted 512 MiB limit would violate the requirement. The protected
snapshot path and actual >50 MiB fixture must therefore pass before the task is
checked. If a platform cannot establish snapshot integrity, retain that platform
requirement as incomplete rather than silently reverting to the original path.

Retain the observed native input handle and verify its document/form before and
after preparation. Assign once. Read-back verifies the selected file's complete
size/digest/name through a trusted browser view, not only a page-supplied label.
The receipt says selection, not remote upload. Failure after assignment retains
dispatched uncertainty. Input/change callbacks can have effects; never treat
failed read-back as permission to select again.

### Establish native storage authority

Managed transfers use a storage-only operation in the existing verified native
computer binary, before input ownership, UI drivers or a Stop window are opened.
Its bounded private request carries only trusted host workspace roots; the fixed
host state directory comes from the supervisor. New owned leaves use the existing
native private-directory primitive. Existing state and retained upload descendants
must already pass native ownership/ACL checks; startup does not repair permissions,
adopt orphan snapshots or delete retained evidence. Check again on explicit resume
and before relying on newly created snapshot contents.

The execution host reserves the default browser state and any explicitly supplied
isolated profile root before folder approval/pairing. Canonical overlap is denied
in both directions, including aliases and missing state leaves. Desktop passes its
fixed browser directory through private native enrollment, never renderer input;
browser activation independently checks active host workspace roots. Native hosts
also coordinate through a bounded private account registry, independent of desktop
profiles: reserve browser locations before creation and execution roots/journals
before worker activation. Missing-path reservations pin their existing ancestor;
only the original capability can finalize them. Release execution reservations
only after the existing worker confirms endpoint and process cleanup. Lost owners,
changed identities and failed legacy recovery remain uncertain; PID, age and EOF
alone never retire their records. Only verified released rows may be reclaimed.
For updated local native workers, the original stdio supervisor may retain the
same reservation capability through a bounded private startup handoff. Validate
the complete original grant digest, durable row binding and exact existing journal
file identity before acknowledging startup; only then accept dispatch. Missing,
partial or rejected acknowledgments leave the reservation uncertain. Keep the
capability out of public worker identity, task receipts, inspection and child
process environment. A surviving Linux/Windows endpoint may publish settlement
only after its retained native process owner proves descendant cleanup, preserving
the worker's failed exit and uncertain operation receipts. Retry only that
publication with the original in-memory authority; never recreate/open a missing
journal or repeat an effect. Legacy workers, lost handoffs, loss of all owners and
isolated backends do not gain this recovery path. Isolated cleanup's existing
missing-journal success is not settlement evidence.
This excludes rooted file operations across independently configured profiles.
It does not establish containment of unrestricted same-user commands, which
requires separate integrity qualification before the full storage gate can close.

Older/browser-only packs retain ordinary browsing. Without a successful native
storage check, protected transfers are unavailable. The provisional private
`transfers:1` field currently negotiates task/action binding only; no public
`browser-file-transfers-v1` capability is advertised as completed. Public transfer
availability and canonical evidence remain tasks 3.1 and 3.2.

### Own the download lifecycle

Use the existing managed context and exact initiating page/frame, not a global
browser listener that can consume another task's download. A bounded native
download monitor associates browser-issued identities with one admitted action,
observes progress, rejects unsolicited/ambiguous transfers and cancels excess
bytes. Prototype this against the installed Chromium before relying on CDP event
coverage; no post-completion size check alone establishes a bounded download.

The pinned native progress probe exceeded a 256 KiB staging budget before its
first positive callback, including responses without Content-Length. Progress
cancellation alone is therefore insufficient. The bounded-response prototype
instead takes the original response through Fetch/IO with explicit chunk limits
and denies the automatic native download sink; it detects one byte beyond the
budget without writing it or repeating the HTTP request. Preserve native event
monitoring for attribution/unsolicited-transfer detection, but establish disk
bounds at the write boundary. Blob/data sources, frame/action correlation and
durable publication must meet the same acceptance requirements before this
replaces production downloads. See [actual probe evidence](../../../docs/quality/evidence/browser-file-transfers-2026-11-20.md).

Create an exclusive per-artifact directory under private task storage, retain
opaque IDs and publish its manifest only after completed bytes, regular-file
identity, size and digest are independently established. Keep partial files
unpublished, bounded and owned through cancellation/cleanup. Use a native
read-only/exclusive publication path where required, not timestamp uniqueness.
Task storage accounts for aggregate retained and in-flight bytes, with no budget
reset on reconnect. A source filename/type is a hint; content detection is
recorded separately and does not claim malware inspection or safety to execute.

Playwright's download stream/path becomes available only after completion;
`saveAs` and `failure` can wait for completion. Those APIs cannot alone enforce
in-flight quotas. A fixed reviewed CDP monitor is an implementation choice,
not a model-callable CDP bridge. Confirmed Stop must retire its listeners and
browser transfer, preserving uncertainty when closure is not confirmed.

### Integrate artifacts and compatibility

Extend existing companion result/journal and task receipt projection with a
versioned file-evidence contract. Negotiate support before advertising stronger
upload/download tools; preserve legacy fields without interpreting them as the
new evidence. Bind artifact manifests to task/session/action/workspace and actor;
revalidate them for recovery and host-mediated inspection/export. Reuse existing
task artifacts where their remote/local ownership matches; never expose a
companion-local path as a service-local artifact or silently upload file bytes to
a separate server. User export uses a trusted picker and an exclusive destination
with byte verification, never a renderer-supplied path or automatic opening.

Public provenance uses sanitized source origin/path with explicit URL redaction;
no credentials, query values or fragments enter model context. Exact private
source details remain behind the local artifact reference where retention is
appropriate. Provenance labels identify time, action, tab/frame/document and the
verification stage rather than presenting a digest as proof of useful content.

## Primary sources

Read 2026-11-20: [Playwright download lifecycle](https://playwright.dev/docs/api/class-download)
documents start/completion and context-close deletion, cancellation and stream
behavior. [File input API](https://playwright.dev/docs/api/class-elementhandle#element-handle-set-input-files)
describes paths/payloads; the 50 MiB buffer limit was verified in the installed
1.62.1 `coreBundle.js`, not assumed from a generic feature list.
[Chromium Browser domain](https://chromedevtools.github.io/devtools-protocol/tot/Browser/)
provides candidate download progress/cancellation controls. Their actual pinned
runtime behavior, correlation and cleanup require the executable probes above.

## Risks / Trade-offs

- Large snapshots consume disk/memory and may race source edits → bounded
  streaming, exact captured digest, cancellation, retained ownership and >50 MiB
  checks; no allocation from an untrusted file size without admission.
- Browser progress or cleanup may fail → retain uncertainty and stop dependent
  dispatch; do not label post-hoc rejection as an enforced in-flight bound.
- Artifact stores have different host/service ownership → reuse only compatible
  existing surfaces and test remote-service separation explicitly.
- Source names, URLs and MIME headers are untrusted → bounded sanitation,
  explicit redaction/type provenance and no automatic open/execute action.

## Migration Plan

Additive negotiated capability and versioned manifests. Old receipts remain
inspectable without upgraded verification claims. Rollback disables new transfer
admission, preserves completed artifacts and tombstones, and never replays an
uncertain transfer. No installed-user migration, registration or publication is
performed by development fixtures.
