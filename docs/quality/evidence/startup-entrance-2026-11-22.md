# Compact startup entrance — 22 November 2026

The owner rejected the expanded stack, glow, repeated greeting and visible timer,
then requested that animation remain and the recovery divider be removed.
This increment implements that direction in the isolated UI checkout. It does
not change the service controller, process ownership, authentication or native
startup-only IPC authorization.

## Result

- A 38px intact OffGrid mark sits beside the wordmark, with one localized status.
  The complete mark changes opacity over 2.8 seconds; its paths never separate.
- Native and renderer loading share layout and palette. Neither loads the legacy
  greeting carousel nor displays a seconds counter. No artificial delay or progress
  percentage was added.
- App and system reduced-motion preferences disable animation. Recovery also
  stops animation and retains readable reasons and explicit actions.
- Failed initial access stays in the same entrance. Retry repeats the existing
  initial reads; successful readiness removes the gate. A stalled bundle offers
  reload after 30 seconds while the app remains inert.
- Native recovery uses the same brand and flat surface. Technical details remain
  expandable without a horizontal divider. Existing local recovery, browser and
  help controls retain their authorization boundaries.
- Help, Workspace access and Settings search fixes are included in the renderer;
  their earlier evidence is in [the layout record](help-workspace-layout-2026-11-22.md).

## Actual verification

All logs are retained under `build/ui-completion` in the isolated checkout.

| Check | Actual result |
| --- | --- |
| `npm --prefix apps/web run build` | TypeScript and production build passed |
| `startup-readiness.spec.ts` | 13 passed: readiness, failed access/retry, failed bundle, motion, 29 locales, narrow/short/forced colors and transcript focus |
| `desktop-startup-craft.spec.ts desktop-surface-polish.spec.ts` | 15 passed |
| Final no-divider startup cases | 2 passed |
| `npm --prefix apps/desktop test` | 290 passed, 4 skipped |
| `test-desktop-startup-entrance.mjs` | 3 staged Windows cases passed: English light/dark at 100%, Arabic dark at 200%; native/renderer loading geometry, recovery, retry and actual workspace reveal |
| `test-desktop-help-panels.mjs` | 3 combined Windows cases passed: Help, Workspace, Settings result navigation and retained draft |
| Production responsiveness comparison | 1 passed; baseline/candidate p95 29.6/29.8ms, heap 12,828,152/13,273,912 bytes, 180/180 requests; existing thresholds unchanged |
| API schema, translation inventory, source-size checks | Passed; linguistic review remains separate |
| OpenSpec strict validation and maintained docs | Passed |

Native profiles use synthetic HTTP responses, no permission grants, commands,
personal data or service writes. Native busy presentation is held using a synthetic
host event against the actual packaged assets; retry/navigation and renderer
readiness use the real packaged controller. The 200% test reapplies stress zoom
after navigation; it does not claim DevTools zoom persistence. Slow states are
fixtures, not measurements of real startup duration.

Final startup captures and results:
`C:/Users/phil/AppData/Local/Temp/offgrid-startup-entrance-QlUT9S`.
Combined Help evidence:
`C:/Users/phil/AppData/Local/Temp/offgrid-help-panels-BWZ5GV`.
Stable preview copies are `build/ui-completion/startup-preview-light.png`,
`startup-preview-dark.png` and `startup-recovery-preview.png`.

Iteration history is retained: the first browser run needed a JSON import
attribute, the first hidden native capture had no compositor surface, and an
early 200% comparison crossed a navigation that reset test zoom. These were
corrected before the final passes. Vite development still reports its public JSON
import warning for the shared presentation catalog; the production build and
actual packaged locale/retry checks passed. Existing Vite configuration and
large-chunk advisories also remain.

## Staged identity and delivery

Candidate: `C:/Users/phil/AppData/Local/Temp/offgrid-ui-completion-package-7qnTMR/OffGrid`.

- Native archive SHA-256:
  `cb64f41bf064a3d5a123784c659bed68bed03097d349b0faf2262786a8ec62d4`.
- Renderer index SHA-256:
  `15a95ea235d3cfb02ec05d67c9ea00f9339bd4041b5fc767213a68010e6c602b`.
- Only `loading.html`, `startup.css`, `startup-surface.css` and `startup.js`
  differ from the installed native archive; 412 other native files and 166
  unpacked files match. All 67 renderer files match the production output.
- The desktop executable, service and document/OCR workers retain their verified
  installed hashes. No unfinished native backend/recovery implementation is copied.

The candidate was installed on **23 November at 22:18 China time**, after the
installed app was confirmed idle. The guarded installer verified the exact pair,
preserved all runtime hashes and changed no user data or active processes.
Rollback: `D:/offgrid-ui-20261121/build/ui-install/rollback-startup-20261123-221802`.
Actual installed startup/recovery/handoff checks passed in disposable profile
`offgrid-startup-entrance-7o37qA`; installed Help, Workspace and Settings navigation
checks passed in `offgrid-help-panels-kQBMMx`. Both include enlarged RTL coverage.
The installed loading capture was visually inspected, and all 67 installed
renderer files were matched to the qualified manifest. The finalized receipt is
`build/ui-completion/installation-startup.json` in the isolated checkout.

The owner subsequently authorized synchronizing the UI work into local `main`.
The original snapshot was used for a three-way merge, preserving the other
thread's newer voice and endpoint-owner behavior. No commit, push or publication
was performed. Main integration evidence is recorded separately from this
installed presentation package; the installed service and native backend remain
the earlier verified binaries.

Full repository CI, real models, other operating systems, physical assistive
technology and independent visual/linguistic review were not run in this increment.
This is scoped implementation evidence, not full-program release qualification.
