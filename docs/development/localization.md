# Translation quality and language coverage

OffGrid keeps interface language, model language capability and speech settings
separate. Adding a language name does not establish translation quality or model
support. The November 12 completion pass provides all 1,356 catalog messages
across all 29 registered languages, with zero missing entries. Existing
dictionaries are retained; the 20 global preview packs remain previews until
independent language review is complete. Draft coverage is not linguistic approval.

## Sources and implementation

- `apps/web/src/i18n/locales/` contains the nine existing 278-entry core dictionaries.
- Feature dictionaries in `apps/web/src/i18n/` cover additional workflows,
  permissions, errors, recovery and speech. Core coverage is not total app coverage.
- `apps/web/src/i18n/language-packs.json` contains 20 preview definitions, initial
  core overrides and pending review notices. `feature-packs.json` supplies the
  remaining contextual and core translations.
- `apps/desktop/language-registry.json` shares supported identifiers, fallback
  dictionaries, direction and preview state with desktop persistence.
- `useI18n().language` and `useI18n().locale` both retain the selected language.
  Wrap feature tables with `defineMessages(namespace, legacyMessages)` from
  `i18n/catalog.ts`. Contextual overrides resolve per message, preserving the
  existing dictionary or English only for missing messages. Do not replace the
  selected language globally before looking up feature copy.
- `feature-packs.json` adds contextual messages; `feature-sources.json` and
  per-language `translation-sources.json` record their English source.
  Source changes require translation review and an
  explicit matching source update. Missing previously translated keys fail QA.
- The inventory covers core/features, voice/computer, desktop presentation,
  folder trust and terminal. Run `i18n:audit` for current counts rather than
  assuming that navigation or a particular screen represents full coverage.
  Terminal catalog messages, Voice preferences and runtime status, the terminal
  settings explanation, model-choice filtering and shared navigation now have
  authored text across all 29 languages. All inventoried workflows have draft
  coverage; independent language review is still pending.
  The completion pass adds contextual descriptions for the built-in model catalog
  and translated terminal error summaries with separately preserved diagnostics.
  Model quantities use the selected locale. This is broader than core coverage,
  but does not inventory all CLI output,
  server-generated diagnostics or OS-owned menus. Do not advertise it as 100%
  of every possible application string.
- Preview selection is opt-in. The language selector explains in the selected
  language that native-speaker review is pending. English fallback remains a
  compatibility mechanism, while CI rejects missing catalog entries. Never render
  an empty label or pass an unknown identifier to a permission dialog.

Identifiers follow [W3C language-tag guidance](https://www.w3.org/International/questions/qa-choosing-language-tags)
and [Unicode CLDR guidance](https://cldr.unicode.org/index/cldr-spec/picking-the-right-language-code).
Use `zh-Hans`/`zh-Hant` for scripts and `pt-BR`/`pt-PT` for distinct regional copy.
Retain explicit scripts before inferring from regions. Language selection must
not choose a speech model or initiate network, microphone or inference activity.

## Required translation workflow

1. Inventory untranslated core and feature messages for the language. Include
   desktop menus/startup, terminal controls, CLI output and accessibility labels.
   Keep a list of intentional technical terms and brand names.
2. Translate complete messages in context. A translator should see the screen,
   action and resulting state. Preserve placeholders and distinguish Stop,
   Cancel, Close, Delete and Revoke; these are not interchangeable.
3. Maintain a per-language glossary for model, workspace, knowledge, agent,
   approval, permission and service. Choose ordinary product wording over literal
   word substitutions. Preserve commands, paths, model IDs and protocol names.
4. Have a fluent native speaker review naturalness, register and regional usage.
   Have a second qualified reviewer check permission, deletion, privacy and
   recovery wording against actual behavior. Record reviewer, date, exact source
   revision and outstanding issues; never populate review metadata speculatively.
5. Inspect real screens at narrow and wide widths, increased text size, RTL and
   keyboard/screen-reader navigation. Check Indic joining, CJK line wrapping and
   font fallback on supported systems. Preserve IME composition before Enter.
   Apply [bidirectional markup guidance](https://www.w3.org/International/articles/inline-bidi-markup/)
   to mixed paths, numbers and translated text. Do not insert hidden direction
   control characters into translations.
6. Promote only after all core and feature keys are complete, automated checks
   pass and reviewer evidence is attached. Update the explicit preview gate,
   registry and documentation together. An English fallback is a compatibility
   mechanism, not a completed translation.

## Automated checks

Run `npm --prefix apps/web run i18n:check`. The check reports catalog-wide coverage,
compares message keys and placeholders, rejects empty strings and suspicious
encoding/direction characters, scans supplemental source literals, and rejects
unreviewed promotion. `i18n:complete` additionally fails if any registered language
has a missing catalog entry; CI runs both. This content gate does not certify
linguistic quality or replace `i18n:ready`, which requires recorded human review.
Wrong or mixed scripts, common Chinese script-variant leaks, uncatalogued visible
JSX text (including short labels and template fragments), and unwrapped feature
tables also fail structural checks. Exact file-scoped allowances identify brands,
units, keyboard shortcuts and internal diagnostics; they do not exempt sentences.
These checks detect defects, not grammatical correctness or natural phrasing.
The checks run in CI. Their tests deliberately inject
missing keys, broken placeholders, stale source text, removed translations and
corrupt text to verify rejection. Existing nine-language key parity includes new
terminal/menu/error drafts; it is not evidence of native-speaker approval.

Browser tests cover all selectable previews, persisted selection, direction,
narrow layout, complete feature fallback, desktop synchronization and unchanged
speech preferences. These tests establish software behavior, not linguistic
accuracy. Existing dictionaries also need ongoing contextual and native-language
review; a complete key count does not certify them.

## All-language review packs

Use the [translator handoff](translation-review.md) to export a separate pack
for each of the 29 languages, reconcile reviewed text and run the strict
`i18n:ready` gate. `i18n:audit` now reports missing text and exact-source review
coverage for existing languages as well as previews. The readiness gate currently
fails; no native-speaker review has been recorded. Desktop consumes the same
contextual overrides for startup, menus, computer access, folder trust and terminal
controls, and package verification checks the copied override file.

## Rendered text and terminology audit

`npm --prefix apps/web run i18n:rendered:report -- --workers=4` traverses isolated
fixtures for every language and attaches `rendered-text.json` per language. It
collects actual text nodes, options, titles, placeholders and accessibility
labels, including open Help/setup, all Settings sections, voice choices and empty
search, model categories/discovery and Activity views. Technical identifiers and
sample user content are explicit exclusions; unknown service-authored prose is
reported as unclassified, not automatically exempted. The report records the
translation revision used. This is a bounded state inventory, not proof of every
error, permission, modal or lifecycle state.

`i18n:rendered` runs the same traversal with a strict gate that fails on
untranslated or unclassified elements. Normal browser/CI runs also enable it;
only the explicit report-only command disables this assertion. The completion
pass passed 47 states per language (1,363 states total). Collection passing alone
is not that gate passing. `i18n:ready` remains blocked by missing human review.
Use `OFFGRID_FIXTURE_PORT` to isolate concurrent runs without reusing another
developer's server or reaching an installed service.

`i18n:consistency` enforces the shared Chat destination name across main and
terminal navigation, matches six route headings to their navigation labels, and
reports other repeated-source wording variants for
contextual review. Variants may be justified by grammar or context; the check
does not blindly replace them. Review packs include those source locations and
wordings, along with candidates from first-party service catalogs, terminal-host
errors and CLI presentation outside the renderer inventory. Candidates require
triage and are not counted as translated catalog entries.

The terminal check (`node scripts/test/test-terminal-display.mjs`) now loads the
real renderer in headless Chromium with a synthetic bridge across all 29
languages. It checks labels, xterm accessibility strings, open command previews,
edited questions/output, search selection and Agent tab names. It does not
launch a shell, control the desktop or translate historical process output.

The counts reported as `authored` distinguish supplied catalog entries from
fallbacks. They include local-model drafts and assistant corrections; they do not
mean a human translator authored or approved every entry. Review metadata remains
pending and exact-source approval is required before promotion.
