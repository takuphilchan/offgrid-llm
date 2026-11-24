# Structured browser controls — 2026-11-20

Status: implementation and integration verification in progress. This increment
extends the existing managed workspace and retained-target driver. It does not
complete M2, attach personal browsers or qualify any real model workflow.

## Implementation and acceptance mapping

- Task 1.1: `browser_search` performs literal role/name/text filtering within
  existing bounded observations. Cursors retain the query, ambiguous matches keep
  separate exact identities, and full accessible names are searched even when
  their output labels are truncated. Coverage and transport bounds remain visible.
- Task 1.2: open-shadow host/label/form/property freshness; contenteditable
  Unicode/multiline fill and independent text read-back; typed native/custom
  checked-state outcomes; freshly rendered combobox options. Custom clicks remain
  consequential and sensitive controls remain unavailable for agent input.
- Task 2.1: task-page-owned frame IDs/document IDs, explicit inventory/selection,
  ancestor origin scope and no fallback after detach. The origin check uses a
  Chromium isolated realm; retained Playwright Frame/ElementHandles own targets.
- Task 2.2: child/open-shadow protected-field masking, opaque-child whole-surface
  masks, explicit closed-shadow/incomplete-coverage failure, and post-capture
  document/DOM/property checks. Real PNG pixels are decoded and checked in fixtures.
- Task 3.1: companion pairing is additive; Go negotiation/descriptor/dispatch and
  durable read/effect validation are being checked independently.
- Task 3.2: complete companion/desktop/Go/race/web/docs checks remain required;
  focused passing checks below do not replace them.

Source: `apps/computer/browser-dom.mjs`, `browser-text.mjs`,
`browser-observation.mjs`, `browser-frames.mjs`, `browser-capture.mjs`,
`browser-shadow-coverage.mjs`, driver/session/pack and regression tests.
The implemented limits and tool arguments live in
[the companion reference](../../../apps/computer/README.md).

## Executed checks and retained failures

Windows, Node 24.18, pinned Playwright 1.62.1 and its isolated Chromium. All pages,
ports, data and browsers are disposable fixtures; no personal profile/session,
external website, downloaded model or installed service is used.

- Initial structured/discovery/recovery run: **12 passed / 2 failed**. One change
  bypassed the existing native `inputValue` failure injection; native read-back
  was restored and contenteditable read-back added separately. The other fixture
  contained malformed nested HTML quoting; its click handler was corrected.
  Unchanged assertions then passed: **14/14, 9.615 s**, later **14/14, 9.948 s**
  with frame-scoped observation/text integration.
- Frame test initially failed opaque-origin rejection (**3/4 passed**): Chrome's
  `Frame.securityOrigin` returned a sandboxed document's precursor origin.
  The replacement reads `globalThis.origin` in a fresh browser-isolated world
  without universal access. The page deliberately spoofs its main-world origin;
  the unchanged rejection assertion now passes. Focused frame suite: **4/4,
  1.585 s**. Later parent-replacement and approved/denied cross-origin cases were
  added. The cross-origin fixture first omitted its HTML MIME type; correcting
  the fixture response made the actual typed-input scenario pass (**0.868 s**).
- Capture checks initially failed because Playwright's hidden-caret operation
  normalized input style attributes and its mask overlay changed mutation counts.
  Capture now preserves styles and checks exact restored DOM/property/document
  identity, independently of overlay mutation counts. Real child/shadow pixel
  masking, changed-during-capture rejection, opaque whole masking and closed-root
  failure all pass: **4/4, 1.878 s**.
- First complete companion run: **105 passed / 2 failed**, 18.807 s. Both failures
  were old download-error page doubles without a frame lifecycle. Those tests now
  use actual Chromium observations and inject only the two independent failing
  transports; neither assertion nor no-replay requirement was removed. Download
  and pairing suites then passed **13/13, 1.739 s**.
- Source-size ratchet, maintained-doc checks (446 files at that checkpoint) and
  strict OpenSpec validation passed. Full final outcomes will be recorded below.
- The complete companion suite then passed **113/113, 24.284 s**. A stronger
  distinct-site fixture exposed missing origin proof for Chromium frames running
  in separate processes. The driver now binds child CDP sessions through exact
  public Playwright Frame objects and reconciles their browser frame IDs. Actual
  scoped fill, protected pixel masking and closed-shadow rejection pass across
  that process boundary. A preceding fixture syntax error is retained as a
  failed invocation, not behavioral evidence. Frame/capture suites pass **11/11,
  3.657 s** after this correction.
- Earlier complete companion checkpoint: `node --test apps/computer/test/*.test.mjs`,
  **114/114 passed, 19.380 s**, zero skips. Log:
  `build/transformation-browser-structured-process-full-20261120.log`.
  This checkpoint predates the independent-review corrections below.

## Independent review corrections

Review reproduced missing explicit radio/textbox/link selectors, protected text
leaking through associated/descendant labels, and same-address replacement during
a fresh observation publishing mixed document evidence. Exact role selectors,
private-label exclusion and final document/epoch checks corrected those cases;
the focused regressions passed **2/2, 1.684 s**, independently **1.263 s**.

A transient sensitive input inserted only around the actual screenshot escaped
endpoint hashes. The retained failing fixture now passes with a separate
Chromium-world mutation witness and local PNG masking. Further review reproduced
CSSOM insert/delete changes and paused-animation seeks; the three-case suite
initially passed **1/3**, failing the latter two. Native CSS events retain
stylesheet changes, and isolated animation inventory rejects present animations.
The same three strict regressions subsequently passed. Listening indiscriminately
to `CSS.mediaQueryResultChanged` also rejected ordinary unchanged screenshots:
**12/15 passed, 3 failed**. Chromium's capture emits that notification itself;
actual viewport change is covered by isolated resize/scroll events and dimensions.
The corrected combined frame/capture suite passed **15/15, 4.381 s**.

Independent tests also verify transient CSSOM changes in open shadows and a
distinct-site frame process, viewport change/restoration, unchanged media-query
pages, and listener/global cleanup after success and failure: **7/7, 3.488 s**
including the three transient fixtures. The native-animation-freeze experiment
was a disposable probe, not shipped behavior or qualification of arbitrary media.

Review then found private `display: contents` and overflowing descendants outside
their ancestor mask: **1/3 passed, 2 failed**. Masks now include private descendant
and direct-text rectangles with exact frame offsets. Paint that cannot be bounded
is rejected. The unchanged tests require both black protected pixels and visible
unrelated green public pixels. Combined capture/frame/pack checks passed
**28/28, 8.481 s**, zero skips, after this correction.

`pngjs` **7.0.0**, MIT, is pinned in the companion lockfile and copied with the
desktop runtime dependencies. A real isolated temporary payload imports and
executes the masker, and rejects a mismatched decoder version. Pack tests passed
**6/6, 1.052 s** separately. No browser/model/installed edition is certified by
that packaging fixture.

Go integration records full affected normal suites: `internal/computer` **2.239 s**,
`internal/server` **157.251 s**, `internal/agents` **109.921 s**, and
`internal/capabilities` **0.416 s**. After descriptor polish, focused checks passed
**0.198 / 5.189 / 2.915 / 0.451 s**, respectively; focused Linux race checks passed
**1.028 / 1.162 / 2.728 / 1.017 s**. Tests first exposed a real initial-observation
gate blocking new browser reads; exact identity-bound read admission corrected it.
An earlier missing test helper and fixture JSON content type were corrected;
the source-size check also caught added durable-handler lines, resolved by combining
equivalent existing predicates without changing native admission.

## Integrated checkpoint

- Independent paint/witness review extended direct-text and generated/effect
  coverage: **16/16 passed, 3.587 s**. A raw positive control proves visible red
  private glyphs exist; protected output contains no such pixels and retains
  unrelated public content.
- Complete companion: `node --test --test-concurrency=2 apps/computer/test/*.test.mjs`,
  **133/133 passed, 32.692 s**, zero skips. Log:
  `build/transformation-browser-structured-reviewed-20261120.log`.
- Complete desktop: `npm --prefix apps/desktop test`, **267 passed, 3 skipped,
  3.442 s**, zero failures. Skips are opt-in real recovery binary, Unix FIFO
  on Windows, and the existing optional fixture. Log:
  `build/transformation-desktop-structured-20261120.log`.
- OpenAPI generated contract check passed; maintained-doc check **449 files,
  zero problems**, source-size ratchet and strict change validation passed.
  An initial source-size invocation used an incorrect `scripts/quality` path and
  executed no check; the corrected `scripts/maintenance` invocation passed.
- Go snapshot **20261120113414550**, 3,898 tracked/nonignored source files,
  archive SHA-256 `a07d99ae5b0d76547828c72889b3fcd2251d7bcd470cbaf0adc1dc9e817e925a`,
  was verified file by file after extraction to a fresh native Linux directory.
  Go 1.26.6, CGO enabled, offline module cache: `go test -p 2 ./... -count=1`
  **passed, 136.193 s**. Log:
  `build/transformation-20261120113414550-normal.log`. The snapshot precedes
  subsequent CLI source-attachment, recovery Settings and ONNX work; it is exact
  checkpoint evidence, not certification of later changes.
- Same exact snapshot, `go test -race -p 1 ./internal/computer ./internal/server
  ./internal/agents ./internal/capabilities ./internal/inference -count=1`:
  **passed, 607.715 s**, with vet enabled. Package durations were **3.053 / 260.443 /
  250.962 / 1.028 / 46.699 s**, respectively. Log:
  `build/transformation-20261120113414550-browser-race.log`.
- Final shared-renderer `api:check`, `check`, and `build` passed after recovery
  Settings integration. Vite **2.20 s**, existing large-chunk advisory retained.
  Log: `build/workspace-recovery-combined-web-build-20261120.log`. Recovery's
  separate 36 renderer cases and 29-locale coverage are recorded in its own
  evidence; they are not browser model-workflow qualification.

The six implementation tasks in this increment have their scoped evidence.
Broader browser, platform, visual and real-model qualification remains open below.

## Remaining limits and qualification

Closed roots are not structured targets and prevent captures whose protection
cannot be established. Unknown/inherited/opaque frame origins remain unavailable.
Distinct-site out-of-process frames have isolated Chromium coverage; additional
platform/site/process combinations remain part of real-workflow qualification.
Unsupported origin/masking access fails rather than guessing. Large documents
can exceed bounded inspection and produce an explicit error. Screenshot sampling
is not an adversarial atomic snapshot or visual-action qualification.
Present animations/SMIL, protected generated content and shadows/reflections/
filters, and transformed child-frame geometry are explicit capture limitations;
general dynamic visual workflows remain open. No private pixel result is returned
when a witness or paint-coverage check fails.

Signed-in Chrome/Edge attachment, visual-coordinate evidence, richer file
provenance, manual takeover/product integration and the frozen repeated real-model
browser task suite remain required M2 work. No task is complete merely because
its fixtures or source files exist.

## Primary sources consulted

- [Playwright frame lifecycle](https://playwright.dev/docs/api/class-frame)
- [Playwright CDP sessions](https://playwright.dev/docs/api/class-browsercontext#browser-context-new-cdp-session)
- [Chromium Page protocol](https://github.com/ChromeDevTools/devtools-protocol/blob/master/pdl/domains/Page.pdl)
- [Chromium DOM protocol](https://github.com/ChromeDevTools/devtools-protocol/blob/master/pdl/domains/DOM.pdl)
- [Chromium Runtime protocol](https://github.com/ChromeDevTools/devtools-protocol/blob/master/pdl/js_protocol.pdl)
- [Chromium CSS protocol](https://chromedevtools.github.io/devtools-protocol/tot/CSS/)
- [Chromium Animation protocol](https://chromedevtools.github.io/devtools-protocol/tot/Animation/)
- [Web Animations document/shadow inventory](https://drafts.csswg.org/web-animations/#dom-documentorshadowroot-getanimations)
- [pngjs source and MIT license](https://github.com/pngjs/pngjs)

The isolated-origin checks, exact-target ownership and conservative capture
restrictions are OffGrid decisions, validated by the named fixtures; these sources
describe interfaces and do not certify the resulting product.
