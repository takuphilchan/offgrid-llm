# Sidebar and attachment presentation, 2026-11-06

This is a local Windows preview of the current terminal-enabled application.
The reference supplied by the owner is Codex, not a screenshot of OffGrid.
It is used for navigation hierarchy, not as evidence of OffGrid behavior.

## Changes

- Separate global navigation into a narrow rail and put conversation browsing
  in its own panel. Keep a visible New chat action, readable history titles,
  independent scrolling and rail-bottom workspace settings. Other routes use
  the compact navigation rail. Labels remain available to keyboard, pointer
  and assistive technology; narrow layouts use the existing modal drawer.
- Center the blank Chat heading and editor together. Retain a bottom editor
  when reading messages or inspecting attachments. Remove the decorative
  welcome icon and storage explanation from the blank conversation.
- Inspect image attachments in a local fit/actual-size modal without changing
  bytes, sending a request or switching models. Preserve removal, Escape,
  focus restoration, multilingual names and narrow-screen behavior.
- Separate document identity, coverage and actions. Use a compact reader
  toolbar, committed page-number edits, page source links and independent
  original-download errors. Rendering or OCR success is not inferred from UI.
- Show actual microphone level history, elapsed capture time and transcription
  state. Retain cancellation, draft preservation and reduced-motion behavior.

## Automated evidence

The full built renderer and isolated service suite passed 1,061 tests with 31
skipped before the final rail change. The final rail then passed 69 focused
navigation, history/recovery, localization and journey checks. A further six
shell checks passed after preserving New chat in collapsed navigation.

```
npm.cmd --prefix apps/web run build
npm.cmd --prefix apps/web run api:check
go build -o build/ui-regression-offgrid.exe ./cmd/offgrid
node scripts/test/test-web-workspace.mjs build/ui-regression-offgrid.exe --workers=3
node scripts/test/test-web-workspace.mjs build/ui-regression-offgrid.exe workspace-shell.spec.ts workspace-order.spec.ts workspace-feedback.spec.ts workspace-baseline.spec.ts workspace-journeys.spec.ts workspace-craft.spec.ts work-continuity.spec.ts --workers=3
node scripts/test/test-web-workspace.mjs build/ui-regression-offgrid.exe workspace-shell.spec.ts --workers=2
npm.cmd --prefix apps/desktop test
node scripts/maintenance/check-docs.mjs
node scripts/maintenance/check-source-size.mjs
node scripts/maintenance/check-repository-hygiene.mjs
openspec validate workspace-outcome-experience --strict
```

Desktop unit tests: 129 passed, one platform skip. Documentation, source-size,
repository hygiene and OpenSpec checks passed. The first rail run caught a
mobile drawer visibility regression and an obsolete label-height assertion.
The drawer was corrected, and the label test now checks keyboard visibility
and viewport placement. A subsequent focus check exposed the distinction
between programmatic focus and `:focus-visible`; rail labels now appear for
both keyboard and explicit focus. Failed logs remain in ignored local build
evidence; the later passing results do not erase them.

Reviewed screenshots cover populated history in both themes, 900px and 480px
heights, mobile RTL, blank Chat, full-size image inspection and document reading.
The history fixture contains 80 test conversations. These are isolated fixtures,
not user records or a measurement of real-model answer quality.

## Qualification boundaries

Real speech accuracy, human usability and screen-reader evaluation are not
established by these tests. macOS/Linux installed UI qualification is not
claimed. The final rail package was independently tested and installed; see below.

## Final rail package and installation

The packaged Windows theme and terminal suites passed, including theme/locale,
200% zoom, CLI/Agent entry, cursor/tab retention, `/clear`, and descendant cleanup.
The final rail installer completed successfully. Five installed payloads match
the staged build. All 382 root-data files and 10 desktop-workspace data files
remain unchanged, as do the model inventory and native inference runtimes.
Installer SHA-256: `A82240FB4776BFCC7B6BCF1A15BA20D570FAACC147DAA72A68AE4A3C352EA240`.
Rollback: `D:/OffGrid-install-backups/v0.4.26-before-conversation-rail-20261106-144439`.
This is a local 0.4.26 preview, not a published release.

## Document interaction follow-up

Reopening the same attachment now retains its current page and zoom. A new
citation or different attachment still navigates deliberately. The focused web
suite passed 91 tests with one native-service-only skip (document reader, image
attachments, voice playback, workspace shell and ordering). API generation,
source-size, maintained-link and diff-whitespace checks passed. The web build
passed with the existing large-chunk warning. The first packaging invocation used
the wrong working directory and was rejected before packaging; the corrected
invocation uses `apps/desktop`, with the verified cached Electron archive.

### Follow-up installation

The interaction follow-up installed successfully after packaged verification.
The desktop executable, app archive, bundled CLI, UI index and terminal host
match the staged build. All 382 root-data files and ten desktop-workspace data
files remain unchanged; models and native inference runtimes are preserved.
Installed CLI version and agent-help smoke checks passed. It remains a local
0.4.26 preview; no release, WSL CLI or global PATH changes were made.

Installer SHA-256:
`83C05C8B00C7CBC86A50104DAAD8BA12571C1868A0C79AC3B3A827C113147C93`.
UI SHA-256:
`F49A9F3AC9C1280EAEFE19E97C5D6074D6BE9B5D943B0472B285E0F95CC2E47B`.
Rollback:
`D:/OffGrid-install-backups/v0.4.26-before-interaction-polish-20261106-150316`.
Use the desktop's OffGrid CLI or Agent entry for the matching CLI. The app was
left closed; packaged fixture execution is distinct from opening user state.
