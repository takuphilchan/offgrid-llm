# Browser workspace consent evidence

Recorded 2026-11-20 against the uncommitted `browser-task-workspaces` increment
on base commit `b19c658736d5f5f000710c6c608644564cc88cd4`, Windows, Node.js
24.18.0, and the repository's existing desktop and Playwright dependencies.
This record covers the renderer/native consent portion of OpenSpec task 2.2.
It does not certify the complete browser increment, M2, or the transformation.

## Implemented contract

- The task access form accepts up to fifteen additional public HTTPS origins.
  Its presentation helper normalizes bare URLs; the desktop independently
  requires canonical exact origins, rejects duplicates and malformed scope,
  and bounds each canonical origin to 512 UTF-8 bytes. The initial URL supplies
  the first origin. Neither task text nor model output adds sites automatically.
- The native confirmation displays the exact approved origin set and either
  one tab or the negotiated eight-tab limit. Expanded scope requires the
  service's `browser-workspace-v1` identity capability. A legacy service rejects
  expanded scope before consent, pairing or child creation; its previous
  single-origin path remains available.
- The desktop snapshots scope before asynchronous consent, then verifies the
  service, workspace, principal and tab capability still match. Its worker
  message carries an internal `expectedBrowserWorkspace` pin so the companion
  can reject a subsequent capability change. Renderer IPC cannot supply that
  internal field. Companion startup enforcement has separate owner evidence.
- All 29 supported locale dictionaries contain the new field/help, errors,
  native consent and four tab-tool labels. New wording has not received human
  language review and no review attestation was fabricated.

Implementation is in `apps/desktop/computer-runtime.js`, `main.js`,
`apps/web/src/features/agents/TaskAccess.tsx`, `browser-scope.ts`, the platform
bridge type, and existing locale catalogs. The user-facing behavior is described
in [Computer tasks](../../guides/computer-tasks.md) and
[desktop documentation](../../../apps/desktop/README.md).

## Executed checks

| Command | Actual result and scope |
|---|---|
| `node --test apps/desktop/test/computer-runtime.test.js` | 29 passed, zero skipped. Includes canonical/Unicode/local/numeric-IP/path/credential/oversized origin cases, immutable pending consent, old service rejection, untrusted IPC field rejection, internal worker pin, changed service/workspace/principal/capability and all locale keys. Repeated after the worker pin and byte bound changes. |
| `npm.cmd --prefix apps/desktop test` | 239 passed, one existing symlink-fixture skip, zero failures, 240 total. This full run preceded the final worker pin and byte bound additions; the affected 29-test suite passed afterward. |
| `npm.cmd --prefix apps/web run test:e2e -- browser-workspace-access.spec.ts --workers=1` with `OFFGRID_FIXTURE_PORT=5319` | Five passed. Isolated mock service and desktop bridge: explicit scope forwarding/one attach, invalid scope and legacy rejection, Arabic and Simplified Chinese narrow layouts, labels/help and keyboard focus. |
| Same renderer command with `--grep 'additional origin presentation'` | One passed after the 512-byte constraint was added; validates the affected pure helper without treating fixtures as a real browser workflow qualification. |
| `npm.cmd --prefix apps/web run check` | Passed after final UI changes. |
| `npm.cmd --prefix apps/web run i18n:check` | Passed all 29 locales; 169 catalogs and 1,564 authored messages, no missing preview-locale messages. Catalog completeness does not establish translation quality. |
| `npm.cmd --prefix apps/web run i18n:consistency` | Exit zero; no shared navigation conflicts. Existing contextual wording variants remain reported. |
| `node scripts/maintenance/check-source-size.mjs` | Clean after final UI changes. |
| `git -c core.safecrlf=false diff --check` | Passed after final UI changes. |

The first renderer attempt exposed an ESM JSON import missing its import
attribute; the actual source import was corrected. The next attempt exposed
an accessible label that unintentionally included the entire hint. The field
now uses a concise `aria-labelledby` label and separate `aria-describedby`
help; the same accessible-name tests then passed. Electron's actual wrapped
IPC error shape was added to the regression fixture so legacy-service rejection
preserves the draft and displays the specific translated error.

Playwright screenshots for Arabic and Simplified Chinese at 390 by 740 were
visually inspected: no horizontal overflow or clipped form fields, visible
keyboard focus, localized instructions, and left-to-right URL entry. These are
ephemeral test artifacts, not retained independent usability evidence. Existing
Vite native-config import-extension warnings remained. No live installation,
personal browser, model weights or signed-in account was used.

## Remaining acceptance evidence

The parent increment must integrate and verify the CLI, companion capability
pin, transport and service origin policy, then run its complete applicable CI
and strict OpenSpec validation. This UI slice alone cannot close task 2.2 or
3.1. Real-model planning/navigation, packaged desktop/native dialog behavior,
Chrome/Edge signed-in attachment, Windows/macOS/Linux qualification, assistive
technology testing and human locale review remain separate required evidence.
The isolated renderer proves consent presentation and request boundaries; it
does not prove browser task usefulness or a user's understanding of consent.
