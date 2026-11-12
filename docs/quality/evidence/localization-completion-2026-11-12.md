# Catalog localization completion — 2026-11-12

## Result and qualification boundary

All 29 registered language catalogs contain all 1,356 messages across 137
catalogs, with zero missing entries. The previous 1,075-message gap in each of
the 20 preview languages is closed. This pass also adds 24 built-in model
descriptions and two terminal diagnostic labels, including translations in the
eight original non-English languages.

These are complete **draft catalogs**, not native-speaker approval. Existing
dictionaries, local-model drafts and assistant corrections supplied the text.
No human review identities or evidence were invented. All 20 preview languages
remain previews; all 28 non-English languages still require independent review.
`i18n:ready` remains intentionally failing while the completeness and structural
gates pass.

The inventory includes renderer copy, accessibility labels in its catalogs,
desktop startup/menus, computer/folder access and terminal controls. Arbitrary
shell output, user content, external/custom catalog prose, raw diagnostics and
OS-owned copy are preserved. The Go CLI is not claimed to be fully localized.

## Implementation

- Completed contextual overrides, tracked exact English sources and retained
  pending per-message review metadata. Language notices now describe pending
  linguistic review rather than claiming missing screens remain.
- Localized built-in model descriptions only when both the model identifier and
  English source match; custom or changed service descriptions remain intact.
  Model quantities use the selected locale's number formatting.
- Localized terminal error summaries and technical-details labels, preserving
  the original diagnostic separately as text. Language changes retain terminal
  input, output, command edits and search state.
- Added a zero-missing-entry CI gate, wrong/mixed-script and common Chinese
  script-variant checks, exact command/path/URL preservation, scoped literal
  allowances and a broader JSX text/attribute/template audit. Script checks do
  not establish grammar or fluency.
- Enforced navigation/page-heading consistency and retained repeated-source
  wording variants for contextual translator review.

## Verification

- `i18n:check`, `i18n:complete`, `i18n:consistency`: passed. No missing entries,
  placeholder/source drift, detected script errors or shared-navigation conflicts.
- Strict rendered audit: **29 passed**, 47 states each, **1,363 states** and
  **70,309 text elements**, zero unresolved elements. Includes expanded Help/setup,
  all Settings sections, Voice choices/empty search, model categories/discovery,
  Activity views and four narrow states. Normal/CI runs now enforce this gate.
- Management screens, live Help language switching/draft preservation, catalog
  description and number-format tests: **36 passed** alongside the first full
  rendered run. One traversal timing failure was corrected by waiting for the
  mounted route/Settings content and the two Voice controls before collection; the final strict
  29-language run passed.
- Affected model/choice/import/appearance tests: **67 passed, 8 skipped**. Skips
  require real-service fixtures and were not claimed as passing.
- Desktop unit suite: **202 passed, 1 skipped**. Quality-script suite:
  **11 passed**. Actual headless terminal renderer: passed across all 29 languages.
- TypeScript/production web build, API schema check, source-size ratchet and
  whitespace checks: passed. Vite reports a large-chunk advisory; all language
  catalogs remain bundled for offline availability.

The rendered audit is a bounded fixture inventory, not proof of every possible
error/lifecycle state or native linguistic quality. Its known fixture dates are
formatted inside the browser because ICU locale coverage can differ from Node.
No desktop-control automation, live user task, microphone or actual shell was
used by these localization tests.

## Translator handoff

`build/translation-review-complete-20261112` contains 29 contextual JSON packs,
coverage, source/translation hashes, placeholders, glossary, terminology variants,
pending review fields, 29 rendered reports and 116 context screenshots. Exported
screens use isolated sample data. Review import rejects stale/conflicting edits
and invalid placeholders and retains backups. Consequential permission, deletion
and recovery text requires a separate second reviewer.

Validation logs use the `build/localization-completion-` prefix. Rendered reports
and images are in `build/localization-completion-rendered`.

## Installation

Installed **0.4.28** into
`C:\Users\phil\AppData\Local\Programs\OffGrid LLM Desktop` with installer exit code
0. The package's service and desktop versions match. The first packaging attempt
rejected a version mismatch; the verified rebuild explicitly pins 0.4.28.

The packaged and installed archives match. All 53 renderer files match the final
web build, and presentation/translation assets pass package verification.

- UI build ID: `0cad8aa77f74918fb83a82079231be04e61124f8109908934dbae3053292f9d4`
- Installed `app.asar` SHA-256: `1289939786cbfb0d4d49b9ad38be83c286c71acad31df498169a1ec1146045d6`
- Installer SHA-256: `3867377db318b2a44b3f6160ed538d4380c5d6a2cd2e9ba61df51a6dc0fb550e`
- Before/after metadata for all **576 workspace files** matched; SHA-256 hashes of
  the four configuration/presentation/window preference files matched.
- The application was not automatically launched. The task-owned local translation
  model was stopped. No computer-use automation was used.

Installation evidence: `build/localization-install-state-20261112/result.json`.
Installer: `build/localization-completion-20261112/installers/OffGrid-0.4.28-windows-x64-setup.exe`.
Translator bundle: `build/OffGrid-translation-review-complete-20261112.zip`.
