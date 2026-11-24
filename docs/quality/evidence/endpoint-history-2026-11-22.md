# Endpoint history browsing and page export — 2026-11-22

This continues M5.04 and `inference-provider-runtimes` task 8.4.4 after
[maintenance controls](endpoint-maintenance-ui-2026-11-22.md). It implements
read-only receipt/policy browsing and explicit JSON export of the displayed
page. It does not complete owner recovery, installed qualification or the program.

## Boundaries and behavior

`GET /api/v2/inference-endpoints/configuration/history` returns at most 32
validated immutable receipts in lexical request-hash order. A continuation
contains the last hash and original journal revision. A changed journal rejects
the continuation instead of silently mixing snapshots. Refresh starts a new
first page. Copies still represented by active journal records are excluded,
including copies retained after an interrupted archival publication.

The implementation reads directory names in batches of 128 and retains only
33 candidates, including look-ahead. It scans at most 200,002 root directory
entries, matching the existing backup path-entry scale; exceeding that bound
returns an error, never a partial page reported as complete. It opens and
validates only the chosen receipt files, with the existing 4 KiB limit per file.
Invalid names, identity/hash mismatch, archive mismatch, oversized or nonregular
receipts fail closed. Query strings are limited to 512 bytes; unknown, duplicate
and malformed fields are rejected. Directory enumeration order is not treated
as stable order. The batch behavior is grounded in
[Go's Readdirnames contract](https://pkg.go.dev/os#File.Readdirnames), checked
on 2026-11-22. Filesystem calls themselves do not gain a hard wall-clock deadline.

`GET /api/v2/inference-endpoints/configuration/policy-history` reads one bounded
immutable historical snapshot. Omit `revision` to inspect the current policy's
immediate predecessor, or supply a verified historical revision. A snapshot's
`previous_snapshot` links to the preceding page. A missing predecessor yields
null; a missing/corrupt referenced file yields an error. There is no recursive
history scan and no historical-grant activation path. Explicit revision lookup
can inspect a retained snapshot even when live-policy admission is fenced.

Both routes require a current administrator and recheck authority before
disclosure. They perform no configuration, journal, policy or archive writes.
The existing archive format remains authoritative; no database/index migration
or new writer is introduced. The existing private workspace/same-account trust
boundary still applies; this is not a tamper-proof external audit service.

Models → Configure endpoints exposes **Archived receipts** and **Policy history**
only when the service advertises the history routes. Reads cancel on stale
panel state/unmount. Historical access is labeled as allowed/denied **in that
snapshot**, with a visible statement that evidence never grants current access.
The client validates receipt ordering, cursor binding, identities, historical
policy fields and predecessor shape. Failures clear the page/export action;
neither route retries automatically. All 29 catalogs have authored copy; human
language and assistive-technology qualification remain open.

**Export this page** downloads the validated displayed page with observation
time, exporting actor, evidence-only marker, original records and revision/
archive provenance. It exports one receipt page or policy snapshot, not an
unbounded all-history download. Use continuation for further pages. Whole
workspace backups already retain the underlying audit files. Page exports are
not signed attestations, backup files, import commands or activation authority.

## Actual checks

- Focused history tests plus the 66-publication workflow: PASS 14.776 s,
  `build/endpoint-history-focused-20261122.log`. The real workflow browses all
  64 archived publication receipts after restart. Separate synthetic validated
  journal batches exercise 70 receipts across three pages, lexical ordering,
  stale continuation, no writes, active-copy exclusion, corruption and invalid
  queries/cancellation. They are metadata tests, not model qualification.
- The additional current-administrator test passed in 0.748 s; ordinary users
  receive 403 for both history routes.
- Typed history and owner client contracts: 11 PASS, 1.5 s,
  `build/endpoint-history-contract-20261122.log`. They reject reordered,
  duplicated, mismatched or forged pages, incompatible policy metadata and
  unexpected null lookup results without an automatic retry.
- Rendered changed-journal/explicit-refresh test: 1 PASS, 4.6 s (body 2.1 s),
  `build/endpoint-history-ui-20261122.log`. A failed continuation disables
  export, then Refresh makes an unbound fresh first read with no POST.
- Fresh production service and renderer: 1 PASS, 7.1 s (body 6.4 s),
  `build/endpoint-history-service-20261122.log`. The create/update/lost-response/
  reconcile/restart/maintenance journey now browses archived receipts and policy,
  downloads both JSON page types and compares them with original records and the
  actual retained policy. Configuration remains unchanged and access still
  requires review. Its rendered policy-history screenshot was visually inspected.
  Disposable workspace:
  `C:\Users\phil\AppData\Local\offgrid-owner-qualification-a006eca469ad44fb9cd39d9be04465d2`.
- Full Windows server/API/configuration: PASS 139.868 / 1.528 / 1.581 s,
  `build/endpoint-history-full-windows-20261122.log`.
- Linux endpoint race checks: PASS 16.728 s,
  `build/endpoint-history-linux-race-20261122.log`. Source snapshot: 4,167
  byte-verified maintained files, SHA-256
  `d21677d54bb039a9bf077cfd7b109bbd657655bf0879bbc2270bad516009f6fd`.
  Go 1.26.6, CGO enabled, GOPROXY off, GOMAXPROCS=4, fresh ext4 state.
- TypeScript, generated API consistency, source-size ratchet, strict OpenSpec,
  29-locale catalog checks and production renderer build passed. Logs include
  `build/endpoint-history-i18n-20261122.log` and
  `build/endpoint-history-web-build-20261122.log`. Existing bundle-size/native
  Vite-loader warnings remain recorded.

## Remaining acceptance

Task 8.4.4 remains unchecked for damaged-state recovery and broader installed,
managed-runtime, independent/human qualification. Real models/providers/hardware,
power-loss/storage behavior and full workflow qualification remain separate.
The previously failing full Linux server race deadline remains unresolved;
the focused race result does not replace it. No additional commit, push, publish
or release was performed.

Subsequent evidence: the [full server/CI race recheck](server-race-recheck-2026-11-22.md)
passed on the follow-up checkpoint. It does not erase earlier failures or qualify
skipped runtime cases. A final client review also added explicit rejection of a
non-object historical decision map; all three history contract tests passed in
1.4 s, `build/endpoint-history-contract-final-20261122.log`.
The final combined owner/history client/UI suite then passed all 28 tests in
47.1 s with a successful production build, recorded in the recheck link above.
