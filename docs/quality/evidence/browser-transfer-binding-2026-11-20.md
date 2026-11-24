# Private browser transfer binding foundation — 2026-11-20

This is the Go protocol/hub portion of
[browser-file-transfers task 3.1](../../../openspec/changes/browser-file-transfers/tasks.md).
The [next ownership increment](browser-upload-ownership-2026-11-21.md) adds host
and companion checks and a separate durable snapshot owner. Protected storage,
full browser integration and negotiated artifact evidence remain required.
The system capability is deliberately not advertised, and no
transfer task is marked complete by these checks.

## Implemented service boundary

`BrowserFeatures.Transfers` accepts only negotiated `transfers:1` with existing
discovery/workspace/structured support. Generic legacy pairing cannot request
the new transfer semantics. Its normal reply/poll projection remains unchanged.

The existing task-pairing path now supports a first browser access request with
an exactly empty saved previous-session field. It checks the authenticated actor,
canonical task and input IDs, saved waiting-for-input/browser state and requested
approval policy. If the saved request specifies a URL, its validated origin is
pinned into the initial code; path/query values do not enter the binding. The
first consuming companion must negotiate transfers and match the saved origin
and policy. Its approved origin set and negotiated features are frozen before
the hub publishes the session. Renewal still requires the stopped previous
session and its exact existing origin/features/policy.

The private pair reply and upgraded polled actions carry
`file_binding:{version,actor_id,workspace_id,run_id,input_id,session_id}` derived
from the hub's task-bound session. Native control's separate target binding is
unchanged. Public BrowserSession JSON still omits Actor and does not acquire a
public file binding. A supplied client binding is rejected for upgraded peers,
and strict duplicate/unknown metadata decoding is applied to that negotiated
contract while preserving legacy decoding.

Actual actor formats were inspected: unauthenticated local service uses
`local-admin`; UserStore creates 32-hex user IDs and the guest fallback is `guest`.
The private bounded opaque validator accepts these. Workspace identity is also
generated as 32 hex. No actor, task or input is fabricated by splitting an action
ID. The companion must still compare these service-derived fields with trusted
host task context before using a selected local grant.

## Executed findings and checks

An actual regression reproduced a shared pointer in the new receipt callback
path: mutating the callback's FileBinding changed the pending evidence and the
next durability attempt observed the changed actor. The baseline failed in
0.452 s. Copying that binding before the callback fixed the failure; poll results
also retain detached copies. A failed durable receipt write cannot acknowledge
completion or cause the already delivered action to be polled again.

The first server test also found that the existing permissive JSON helper ignored
a client-supplied `file_binding` field. It derived the correct server binding but
accepted the conflicting request instead of rejecting it, consuming the code.
The baseline failed in 0.743 s. The negotiated transfer path now applies the
existing strict `DecodeControlMessage` checker to the bounded raw request before
code consumption. Legacy peers retain their existing decoding behavior.

```text
go test ./internal/computer -run 'TestBrowserFile|TestBrowserTaskPairing' -count=1
PASS 0.444 s

go test ./internal/server -run 'TestBrowserFile|TestBrowserTaskPairing' -count=1
PASS 0.714 s
```

These focused checks exercise initial and renewed task binding, real generated
authenticated user IDs, foreign actor/task/input rejection, stale previous
session, saved site and policy tampering, feature downgrade/unknown values,
one-time code consumption, old projection compatibility, detached private
bindings and receipt-write uncertainty. The server fixture runs the actual saved
task access path and HTTP handlers with controlled model output; it is not real
model, browser, hardware or installed workflow qualification.

The source-size gate passed. Full affected Go suites, Linux race checks, generated
contract checks and composed host/companion/browser transfer evidence are still
required after integration. Initial consent/ready-time task revalidation and
durable resource lifetime remain the host/session owners' responsibility; these
focused issuance tests do not prove the complete composed flow.
