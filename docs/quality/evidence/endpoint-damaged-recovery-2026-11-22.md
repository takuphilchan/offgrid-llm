# Endpoint damaged-state recovery — 2026-11-22

This continues task 8.4.4/M5.04 using the existing
[workspace recovery](../../advanced/workspace-recovery.md) architecture. It does
not introduce an endpoint reset, overwrite a damaged workspace or reconstruct
uncertain history from guesses.

## Implemented and exercised path

Configure endpoints links to **Workspace recovery**, including when the owner
file cannot be read. The existing Settings route selects the storage/recovery
section. The browser edition explains the host-local recovery boundary; the
desktop uses its existing trusted host bridge. Opening that route performs no
backup, stop, restore, publication or access change by itself.

Recovery uses a verified known-good workspace backup and a new target directory,
leaving the damaged source intact. Restored receipt archives, policy history and
original request identities remain evidence. A fresh workspace activation epoch
prevents the backup's access grants becoming current. Historical duplicate
requests return their original receipts and cannot activate an endpoint. The
restored workspace requires a new explicit administrator access review.

Tests deliberately damage six separate files: owner configuration, active
journal, live policy, receipt archive identity, an archived receipt and historical
policy. Each scenario runs with both plain and X25519-encrypted backups. Damage
is reported instead of normalized; damaged admission metadata blocks endpoint
selection. An isolated damaged historical receipt blocks its historical read,
not unrelated current authority. Restore compares all six recovered files with
the verified backup, preserves all original source bytes including damage, and
checks audit browsing, duplicate proof and fresh-review semantics.

## Actual evidence

- Initial 12 Windows recovery cases passed in 5.573 s,
  `build/endpoint-damaged-recovery-20261122.log`. Expanded cases additionally
  verify healthy selection before damage and refusal after admission metadata
  damage: PASS 5.225 s,
  `build/endpoint-damaged-recovery-admission-20261122.log`.
- Linux race run of all 12 expanded cases: PASS 1.404 s, first result in
  `build/endpoint-damaged-and-full-linux-race-20261122.log`. The verified
  4,169-file source archive SHA-256 is
  `bb6d2e87b53a7024979d506b12af3a8b39ac762cce5958432dc0dedf876ca8b5`.
  Go 1.26.6, CGO enabled, GOMAXPROCS=4, GOPROXY off, fresh ext4 test state.
- Rendered unreadable-owner-file recovery navigation: 1 PASS, 3.7 s (body
  1.7 s), `build/endpoint-damaged-recovery-ui-20261122.log`. The owner file
  response is an explicit 409 fixture; creation stays disabled, the recovery
  heading is reachable and the journey sends no POST.
- The production-renderer/fresh-service journey then passed with actual deliberate
  journal damage in its disposable workspace: 1 PASS, 7.6 s (body 6.9 s),
  `build/endpoint-damaged-service-20261122.log`. After browsing/export, the test
  truncates only the fixture journal, reloads, verifies creation is blocked,
  opens Workspace recovery and confirms the damaged bytes were not reset.
  Its fixture is
  `C:\Users\phil\AppData\Local\offgrid-owner-qualification-a3e0a17d69b34b628bb77c4d5b3a3679`.
  This is actual service-to-renderer discovery, not native host-dialog execution.
  The production build passed in `build/endpoint-damaged-ui-build-20261122.log`.
- TypeScript and source-size checks passed. The prior full Windows and focused
  Linux endpoint checks are recorded in [history evidence](endpoint-history-2026-11-22.md).
  This follow-up changes navigation and adds recovery tests; it does not change
  the backup engine or backend admission implementation.

A separately instrumented full Linux server race run follows the focused test,
with the original eight-minute deadline and per-test JSON/resource output in
`build/endpoint-history-full-linux-race-20261122.jsonl` and
`build/endpoint-history-full-linux-race-20261122-resources.txt`. Its result is not
implied by the focused recovery pass. It subsequently passed in 266.212 s, and
the complete 22-package CI race set passed too; see the
[scoped recheck record](server-race-recheck-2026-11-22.md), which retains the
earlier timeout failures and distinguishes 85 skipped qualification/helper cases.

## Operational procedure and remaining qualification

Preserve the damaged workspace, its external owner configuration if applicable,
and available backups. Use Settings → Workspace recovery on an eligible desktop,
or the documented host-local CLI, to verify a matching-version backup and restore
to a new folder. Inspect the restored records before explicitly selecting that
workspace and reviewing access. Preserve newer data in the original workspace;
restoring an older backup does not merge later changes.

Without a usable backup, do not delete a damaged journal/policy/archive identity
to manufacture an empty healthy state. Missing evidence cannot be reconstructed
with certainty. Retain originals and available exports for investigation. A
permissive endpoint reset would violate no-replay and preserved-history
requirements and is not provided.

Real native file-picker interactions, installed desktop/platform recovery,
external owner-file restoration, independent security review, power-loss/storage
behavior and human usability/accessibility remain separate acceptance gates.
These isolated storage and renderer results do not complete task 8.4.4, M5 or
the full program.
