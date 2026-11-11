# All-language localization follow-up — 2026-11-11

The owner supplied mixed Simplified Chinese/English screenshots, requested the
same correction across all languages, stopped computer-use automation, chose
translator review packs and authorized installation after checks. The native
automation kernel was reset and no subsequent computer-use actions were issued.

## Implemented scope

- Audited 29 selectable languages, 135 contextual catalogs and 1,319 messages per
  language. Existing nine dictionaries have key coverage. Twenty preview packs
  initially had only 90 authored messages and 1,229 fallbacks each.
- Added 49 missing management-screen messages per preview (980 new translations)
  and clarified the model-category label. Coverage is now 139/1,319 per preview;
  1,180 entries per preview remain missing. These are drafts, not human-reviewed
  language qualifications. Existing user agent names and model IDs are unchanged.
- Covered the reported Agents tabs/description/selection prompt; Knowledge
  heading/add/options/empty/library explanation; Models navigation/category/
  selection/readiness/details/size/guidance; Activity filters/headings/guidance;
  and Settings general, response-limit and local-access/storage copy.
- Fixed native startup's unconditional fallback-language snapshot. Shared
  overrides now reach startup/menus, computer-access copy, remembered-folder
  dialogs and terminal chrome. This fixes routing, not missing translations.
- Added a shared packaged override asset with exact-source archive verification.
  The first packaging attempt failed closed on a missing file; directory-based
  mapping corrected it. A later network failure was avoided by using the already
  installed matching Electron distribution. No failed package was installed.
- Added source-only JSON review exports, detailed per-catalog coverage, a glossary
  and literal audit. Imports check complete key sets, source hashes, current-text
  conflicts, placeholders and review evidence. Staged replacement has backups and
  rollback; an existing staging file is preserved. Sensitive copy needs a distinct
  second reviewer. Readiness is separate from structural validation.

## Evidence

- `build/localization-all-languages.log`: 33 passed, including all 29 languages
  through seven management/settings route states. Tests check body copy and
  labels, language/direction, unchanged user names/paths and no mutations.
- `build/localization-settings.log`: 83 passed, including previews, reload,
  regional tags, RTL and desktop preference synchronization. Replaced the old
  assertion that incorrectly expected the English “Computer use” fallback.
- `build/localization-desktop.log`: 201 passed, one skipped. Includes selected
  language resolution, placeholder-safe user values, archive asset checks,
  import conflicts/rollback and review invalidation.
- Web type check, API generation check and build passed. The existing large
  output-chunk warning remains; it is not a translation readiness result.
- `build/localization-full-browser.log`: 1,535 passed, 32 skipped, two failed
  because the bare test service lacked the packaged PDF worker. Both failed
  cases passed with the existing verified packaged service and its worker:
  `build/localization-document-recheck.log`. No application code was changed to
  suppress the failures; this is a focused fixture-corrected rerun, not a single
  all-green full-suite run.
- `build/localization-terminal.log`: headless renderer/xterm check passed with a
  synthetic bridge, without opening a native shell or using installed-user state.
- `build/localization-quality.log`: structural translation checks passed.
  `build/localization-readiness.log`: readiness correctly exits 1; all 28
  non-English languages lack recorded native review, and previews remain partial.
- Documentation check: 354 maintained Markdown files, no problems. OpenSpec
  strict validation and diff whitespace checks
  passed. Source-size check retains an unrelated failure in `install.sh`
  (686 lines versus its 659-line baseline); that concurrent installer work was
  not modified. The terminal dictionary has a separate documented size baseline.

## Deliverables and limits

Review bundle: `build/OffGrid-translation-review-20261111.zip`; expanded files:
`build/translation-review-20261111/`. Includes all 29 language packs, English
reference, context, placeholders, pending reviews, glossary and coverage report.
No private workspace data is included and no messages were sent to translators.
Workflow: `docs/development/translation-review.md`.

Candidate: `build/localization-followup-20261111-v3/win-unpacked/`.
App archive SHA-256:
`55371ACA1F90B80B94F8E46CF0D0D9EBC8803703A523A490B3B8FEC0269CC2B5`.
Package hooks verified presentation/translation assets and existing native
service, document, speech, terminal, execution and computer payloads.

No native-language, screen-reader or additional native-platform qualification
was performed. The selected-language screen tests and complete key inventory
must not be advertised as full app-wide linguistic completion. OpenSpec 32.3
remains open for missing translations and actual reviewer evidence.

## Installation

Built the NSIS installer from the exact verified `win-unpacked` candidate and
installed silently to the existing per-user OffGrid installation. Installer exit
code: 0. The installed archive matches the SHA-256 above. All 53 renderer assets
match the candidate byte-for-byte; installed presentation assets and the UI
compatibility manifest pass verification.

Before/after preservation snapshots contained 568 user-workspace files with zero
path, length or modification-time differences. The four checked preference files
also have identical SHA-256 hashes. Snapshots are local under
`build/localization-install-*-before.xml` and are not included in translator packs.
The application was already closed; no process was killed. It was not reopened,
and no installed-UI/computer-use verification was performed in this follow-up.
The earlier verified installer remains available for rollback. No commit, push,
release publication or external translator communication was performed.
