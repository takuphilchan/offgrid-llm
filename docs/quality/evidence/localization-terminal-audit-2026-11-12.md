# Rendered localization and terminal follow-up

This follows the owner's request to inspect every UI text element, include the
terminal, and apply the same standard to all 29 supported languages. Computer
control remained stopped. Browser verification used isolated headless fixtures;
the terminal bridge never launched a shell or sent input to a real process.

## Implemented corrections

- All 90 terminal catalog messages have authored text in all 29 languages,
  including xterm input and output-reading accessibility announcements. These
  are drafts, not native-speaker approval or a claim that CLI output is localized.
- Language changes update Agent tab names, open command context, display/save
  status, search result counts and terminal accessibility labels. Edited output,
  questions, search selection and process state are retained.
- Voice preferences, speech runtime status and the Voice/Terminal settings
  explanations have complete authored coverage across the registered languages.
- Model-choice filtering now owns its empty message instead of incorrectly
  displaying the History feature's message. Shared Library/Quick actions copy
  and model-choice filtering have preview-language drafts.
- French and Shona terminal Chat destination names now match navigation.
  A CI guard checks that shared destination across all languages. Other repeated
  source phrases with different wording are exported for contextual review;
  grammar-dependent variations are not blindly replaced.

## Actual rendered audit

`localization-rendered-audit.spec.ts` captures text nodes, options, placeholders,
titles, labels and descriptions in 43 states per language: main routes, Activity
filters, all Settings sections, open voice choices and no matches, each model
category in installed/discover views, expanded disclosures, Quick actions, Help
and setup. Fixtures include the English service-provided descriptions from the
reported catalog cards. No model is downloaded, imported or executed.

Across 29 languages, the report contains **58,623 element observations**. This is
a repeated-state observation count, not unique translations. It is a bounded
inventory; permissions, every backend error, every modal and native OS dialogs
are not exhaustively exercised. Reports distinguish known missing/stale source
copy, catalogued text, explicit technical/user-data exclusions and unclassified
text. Unclassified service text is not silently treated as user content.

The strict rendered check was run for Simplified Chinese and exited **1** with
untranslated/unclassified elements, as expected. Report collection passing must
not be described as translation readiness passing.

## Verification

- Web type/build and API contract check passed. Build retains the existing chunk
  size warning; no claim of a performance improvement is made.
- Latest affected browser run: 69 passed, with two substring-selector failures
  in the older Korean/Bengali selected-message tests. Both selected a hidden
  translated Quick actions span containing the word Work. The tests now select
  visible copy; the focused rerun passed both. All 29 rendered audits and 13
  model-choice tests passed in that run.
- Terminal real-renderer check passed all 29 languages, open command-preview
  edits, unchanged output and search selection, labels and Agent tab names.
  Native PTY and computer control were not used.
- Desktop suite: 201 passed, one skipped. Updated VM fixtures load the new
  accessibility catalog in the same order as the packaged terminal.
- Audit/consistency regression tests: five passed, including missing/stale text,
  contextual exclusions, a deliberately conflicting navigation label, and
  complete authored Voice/terminal catalog coverage across all 29 languages.
- Structural translation checks, source-size ratchet, documentation validation
  and strict OpenSpec validation passed. Human language review remains absent.
- Packaged presentation assets and all 53 UI files match the tested source build.
  Compatibility protocol is 1; UI build identifier:
  `c95f78676f3bb6cc1c0d7b0342ddf3378b9a5d64c31d5524c78e40eff31db8fa`.
  Candidate `app.asar` SHA-256:
  `777a6b7f56ceefe38982798b9b6dd1569a9933a66dd00ba2d8edce67d20b959e`.

## Handoff and remaining work

Local bundle: `build/OffGrid-translation-review-20261112.zip`. It contains 29
review packs, glossary and terminology variants, complete source/placeholder
metadata, coverage, rendered reports tied to their translation revision, and
2,159 runtime source candidates requiring triage. The candidate count includes
possible non-UI literals; it is not a translation completion count.

Current catalog inventory: **1,330 messages per language**. Each preview has
255 authored messages and **1,075 missing messages**. Existing dictionaries also
need contextual/native review. No reviewer identities or approvals were invented.
`i18n:ready` exits 1. The application is **not fully localized or language-ready**.

The remaining implementation must:

1. Add contextual, versioned keys for first-party service catalog descriptions;
   preserve custom service descriptions and model identifiers instead of blindly
   replacing arbitrary API text.
2. Route terminal-host failures and other first-party error codes through
   localized recovery messages, retaining technical details separately.
3. Give CLI-authored prompts/help their own localization boundary. Commands,
   flags, machine-readable JSON and arbitrary process output must stay intact.
4. Complete missing Help/setup/discovery/agent/workflow catalogs with the same
   per-language glossary, then import reviewed text using the conflict checks.
5. Triage every unclassified rendered element, broaden state coverage to error
   and permission flows, and require complete translations plus recorded native
   and consequential-copy review before promoting previews.

## Installation

After the owner confirmed OffGrid was closed, the silent per-user installer exited
0. The installed archive matches the candidate SHA-256 above, all 53 UI files
match the source build, and packaged presentation assets/compatibility verify.
All 568 saved workspace files retained their paths, lengths and modification
timestamps; all four checked preference files retained their SHA-256 hashes.
The app was not reopened or controlled automatically.

Installer: `build/terminal-language-audit-20261112/installers/OffGrid-0.4.28-windows-x64-setup.exe`.
Installed location: `C:\Users\phil\AppData\Local\Programs\OffGrid LLM Desktop`.
This installs the verified terminal/Voice correction; it does not promote the
preview languages or complete the remaining application translation work.
