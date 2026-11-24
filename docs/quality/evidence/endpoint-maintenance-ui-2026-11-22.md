# Endpoint maintenance controls — 2026-11-22

This continues task 8.4.4 and M5.04 after the
[policy/receipt maintenance backend](endpoint-policy-maintenance-2026-11-22.md).
It does not complete the owner administration task or transformation program.

## Behavior

Models → Configure endpoints now offers **Archive settled receipts** and
**Retire obsolete decisions** when the server supplies eligible, bounded state.
Each action opens a native modal containing the actor and exact receipt/identity
list. Cancelling or pressing Escape performs no mutation and restores focus.
Confirmation uses the captured configuration plus journal/policy revisions;
it cannot silently substitute a newer review while the dialog is open.

The shared panel operation lock prevents simultaneous submissions. The local
review is consumed before the POST. A failure or lost acknowledgement closes
that review, displays uncertainty and refreshes actual state without retrying.
Current access controls refresh too, including an unavailable policy after an
uncertain write. Empty, unsettled, unsupported and read-only snapshots do not
offer an eligible maintenance action.

The identity list wraps long values, uses bidirectional isolation and provides
keyboard focus for scrolling a full 64-identity review. Author-written strings
cover all 29 supported locale catalogs. Automated catalog checks do not certify
native-language accuracy or replace human accessibility testing.

## Executed checks

- Client and rendered owner tests: 20 PASS in 31.6 s,
  `build/endpoint-maintenance-ui-20261122.log`. New cases verify exact archive
  request binding, cancellation/focus restoration, absent preview controls,
  lost retirement response, consumed review and no repeat after Refresh. Existing
  owner create/update, credential, managed-template, stale review, authorization
  and Arabic dialog checks also passed.
- Production renderer with the freshly built actual service: 1 PASS in 6.8 s
  (body 6.0 s), `build/endpoint-maintenance-ui-service-20261122.log`. Both
  maintenance actions now use the rendered review and confirmation buttons.
  The journey restarts after each action, verifies original duplicate receipts,
  exact archived policy, unchanged configuration and no implicit activation.
- A visual-evidence run passed in 7.0 s (body 6.2 s),
  `build/endpoint-maintenance-service-visual-20261122.log`, producing actual
  archive/retirement dialog screenshots under
  `build/endpoint-maintenance-service-visual-results`. Retirement review was
  visually inspected. Disposable workspace:
  `C:\Users\phil\AppData\Local\offgrid-owner-qualification-45ed928c3984488fb1cf01455f396277`.
- Narrow English, Arabic and Simplified Chinese 390×844 checks each rendered 64
  maximum-length identities and passed viewport/overflow/focus checks: 3 PASS,
  9.9 s, `build/endpoint-maintenance-layout-20261122.log`. Arabic and Chinese
  screenshots were visually inspected. A subsequent focused run adds keyboard
  End navigation to the final identity: 3 PASS in 8.9 s,
  `build/endpoint-maintenance-layout-final-20261122.log`. All three lists
  reached their final identity by keyboard and restored opener focus on Escape.
- TypeScript, generated API consistency and 29-locale translation checks passed.
  Translation output is in `build/endpoint-maintenance-ui-i18n-20261122.log`.
  The production build passed in `build/endpoint-maintenance-ui-build-20261122.log`;
  existing large-bundle warnings remain. The final build including list focus
  also passed, `build/endpoint-maintenance-ui-final-build-20261122.log`.
  Source-size validation passed.

The final list-focus attribute was added after the actual-service run. It does
not alter API behavior; the narrow-layout test exercises its keyboard behavior.

## Still incomplete

Archived receipt/policy browsing and export, damaged-state owner recovery,
installed desktop/platform qualification and independent security review remain
open. Native-language reviewers, assistive-technology users and real model/
hardware workflow qualification remain external prerequisites. The earlier full
Linux server race deadline failure is still open; these UI tests do not replace
that gate. No task checkbox, additional commit, push or release is implied.
