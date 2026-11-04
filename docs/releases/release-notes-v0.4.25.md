# OffGrid LLM v0.4.25

This release improves document questions, first-use recovery, desktop speech,
and the optional Windows terminal. It preserves existing conversations, models,
profiles, grants, and local workspace data.

## Changes

- Chat offers a responsive document reading workspace alongside the conversation.
  Readable PDF text remains usable when figures have not been interpreted;
  answers must disclose that visual limit. Original pages and extracted source
  locations remain available for inspection.
- Document excerpts are selected within the model's measured context budget.
  The selected source ranges are retained with the request, so asking another
  question about the same attached PDF does not require re-uploading it or
  silently dropping history.
- Knowledge retention removes obsolete rebuild caches and does not reuse a
  cached result after a retention failure. Existing hold and backup limits still
  apply; this does not establish full organization-wide retention qualification.
- The native Windows desktop bundles its pinned Whisper and Piper runtimes.
  Ordinary Chat can use editable pause-detected dictation or opt into a
  session-only Talk loop with visible stop/relisten controls. Speech does not
  silently change the selected chat model or grant computer access.
- The optional Windows terminal has separate **OffGrid CLI** and **Agent**
  entries. The CLI entry opens a native shell with the matching bundled
  `offgrid` executable; Agent opens the existing agent profile chooser. Terminal
  tabs retain input/cursor state, and owned processes are cleaned up on close.
  The terminal control appears only when its native runtime is available.
- First use gives a direct path to select a different installed ready model
  when the saved selection is unavailable, or to browse curated models when
  none is ready. Selecting or downloading a model still requires an explicit
  action.

## Verification and limits

Go, API/type/build, desktop unit, source-layout, documentation and OpenSpec
checks passed locally. The isolated browser run passed 983 tests with 31
documented skips. A packaged Windows x64 Electron/ConPTY check exercised
`offgrid --help`, Agent profile selection, tab and cursor continuity, Unicode,
resize and owned child-process cleanup. Additional first-use and terminal
browser checks passed against a disposable service. Exact-tag CI and release
artifact checks are required before publishing.

These checks do not establish real microphone accuracy, acoustic echo handling,
macOS/Linux terminal availability, macOS Piper readiness, full document-answer
quality, scanned-image understanding, enterprise retention qualification, or
human accessibility/usability results. The source text of PDFs with figures is
available, while their unread visual content remains explicitly limited. No
model, speech asset, or runtime is downloaded automatically by this update.

## Upgrade

Finish active work and quit the desktop app before installing a matching service
and desktop build. Back up the workspace, and keep existing model and data
directories. This release does not clear history, migrate an existing Docker
workspace, or change the selected service.
