# Endpoint policy capacity and recovery — 2026-11-22

This increment continues M5.04 and OpenSpec `inference-provider-runtimes` task
8.4.4. It adds explicit retirement of obsolete access decisions to the existing
[receipt archival](endpoint-owner-archive-2026-11-22.md) workflow. The task and
program remain incomplete.

## Implemented contract

The owner configuration response includes an optional `policy_maintenance`
preview: policy revision, decision count and exact eligible endpoint identities.
`POST /api/v2/inference-endpoints/configuration/retire-decisions` requires the
reviewed configuration and policy revisions plus a nonempty, unique list of
eligible identities. It cannot retire a current configuration entry, an installed
runtime entry or an identity with an active request. Absence always denies access.

The owner writer lease and owner/runtime/policy locks serialize publication.
The command copies the complete previous policy to an immutable, hash-named
workspace history file, verifies it, rechecks administrator/context,
configuration, installation policy and the entire live policy/restore epoch,
then publishes a version-2 policy without the selected decisions. Version 2
retains a hash link to its previous snapshot; ordinary later reviews preserve
the link. History is bounded evidence, never a source of activation grants.
Each history page retains its predecessor reference without recursively loading
unbounded history on the access path. A missing or corrupt immediate history
page fails closed. Older services do not understand version 2 and reject it.

Archive-only failure leaves current access untouched. An uncertain live-policy
publication cancels active requests and fences new admission until restart
inspects actual state. Neither server nor typed client automatically repeats the
operation. At the 64-decision limit, automatic revocation of an absent identity
does not allocate a redundant denial; it still cancels any associated live
requests. New access requires capacity and an explicit current review.

The 64-decision live limit remains. Explicit maintenance frees obsolete entries;
it does not expand configured endpoints or claim unlimited disk capacity.
Historical files grow within the existing workspace backup limits and private
same-account trust boundary. They are not an independently tamper-proof ledger.
Primary-source rationale for request identity, durable publication and the
no-replay boundary remains in the linked archival evidence.

## Actual verification

- Windows maintenance/recovery tests: PASS 14.454 s before the extra guards;
  final maintenance/guard set PASS 14.246 s. Logs:
  `build/endpoint-policy-maintenance-recovery-20261122.log` and
  `build/endpoint-policy-maintenance-guards-20261122.log`.
- The sequence test performs 66 actual prepare/commit/reconcile/restart/access
  review cycles against disposable local state. It archives 64 settled receipts,
  retires 63 obsolete decisions and verifies each new identity denies access
  before fresh review. No provider, model, live installation or personal session
  is used. This is metadata lifecycle evidence, not provider qualification.
- Process-exit tests terminate after real history and policy writes; restarted
  instances inspect the actual old/new policy without replay. Failure injection
  covers before/after both publications and cancellation of live authority on
  uncertainty. Additional tests cover stale reviews, current/installed/active
  protections, concurrent policy-content changes even with an unchanged revision,
  configuration change, cancellation, fresh epoch, missing history and capacity.
- Typed client contracts: 8 PASS, 7.2 s. Exact request binding, malformed results
  and no retry after lost acknowledgement are checked. The initial TypeScript
  check caught an optional-value narrowing error; it was corrected. Final
  TypeScript check passed in `build/endpoint-policy-maintenance-types-20261122.log`.
  An attempted nonexistent `typecheck` npm script was a command-selection error;
  the actual `tsc --noEmit` check and production build passed.
- Fresh service binary and production renderer: 1 PASS, 6.0 s (test body 5.5 s),
  `build/endpoint-policy-maintenance-service-20261122.log`. The journey creates,
  updates, loses a publication response, reconciles, restarts, archives receipts,
  retires the obsolete decision and restarts again. It verifies unchanged
  configuration, the exact historical policy, retained current review requirement
  and denial for the retired identity. Maintenance is currently invoked via the
  real API; this does not establish rendered maintenance-control acceptance.
- Production renderer build passed in
  `build/endpoint-policy-maintenance-web-20261122.log`; existing bundle-size
  warnings remain. Full Windows server/API/configuration passed in 151.484 /
  1.845 / 1.834 s, `build/endpoint-policy-maintenance-full-windows-20261122.log`.
- Linux endpoint race checks passed in 17.412 s,
  `build/endpoint-policy-maintenance-linux-race-20261122.log`. Source snapshot:
  4,158 byte-verified maintained files, SHA-256
  `1c75e20dd26fb9864b85e7f125ed2a4edfe901cdf18a510761e723847a6cfcf3`.
  Go 1.26.6, CGO enabled, GOPROXY off, GOMAXPROCS=4, fresh ext4 test state.
- Subsequent plain and X25519-encrypted actual workspace backup/restore tests
  passed on Windows in 1.241 s,
  `build/endpoint-policy-maintenance-backup-20261122.log`. They compare restored
  configuration, active journal, archive identity, receipt, live policy and policy
  history byte-for-byte; verify original duplicate receipts and conflicting
  request rejection; require fresh access review after restore; retain denial of
  the obsolete identity; and leave the original revoked workspace unchanged.
  These extra tests were added after the full Windows/Linux runs above. They
  exercise the existing storage engine, not the installed desktop recovery UI.
- A subsequent Linux endpoint race run including both backup tests passed in
  15.524 s, `build/endpoint-maintenance-backup-linux-race-20261122.log`. Its
  4,160-file byte-verified source archive SHA-256 is
  `f35fd560062f57ac81e8dd77540e644be47f1a23446f79f0f7c6d5ef329dbe88`.
  Backend production code is unchanged from the full Windows pass above.

```sh
go test ./internal/server ./pkg/api ./internal/config -count=1 -timeout=8m
go test -race ./internal/server -run 'Test(Endpoint|InferenceEndpoint)' -count=1 -timeout=8m
go test ./internal/server -run '^TestEndpointMaintenanceWorkspaceBackup' -count=1 -timeout=3m
```

## Remaining requirements

Rendered explicit maintenance review is now implemented and checked in the
[UI follow-up](endpoint-maintenance-ui-2026-11-22.md). Bounded historical browsing/export,
damaged-state recovery, installed desktop/platform workflows, independent
security review and human language/accessibility qualification remain open.
Real providers/models, hardware, power-loss and frozen workflow qualification
remain separate gates. No checkbox, commit or release was completed here.

The earlier full Linux server race run exceeded its unchanged eight-minute
deadline; see [task-writer evidence](task-writer-reservation-2026-11-21.md).
The focused race result and full Windows pass do not close that aggregate gate.
