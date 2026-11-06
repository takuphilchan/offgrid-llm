# OffGrid v0.4.27

This release improves the desktop startup, Chat workspace, Activity and CLI.
Existing models, conversations, agent profiles and permissions are preserved.

## Changes

- The app and shortcuts are now named OffGrid. Desktop downloads identify the
  OS, architecture and Windows setup/portable choice. Existing workspace paths,
  application IDs and CLI commands remain compatible. See the
  [download guide](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.27/docs/setup/installation.md#download-and-verify-a-release).
  Windows caption controls share the sidebar surface without a duplicate title.
- Desktop startup uses a compact status screen, a plain OffGrid window title
  and one primary recovery action. Technical details stay collapsed. Packaged
  services embed their version and are checked before packaging, preventing
  Windows from confusing Electron's version file with OffGrid's version.
- Chat separates global navigation from conversation history, keeps the editor
  at the bottom and uses a quieter empty state. Attachment menus, image previews,
  document navigation and model-library controls have more consistent layouts.
  Document previews retain their page and zoom when reopened.
  Agents and Models share section navigation; disclosure chevrons, search focus
  and action sizes are consistent across pages. Connections uses a compact setup
  form and aligned actions. Chat and terminal meet the sidebar with rounded
  corners and fewer separating lines.
- The primary Chat action switches between Talk, Send and Stop according to
  the current state. Dictation shows actual microphone levels, elapsed time,
  cancellation and transcription progress. Whisper silence annotations no
  longer appear as transcript text; silence leaves the draft intact. Talk
  auto-send remains opt-in and separate from dictation.
  Talk shows one transcription status and offers capture recovery after silence
  without incorrectly directing users to install a model.
- Activity uses authorized, paginated summaries with consistent attention,
  running and history states. Stop and deletion reconcile the displayed work;
  protected or uncertain work remains explicit rather than silently discarded.
  Inactive records can be archived out of everyday Activity and restored without
  rerunning work; their original outcome and recovery evidence remain available.
- Help groups guidance into focused topics. Regional language preferences and
  browser language ordering select supported UI languages more reliably.
- Terminal opening is serialized so simultaneous requests do not show a false
  navigation error. Returning to Chat while it loads is respected. Tab focus,
  cursor preservation and owned-child cleanup remain covered by packaged tests.
- CLI command layouts, help, progress and agent selection use consistent,
  width-aware presentation. Profile search, explicit remembered defaults and
  saved-conversation continuation reduce repeated setup. Plain and JSON modes,
  exact identifiers and authorization boundaries are preserved.
- Leaving Agent with `/exit` returns to the same-folder shell. Terminal creation
  is grouped under one menu; sidebar Chat and New chat reveal Chat while keeping
  terminal sessions intact.
- MCP connections appear before the searchable tool inventory. Hermes and
  OpenClaw have a dedicated External agents section and focused setup dialogs.
  Cancelled GGUF downloads can be discarded explicitly, including partial files,
  without deleting installed models.
- Generated conversation names handle Windows filename restrictions. Computer
  settings distinguish the installed companion from an active control session.
- Browser downloads handle both click and download-event failures without an
  unhandled rejection or replaying the click. Browser control still requires an
  approved managed session; this is not general personal-browser qualification.

## Verification and limits

Local Go tests passed across the repository. Linux race checks passed for agents,
models, sessions and server. Web type/build/API checks passed, and the final full
isolated built-service renderer suite passed **1,115 tests with 32 skips**. Desktop
tests passed **139 with one skip**; computer companion tests passed **75**.
Packaged Windows startup, terminal, appearance and real-service document checks
also passed. Earlier failures and their follow-ups remain in the evidence records.
Exact-release-source CI and release artifact checks must pass before publication.
See the [verification record](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.27/docs/quality/evidence/release-0.4.27-2026-11-06.md).
The [branding, speech and chrome follow-up](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.27/docs/quality/evidence/branding-speech-chrome-2026-11-06.md)
records the subsequent regression checks and verified Windows upgrade; unavailable
platform and human checks remain explicitly unverified.
The [terminal/session follow-up](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.27/docs/quality/evidence/terminal-session-followup-2026-11-07.md)
records the final full-suite commands, screenshots and environment-specific skips.

These checks do not establish broad document-answer accuracy, interpretation
of unread figures, enterprise retention qualification, human usability,
manual screen-reader/translation quality or every native model/hardware
combination. Existing experimental capability boundaries remain in effect.
No models are downloaded automatically, and no user history is cleared.

## Upgrade

Finish active work and quit OffGrid before replacing the desktop and its matching
service. Preserve the workspace and model directories and retain a backup.
This release does not migrate Docker data, change service ownership or grant
new folder/computer access. Use the bundled OffGrid CLI entry for the matching
desktop CLI; separately installed clients can be updated with the release assets.
