# Reader, Library and work controls — 2026-11-15

## Scope and provenance

Continues the approved `workspace-outcome-experience` change, tasks 44.30–44.35.
The owner requested quieter attachments and document controls, a usable comparison
flow, consistent menus and agent management, configurable streaming and explicit
profile deletion. The final reader revision follows the supplied reference's
separation of file tabs, reading tools and page content.

Renderer evidence uses isolated synthetic HTTP fixtures and headless Chromium.
Screenshots explicitly label synthetic document pages. They are not real parser,
OCR, model-answer or native desktop qualification. Installed-user data was not used
for these tests. No new goal, commit or publication was created.

## Implemented behavior

- Compact attachment tiles retain unsent drafts, removal, real processing states
  and error recovery. A small pending indicator replaces the decorative sweep;
  it respects reduced motion and does not invent a progress percentage.
- The shared reader has a file tab row with format badges, close-file, add-file,
  expand and split-view controls. Add-file uses the existing unsent attachment
  chooser; closing a tab does not delete its attachment. Up to eight metadata-only
  tabs retain independent reading positions, while only the active reader mounts.
- A filename pill opens a scrollable vertical page panel. At most four thumbnail
  rows/images are mounted, fetched serially, aborted and released on change or
  close. Scrolling thumbnails does not change the selected source page. The page
  canvas still displays one original page at a time; continuous PDF scrolling is
  not implemented or claimed.
- Zoom and download are direct actions; search, alternate views and extraction
  review are secondary tools. Coverage remains visible in a compact footer.
  Digests, extraction revisions, access checks and exact source highlighting are
  preserved. Expand restores focus and reading state; nested utilities own the
  first Escape. Popover surfaces are opaque, including over document content.
- Comparison always retains editable Before/After choices. Changing either hides
  the old result; a fresh submission resolves current extraction revisions, while
  saved comparison history remains pinned. Coverage and exports remain inspectable.
- Library collections, search and related actions have a compact hierarchy.
  Agents, Tools and external-agent layouts use semantic icons and bounded fields;
  editor actions no longer cover instructions. Setup is inline and Help searchable.
- Response display is a persisted, scoped preference: live text or completed
  response. Canonical previews observe the existing run and require service
  permission. Evidence-dependent/restricted previews remain withheld; revocation,
  identity changes and denied reads clear transient content. No second executor is
  created to obtain a preview.
- Permanent profile deletion requires an owner-scoped reviewed revision/token.
  The service rechecks protected work, ownership and the exact work/memory snapshot
  atomically. Active or dependent work blocks deletion; stale reviews require a new
  review. Eligible standalone work/private memory are removed with the profile.
  Shared originals, exports and backups are not implicitly deleted. CLI, Python,
  generated TypeScript and the renderer use the same contract.

## Verification record

Logs below are local artifacts under `build/`; overlapping runs are not added up
as a unique test total. Earlier broad runs were stopped by failure limits. They are
retained as failed evidence, not represented as passing full-suite executions.

| Check | Result / artifact |
| --- | --- |
| Go agents, server, CLI and command packages | Passed; `flow-go-suite.log` |
| Final exact-memory deletion snapshot regression | Passed; `flow-deletion-final.log` |
| Python SDK | 149 passed; `flow-python-pytest.log` |
| Comparison, external agents and overlay presentation | 85 passed; `comparison-presentation-verified.log` |
| Library flow including new comparison/source updates | 6 passed; `library-flow-final.log` |
| Reader, TIFF, source region, canonical preview and recovery reruns | 48 passed; `reader-pane-final.log` |
| Latest file tabs, expand, 37-page visual layout, keyboard, stale source and zoom | 23 passed; `reader-chrome-verified.log` |
| Remaining review/navigation/streaming checks | 115 passed, two obsolete workspace-menu selectors failed; `reader-flow-tail.log`. Both failures passed after updating the navigation in the focused reruns below. |
| Shared controls, opaque popovers and identity transition | 14 passed before failure limit; `reader-final-controls.log`. This includes all five shared-control tests, six overlay tests and agent identity isolation. The thumbnail scroll jump and obsolete Appearance selectors failed; their corrected rerun is below. |
| Final virtual-thumbnail keyboard/scroll, Appearance dropdowns and sign-out isolation | 7 passed; `reader-last-regressions.log` |
| Searchable Help across all 29 languages | 30 passed; `help-center-final.log` |
| TypeScript, generated API and renderer production build | Passed; `reader-chrome-types-final.log`, `reader-flow-api-final.log`, `reader-chrome-build-final.log` |
| Translation validation/completeness/consistency | Passed; `reader-chrome-i18n-final.log`, `reader-chrome-translations-final.log`, `reader-chrome-consistency-final.log` |
| Translator handoff | 29 packs exported to `build/translator-review-20261115-flow`; `reader-chrome-review-export.log` |

The build retains its existing large-chunk warning. Translation completeness
means authored strings, not linguistic approval: preview languages have 1,551 of
1,551 messages with no missing catalog copy; native review and contextual wording
review remain pending. New expand/restore labels are tracked against their English
source without marking them translator-approved.

Earlier failures in `flow-controls-final.log` and `flow-review-final.log` exposed
old selectors after controls moved into menus/settings. Corrected reruns preserve
the original behavior assertions. `reader-tabs-final.log` also found an expanded
reader focus-restoration defect, fixed in the passing 23-test rerun. One Italian
fixture failed while source edits reloaded the dev server; it passed in the stable
rerun. No failed or unrun broad suite is claimed as entirely passed.

The final keyboard/scroll regression reproduced a narrow-layout jump when browser
scroll anchoring reacted to replaced thumbnail rows. The virtual panel now opts
out of scroll anchoring. Home/End also explicitly reveal the target row when the
selected page was already current but its thumbnail had scrolled out of view.
The final seven-test run verifies this in light/dark desktop and narrow Arabic,
plus the updated Settings dropdown path and workspace sign-out boundary.

The streaming timing fixture measured p95 text feedback at 99.2 ms and local Stop
feedback at 42.1 ms. These are mocked-stream renderer measurements, not model
generation latency or a hardware-profile qualification.

Visual inspection artifacts include light desktop, dark desktop, narrow Arabic,
two-file tabs and independent page/zoom state under
`build/ui-craft/reader-chrome-verified`. The 37-page fixture checks that the page
canvas retains more than 58% of reader height, the thumbnail list stays bounded,
and the viewport does not overflow horizontally.
The final matching light/dark/RTL captures are in
`build/ui-craft/reader-last-regressions`.

## Memory, archive and installation boundaries

This pass also finishes the documentation/evidence for
`inference-provider-runtimes` task 7.3. Resource/inference checks passed in
`memory-resource-inference-full.log`; server checks passed in
`memory-server-final.log`. The runtime policy and its limits are documented in
[Memory and disk use](../../advanced/low-memory.md). Windows commit admission and
bounded speculative prewarming are safeguards, not a hard total-process RAM cap.

The earlier explicitly authorized archive relocation copied and hash-verified
16,187 old backup files (36,205,546,765 bytes) and 2,398 old preview-build files
(2,607,812,077 bytes) before removing the C: copies. A total of 36.15 GiB moved to
`D:\og-archive\20261115`; the receipt is `receipt.json` there and the operation log
is `build/archive-relocation-final.log`. This is disk recovery, not a claim that
36.15 GiB of runtime RAM was freed. Current models, workspace and desktop profile
were preserved; other recovery snapshots were not automatically pruned.

An earlier, separately authorized install completed as
`0.4.31-preview.20261115114714884.gda2a178e`, with 1,198 payload files verified,
workspace/profile unchanged and no automatic launch. Its rollback is
`D:\offgrid-install-backups\surface-memory-20261115-final`; receipt/log:
`build/surface-memory-install.log`. **The latest reader/Library/streaming/deletion
source changes described above are not in that installed build.** No installation
of these latest changes, new offline packaged run, native-language approval or
physical 8 GB/16 GB/model benchmark is claimed here.
