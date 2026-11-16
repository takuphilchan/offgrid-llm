# Work surface redesign - 2026-11-15

## Delivered implementation

Home now provides recent conversations and direct document workflow entries. The shared sidebar has Home, Chat, Library, Activity, Agents and Models, with a collapsed rail and narrow-screen drawer. Existing route identities remain valid. Chat remains available without creating a project or granting host-folder access.

The composer has one primary Send/Stop location, bounded pending activity and source chips. Empty host-folder scaffolding is removed. Accepted work retains the existing canonical inbox/runner. Changed drafts cannot replace an unresolved request identity. Compatible canonical conversations now accept document, image and Knowledge references; unsupported services retain explicit capability handling.

Document reading uses a compact toolbar, collapsed coverage/thumbnail details, a large canvas and bounded resizable panes. Keyboard resizing supports RTL; narrow screens use tabs. "Ask about this page" explicitly pins an immutable attachment or Library extraction revision and page. "Ask about this selection" pins a Library block. Viewing a different page does not change the submitted scope. Whole-document retrieval remains a separate selection. PDF figures are not silently treated as interpreted evidence.

Comparison and synthesis occupy the main Library work surface instead of large modal dialogs. Existing persistent analyses, revision-aware review, composition and export endpoints are reused. Unsaved review notes recover across navigation and reload in private actor/service draft storage; restoring a note does not publish it. Service receipt checks still precede display and optimistic revision checks precede publication.

## Implementation seams and six journeys

| Journey | Reused authority / change | Evidence |
| --- | --- | --- |
| Quick request / follow-up | ChatPage, canonical inbox and runner; compact pending status | canonical renderer tests, full Go suite |
| Document question | Immutable attachment store; additive typed references and explicit page scope | parser/page, HTTP admission/preparation/restart, renderer snapshot tests |
| Summary / requirements | Existing synthesis and review stores; main workspace frame | 29-language synthesis tests, recovery/revocation/export checks |
| Compare versions | Existing deterministic comparison and history | 29-language comparison/evidence/export tests, desktop RTL screenshots |
| Resume work | Existing sessions, analyses and draft scope | Home recents, review reload, request reconciliation and restart tests |
| Failure recovery | Existing task/model/capability guards | model-load draft retention, uncertain save, stale/revoked evidence and compact Stop checks |

The navigation does not own execution. The reader's viewed page does not own submitted scope. Server state owns accepted work/results; browser draft storage owns only unsent inputs, pane preference and unpublished review notes.

## Actual verification

Windows x64, Node 24.18, Go 1.26.6, Edge headless fixtures, GOMAXPROCS=2. No installed-user data was used for renderer fixtures. Logs below are in build/ and are not model-quality evidence.

- go test -p 1 ./... passed (redesign-go-all.log). The added CLI page/source case passed separately (redesign-cli-source.log). Targeted source admission/preparation/restart and legacy-write guards passed.
- TypeScript check, generated API check, Vite production build, OpenSpec validation, i18n static/completeness/consistency checks passed. All 29 rendered-language audits passed (redesign-rendered-i18n.log), including Home.
- Desktop unit suite: 206 passed, 1 skipped. Preview-builder test: 1 passed.
- Comparison/reader/synthesis integrated runs passed their locale and recovery cases; initial failures were stale modal/coverage/Home selectors and were corrected. Focused final integrated journey run: 14 passed (redesign-final-ui.log).
- Final pane/review additions: 4 passed; initial reload test incorrectly expected an ephemeral active-view ID to survive reload. It now reopens the persisted result from history and passes, proving unpublished note recovery (redesign-review-reload.log).
- Screenshots inspected: Home desktop/light/dark and Arabic narrow, comparison English/Arabic, resizable reader, and 200% review keyboard flow. Test documents/model responses are explicitly fixtures.
- Translator review packs exported for all 29 languages to build/work-redesign-translator-packs. No native linguistic sign-off is implied.

## Full-plan follow-through

Library page/block scope now uses the existing canonical inbox and retained extraction graph. Selection is bounded, must name an existing immutable revision, preserves native offsets and linked table headers/footnotes, and checks current collection access before and after retrieval. It bypasses semantic-search requirements without bypassing document permissions. Broad and precise scope cannot be combined accidentally. Context limits reject an oversized selection rather than silently truncating it. Generated OpenAPI/TypeScript, CLI `--knowledge-selection` and Python payload parity use the same contract.

Scope chips and the unsent question persist in actor/service-bound draft storage across reload and transfer to a newly created conversation. A queued "Use in Chat" agent selection now waits for history restoration so it cannot be overwritten by an older selection. Active Talk keeps its single Stop Talk control while generating; cancellation retains the transcript and prevents speech or another capture. An obsolete higher-specificity sidebar gradient rule was removed so the redesigned navigation has the intended flat surface.

### Integrated verification and corrections

- Full Windows suite: `go test -p 1 ./...` passed (`build/redesign-go-integrated.log`). Focused source selection, HTTP/restart and CLI regressions passed (`redesign-scope-go-final.log`).
- CI race package set: successful package results are recorded in `redesign-race-complete.log`, `redesign-race-native-final.log`, and `redesign-race-rag-fixture.log`. The first run caught an incorrect internal input-type reference, corrected before the final build. Windows-mounted subprocess compilation exceeded test deadlines; the native WSL source copy passed server, CLI and executor. Its missing synthetic PDF fixture was corrected, explicitly included in `.gitignore`, and the full RAG race package passed. No failing race invocation is described as a pass.
- Python SDK/runtime suite: **184 passed**, WSL Ubuntu Python 3.14.4 (`redesign-python-complete.log`). Generated API, TypeScript, static translations, completeness and consistency checks passed. All 29 locales contain 1,527/1,527 authored application messages; translator packs were regenerated. Authored coverage is not native-speaker approval.
- Renderer inventory: **2,500 registered tests accounted for; 2,454 passed and 46 explicitly skipped opt-in cases**, across staged runs and corrected focused reruns. `build/redesign-ui-coverage.json` maps the inventory to logs. Skips include external-service/model and optional performance/capture scenarios; they are not counted as passes. Real isolated service navigation/history/catalog/authentication tests passed separately (4 tests, `redesign-real-storage.log`).
- Initial renderer failures included stale widths, route names and translated introduction expectations; these were aligned with the implemented contract. Multi-language loop tests now have a total deadline sufficient for all their navigations while individual assertion deadlines remain unchanged. Actual agent handoff, voice cancellation and sidebar style defects were fixed. Final focused corrections: 18 passed, followed by the remaining three shell cases passing (`redesign-final-fixes.log`, `redesign-shell-tail.log`).
- Desktop unit suite: **206 passed, 1 skipped** (`redesign-desktop-final.log`). The production renderer build passes; Vite's large-chunk warning remains recorded. No automatic dependencies or models were downloaded.
- Screenshots inspected include the current light desktop conversation/sidebar, narrow Arabic dark navigation, document extraction/review and earlier reader/result/theme captures. Fixture content is not presented as live model output. No visible computer-use session was used.

### Performance, real inference and recovery

The opt-in production comparison uses the retained pre-redesign bundle and current production bundle, identical fixtures, 40 measured warm navigations after warmup, and one Edge worker. On this 16 GB host: baseline navigation p95 **16.5 ms**, candidate **14.3 ms**; baseline JS heap **12,155,500 B**, candidate **12,361,964 B**; both **180 API requests**; first visible submission acknowledgment **15.7/16.1 ms**. The fixture feedback check with 1,000 history entries measured **18.7 ms p95** and bounded rendered rows. Logs: `redesign-production-performance.log`, `redesign-final-fixes.log`. JS heap is not whole-process memory; these measurements do not qualify an 8 GB CPU deployment.

`scripts/test/test-workspace-real-model.mjs` ran the installed Qwen2.5 3B Q4_K_M and local llama.cpp runtime against disposable service state. It hard-linked the model into a separate model directory without modifying the installed workspace. Actual durable streamed answers greeted the user, cited [S1] for the July 31 rural exception and USD 17.25 budget, and explicitly declined the absent Figure 9 value. Recorded latencies: 2,593 / 795 / 796 ms. Answers and saved evidence are retained in `build/workspace-real-model-xk3hnz`; log `redesign-real-model.log`. This is a three-case smoke check, not the 1,000-question benchmark or independent claim verification.

`scripts/test/test-workspace-upgrade.mjs` passed old-build creation, candidate reading, canonical import/admission, forced service restart, idempotent re-admission and rollback using a matching older data snapshot. It verified workspace identity, owner, counts, immutable original bytes/hash and revision. The accepted user input appears exactly once; no model was invoked. Receipt: `build/workspace-upgrade-Q7oXly/receipt.json`; log `redesign-upgrade-rehearsal.log`. Installed-user data was not used for this rehearsal.

## Open qualification gates and limits

The work/results redesign implementation and its automated contract checks are delivered. This does **not** close the original seven-phase document qualification program. Physical 8 GB CPU testing, representative real-model repetitions across every journey, the licensed 200-document/1,000-question corpus, native-language expectations/review and departmental pilot evaluation remain open. The available host and three real answers are not substitutes for those requirements. OpenSpec 43.16 retains that distinction. No aggregate completion percentage is asserted.

The implementation continues workspace-outcome-experience, agent-conversation-workflows and document-ingestion-reliability. No new executor, vector store, cloud dependency or automatic model download was introduced. Historical installation receipts below are retained; the final follow-through package has its own receipt.

## Earlier preview packaged validation

Final preview: 0.4.31-preview.20261114171251787.gda2a178e. Renderer build identity: f641c1de5edb161c582acc771828d3965410a8ce603e5e6fdcd5dfcdff599c4a. The builder completed its asset/trust inspection and wrote preview.json; Vite's chunk-size warning is recorded as a PowerShell NativeCommandError stderr wrapper, not a failed compilation. This unsigned local preview is explicitly not release-qualified.

The real packaged Electron main/preload/renderer and bundled Go service passed the isolated startup suite using a disposable profile, loopback fixture, and hidden native window. It exercised service mismatch/recovery, compatible cross-version attachment, restart, waiting for an unavailable external workspace, reconnect, mixed-bundle rejection with restored test metadata, timeout and keyboard recovery. Hidden-window mode replaces the stock visible-window assertion; it does not claim a visible-window launch test. First window was 849 ms, first recovery 1056 ms on this machine; these are single startup observations, not a p95 or hardware qualification. Log: build/redesign-packaged-startup.log. Evidence: build/test-temp/offgrid-desktop-startup-N5b4RD.

## Earlier preview installed result

Installed version 0.4.31-preview.20261114171251787.gda2a178e in the existing per-user application directory. Installer exit code 0; installed service --version and renderer identity agree with the preview receipt. All 1,191 application payload files passed SHA-256 comparison. Existing workspace file hashes/inventory and desktop profile hashes were preserved. App left closed.

Rollback backup: build/work-redesign-install-backup-20261115. Receipt: build/work-redesign-install-backup-20261115/installed.json. Installer SHA-256: E789DB07C13124664E96DEE592B2B30A35C2A35480B40CB29DCA5D4BF151D144.

The initial post-install verifier included debug.log, which the hidden packaged test created after the installer was built. It stopped on that absent, non-payload diagnostic. Verification then excluded only that identified test log and passed all application and data checks; the installer was not rerun. The original diagnostic remains preserved in the build directory.

## Final follow-through package

Package **0.4.31-preview.20261115044922071.gda2a178e**, renderer identity **9fab6d66ac277da9691f5de4587d2df4c314073586d872ce60d96ce0c9a932ec**. The complete local builder and artifact inspection passed (`build/redesign-final-package.log`). This remains an unsigned local preview, not a signed public release. Hidden packaged Electron/service startup and recovery passed with a disposable profile: first window 421 ms, first recovery 474 ms (`redesign-final-startup.log`, evidence `build/test-temp/offgrid-desktop-startup-MTrTKP`). These startup numbers are single observations.

The final production bundle was measured again after the voice/sidebar fixes. Baseline/candidate warm navigation p95: **16.0/15.3 ms**; JS heap: **12,154,108/12,329,832 B**; requests: **180/180**; immediate acknowledgment: **16.0/13.9 ms**. The stated regression bounds pass (`redesign-final-production-performance.log`).

Backup preparation initially ran out of space while duplicating a legacy nested `desktop-workspace/models` GGUF. Installation had not started. Recovery verified the existing application/profile copies and backed up non-model workspace files, excluding `models` and `bin` path segments at every depth. The full installed workspace inventory, including models, remains recorded for before/after verification. No original or old build artifact was deleted. Verified backup: `build/work-redesign-install-backup-20261115-followthrough`; logs `redesign-final-backup.log` and `redesign-final-backup-recovery.log`. The original earlier rollback backup also remains intact.

Final installation completed with installer exit code **0**. All **1,191 application payload files** match the packaged hashes; workspace inventory and backed-up data/profile hashes are unchanged. Installed service version and renderer identity match the package above. The app is left closed. Receipt: `build/work-redesign-install-backup-20261115-followthrough/installed.json`; log `build/redesign-final-install.log`. Installer SHA-256: `EAE09309A514BD0C576AF73B7B5211138949857BBCF49A2DFAC7D557B342307F`.
