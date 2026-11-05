# Workspace design follow-up — 2026-11-05

Scope: current v0.4.25 source, following the [design audit](../../advanced/design-audit.md),
with focused changes to navigation, onboarding, document layout and CLI discovery.
This is incremental implementation evidence, not whole-product qualification.

## Behavior

| Journey | Before | After |
| --- | --- | --- |
| Saved chat model unavailable; another installed model is eligible | Model-management action was primary; the installed suggestion was secondary | Explicit selection of the named installed alternative is primary; other choices remain available |
| First-use guide | Large separate mark and eyebrow, repeated connection explanation and three status rows | Compact heading, visible service state, expandable connection explanation/model count, one primary next action |
| Short desktop navigation | Two columns with collapsed grouping | One vertical sequence; every destination retained |
| First question beside a document | Large centered welcome graphic competed with the source | Compact left-aligned heading; draft and source controls retained |
| CLI first minute | Large command catalog and fixed model examples | Installed-model, conversation and diagnostic commands first; full registry-covered catalog available through `help all` |

The model-selection shortcut remains one deliberate click. This increment improves
action prominence; it does not claim a measured reduction in time to first reply.
No API schema, authority, model selection or user data is migrated.

## Checks

- `go test ./internal/cli ./cmd/offgrid -count=1`: passed.
- `npm.cmd --prefix apps/web run build`: TypeScript and Vite passed. The existing
  large-chunk warning remains (main bundle approximately 1.21 MB before gzip).
- `npm.cmd --prefix apps/web run api:check`: passed.
- `node scripts/maintenance/check-source-size.mjs`: passed.
- `node scripts/maintenance/check-docs.mjs`: passed, 316 maintained files at the
  time of the first run; rerun after this evidence file is added.
- Focused isolated renderer run: 49 passed, one private-PDF check skipped because
  its optional supplied-file environment variable was absent. Covers setup,
  retained selections/drafts, keyboard traversal, nine locales, narrow/RTL,
  reduced motion, 200% layout and document read/recovery. Command:
  `node scripts/test/test-web-workspace.mjs build/ux-audit/offgrid.exe workspace-shell.spec.ts chat-readiness.spec.ts document-reader.spec.ts --workers=2 --reporter=line`.
- `npm.cmd --prefix apps/desktop test`: 126 passed, one platform-specific skip.
- `node scripts/test/test-desktop-theme.mjs "apps/desktop/dist/win-unpacked/OffGrid LLM Desktop.exe"`:
  passed against isolated HTTP fixtures serving the current renderer, including
  saved/native appearance, nine-locale options, all main screens and Help at 200%.
  Local capture directory: `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-theme-IixnGA`.
- `node scripts/test/test-desktop-terminal-packaged.mjs "apps/desktop/dist/win-unpacked/OffGrid LLM Desktop.exe"`:
  passed. Exercises main-area terminal tabs, focus/cursor preservation, search,
  copy, Unicode, interactive CLI, agent draft and `/clear`, service identity,
  blocked OSC clipboard requests, and child cleanup on reload/close/quit.
  Local capture directory: `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-theme-yqJBTL`.

The initial focused run failed two new assertions: a case-sensitive readiness
label and a document test that accidentally loaded a populated conversation.
Those fixtures were corrected. Its real PDF preview also failed because the
isolated service binary had no adjacent document worker. Building the worker
from `runtimes/pdfium` restored the real preview check; no runtime policy was
relaxed. The final focused result above includes that real preview test.

## Remaining gates

The first focused run did not establish full renderer coverage; the follow-up
results below supersede that pending check. No release or installed-workspace
replacement is established by these checks. The first-use offline acquisition
path, complete journey comparison, real-model answers, non-developer usability,
manual screen-reader/translation review and other native platforms remain
separate gates. Existing OpenSpec tasks remain open where these checks cover
only part of their acceptance scenarios.

## Installed-app audit implementation follow-up

The baseline is the installed Windows 0.4.25 application and its served UI,
not the older screenshots in the conversation. The installed UI hash was
`a7cdabbdfe70ab88a3496163a906fd24b562ec7ecec0f1e92658d1765ec33e7b`.
This follow-up remains a source candidate; the installed app was not replaced.

- Saved document answers render validated source references as readable document
  chips. A new authenticated, no-store passage endpoint reads the exact retained
  UTF-8 range from the pinned attachment revision. Legacy offset references are
  accepted only when unambiguous and equal to a retained selection. Forged,
  malformed, out-of-selection and unavailable references do not acquire links.
- A unique exact passage opens its preview page and highlights the text. Repeated
  or cross-page passages display the retained excerpt without inventing a page.
  This verifies source identity and bytes, not whether the source proves the claim.
  Closing the reader restores citation focus. Streaming drafts are not treated
  as validated saved citations.
- Models explicitly identifies the selected model and initially collapses
  cancelled download history; active transfers and failures remain visible.
- Activity and the header use the same attention/running/history classification.
  Filters retain their route after inspection, deletion and reload. This changes
  presentation, not task authority or deletion protection.
- The empty Knowledge library explains its distinction from conversation files,
  with copy in all nine locales, and omits an unusable Ask action.

Unlike the initial navigation-only increment above, this increment includes an
additive API schema operation and regenerated TypeScript types. It does not
migrate installed data, grants, model choices or extraction records.

### Final checks

| Command | Observed result |
| --- | --- |
| `go test ./internal/sessions ./internal/server ./internal/cli ./cmd/offgrid ./pkg/api -count=1` | All five packages passed |
| `npm.cmd --prefix apps/web run build` | TypeScript/Vite passed; existing large main-chunk warning remains |
| `npm.cmd --prefix apps/web run api:check` | Passed |
| `node scripts/maintenance/check-docs.mjs` | 317 maintained files checked, zero problems; external URLs/anchors excluded |
| `node scripts/maintenance/check-source-size.mjs` | Clean |
| `git diff --check` and `openspec validate workspace-outcome-experience --strict` | Passed |
| `node scripts/test/test-web-workspace.mjs build/ux-audit/offgrid.exe --workers=3 --reporter=line` | 1,002 passed, 31 skipped; isolated state |
| Focused `history-controls`, `attachment-citations`, `installed-audit-followup`, `document-reader` renderer run | 46 passed, one optional private-PDF test skipped |
| `npm.cmd --prefix apps/desktop test` | 126 passed, one platform-specific skip |
| `node scripts/test/test-desktop-theme.mjs "apps/desktop/dist/win-unpacked/OffGrid LLM Desktop.exe"` | Passed; current renderer in isolated packaged Windows fixture, appearance and 200% layout |
| `node scripts/test/test-desktop-terminal-packaged.mjs "apps/desktop/dist/win-unpacked/OffGrid LLM Desktop.exe"` | Passed; tabs, cursor, Unicode, CLI/agent interaction, clipboard restriction and child cleanup after reload/close/quit |

Final renderer log: `build/ux-audit/full-renderer-verified.log`. Packaged appearance
captures: `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-theme-9WLvYc`;
terminal captures: `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-theme-i4Y8H0`.
Inspected candidate screenshots include the readable source passage, narrow
Activity, light/dark Models and the RTL/200% reader. The Models screenshot shows
the deliberately expanded download disclosure; the test separately verifies
that it starts collapsed. These fixtures are not installed-user screenshots.

Earlier renderer failures exposed real filter restoration and citation-focus
regressions; both were corrected before the final full run. Existing assertions
were also updated for the deliberate Selected label and explicit All work filter.
The initial packaged terminal check matched both a closing and an active pane;
its locator now targets the visible pane. Process cleanup assertions remain and
passed on the rerun. The initial passage-handler test exposed incorrect error
mapping; the final route preserves the scoped session error contract.

`go test -race` could not execute: CGO is disabled and no compatible C compiler
was available in the checked environment. Race coverage remains unverified.
No real-model citation/follow-up qualification, new installation, human usability,
manual screen-reader/translation review, macOS or Linux native qualification is
claimed. The remaining audit phases and release gates are still open.
