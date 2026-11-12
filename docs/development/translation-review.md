# Translator review handoff

The November 12 completion covers all 29 selectable language catalogs, including
the eight non-English dictionaries that predate the preview packs. Each catalog
has 1,356 entries and zero missing messages. Sources include existing dictionaries,
local-model drafts and assistant corrections. No native-speaker approval is claimed.

The exported `coverage.json` records current catalog and message counts. Existing
dictionaries have key coverage, but their linguistic review is also pending.
The inventory covers the renderer, accessibility copy in those catalogs, desktop
startup/menus, computer-access copy, folder trust and terminal controls. It does
not claim every CLI diagnostic, server error or OS-owned label is localized.

## Create and review packs

Run from the repository root:

```sh
node scripts/quality/translation-review.mjs export build/translation-review
node scripts/quality/translation-review.mjs audit
```

There is one JSON file per language, including an English reference, a detailed
`coverage.json`, and a `literal-audit.json` for remaining source-literal inspection.
No application data, credentials, model names from installed profiles or saved
conversations are exported. No translation service is contacted.

For each language, assign a fluent native reviewer familiar with software UI.
Use the Simplified/Traditional Chinese and Brazilian/European Portuguese packs
separately. Preserve the existing African languages and review their technical
terminology rather than replacing them with another language.

Edit only `translation` and `review` in each message entry. Each entry supplies:

- `namespace`, `key`, `source` and `context`: the exact English message and source
  catalog location. Interpret the whole action or sentence in that feature.
- `currentText` and `coverage`: distinguish an authored translation from an
  English fallback. Identical text may be a valid technical term; review it in context.
- `placeholders`: preserve each named variable exactly, including repetitions.
- `requiresSecondReview`: permission, deletion, stopping and recovery wording
  requires a second reviewer distinct from the primary reviewer.
- `review`: keep `status` as `pending` until review is actually done. Approval
  requires the reviewer's name and an evidence reference; sensitive copy also
  needs `safetyReviewer`. No reviewer identities have been fabricated.

Use a consistent glossary for the navigation terms, agent, model, workspace,
context, Knowledge, service and permissions. Keep Stop, Cancel, Close, Delete,
Archive and Revoke distinct. Preserve brands, commands, model identifiers, file
paths and literal user content. Do not translate technical tokens by guesswork.
Check naturalness, ambiguity, punctuation, grammatical forms and regional usage.
Check screen-reader names and narrow/large-text/RTL presentation after import.

Blank translations remain missing. Do not copy English into a blank cell to
increase coverage. Existing translations cannot be erased by a blank import.
English is a reference pack, not an import target; change canonical source copy
in the code and re-export affected packs instead.

## Validate and import

```sh
node scripts/quality/translation-review.mjs import path/to/reviewed-language.json
node scripts/quality/translation-review.mjs import path/to/reviewed-language.json --apply
npm --prefix apps/web run i18n:check
npm --prefix apps/web run i18n:ready
```

The first command validates only. Import rejects unknown/duplicate/missing keys,
changed English source, concurrent translation edits, altered placeholders,
empty replacements, corrupted encoding and unsupported review claims. Changes
are staged, previous files are backed up, and a failed replacement is rolled back.
Re-export after another source change and reconcile the translator's work; do not
delete hashes or overwrite `currentText` to bypass conflict checks.

Imported overrides are used by the renderer and desktop. Desktop packaging
copies the same override JSON and rejects missing or stale packaged copy. Native
startup, menus, access dialogs and terminal chrome use the selected language;
translation never changes command output, permissions, paths or submitted work.

`i18n:check` validates structure. `i18n:ready` is a separate readiness gate that
fails until every non-English registered language has full authored coverage and
review records matching its exact source and translation hashes. A changed
translation invalidates an earlier review. Passing structural or browser tests
is not native-language approval. Currently the readiness gate intentionally fails.

## Rendered context and terminology follow-up

Each pack also includes `terminologyVariants`: repeated English phrases with
different translated wordings and their source contexts. Decide whether each
variation is required by grammar or represents inconsistent terminology; do not
normalize words mechanically. The shared Chat destination is checked across
main and terminal navigation.

The follow-up bundle includes a `rendered/` folder with the actual text and
attributes captured in isolated screen states. Entries classified `untranslated`
or `unclassified` need attention. Unclassified text can be first-party catalog
prose, a combined label and model name, or an intentional identifier. Check the
element and state before editing. The files contain sample fixture content, not
the installed user's conversations or model data.

`runtime-copy-candidates.json` inventories source candidates outside the existing
renderer dictionaries: service-provided catalog descriptions, CLI wording and
terminal-host errors. It is a review queue, not an exact measure of UI strings.
This is a source candidate list, not a list of confirmed remaining omissions.
Built-in model descriptions now have contextual catalog entries, and terminal
errors have translated summaries with separate raw diagnostics. Remaining
candidates need contextual triage before entering the catalog pipeline. Do not translate commands,
flags, structured JSON, paths, model identifiers or arbitrary process output.

All newly completed translations remain drafts. Key coverage, automated screens
and consistent destination labels do not establish naturalness or native review.
The completed bundle includes 47 rendered states and four context screenshots per
language, a terminology glossary, wording variants, exact-source hashes and
pending review fields. Native-language review is the remaining qualification gate;
CLI/process output and OS-owned copy remain outside this catalog inventory.
