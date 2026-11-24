# Attachment control protocol v1

This document fixes the metadata/framing names for tasks 1.2 and 2.1. The current
foundation validates messages only; it is not an installed native host and has no
browser dispatcher, authenticated peer channel or granted session. The failed
network feasibility gate remains blocking. A structurally valid `bind` is data,
not proof that local consent, saved task validation or service pairing occurred.

## Framing and data bounds

Use the native-messaging standard four-byte unsigned native-endian UTF-8 byte
length followed by one JSON object. OffGrid's application limit is 65,536 bytes
per message, below the browser's transport maximum. JSON must be well-formed UTF-8
and Unicode, with finite numbers, at most 32 nesting levels and 4,096 values.
Duplicate object member names are rejected before semantic parsing, including
escaped-equivalent names at any depth. Unknown message types/fields and wrong
directions are rejected. No newline-delimited fallback exists.

The encoder checks cumulative size before constructing the complete JSON string;
the decoder checks the length before allocating a payload. Reading is an async
iterator with one current frame, and writing awaits its callback. Owners must
serialize writes and bound their own pending-operation queues. EOF in a header or
payload, cancellation and IO failure are errors with fixed codes; payloads and OS
error details are not interpolated into errors. These limits are not a benchmark
or assurance against a fully compromised local process.

## Exact metadata messages

Every message contains `protocol_version: 1` and `type`. The sender column is
enforced by the schema helper. Fields shown below are exhaustive.

| Type | Sender | Additional fields |
| --- | --- | --- |
| `hello` | Extension | `extension_id`, `browser_version`, `client_nonce` |
| `hello_ack` | Host | `connection_id`, `client_nonce`, `host_nonce` |
| `select` | Host | `connection_id`, `request_id`, `scope`, `origins`, `approval_mode` |
| `selection` | Extension | `connection_id`, `request_id`, `selection_id`, `tab_ref`, `document_id`, `origin`, `title` |
| `bind` | Host | `connection_id`, `request_id`, `scope`, `selection_id`, `session_id`, `previous_session`, `attachment_id`, `grant_epoch`, `origins`, `approval_mode` |
| `release` | Host | `connection_id`, `request_id`, `session_id`, `attachment_id`, `grant_epoch`, `reason` |
| `released` | Extension | `connection_id`, `request_id`, `session_id`, `attachment_id`, `grant_epoch`, `confirmed` |

`scope` has exactly `actor_id`, `workspace_id`, `run_id`, `input_id`. Run and input
values are the existing saved service task and pending access IDs, not generated
transport aliases: `run-[a-f0-9]{32}` and `input-[a-zA-Z0-9_-]{1,122}` match current
desktop admission. `request_id` is a separate trusted-host operation correlation
ID; it never replaces `input_id`. Actor IDs retain the existing bounded colon,
hyphen and underscore alphabet. Other opaque IDs use 1–128 ASCII letters, digits,
hyphens or underscores. Connection and attachment IDs must be generated and
owned by the eventual trusted endpoint; syntax alone does not enforce ownership.

Nonces are 32 random bytes represented by 64 lowercase hexadecimal characters.
The foundation checks their shape, not randomness or authentication. An extension
ID has Chromium's 32-character `a`–`p` form and remains an informational identity,
not authorization against spoofed local callers. Browser versions are bounded
numeric dotted strings, not evidence that a capability works.

`origins` is the canonical ordered list of 1–16 unique public-HTTPS origins using
the existing browser scope validator; paths, credentials, aliases, trailing
slashes and normalization changes are rejected. `approval_mode` reuses exactly
`ask_every_time`, `scoped_changes`, `full_task`. The selected origin must later be
matched to the consented list by the owner; the stateless schema does not authorize
that relationship. Titles are bounded to 512 UTF-8 bytes without control characters
and remain untrusted display data.

`previous_session` is always present: empty for initial attachment, otherwise the
exact old service session. The live service/owner must decide which is appropriate
from the saved pending request. `grant_epoch` is a positive safe integer and must
be monotonically fenced by the eventual owner. Release reasons are `stop`,
`manual`, `revoked`; `confirmed` is boolean. A claimed confirmed release is usable
only after exact connection/request/session/epoch correlation and verified local
cleanup. None of that stateful enforcement is supplied by schema validation.

## Authority and future dispatch boundary

The final flow must authenticate the local native peer, validate current service,
principal and workspace, bind browser-owned selection to native consent, and then
publish an atomically task-bound service session. Challenge nonces or a static
extension ID alone cannot meet those requirements. Stored preferences and
renderer-supplied IDs cannot create grants. Reconnecting after EOF cannot revive
authority or replay an uncertain action.

The public capability name is reserved as `browser-attachment-v1`, the additive
browser feature as `attachment:1`, and saved request intent as `browser_source`
with `managed`/`attached` values (omission means managed). **None is advertised or
wired by this foundation.** Metadata protocol version 1 accepts no `action`, CDP,
JavaScript or generic RPC message. A later reviewed dispatch contract must use an
explicit negotiated protocol extension/version, preserve the original canonical
`action.id` and service task IDs, and retain the single companion dispatch journal.
No old component can silently interpret an unknown message as execution.

Large capture transfer, queue/state-machine enforcement, peer authentication,
initial/renewed service pairing, extension action UI and installation remain
unfinished. Do not mark tasks 1.2 or 2.1 complete from the framing tests alone.
