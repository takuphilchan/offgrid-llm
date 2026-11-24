# Isolated interface refinement — 2026-11-21

## Scope and ownership

This increment continues `workspace-outcome-experience`. It refines the existing
renderer and shared desktop presentation. It does not deliver new Projects or
Workflows applications, a new execution engine, or a release qualification.

Implementation is in `D:/offgrid-ui-20261121`, a detached worktree based on
`f3aaf3e79595e9ef4940b1f89ee434c9889f9580`. The active thread's dirty/untracked
snapshot was copied into that worktree before editing (627 paths). Hashes and
original file bytes are retained in `build/ui-baseline`. Dependencies, fixtures,
test outputs and builds belong to the isolated directory. Nothing from this
increment was applied to the active checkout or installed application at the initial
implementation handoff. Subsequent user-authorized UI installation is recorded
below. No commit, push or native computer-control test was performed.

## Audit findings and changes

| Finding in the retained renderer | Implemented behavior |
| --- | --- |
| Recent conversations existed in Home markup but were hidden by CSS | Empty Chat offers at most three recent conversations with dates, readable long names and keyboard focus. The original unsent new-chat draft survives opening and leaving a saved conversation. |
| Quick actions was icon-only even in wide windows | Wide workspaces show the existing label and shortcut; compact layouts keep their current footprint. |
| Rail selection relied primarily on surface changes; focus could position drawer labels as rail tooltips | Active destinations have a positional marker. Drawer labels remain inside their rows during keyboard focus. |
| Models, Agents and Activity used competing view-navigation styles | Shared navigation classes provide consistent padding, selected state, focus, wrapping and forced-colors boundaries. An Activity legacy override found during screenshot review was corrected. |
| Management headings, content widths, rows and empty states used inconsistent spacing | Existing tokens now govern the refined typography, content measure, radii, separators and recovery treatments. Existing palettes and translations remain authoritative. |
| Saved answers repeated agent metadata above the result | Completed saved work has a compact status summary, with profile name/revision in Details. Inspect and verification limitations remain visible. Approval, uncertain work and lifecycle controls retain their behavior. |

No new external fonts, runtime packages, requests, downloads, permissions or
translation strings were introduced. Quick Access retains its compact renderer;
the full workspace remains the place for management and detailed inspection.

## Verification

Windows x64, Node 24.18, headless Edge. Browser tests intercept API traffic and use
disposable browser contexts. Vite fixtures use `OFFGRID_FIXTURE_TEST=1` and private
ports. Production comparison servers serve only isolated UI assets and reject
unintercepted API traffic. No installed-user workspace or real model is used.

| Check | Result and retained log |
| --- | --- |
| Generated API contract | Passed: `api-check-after.log` |
| Translation check, audit, completeness and consistency | Commands passed: `i18n-*-after.log`. Authored coverage is not native-language review; preview languages remain marked unreviewed. |
| TypeScript and production build | Passed on the final renderer: `check-final.log`, `build-final.log` |
| Desktop contract suite | 288 passed, 4 skipped, 0 failed: `desktop-tests.log` |
| Broad renderer regression | Initially 184 passed, 17 skipped, 3 failed: `regression.log`. Failures were document-page submission and two Settings startup cases. All three passed against the final production bundle in `rechecks.log`. |
| Readiness, controls and workflow suite | Initially 58 passed, 3 failed: `additional-tests.log`. The three action/startup timeouts passed against the final production bundle in `rechecks.log`. |
| Final interface and screenshot run | 17 passed: 12 new regression scenarios plus 5 existing documentation captures, `final-captures.log`. Covers all final test-file edits. |
| Baseline document-page submission | Passed against the retained original production bundle: `baseline-page-selection.log`. |
| OpenSpec structural validation | `openspec validate workspace-outcome-experience --strict` passed. This is not application-test evidence. |

All logs above are under `build/ui-baseline/`. The initial six renderer failures
were not reproduced by the production rechecks; no application code was changed
to make those checks pass. Their initial traces and error contexts remain under
`apps/web/test-results` and `build/ui-additional-results`. The first added
saved-result test used the wrong accessible link label (Inspect); it was corrected
to the existing label Inspect work and passed in the final 17-test run. No failed
invocation is counted as a clean suite pass. The 17 broad-suite skips are the
16 opt-in full visual-matrix cases and one canonical visual-evidence case.

### Performance comparison

The existing opt-in production test used the retained original bundle and final
bundle, identical fixtures, one worker and 40 measured warm navigations. The first
run overlapped the end of the broad regression suite and failed its relative p95
gate (20.4 ms baseline, 29.6 ms candidate; `performance.log`). After the other
renderer tests ended, the comparison passed (`performance-final.log`):

| Measurement | Baseline | Candidate |
| --- | ---: | ---: |
| Navigation p95 | 34.3 ms | 35.6 ms |
| Post-GC JavaScript heap | 12,964,644 B | 12,870,364 B |
| API requests | 181 | 181 |
| Visible Send acknowledgment | 18.9 ms | 23.9 ms |

The final run met the existing +10% p95/heap, no-extra-requests and <100 ms
acknowledgment gates. Both runs are retained because timing was variable on this
shared host; this is a local regression observation, not a stable performance
promise or whole-process memory measurement.

### Commands

From the isolated worktree:

`npm --prefix apps/web run api:check`, `i18n:check`, `i18n:audit`,
`i18n:complete`, `i18n:consistency`, `check`, `build`, and
`npm --prefix apps/desktop test`.

Browser invocations used `npm --prefix apps/web run test:e2e --` with:

- `interface-refinement.spec.ts workspace-shell.spec.ts visual-craft.spec.ts work-continuity.spec.ts desktop-command.spec.ts management-refinement.spec.ts agent-management-layout.spec.ts settings-navigation.spec.ts canonical-conversation.spec.ts --workers=1 --reporter=line` (fixture port 5318; the file filter also selected agent-management-refinement).
- `interface-refinement.spec.ts chat-readiness.spec.ts workflow-presentation.spec.ts action-presentation.spec.ts control-consistency.spec.ts --workers=1 --output=../../build/ui-additional-results --reporter=line` (fixture port 5319).
- The six failing scenarios were selected by their exact titles from the same files against the production fixture URL on port 5321. See `rechecks.log` for the full grep expression.
- `interface-refinement.spec.ts documentation-captures.spec.ts documentation-agent-capture.spec.ts documentation-quick-captures.spec.ts --workers=1 --output=../../build/ui-final-results --reporter=line`, with `OFFGRID_DOC_CAPTURES=1`, `PLAYWRIGHT_CHANNEL=msedge` and the production fixture URL.
- `work-redesign-performance.spec.ts --workers=1 --reporter=line`, with `OFFGRID_REDESIGN_PERF=1`, `OFFGRID_BASELINE_UI=D:/offgrid-ui-20261121/build/ui-baseline/dist` and the production fixture URL.


Screenshots retained in `build/ui-baseline/captures` (before),
`build/documentation-captures` and `build/ui-refinement-captures` (after). Inspected
views include light/dark Home, saved work, Models, Knowledge, Activity, Agents and
Settings; Arabic narrow navigation and model content; explicit empty/error
recovery; keyboard focus; and forced colors with 200% root text scaling. Root text
scaling is not a native browser-zoom or screen-reader qualification. Fixture
answers and task receipts are presentation evidence, not actual inference.

The existing Vite large-chunk and native-config-loader warnings remain. Full Go,
Docker, macOS/Linux packaging, installed Electron smoke, real inference, physical
8 GB hardware, native screen readers and human language review were not run for
this renderer-only increment. This record does not claim full CI or merge/release
readiness and does not close unrelated historical OpenSpec qualification tasks.

## Integration

`build/ui-integration/offgrid-ui.patch` is generated against the frozen dirty
baseline, not against Git HEAD. `before/`, `after/` and `manifest.json` retain the
exact edited files, SHA-256 identities and active-checkout comparison. The patch
therefore excludes the hundreds of unrelated baseline edits copied from the
other thread. Its reverse dry-run is verified against this worktree. A read-only
forward dry-run also passed against the active checkout at packaging time.
The final file audit reviewed 647 dirty/untracked paths and found no changes
outside the 22 intended paths relative to the retained snapshot. Temporary
production fixture servers were stopped after verification.

The current comparison distinguishes substantive content changes from CRLF/LF
differences. Recheck the manifest against the active checkout at integration
time; the other thread can continue changing files. Preserve its subsequent
edits and reconcile any overlap. The active checkout has not received this patch.

## User-authorized installation

After reviewing the isolated implementation, the user explicitly requested
installation to see the result. On 2026-11-21T10:45:13.0422084Z, the 67 built UI files
were installed under the existing desktop's resources/ui directory. The prior
UI is retained at `C:/Users/phil/AppData/Local/Programs/OffGrid LLM Desktop/resources/ui.rollback-20261121-184503`.

The installed desktop's own compatibility verifier accepted protocol 1 and
renderer identity `cbb066686e1a4c7131eeab968156bea8517da1424eb38dac79e93b53e8ed3724`. The executable,
app.asar and bundled service SHA-256 hashes are unchanged; desktop/service version
remains `0.4.32-preview.20261116110208432.g9adc304f`. This is a renderer update, not a full
desktop/runtime package replacement. The other thread's source checkout remains
untouched. No models, conversations or application data were replaced.

A packaged startup smoke test used a fresh temporary profile and port 5322,
confirmed the new UI identity and owned-backend readiness, displayed the Models
navigation and checked overflow. It passed and closed its temporary instance.
The normal installed app was then opened; its real service reported the new UI
identity and a ready desktop bridge, and the workspace was visually confirmed.
It is left open for the user with the saved language and theme.

Receipts and reproduction script: `build/ui-install/installation.json`,
`receipt.json`, `smoke.json`, `smoke.log`, and `smoke.cjs`.
