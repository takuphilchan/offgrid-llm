# Endpoint owner configuration: source and isolated-service evidence

Date: 2026-11-21. Scope: roadmap M5.04 and existing
`inference-provider-runtimes` task 8.4.4. The task and program remain incomplete.
This record extends the [administration foundation](endpoint-management-2026-11-20.md),
not a replacement architecture or a real Ollama/vLLM qualification claim.

## Implemented boundary

The existing owner file now has administrator GET/prepare/commit/reconcile
operations, OpenAPI schemas and generated renderer types. Preparation binds the
whole-file digest, current actor/epoch, installation policy, exact endpoint and
expiring review. Publication first revokes affected access, durably records
intent, writes the owner file once, then records publication. Explicit
reconciliation compares stored bytes with the recorded before/after digests;
it never repeats publication or grants access. Changed revisions require service
restart and a separate explicit access review.

`internal/server/inference_endpoint_owner_*` owns this protocol and its journal;
the existing endpoint policy and provider scheduler remain the admission owners.
Credential references come only from the exact-origin host allowlist. Managed
entries come only from immutable host templates; the browser cannot submit
executables, arguments, environment variables or raw credentials.

The Models owner panel (`EndpointOwnerPanel`, `EndpointOwnerForm`,
`EndpointOwnerDetails`) renders creation/update, exact review, read-only status,
history and explicit reconciliation. `apps/web/src/api/endpoint-owner.ts` checks
response identities against each request. Commit consumes the local review
before sending; an unknown response leaves the request identity and durable
inspection path, without an automatic retry-save action. Reload discovers the
server journal. Changing credentials requires an explicit approved reference or
explicit removal of the prior credential. Editing a review preserves entered
fields. Managed forms send only template and identity/actor/attestation fields.

`inference-endpoint-owner-v1` is protocol discovery in the existing system
capability list. It remains available while ordinary endpoint policy reads
return 503 for unresolved publication. It neither promises writable storage nor
grants configuration permission. Successful and failed owner mutations refresh
the separate access-policy panel so stale enabled controls are not retained.

Automatic owner revocation now persists `review_required` on the subtractive
decision. It is not reported as a completed human review. A contradictory enabled
decision requiring review is rejected. Explicit human review replaces that
marker through the existing policy transaction. Older policy readers reject the
new optional field rather than silently authorizing it.

## Actual checks and retained failures

Environment: Windows, Go 1.26.6 with CGO disabled, installed Node/Playwright and
owned disposable Chromium profiles. No weights, personal tabs, live installs or
real provider accounts were used. Provider addresses use `.example.test` and
documentation-only IPs; the fixture credential is deliberately absent.

| Check | Actual result |
|---|---|
| Owner/management/system recovery subset | PASS, 2.021 s, `build/endpoint-owner-recovery-contract-final-20261121.log` |
| Owner/management/policy/system subset after human-review correction | PASS, 2.439 s, `build/endpoint-owner-human-review-20261121.log` |
| Full Windows server, API and configuration packages | PASS: server 146.724 s, API 1.770 s, config 1.834 s; `build/endpoint-owner-full-windows-20261121.log` |
| Full Linux race server, API and configuration packages from verified ext4 snapshot | PASS: server 257.847 s, API 1.037 s, config 1.012 s; `build/endpoint-owner-linux-rehydrated-race-20261121.log` |
| Client contracts + initial owner renderer + previous endpoint renderer | 17/17 PASS, 30.7 s; `build/endpoint-owner-renderer-final-20261121.log` |
| Expanded owner renderer, including managed templates and stale preparation | 9/9 PASS, 19.3 s; `build/endpoint-owner-ui-expanded-20261121.log` |
| Arabic 390×844 viewport and focus restoration | PASS, 4.0 s including startup; actual horizontal/vertical bounds checked and viewport screenshot inspected |
| Actual Windows service and built renderer, create/update/lost-response/reconcile/restart/access review | 1/1 PASS, 4.8 s including startup (body 4.3 s); `build/endpoint-owner-service-review-fixed-20261121.log` |
| API generation consistency, TypeScript and production renderer build | PASS; existing Vite large-chunk warnings retained |
| Locale catalog validation | PASS, all 29 languages have authored messages; linguistic/safety review remains pending |

The service test starts an owned binary on an ephemeral loopback port with a new
private directory under LocalAppData. Only that new directory receives a private
ACL; no existing ancestor permissions change. The service itself verifies writer
ownership. Final fixture:
`C:\Users\phil\AppData\Local\offgrid-owner-qualification-66f4f4048d28404a81acc167d722e79f`.
Its journal, owner file and service log retain the actual effects. The test
observes two commits for two revisions, no probes, no file creation during
preparation, blocked access while publication is unresolved, credential-reference
preservation, and fresh review after each restart. For the update, Playwright
lets the real service publish, then deliberately drops only the UI response.

Two actual-service failures were investigated without weakening assertions:

- Populated select options interfered with the exact accessible label lookup.
  Selects now reference their visible labels explicitly; populated credential
  and managed-template regressions pass. Initial 60-second failure log and trace:
  `build/endpoint-owner-service-20261121.log` and
  `build/endpoint-owner-service-label-failure-20261121.zip`.
- Automatic access revocation was incorrectly displayed as a human-reviewed
  disabled decision. The durable marker and projection above correct it; the
  original “Review required” assertion now passes. Prior failure:
  `build/endpoint-owner-service-label-fixed-20261121.log`, with preserved
  `build/endpoint-owner-service-human-review-failure-20261121.zip`.

An earlier capability experiment tied discovery to the ordinary policy response.
A backend test proved that response correctly becomes unavailable after
publication, making recovery undiscoverable. Discovery moved to system identity;
the new test requires policy 503 and owner inspection 200 concurrently.
`build/endpoint-owner-capability-backend-20261121.log` preserves that failure.

Linux race launch attempts first failed in shell argument processing (the regex
pipe and then an empty extraction-path argument); neither ran the intended suite.
They are retained in `build/endpoint-owner-linux-race-20261121.log` and
`build/endpoint-owner-linux-full-race-20261121.log`. The corrected script restores
and byte-verifies 4,145 maintained files onto Linux ext4 before running the full
server/API/config race suite. Archive SHA-256:
`a9f2319d86e13c05579c00560263a4a02e9186fcd19ffdc50eb39bed6dd15ef4`.
`build/endpoint-owner-checkpoint-20261121-manifest.json` records every source hash.
The final Linux suite passed as recorded above. Later documentation and the additional
viewport assertion are outside that frozen snapshot; server source is unchanged.

Documentation checks passed for 488 maintained Markdown files; source-size
ratchet, scoped whitespace checks and strict provider OpenSpec validation also
passed. These checks do not execute model, hardware or participant qualification.

The separate [Linux lifetime checkpoint](worker-linux-namespace-2026-11-21.md)
passed all 22 CI race package groups before this owner increment. That earlier
pass does not qualify these later changes or the whole current product.

## Journal integrity follow-up

Further review reproduced two independent audit defects before changing code.
Eleven malformed-proof variants were accepted (invalid digest encodings,
endpoint identity, timestamps, missing resolver/epoch and resolution fields on
unresolved state). Reconciliation of an already committed historical record
after a newer publication rewrote the old receipt as a conflict and blocked
the current policy. The unchanged regressions and failure output are preserved
in `internal/server/inference_endpoint_owner_journal_integrity_test.go` and
`build/endpoint-owner-journal-before-20261121.log` (FAIL, 2.008 s).

The journal reader now validates lowercase digest encodings, endpoint identity,
RFC3339 timestamps, bounded actor identities and state-consistent resolution
fields. Malformed evidence blocks admission and remains on disk. Reconciliation
returns an already committed/not-applied receipt unchanged; later file contents
cannot rewrite a historical outcome. The renderer also rejects contradictory
resolution fields. Focused backend checks pass, 3.851 s, in
`build/endpoint-owner-journal-fixed-20261121.log`.

Full affected normal/race checks are repeated for this source, separately
from the earlier passes above. The new 4,147-file snapshot has SHA-256
`663b67bb91d20a1d7b5d49fbbf357caebbdccf57f1f2c29738187e1fb18e7e4f`;
its manifest is `build/endpoint-owner-journal-checkpoint-20261121-manifest.json`.
Windows normal checks passed: server 251.574 s, API 1.974 s, configuration
2.027 s (`build/endpoint-owner-journal-full-windows-20261121.log`). All 20 client
and endpoint renderer checks passed in 56.1 s
(`build/endpoint-owner-journal-renderer-20261121.log`). Rebuilt service plus
production renderer passed the actual creation/update/recovery journey in 8.5 s
(body 7.4 s), with private fixture
`C:\Users\phil\AppData\Local\offgrid-owner-qualification-d7bc1b0276ce47a2a56d88af6b9832bf`
and log `build/endpoint-owner-journal-service-20261121.log`. API generation,
production build, 488-document check, source-size and strict OpenSpec checks
also passed. The repeated full Linux server race run **failed its eight-minute
package deadline** at 480.087 s. API/configuration race packages passed at
1.070/1.089 s. At timeout, `TestAutomaticDependentWorkerValidatesSealedEdits`
had run for 36 s and its `lost_integration_result` subtest for 6 s; the executing
goroutine was in modernc SQLite database opening. There was no preceding failed
assertion or data-race report in this output. This establishes an aggregate
deadline failure, not a proven deadlock or a proven explanation for the slowdown.
The full dump remains in `build/endpoint-owner-journal-full-linux-race-20261121.log`.

An identical byte-verified source rerun used `go test -race -json` with the same
eight-minute deadline and no competing agent build. It records per-test timings
in `build/endpoint-owner-journal-timed-linux-race-20261121.log`. It also **failed
the eight-minute package deadline**, 480.053 s. There were 1,126 passing
test/subtest events across the invocation and no failed assertion/data-race event
before the timeout. API/configuration passed, 1.077/1.029 s. The earlier active
`lost_integration_result` case passed in 7.95 s; the later `integrated_followup`
case was active when the package budget expired. The timing summary is retained
at `build/endpoint-owner-journal-timed-summary-20261121.json`. This is repeatable
aggregate runtime failure, not a full-package pass or a proven specific hang.
WSL restarted between invocations; Docker services were observed using
CPU during startup. Host contention is a hypothesis, not a certified root cause,
and no system service was stopped to force a pass. Neither timeouts nor assertions
were relaxed.

A separately recorded focused owner/management/policy/system/worker race run
passed with the same source and eight-minute bound: 71.227 s, 47 passing
test/subtest events, no failures or skips. The complete dependent-worker test
passed in 65.83 s, including `integrated_followup` (14.17 s) and
`lost_integration_result` (7.85 s). Output is retained in
`build/endpoint-owner-journal-focused-linux-race-20261121.log`. This focused
pass cannot replace the failed aggregate check. The full current Linux server
race/performance gate remains open; the successful earlier source checkpoint
does not erase these two failures.

Before this follow-up, an additional actual service run added a stale-control
assertion and retained review/restart screenshots: 1/1 PASS, 3.8 s including
startup (body 3.2 s), `build/endpoint-owner-service-final-20261121.log`. The
screenshots were visually inspected. This run does not substitute for rebuilding
and checking the later journal fix.

## Runnable isolation procedure

```powershell
go build -o build/offgrid-endpoint-owner-fixture.exe ./cmd/offgrid
npm.cmd --prefix apps/web run build
node scripts/test/test-web-workspace.mjs build/offgrid-endpoint-owner-fixture.exe --endpoint-owner-fixtures tests/endpoint-owner-service.spec.ts --workers=1
```

The wrapper creates its own state and can restart only its own child. It never
falls back to an installed service. Real model/runtime qualification remains a
separate procedure in the [provider guide](../../advanced/http-inference-endpoints.md).

## Remaining work and qualification

Task 8.4.4 stays open. Durable history and endpoint decision stores have bounded
capacity (64 records/decisions); a verified archival/maintenance lifecycle is
still needed. Do not clear records, erase tombstones or raise limits to conceal
that gap. Finish damaged-file/journal recovery usability, remaining journal
integrity review, managed-owner provisioned-runtime journeys, cross-platform
installed qualification, independent security review and final combined CI.
The remote form and synthetic template test do not qualify managed launch.

All 29 catalogs contain authored translations; native language reviewers and
consequential-copy approval are unavailable and remain open. Browser tests and
visual inspection do not establish accessibility or participant qualification.
Real Ollama/vLLM model text/tools/cancellation/offline/resource, local vLLM,
macOS, matching hardware, frozen workflows, soak and Nativ comparisons remain
separate incomplete roadmap requirements. No task checkbox was completed here.
