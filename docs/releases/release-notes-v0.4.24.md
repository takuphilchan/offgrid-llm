# OffGrid LLM v0.4.24

This release simplifies Chat and Knowledge, adds saved document attachments and
a Windows desktop terminal, and repairs PDF extraction and incomplete vision
model downloads.

## Changes

- Chat uses a compact composer with attachment and response menus. Optional
  configuration stays available without occupying the main writing area.
- Attach documents privately to conversations in web, desktop and CLI. Failed
  sends retain their attachments; saved sources can be inspected or explicitly
  added to Knowledge. CLI file chat supports `--file PATH` and `/attach PATH`
  through the same authenticated service. Agent attachment execution remains
  gated rather than silently dropping unsupported files.
- PDF extraction uses a pinned local worker rather than relying on an installed
  `pdftotext` command or the failing fallback. DOCX, XLSX and PPTX extraction
  preserves Unicode and source structure. Imports expose progress, partial
  extraction and recovery; citations link to retained source locations. See
  [supported formats and limits](../guides/document-formats.md).
- Packaged local OCR has separate, explicit language readiness. No language
  data is downloaded during an import. Unsupported, encrypted and unreadable
  documents receive actionable errors rather than invented extracted content.
- Document retention supports explicit expiry, legal hold and revision-checked
  updates. Defaults preserve existing data. Source access is rechecked during
  retrieval, streaming and saved-result delivery; full organization policy and
  retention qualification remain unfinished.
- Windows x64 desktop provides native terminal tabs in the main Chat area,
  interactive input, tab/cursor preservation and owned child-process cleanup.
  Workspace options stay readable beside the terminal; larger overlays suspend
  its presentation without stopping the shell. Native agent chat adds `/clear`
  to clear its display without deleting saved work.
- Explicit GGUF model downloads include the matching vision projector when the
  repository has an unambiguous supported layout. Files are revision-pinned and
  digest-checked; incomplete installations offer **Check and repair files**.
  Download completion does not establish model/runtime vision compatibility.
- Voice setup distinguishes model files from their required speech runtime, and
  CLI audio status inspects the selected authenticated service. Cancelled probes
  no longer cache a false runtime-unavailable result.
- Agent settings brings the pinned model, profile revision, decision budget and
  existing resource restrictions into one panel. Services supporting on-demand
  access request a folder when needed; active access remains visible. Existing
  profiles and permissions are not expanded.
- Help is available from the header and Quick actions, with examples, setup
  guidance and links for returning to saved work. It preserves drafts, supports
  keyboard dismissal and restores focus.
- Response settings replaces the ambiguous Context & response label. Context
  allocation and response-token limits sit under Advanced, with current values
  summarized. Knowledge and voice controls remain available; defaults do not
  change merely by opening the panel.
- Model pickers have one scrolling list, fixed search controls, readable rows
  and stable placement in short windows. Model Verify/Delete menus and history
  actions use compact rows instead of oversized button panels.
- Activity uses compact task links with secondary status, a bounded attention
  menu and visible cleanup for removable history. Long task titles and Show more
  no longer dominate the page. Existing protected-deletion rules still apply.
- Blocked conversations explain the required next action and link to the
  canonical task. Starting a new chat does not retry or change uncertain work.
  Cancelled-task presentation no longer displays a stray zero or oversized
  continuation controls.
- Folder setup has one access label and concise native-desktop guidance.
  Confirmed undersized labels use shared typography tokens. Help and management
  surfaces support narrow layouts, RTL and 200% zoom in automated checks.
- Desktop shutdown preserves cleanup ownership and allows retry when an owned
  process does not stop successfully, rather than silently abandoning cleanup.

## Verification and limits

The complete local renderer run against a disposable service passed 945 tests,
with 28 documented skips. Full Go tests, 78 Python SDK tests, API/type/build,
documentation, source-size and OpenSpec checks passed. Packaged Windows checks
covered menu placement, all seven routes at 200% zoom, Help, appearance/restart,
real terminal input and descendant cleanup. Earlier failures and their
corrections remain in the [local release evidence](../quality/evidence/release-local-checks-2026-11-03.md)
and [UX audit](../quality/evidence/workspace-completion-audit-2026-11-01.md).
Release packaging requires passing CI for the exact tagged source revision.

Unknown-outcome tasks are still protected from deletion. This release does not
claim that an interrupted external action succeeded or was undone. A separate
explicit discard-history change remains pending. Existing experimental model,
image and agent capabilities retain their gates. Human usability participation,
manual screen-reader/native-speaker review and full native platform/model
qualification remain open; automated layout checks do not replace them.
The terminal is enabled only on the qualified Windows x64 path; macOS/Linux and
Windows ARM64 remain gated. Full document-answer quality, arbitrary scanned-file
accuracy and enterprise retention/access qualification are not established by
parser fixtures or UI tests. Existing saved answers and backups have separate
retention; deleting an original is not a promise of secure erasure everywhere.

## Upgrade

Finish or stop active work and quit the desktop app before installing matching
service and desktop components. Back up the workspace and retain models and
configuration. This update does not clear existing conversations or activity,
grant access, download models, change the selected service or migrate a Docker
workspace. Local cleanup performed at the owner's request is not an upgrade
side effect.
