# Document workspace and PDF recovery — November 3, 2026

## Scope and limits

This records the approved document-reader increment in
`workspace-outcome-experience` and `document-ingestion-reliability`. It is source
implementation and Windows automated evidence, not model-answer, all-platform,
enterprise or human-usability qualification. The previously published v0.4.24 tag
does not contain this increment. Local installation evidence is recorded below.

## Reproduced PDF problem

The supplied 37-page PDF contains native text and visual material. The old worker
classified both image objects and PDF Form XObjects as requiring OCR. Forms are
containers, including native/vector content, so that classification was too broad.
The attachment preparation path then discarded partial native text when OCR was
unavailable. Its recovery message suggested scanned pages and blocked the whole
attachment.

The revised pinned worker inspects form children with a shared object budget and
depth limit. Raster content, or a structure beyond those limits, remains partial.
Bounds do not discard already extracted native text or certify visual understanding.
Worker/service revision is `pdfium-8057-0001cb839e62425b-offgrid2`.

The supplied source was inspected in disposable local state, never uploaded to a
remote service or committed to the repository. Both worker versions recovered
171,306 characters across 37 pages. Pages flagged for OCR/review decreased from
12 to 6: 1, 6, 11, 12, 36 and 37. The authenticated service rendered the original
page and accepted an explicit native-text-only revision. No model request or
conversation message was created. Original bytes were unchanged.

This is not successful OCR of those six pages. Their native text remains available;
unread visual content is excluded from the accepted coverage and disclosed in the
model input. Fully scanned documents without native text still require OCR.

## Implemented behavior

- A right-hand reader opens from pending/saved attachments and verified Knowledge
  citations. Narrow layouts switch between Chat and Document without losing the
  draft or reading position. Changing conversation/workspace clears the reader.
- PDFs render one requested page locally, with paging, zoom and extracted-text
  search. Other supported formats explicitly identify structured-text previews.
  Original downloads remain available. Unread pixels are not included in search.
- Citation opening first verifies the stored digest, extraction revision, byte
  range and content hash using the existing source API. Native matching passages
  are highlighted; verified OCR excerpts remain separately labeled when absent
  from a native-only preview. Legacy sources retain the text-dialog fallback.
- Preview routes require current source access, reject changed expected digests,
  and recheck access after bounded parsing/rendering. Responses are non-cacheable.
  Closing or changing source aborts requests and releases page object URLs.
- Partial attachment text is retained but cannot be used until explicit acceptance.
  Accepted coverage is revision-bound and survives restart. Ordinary retries never
  silently accept partial output or publish it to Knowledge.
- Scan motion follows actual pending preparation/render requests. Reduced-motion
  preferences receive static feedback. Attached-source status during generation
  does not assert that every page was examined.
- CLI `/preview-attachment N [unit]` and `/attachment-text-only N` use the same
  authenticated contracts. Output escapes terminal controls; long display text is
  explicitly truncated without changing the stored source.

## Checks and failures retained

Commands ran from the repository root on Windows with `CGO_ENABLED=0`:

```text
go -C runtimes/pdfium test -mod=readonly -count=1
go test ./...
go test ./internal/serviceclient ./internal/cli -count=1
go test ./internal/server ./internal/serviceclient ./internal/cli -run 'TestAuthenticatedSessionIsolation|Test.*Preview|TestAttachmentPartial' -count=1
npm --prefix apps/web run api:check
npm --prefix apps/web run build
npm --prefix apps/desktop test
node scripts/maintenance/check-docs.mjs
node scripts/maintenance/check-repository-hygiene.mjs
node scripts/maintenance/check-source-size.mjs
openspec validate document-ingestion-reliability --strict
openspec validate workspace-outcome-experience --strict
```

All passed. Desktop: 121 passed, one platform-specific skip. The build retains its
existing large-chunk warning. Windows race instrumentation was not run.

The disposable-service browser command:

```text
node scripts/test/test-web-workspace.mjs build/document-reader/offgrid.exe document-reader.spec.ts chat-files.spec.ts chat-file-knowledge.spec.ts knowledge-citation.spec.ts chat-images.spec.ts chat-streaming.spec.ts agent-chat.spec.ts voice-playback.spec.ts terminal-overlay-menu.spec.ts workspace-shell.spec.ts workspace-experience-quality.spec.ts --workers=2 --reporter=line
```

Result: **237 passed, one skipped** in 3.6 minutes. The skip is the opt-in private
sample test, separately passed with the user's supplied PDF. A prior document-only
run with that private sample passed all 66 tests. Nine locales, RTL, reduced motion,
enlarged-type/narrow viewport, actual PDF raster, explicit partial acceptance,
stale-source refusal, keyboard closing and draft preservation are covered. The
existing 1,000-history-item fixture measured 34.4 ms p95 feedback against its
250 ms gate; this is DOM/fixture performance, not PDF processing or inference speed.

After the final tab/chevron styling, the reader and citation subset passed again:
27 passed, one private-sample skip. The final CLI/service-client suites passed,
including rejection of malformed source digests before transport. The isolated
service processes had exited after the checks; no test service was left running.

Earlier checks caught and corrected:

1. Bounded form traversal initially rejected the supplied complex PDF outright;
   it now preserves native evidence and marks unresolved structure partial.
2. Attachment action labels wrapped mid-word in the narrow input slot. Attachments
   now occupy the full composer width, with wrapping action rows.
3. The saved-file picker covered a draft remove button after selection. Selecting
   a source now closes its owning popover and returns focus to the composer.
4. Keying the entire document wrapper by conversation detached Chat's existing
   scroll observers. Only the reader resets now; the growing-activity follow-latest
   regression passes without stealing manual scroll position.

Screenshots were inspected for the supplied paper, light split layout, narrow
layout and dark RTL/enlarged-type layout. Real browser/Electron 200% zoom and
screen-reader/manual-language review remain separate from CSS-viewport fixtures.
No model-answer accuracy, figure comprehension or organization compliance claim
is made. Existing enterprise and human-evaluation tasks remain unchecked.

Local logs are under ignored `build/document-reader/`. Private sample captures
remain outside the repository. A fresh matching service/PDF worker and renderer
must be installed together; replacing only the UI does not activate the parser fix.

## Verified OCR setup and bounded recovery

The locally available English pack matched the pinned SHA-256
`7d4322bd2a7749724879683fc3912cb542f19906c83bcc1a52132556427170b2`.
No download or external document transfer was used. With that pack, the supplied
paper still timed out on page 1 (48.65 seconds including native parsing). This is
not evidence that all six visual-content pages are scans or that installing a
language pack makes figures understood. The original OCR path returned no partial
result on that operational failure.

The recovery correction retains verified native text and completed OCR receipts
after a bounded OCR timeout/unavailability/failure, stops additional expensive
page work, and returns `document_ocr_required`, not success. Invalid protocol,
language identity, output limits and caller cancellation still reject the result.
The supplied paper then retained all 37 native page units; pages 1, 6, 11, 12, 36
and 37 remained flagged. Its 45.74-second OCR attempt did not become a full
extraction claim.

Checks after this correction:

- `go test ./internal/rag ./internal/server ./internal/sessions ./internal/cli ./internal/serviceclient`: passed.
- Actual bundled worker tests `TestParserOCRRealWorker` and
  `TestOCRLanguageActualOfflineImport`: passed in 36.46 seconds. Simple scan,
  180-degree rotation, whole/mixed/same-page and repeated text receipts passed;
  Windows symlink creation was unavailable and was reported by the test.
- The opt-in supplied-PDF browser test, with the verified language imported into
  disposable service state: passed in 1.2 minutes. It verifies the partial error,
  original digest, page rendering, explicit native-text-only recovery and no
  implicit conversation/model submission. This is not model-answer evaluation.
- Source-size, maintained documentation and strict ingestion OpenSpec checks: passed.
- The candidate packaged Electron terminal test passed main-area tabs, interactive
  CLI, `/clear`, draft/cursor preservation, real 200% terminal zoom, and owned child
  cleanup on tab close, reload and app quit. This does not extend the separate PDF
  reader zoom evidence to a real Electron zoom claim.

The local candidate remains version 0.4.24 for installation compatibility, with
the document workspace changes beyond the published release. Its installation
receipt and rollback inventory are retained locally under ignored
`build/document-reader/`; publication requires a separate release.

The user quit OffGrid before replacement. Silent installation completed with exit
code 0. All 786 candidate payload files matched the installed files by SHA-256;
436 existing workspace files retained their hashes, 29 model-directory files
retained their size/modification inventory, and the native CUDA executable
retained its SHA-256. The previous app and non-model workspace files were backed
up externally. The supported import function installed the verified English
language data while holding the workspace ownership lock; it reports
`installed_verified` (broader qualification remains `unverified`). Models were
not downloaded or replaced. The user's subsequent app launch remains a separate
running-installation check.

## Preview correction and local document-model experiment

The installed preview was subsequently checked through `/api/v2/system`: it was
the matching 0.4.24 native service and renderer. That did not prove every page
rendered. The reported page 13 reproduced a separate defect: original-page
viewing incorrectly required valid OCR character boxes. Clipped native glyphs
failed that check even though PDFium could display the page.

The `offgrid3` worker now separates `--preview-page` (`preview1`) from OCR raster
receipts (`raster2`). Preview pixels cannot be accepted as OCR geometry. Both
paths remain bounded and share extraction admission. All 37 pages of the supplied
paper rendered successfully through the new worker (1.76–2.83 seconds per page
in this run). These measurements establish rendering, not text/figure accuracy.
The installed app still has the earlier worker until a subsequent replacement.

Reader controls now use up/down arrows, Page Up/Page Down outside editable
controls, a compact original/text selector, and page-render retry. Preparation
uses a moving border with reduced-motion and forced-colors fallbacks. The focused
browser run passed 28 tests; two real-service/private-file cases were skipped in
that fixture-only invocation. Tests observe the animation angle changing, not
only the presence of an animation class. The production renderer build passed.

### Approved download and benchmark

The user explicitly approved downloading the full-precision Granite Docling
model/projector for an isolated benchmark. Both files were downloaded from
`ggml-org/granite-docling-258M-GGUF`, pinned to commit
`684c1800c5fbd614e29b4d1a82177e6c122af1f3`:

| Asset | Bytes | SHA-256 |
| --- | ---: | --- |
| `granite-docling-258M-f16.gguf` | 331641024 | `282f2192e9baa25e9219f47e68367777ba46adaec8f97d04db58dc584b500976` |
| `mmproj-granite-docling-258M-f16.gguf` | 190031552 | `3b284a5869bf904427c3dc18d7af91fd4c9778fcddddb8c2256f02142c1fb58d` |

The existing native CUDA runner used a private authenticated loopback port,
8,192-token context, one slot, temperature zero and a 4,096-token output cap.
It received local PNG pages and the model's documented conversion instruction.
No cloud inference, chat-model changes, installed-model registration or package
installation occurred. The benchmark terminated its own runner afterward.

The first run omitted structural tokens under the runner's default decoding.
This was a benchmark-adapter error, not evidence that the model cannot emit
DocTags. The corrected run enabled `--special`, preserving the model's typed
tags as required by the [IBM example](https://huggingface.co/ibm-granite/granite-docling-258M).
The pinned local runner source confirms that switch controls token decoding.
Both runs and their failures remain in private local evidence.

| Sample page | Corrected render + request wall time | Output tokens | Result |
| --- | ---: | ---: | --- |
| 1 | 6.30 s | 1076 | Normal stop; not a fidelity qualification |
| 13 | 8.61 s | 1810 | Normal stop; transcription errors found |
| 36 | 5.03 s | 283 | Caption and picture regions; missing plot labels |
| 37 | 6.52 s | 1249 | Lists and picture regions; small denominators corrupted |

Manual source comparison found a decimal value on page 13 changed from `.338`
to `.333`, along with damaged mathematical notation. On page 37, later
`/1000` denominators became `/1/`. Page 36's caption was recovered but several
plot titles/legend labels were absent. Picture tags do not supply chart data or
prove semantic understanding. These are disqualifying findings for replacing
reliable native text with this model's full-page transcription. Four pages on
one GPU do not establish overall accuracy, CPU latency or language coverage.

An earlier native-text masking experiment also failed overlap/geometry checks;
it was removed from the production path. Its failure logs and experimental
source remain under ignored local build evidence. Existing native text is not
silently rewritten to make either experiment appear successful.

### Architecture decision supported by this experiment

Use a hybrid document pipeline, not OCR-only and not VLM-only:

1. Render originals independently of extraction quality. Preserve immutable
   originals and page identity for inspection and citations.
2. Retain native text and geometry as the primary transcription when available.
   Model-generated layout must not overwrite native characters or numeric values.
3. Apply OCR to scanned text regions; preserve its provenance and uncertainty.
   A raster image is not automatically a scanned page or a readable chart.
4. Add separately qualified document-layout/table/equation adapters and optional
   local visual interpretation for relevant regions. Preserve block coordinates,
   model/projector/runtime fingerprints and conflicts with native evidence.
   Granite Docling is an experimental candidate, not a selected production default.
5. Retrieve source-linked text, tables and visual regions through one access-
   checked document representation. Answer verification must distinguish exact
   extracted facts from model interpretation and omitted visual content.

Future model work must use shared inference scheduling, explicit asset readiness,
bounded per-page jobs and cancellation. It must not spawn an independent model
server from the ingestion request as the isolated benchmark does. General users
should see document readiness and a simple setup choice; language/model/runtime
selection belongs in optional Document support details, not the Chat model menu.
Benchmark permission does not install or enable a production document model.

Before activation, freeze expected fields, numbers, reading order, table structure
and formulas across the existing corpus; test native/model conflicts, scans,
multilingual input, malformed/truncated tags, cancellation and retained partial
results. Broader visual interpretation requires its own answer-quality tests.
The enterprise retention and organization isolation gates above remain open.

Final checks for this correction: the five affected Go packages (`rag`, `server`,
`sessions`, `cli`, `serviceclient`) passed; the renderer build passed with the
existing chunk-size advisory; documentation, source-size and strict ingestion
spec checks passed. A newly built service with the matching `offgrid3` worker
passed the private-PDF browser test with verified English OCR in 52.3 seconds,
including opening page 13 through the authenticated preview endpoint. This
uses disposable service state and performs no model answer. The latest renderer
and worker corrections have not yet replaced the installed desktop preview.

## Follow-up installation requested by the user

The subsequent explicit installation request was completed while OffGrid was
closed. The newly packaged candidate passed the private-PDF service/browser test
(53.0 seconds, including page 13) and packaged Electron terminal tests for tabs,
interactive CLI, cursor/draft preservation, zoom and owned-child cleanup.
All 24 packaged UI files matched the tested renderer; the service matched the
freshly built native executable.

The first installation-wrapper invocation stopped before starting the installer
because PowerShell wrapped the JSON manifest in a nested array. Removing that
extra wrapper fixed the preflight; no installed files were changed by the failed
invocation. The installer then exited successfully. All 786 payload files matched
the candidate by SHA-256, all 439 preserved workspace files retained their hashes,
and all 29 model files retained their inventory. The CUDA runtime and verified
OCR language data remained unchanged. A fresh external app/workspace backup was
retained for rollback.

Installed worker: `pdfium-8057-0001cb839e62425b-offgrid3`.
Installer SHA-256:
`98713a5e7855c44188ba93cac42e4fe92c2b6465f0c7f27dd80990b187d5d486`.
This is the updated local 0.4.24 preview, not a replacement of the published tag.
Granite's benchmark assets were not registered or enabled in the user's app.
User-visible app launch remains separate from installed-file verification.

## Readable native text with explicit visual limits

The user approved making the paper's native text usable immediately while
disclosing unread figures. Worker revision `offgrid4` distinguishes substantial
native text with bounded visual regions from scans requiring OCR. The admission
heuristic requires at least 200 native letters/numbers and conservative enclosing
visual bounds below 60% of the page. Unknown geometry, sparse native headings,
large scanned surfaces and unreadable pages do not pass. These thresholds are
conservative admission rules, not evidence that figures were interpreted.

Normal Chat upload and explicit retry of an old failed attachment now persist
`ready` with `coverage=native_text_only` when every page has usable native text
or is empty. Extraction remains partial. The original bytes, source page links,
visual limitations in model context and immutable prepared revisions are retained.
UI drafts, saved files and CLI attachment listings disclose the visual limitation.
Knowledge's partial-source handling is not silently promoted to full coverage.

The supplied 37-page PDF passed isolated upload, saved-failure recovery, exact
native-text preservation and CLI inventory checks with no OCR language pack.
Its six visual pages remain identified; page 13 renders through the original-page
endpoint. The fresh CLI/service regression took 26.38 seconds while packaging
was running. The service callback verifies source admission and coverage, not
actual model answer quality.

Validation: full `go test ./...` passed; the final changed CLI/server packages
passed again. The PDF worker suite, native-text/scan admission regressions,
packaged document corpus and partial-import tests passed. Renderer build passed
(existing chunk-size advisory); browser fixtures passed 20 tests with two
environment-gated skips. Those private-PDF/service cases were exercised separately;
the packaged private-PDF browser run passed. API generation, documentation,
source-size and strict OpenSpec checks passed.

Earlier checks failed because C: was full; rerunning with test temporary files
on D: passed. A fixture initially omitted the system identity endpoint, and one
PowerShell invocation misparsed unquoted test flags; both harness issues were
corrected before the successful runs. These failures are not model qualification.
Cross-platform native checks, visual understanding and the wider enterprise and
human-usability gates remain unverified.

The user then requested installation and closed OffGrid. A pre-install hash
comparison correctly stopped because the development worker had different build
flags from the packaged worker (`-trimpath`); nothing was installed by that check.
The exact packaged worker passed the fresh upload/retry/CLI test in 17.90 seconds.
The packaged service matched the final tested service executable by SHA-256.

Installation completed successfully: all 786 payload files matched the candidate,
435 workspace files retained their hashes, all 29 model files retained their
size/timestamp inventory and the CUDA executable hash was unchanged. All 788 old
application files were hash-verified in the external rollback backup before
replacement. Installer SHA-256:
`7ef5f1f13fc7d3fd1f5740cdaa769f3fa80d3b7fedf554540d4219c19cbef799`.
This remains a local 0.4.24 preview; no published release/tag was replaced.
Existing failed uploads require one explicit preparation retry. No saved question
was submitted or replayed, and user-visible app launch remains unverified.

## Bounded document context and attachment layout

The subsequent screenshot exposed a separate problem: ordinary conversation
assembly inserted complete extracted documents into an 8,192-token interactive
allocation. A new conversation did not solve that mismatch. Assembly now validates
and deduplicates source revisions, preserves history, reserves answer/protocol
space and selects exact passages within a conservative serialized-byte budget.
Original files and full extracted text remain stored. A selected-source receipt
persists exact extracted-text byte ranges and is rejected in client-seeded history.
Web and CLI independently disclose limited coverage. This lexical retrieval is
not complete-document analysis, a semantic-search benchmark or figure recognition.

The reported PDF was tested against the installed local Nemotron model using
disposable service/model state and the native CUDA runner. The unchanged 8,192-token
allocation produced a complete answer with `finish_reason=stop`: 27.84 seconds,
32 streamed deltas, 399 completion tokens, 16.87 seconds cold loading and 5.91
seconds prompt processing. The answer identified the paper's title and central
topic and disclosed selected-excerpt coverage. The 174,401-byte extracted source
was represented by four exact ranges. This focused regression is not the wider
answer-quality qualification. An earlier 512-token probe hit its answer limit;
the final probe used a bounded 1,024-token response allowance and stopped normally.

Commands/evidence for this increment:

- `go test ./...`: passed (`build/document-reader/context-go-full.log`).
- `npm --prefix apps/web run build` and `api:check`: passed; existing chunk-size advisory remains.
- `npm --prefix apps/web run test:e2e -- tests/document-reader.spec.ts tests/chat-streaming.spec.ts --workers=1`: 42 passed, two environment-gated skips, two stale fixture assumptions failed. The Stop observer was updated to the new recovery region and the saved-history fixture now supplies selection metadata through the session list. Both passed on targeted rerun; no timing threshold was relaxed.
- UI fixture stream p95: 89.7 ms; local Stop feedback: 34.7 ms, both within 250 ms.
- Narrow attachment checks at 390 px and 1280 px split-view passed; screenshots inspected. Metadata no longer collapses to one character per line. Reloaded filename/coverage rows and response-settings recovery passed.
- `node scripts/maintenance/check-docs.mjs`, `check-source-size.mjs`, strict ingestion OpenSpec validation and `git diff --check`: passed.

The first recovery test used an obsolete send selector and an ambiguous nested
popover selector; both were corrected. The isolated model harness initially used
an unavailable Python alias/dependency, then ran with the existing interpreter
and standard-library HTTP only. The first packaging invocation used the repo root
instead of the desktop package directory and stopped without changing an install.
No new dependencies or models were downloaded. Human usability, cross-platform
native behavior and full document-answer/enterprise gates remain unverified.

The exact packaged service then passed both the mixed-PDF recovery and supplied
PDF browser cases (2/2, 49.6 seconds). Packaged Electron terminal checks passed,
including tabs, Unicode input, retained cursor/draft, reload, 200% zoom and owned
child cleanup. The packaged service SHA-256 matched the tested final build.

Installation completed under the user's explicit request with OffGrid closed.
All 788 old app files matched the rollback copy. All 786 installed payload files
matched the candidate, all 438 workspace files retained their hashes, all 29 model
files retained their size/timestamp inventory, and the native CUDA runtime hash
was unchanged. Backup: `D:\OffGrid-install-backups\document-context-20261103-184441`.
Installer SHA-256:
`2a975b340ceb4972c96a62e1ff159fe3128d910c87b16248dfd85ea8390a4b45`.
This is the updated local 0.4.24 preview; published tags/assets are unchanged.
No failed question was automatically resubmitted. Installed-user startup remains
separate from the successful isolated packaged checks.

## Follow-up context regression (2026-11-03)

The installed preview's bounded-source selector still charged every UTF-8 byte of
completed history as one token. This rejected follow-ups well before the actual
model allocation was full. The Advanced summary also placed an unlabeled answer
limit beside ?Interactive?, making it look like the context size.

Production source selection now happens under the generation model lease, after
Knowledge enrichment. The complete prompt is measured through the loaded runtime's
input-token endpoint. Candidate excerpt size is bounded, measured and reduced up
to six times without generation replay. Explicit response limits, all history and
original sources stay unchanged. Unavailable counters keep conservative admission;
operational counter failures stop before generation. Genuine exhaustion reports
both the allocated context and answer allowance and offers a lower response limit.
The settings summary no longer presents that allowance as a context size, and all
nine locales explain the shared budget.

An isolated native CUDA test uploaded the supplied PDF once, then asked six
consecutive questions using Nemotron at an unchanged 8,192-token context and
4,096-token answer limit. All six finished with `stop`, retained all twelve messages
and recorded excerpt references. Durations were 22.14, 12.08, 20.11, 13.12, 17.09 and
15.38 seconds; only the first turn loaded the model (10.46 seconds). This proves the
reported early-rejection regression, not unlimited conversation or broad answer
quality. Private results remain under ignored `build/document-reader/followup-model-pk3uqh22`.

Deterministic regressions cover eight successive questions with a source larger
than context, dense Unicode requiring measured reselection, true history overflow,
response allowance consuming the context, cancellation, unavailable/failing counters,
unchanged source bytes/settings and the same model lease for counting/generation.

Validation for the follow-up fix:

- `go test ./...`: passed (`followup-go-full.log`).
- Focused measured-budget, lease, cancellation and attachment regressions: passed.
- Web TypeScript/Vite build: passed, with the existing bundle-size advisory.
- Reader/stream browser suite: 43 passed, two environment-gated skips and one test
  locator failure. The response-limit control was accessible as a named combobox;
  changing the test from exact implicit-label lookup to its accessible role fixed
  the test, which passed on rerun. No product assertion or timeout was removed.
- Packaged Electron terminal/UI check: passed, including Unicode, tabs, cursor,
  retained drafts, 200% zoom and owned child cleanup.
- Source-size, maintained documentation and strict ingestion spec checks: passed.
- Targeted Go race tests were attempted but not run: this Windows Go environment
  has CGO disabled and no gcc/clang on PATH. This is not a race-test pass.

The installed service candidate has SHA-256
`e27fb4b3e55f76c3563c998a0eb08f84759e98aa176b4b2623d283c0395141fe`,
matching the binary used in the six-turn isolated real-model check. This remains
local preview evidence; release-wide gates and unavailable human/platform checks
remain open.

The follow-up preview was installed after the user quit OffGrid. All 788 old app
files matched the rollback copy at
`D:\OffGrid-install-backups\document-followup-20261103-191721`. All 786 candidate
payload files matched the installation, all 450 workspace file hashes remained
unchanged, all 29 model inventory entries remained unchanged and the native CUDA
runtime hash was preserved. Installer SHA-256:
`5050cac464f6b931bb1a3a471d7e4fdfde6b6d7a5a4867b6e6b385e145a3bbeb`.
The app was not launched and no saved question was replayed. This remains a local
0.4.24 preview, not a newly published release.

## Cross-client same-source follow-up check (2026-11-04)

The packaged-worker service regression now sends two CLI turns against each
successful frozen fixture: compressed, font-mapped and object-stream PDFs,
Japanese text, DOCX, XLSX, PPTX and an OCR scan. It checks that the second turn
uses the saved source revision, includes the source context exactly once, retains
four durable messages and does not add another attachment to the follow-up turn.
The isolated packaged-worker run passed all 11 cases. Its inference responder is
deterministic; it verifies source transport and persistence, not the factual
quality of model answers.

The browser regression uploads a frozen text file once, sends two questions,
checks the saved attachment identity and confirms that the second request does
not reattach the file. It passed alone and in the combined document-reader/file
UI suite, whose Playwright result was `passed` with no failed tests. The combined
run used a disposable service state and the installed service binary. The UI
responder was deterministic, so the wider real-model and citation-quality gate
remains open.

Checks for this increment: `go test ./internal/server ./internal/sessions
./internal/cli -count=1`, the isolated packaged-worker test with installed
Windows workers, web TypeScript/Vite build, generated API check, source-size
ratchet, `git diff --check` and strict ingestion OpenSpec validation all passed.
The web build retained its existing bundle-size advisory. No new model, document
or credential was downloaded or added to the installed user workspace.
