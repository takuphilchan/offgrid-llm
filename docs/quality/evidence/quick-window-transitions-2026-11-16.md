# Quick window presentation transitions — 16 November 2026

## Repair

Quick text, voice and full-window changes previously updated native bounds and
renderer layout independently. A resize could expose a layout intended for the
other window size; separate entrance animations added a second movement.

The existing trusted window now coordinates a short fade, bounded native resize
and final reveal. Native navigation waits for a token-bound acknowledgment from
the trusted renderer, with a 500 ms fallback. Windows resize easing lasts 180 ms;
outgoing and incoming content use 70 ms and 120 ms fades. There is no second
renderer, saved screenshot or executor. The selected theme supplies the backdrop.

Reversal, dismissal and window replacement cancel stale visual work. Repeated
expansion waits for its final bounds. Reopening after an interrupted resize fits
the requested presentation. Full-workspace geometry is retained separately from
intermediate native bounds. System and app reduced motion skip interpolation;
hidden/minimized windows do not animate. Capture cancellation is independent of
the visual transition and happens immediately when leaving Talk.

## Checks run

- Desktop Node suite: **230 passed, one skipped, zero failures**. Includes eased
  geometry, reversal, repeated expansion, dismissal, destruction, unavailable
  renderer, acknowledgment validation and both reduced-motion paths.
- Renderer command-window and voice suites: **69 passed, two opt-in screenshot
  captures skipped**. Existing draft, voice lifecycle, focus, theme and RTL checks
  remain green.
- Three additional renderer regressions passed for normal motion, system reduced
  motion and app reduced motion. They hold the native resize reply to verify that
  the new surface is not revealed early, the chat backdrop stays hidden in voice,
  leaving voice stops capture before the resize completes, and saved messages
  remain unchanged. Synthetic microphone input only; no work submitted.
- Hidden native main/preload/renderer integration passed. One window retained
  its draft, acknowledged presentation changes, interpolated eight observed
  native bounds frames in this run, reversed correctly and restored full bounds.
  Reduced motion used a single bounds update. No request was submitted.
- Native capture inspected after the reveal and final compositor paint. An
  initial capture taken before that paint was blank; the capture harness now
  waits for the transition marker to clear, visible opacity, in-viewport input
  and the final paint. The retained compact capture shows the draft and controls.
- TypeScript checking and production Vite build passed.
- The same native transition harness passed against the actual packaged
  executable and its bundled UI. Nine interpolated frames were observed in that
  run. Packaged caption/startup checks passed at 100%, 150% and 200% zoom under
  the strict startup content-security policy; windows remained hidden.

Commands: `npm --prefix apps/desktop test`,
`npm --prefix apps/web run test:e2e -- desktop-command.spec.ts voice-playback.spec.ts --workers=2`,
`npm --prefix apps/web run test:e2e -- voice-playback.spec.ts --grep "quick text and voice"`,
`node scripts/test/test-desktop-command.mjs`, and
`npm --prefix apps/web run build`.

Local logs are under `build/desktop-motion-final-tests.log`,
`build/ui-quick-transitions.log`, `build/ui-quick-motion-gates.log`,
`build/native-command-motion.log` and `build/desktop-transition-preview.log`.
Packaged checks are recorded in `build/packaged-command-motion.log` and
`build/packaged-motion-chrome.log`.

## Local candidate

`0.4.32-preview.20261116072217571.g9adc304f`, built from the dirty working tree at
`9adc304fbab6a21b29f36c22f6d88e0d8bbe7dca`. The local preview builder increments the
base package version; this is an unsigned local candidate, not a published
0.4.32 release. Installer and portable files are under
`build/desktop-0.4.32-preview.20261116072217571.g9adc304f`. Packaging checked the
service version, UI identity and required resources. Signature inspection reports
the candidate as not release-qualified.

## Limits

Native checks use isolated HTTP fixtures and temporary profiles. Show/focus are
intercepted and visibility is simulated only for animation eligibility; windows
remain hidden. This does not establish OS shortcut delivery, physical microphone
quality, real inference, perceived motion on every display, or release readiness.
Broader release-suite failures and language/hardware qualification remain tracked
separately. Local installation and public release are separate steps.

## Installation follow-up
The preview installer completed and the installed payload matched the candidate hashes. The owner reopened OffGrid during the final unchanged-profile check, so browser cache changes prevented that assertion and the installer script did not emit its final receipt. Rollback is retained at `D:/offgrid-install-backups/quick-transitions-20261116`. Subsequent startup/scroll work will use a new backup and installation receipt; no public release was performed.
