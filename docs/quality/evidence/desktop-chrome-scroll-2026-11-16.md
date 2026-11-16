# Desktop captions, reader clearance and chat scrolling — 2026-11-16

## Reported failures and repair

- Startup showed three bare squares at the lower left. The trusted preload
  injected inline style elements, which startup's `style-src 'self'` policy
  rejected. Preload-owned constructable stylesheets now style both the caption
  host and its closed shadow root without weakening that policy or exposing
  native window actions to service scripts.
- Expanded document tabs were behind the Windows title bar. Expanded readers
  now use the native caption-height variable, including fractional zoom values;
  ordinary browsers retain a zero top inset.
- A short upward wheel gesture was caught by the 100-pixel follow-output zone.
  Upward intent now pauses following immediately, and following resumes at the
  actual bottom or through Latest. Keyboard and directional touch scrolling use
  the same policy. Inspecting work continues to pause following.

## Reproduction and checks

Before the fix, the short-scroll test snapped from scrollTop 3116 to 3384 after
output grew. Expanded reader tests reported top 0 instead of 32, 21.333 and 16
CSS pixels. The browser zero-inset control passed. Four of five regressions
failed before repair; all passed afterward.

After repair:

- Full `document-reader.spec.ts`: 141 passed, two opt-in real-service/private-file
  tests skipped. Includes all localized reader cases, RTL, expanded/restore,
  draft preservation and 200% layout.
- Five focused `agent-chat.spec.ts` tests passed: short upward scroll/Latest,
  growing output/manual scroll, handoffs, inline attachments and response/work
  spacing. These use deterministic renderer fixtures, not model-quality tests.
- Desktop Node suite: 223 passed, one skipped, zero failures.
- TypeScript check, Vite build, Go service compilation and Electron packaging
  completed. Package hooks checked resources, service version and UI identity.
- New hidden native caption check passed against source and the packaged
  executable at 100%, 150% and 200%. Closed shadow controls retain labels and
  right-edge alignment under startup's strict CSP. Light/dark captures inspected
  visually; no stray squares. Test windows remained hidden.
- Packaged isolated startup/recovery/owned-service check passed; observed first
  window 485 ms and recovery 607 ms in this run (not a performance guarantee).
- Packaged hidden quick-window check passed: one trusted window, retained draft,
  caption clearance, restored bounds and shortcut-setting validation. No OS
  shortcut delivery, native control clicks or real model execution claimed.

An attempted broader combined Chat/reader run was stopped after six obsolete
Workspace-selector cases timed out (60 earlier cases passed). Those cases still
expect the removed always-visible Workspace control, contradicting the newer
on-demand access tests. This report does not claim that entire suite is green.
The complete reader suite and relevant Chat checks above ran separately.

Local logs: `build/chrome-scroll-regressions-before.log`,
`build/chrome-scroll-regressions-after.log`, `build/chrome-reader-suite.log`,
`build/chrome-chat-scroll-suite.log`, `build/chrome-desktop-unit.log`,
`build/chrome-packaged-check.log`, `build/chrome-packaged-startup.log`,
`build/chrome-packaged-command.log`, `build/chrome-scroll-suite.log`.

## Candidate

- Local preview: `0.4.31-preview.20261115174855379.g9adc304f`.
- Source: `9adc304fbab6a21b29f36c22f6d88e0d8bbe7dca`, dirty working tree.
- UI: `ca1c5092d51ef210fad28541b2814f930841ed60a48ada0daabd25a638ec4400`.
- Artifacts: `build/desktop-0.4.31-preview.20261115174855379.g9adc304f`.
- Unsigned local preview; not a public release qualification. Native linguistic
  review and other roadmap gates are outside these focused repairs.

Installed on 2026-11-16 after the user closed OffGrid. NSIS exited successfully;
all 1,200 payload files matched the candidate. Desktop-profile hashes and workspace
hashes/inventory were unchanged. The application was left closed for the user.
Verified rollback and installation receipt:
`D:\offgrid-install-backups\chrome-scroll-20261116\installed.json`.
