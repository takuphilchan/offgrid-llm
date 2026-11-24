# Terminal, recovery and UI/UX completion review — 2026-11-21

## Conclusion and scope

The full supplied UI/UX brief is **not complete**. The earlier isolated renderer refinement is implemented and installed, but it is a bounded increment, not a whole-product acceptance result. The two user screenshots expose genuine presentation and interaction defects that were not caught by that installation smoke test.

This follow-up is an audit and isolated visual proposal. No production component, installed asset, user setting, terminal process, backup or restore was changed. Work and evidence are in `D:/offgrid-ui-20261121`; the other thread's `D:/offgrid-llm` checkout was only read. No task was marked complete and no change was archived.

**Owner correction:** preserve the existing Models directory/Data directory card, including its original grouping, border and separator. The earlier line-based directory proposal was rejected and removed. Only the Workspace recovery card is being redesigned on this page.

Review gallery: `build/ui-review/review.html`. It contains current/proposed light/dark captures, a 390px recovery view and a 700×420 terminal region. Synthetic paths/output and mock desktop capabilities are labelled. These are not screenshots of new production behavior.

## What is actually installed

The native desktop package remains `0.4.32-preview.20261116110208432.g9adc304f`. The November 21 installation replaced `resources/ui` only. Its built UI index matches this isolated renderer by SHA-256; the prior installation evidence records renderer identity `cbb066686e1a4c7131eeab968156bea8517da1424eb38dac79e93b53e8ed3724` and rollback location.

The installed `app.asar`:

- Contains the terminal renderer and stylesheet separately from `resources/ui`.
- Does not expose `workspaceRecoveryProtocol` or `getWorkspaceRecovery` in the desktop preload.
- Has a transparent terminal pane, a workspace background based on `surface-1`, and an xterm background based on `surface-0` when a palette is supplied.

Read-only inspection is reproducible with `node build/ui-review/inspect-installed.cjs`; its output is retained in `installed-assets.json`. The mock recovery host represents those missing capabilities; it does not infer the user's administrator role from the screenshot.

The earlier boot/Models smoke established generic bridge compatibility and renderer loading. It did **not** establish compatibility of each optional desktop feature. This was an installation verification gap and should have been reported before presenting the UI as finished.

## Prioritized findings

### R1 — High: recovery says Ready while it cannot run

**Evidence:** `apps/web/src/features/settings/WorkspaceRecoverySettings.tsx:9` checks recovery protocol and methods, and line 14 skips status loading when unsupported or unauthorized. Line 33 nevertheless defaults absent state to `idle`; line 52 renders the idle label, “Ready for recovery.” The unsupported notice and disabled operation controls render at the same time. Both supplied screenshots and isolated reproduction support the defect. The same default can misrepresent a pending or failed initial status read.

**User impact:** the page does not explain whether the user needs permission, a supported desktop version, a local managed workspace, or merely to wait. The visible Ready status contradicts the unavailable controls.

**Correction (B, existing contracts):** resolve eligibility before rendering actions; never infer readiness from missing state. Distinguish unsupported host, signed-in role restriction, external/unavailable workspace, checking, failed status read and confirmed ready. Preserve active, stopped, cancelled, failed and uncertain lifecycle handling. Offer read-only retry for a failed status read; never automatically retry a backup/restore mutation.

For the currently installed host, the proposal shows a compact recovery card with “Requires a compatible desktop update,” an explanation and a link to the existing desktop/service details. It does not promise that an update is already available. On a compatible host, retain the real backup/check/restore controls and native confirmation flow.

**Test gap:** existing `workspace-recovery.spec.ts` unsupported-context tests check that operations are disabled and no bridge calls occur, but do not require the Ready message to be absent. Passing these tests therefore does not contradict this finding.

### R2 — Medium: recovery hierarchy and spacing obscure the next step

**Evidence:** `.settings-card` supplies horizontal padding only; `.workspace-recovery` supplies no vertical inset. The title and final status sit against the card edges. Scope, prerequisites, two selectors, native-dialog guidance and a disabled primary action are all shown before capability is established. `apps/web/src/features/settings/settings.css` owns these rules.

**Correction (A/B):** preserve the existing rounded card language and the entire directory card. Give recovery its own balanced padding, a short purpose sentence, a distinct availability/status block and secondary scope disclosure. Unavailable hosts should not show an inert form. Eligible hosts should expose a clear chosen operation, with less-common archive protection settings secondary, without changing encryption defaults or native authority.

**Acceptance:** the directory card's markup and geometry are unchanged by the proposal; recovery is legible at normal and narrow sizes, with theme-aware text, visible focus and no false readiness. Full localization and native 200% zoom remain implementation checks, not claims made by this English prototype.

### T1 — Medium: terminal has an unnecessary inset rectangle

**Evidence:** the installed terminal's body computes to `rgb(25,25,25)` while its xterm palette background is `#101011` in the dark fixture. The transparent pane exposes the outer canvas around the darker rectangular terminal surface. These values are retained in `terminal-dark.json`.

Current source already attempts to match the pane to xterm with a rounded, bordered outer panel. The existing terminal task section 6.3 records that earlier design as tested. The user now wants a continuous surface, so retaining another large inner card would not satisfy the correction.

**Correction (A):** match the terminal canvas and cursor background to the workspace surface, remove the inner panel border/radius, and retain reasonable content padding. Keep the tab strip, selection, prompt and focus clear. Both light and dark proposals use actual installed terminal assets with mock output, not a drawn terminal approximation.

**Acceptance:** pane, workspace and xterm canvas backgrounds agree; ANSI colors, selection, cursor, forced colors and resizing remain readable. Appearance changes must never send terminal input or alter a running process.

### T2 — Medium: “Chat” does not describe the terminal return action

**Evidence:** `apps/desktop/terminal-renderer.js:92` assigns both Chat and the return icon to the `hide` action. The dock hides the terminal without changing the web route. A terminal opened over Settings therefore reveals Settings, while its button promises Chat. The current unit test explicitly checks route preservation, not navigation to Chat.

**Correction (B):** use an accurate “Back to workspace” label or origin-aware equivalent, and make the terminal overlay context understandable. Preserve the underlying route and draft; do not force navigation to Chat or close the terminal session just to match the existing label. The prototype demonstrates the neutral label; it does not yet solve every sidebar/context transition.

**Acceptance:** return only hides the terminal; no close/input/start call is sent. Verify opening from Settings, returning, same-route sidebar activation, Escape/menu handling and draft retention in the matched desktop build.

### V1 — High completion gap: source, renderer install and desktop qualification differ

The active UI change contains completed implementation groups alongside unfinished journey, platform and human qualification. The renderer-only installation left older terminal assets and no recovery bridge. Installing another thread's unfinished backup backend merely to make this screen look enabled would expand the work improperly.

A focused correction can make the existing host honest without adding recovery capability. Updating terminal presentation requires a deliberate native packaging step in addition to a renderer build. A matched packaged smoke must cover optional capability absence as well as a supported fixture before claiming these two defects are fixed in the installed app.

## Supplied brief: completion assessment

The following assesses the twelve sections of the supplied prompt against current source, retained evidence and this focused review. “Partial” does not imply that every listed feature is absent; it means the section's complete acceptance evidence is not established.

| Brief section | Assessment | Evidence and remaining work |
| --- | --- | --- |
| 1. Understand the product | Substantially recorded; ongoing | Existing docs, architecture and OpenSpec are in use. This review corrects the outdated “current installation” label in the historical audit. Concurrent source and installed capabilities must still be reconciled per delivery. |
| 2. Audit journeys and screens | Partial | Historical audit, broad fixture captures and the two concrete reproductions exist. Frozen real first-use/returning/document/offline/error journeys and action counts remain open in 22.1 and 22.7. |
| 3. Product direction | Implemented in part | Compact entry, shared workspace, optional advanced destinations and saved-work continuation exist. Full ordinary navigation/readiness acceptance, including 200% and discovery evidence, remains open in 22.2–22.3. No new dashboard or compulsory project is needed for this correction. |
| 4. Redesign vs expansion | Boundary preserved in this review | Both fixes fit A/B. Native recovery capability, capability-pack plans and structured document coverage remain owned dependencies, not mock product features. |
| 5. Visual/interaction system | Partial; confirmed defects | Tokens, local assets, typography and shared controls exist. Terminal surface and recovery spacing/status violate the intended consistency. Palette/material/contrast coverage remains open in 14.4 and 31.1–31.5. |
| 6. Core work experience | Partial | Chat, saved work, canonical task inspector, sources, models and management are implemented with retained automated evidence. Readiness, model discovery/transfers, structured-source reader integration and complete end-to-end real-service/model qualification remain open. See 44.7–44.9, D1/D2, 22.2/22.4 and 16.17. |
| 7. Desktop continuity | Partial | Existing bridge, Quick Access, draft/work identity and terminal lifecycle have contract coverage. Current terminal return label and missing-recovery fallback need correction. Packaged multi-OS, fresh-install and native continuity gates remain open in 16.5 and 8.4. |
| 8. Accessibility/localization/performance | Partial | Previous renderer checks include translation catalogs, RTL, narrow views, focus, forced colors and text scaling; controlled fixture performance is recorded separately. Native 200% zoom, screen-reader, native-language, full history/stream/cold-warm and physical 8GB qualification are not all established. Catalog completeness is not linguistic approval. |
| 9. Concrete design before implementation | Existing review artifacts; acceptance incomplete | Prior broad comparison and this focused light/dark/narrow proposal exist. This review does not independently verify every required prototype state or claim owner approval. The owner rejected directory redesign; it is removed. |
| 10. Approved increments | Bounded increment delivered | Group 45 is implemented and renderer-installed. That does not close the entire brief. Current terminal/recovery proposals are still review artifacts, not production fixes. |
| 11. Verification and presentation | Partial | Actual focused results are below; prior broad results and failures remain in their dated report. New reproductions expose test blind spots. Full applicable CI, all editions and human acceptance are not complete. |
| 12. Delivery boundaries/reporting | Preserved here; earlier installation explicitly authorized | Isolated worktree, rollback, source/fixture/native distinctions and no commit/push/publication. Prior install is documented. This review changes no installed payload or personal workspace. |

## OpenSpec verification result

CLI 1.14.0. Existing change: `workspace-outcome-experience`. The isolated snapshot records **219/248 checked tasks, 29 open**; the other thread's checkout records **214/243, 29 open**, without this worktree's five group-45 tasks. Counts describe plan state, not completion percentages. Several old unchecked tasks overlap current code and need evidence reconciliation, not automatic reimplementation.

| Dimension | Result |
| --- | --- |
| Completeness | Not complete: 29 unresolved task entries. Five delta specs contain 85 requirements and 172 scenarios; an independent implementation/test mapping of all 85 was not performed in this focused review. |
| Correctness | Confirmed recovery readiness and terminal return-label defects; terminal surface reproduced. Focused tests pass but omit the contradictory Ready assertion. Remaining requirement/scenario correctness is **not reverified** by this report. |
| Coherence | Existing monochrome system and canonical contracts remain appropriate. Recovery hierarchy, unsupported-state presentation and terminal return language need correction. Earlier rounded-pane design must be reconciled with the owner's continuous-surface preference. |

Relevant normative anchors are `workspace-experience-quality` requirements **Journey-level redesign acceptance**, **Predictable settings and native controls**, **Whole-product experience qualification**, and `workspace-management-presentation` **Coherent Settings groups**. The whole-product candidate-completion scenario explicitly requires current group-16 evidence rather than inference from checked historical tasks. It is not satisfied by this review.

Related changes also remain unfinished: `desktop-integrated-terminal` has native dependency/platform acceptance outstanding; `encrypted-workspace-backups` retains CI, real platform recovery and administrator usability/accessibility gates. These must not be marked complete from a cleaner recovery card.

Proposed planning update after direction acceptance: add a small correction group under the existing UI change for R1/R2/T1/T2 with explicit installed capability-mismatch acceptance, and link the terminal task's revised surface decision. Keep backup execution/backend ownership in its existing change. This review does not check off those corrections or duplicate the full roadmap.

## Bounded corrective scope and acceptance

| Proposed action | Class / owner | Acceptance |
| --- | --- | --- |
| Keep directory card exactly as it is | Constraint | No directory markup/style/geometry changes. |
| Fix recovery status and eligibility rendering | B / shared renderer | Missing bridge, denied role, loading and failed status read never say Ready; checking is non-actionable; confirmed eligible idle enables the real operation. |
| Improve recovery card padding and hierarchy | A / shared renderer | Original card language retained; concise purpose/status, useful next step, secondary scope; light/dark/narrow/RTL/long labels/keyboard/native zoom reviewed. |
| Preserve native recovery lifecycle | B / existing bridge | Native dialogs continue to own paths/secrets; Stop and explicit Resume remain; uncertainty never causes automatic replay; opening settings causes no mutation. |
| Remove terminal inset rectangle | A / desktop renderer | One continuous surface across all supported palettes and forced colors, readable selection/ANSI/cursor, correct resize. |
| Correct terminal return wording/context | B / desktop renderer | Reveals prior workspace without changing route, discarding draft, closing process or sending input. |
| Validate matched installed presentation | Delivery | Review exact payload and optional capabilities; isolated startup/terminal/recovery fallback smoke, rollback and explicit install scope; never assume bridge version alone proves feature support. |

Adding recovery support to the older installed desktop is a separate packaging/dependency decision. These UI fixes do not authorize or require enabling that unfinished backend.

## Checks performed in this review

All commands ran from the isolated worktree unless expressly identified as a read-only main-checkout inspection. No model download, model inference, real shell command through OffGrid, native recovery, personal-data fixture or actual backup/restore was used.

| Check | Actual result / limitation |
| --- | --- |
| `node --test apps/desktop/test/terminal-preferences.test.js apps/desktop/test/terminal-layout.test.js apps/desktop/test/terminal-navigation.test.js` | **9 passed**, zero skipped/failed. Contract tests, not a real PTY or native window. `build/ui-review/desktop-contracts.log`. |
| `npm --prefix apps/web run test:e2e -- workspace-recovery.spec.ts terminal-entry.spec.ts terminal-overlay-menu.spec.ts settings-organization.spec.ts --workers=1 --output=../../build/ui-review/regression-results --reporter=line` | **56 passed** (2.2m), Edge, API/desktop fixtures at isolated port 5324. Includes existing localization/state coverage; not new human language review. `regression.log`. |
| `node apps/web/node_modules/@playwright/test/cli.js test --config=build/ui-review/playwright.config.cjs --reporter=line` | Final **4 passed** (7.9s): two terminal and two recovery cases, each in light/dark. Recovery asserts that the existing contradiction exists, so this is defect reproduction, not acceptance of a fix. Proposals also check no input/mutation, unchanged directory geometry, truthful return action and narrow overflow. `audit-final.log`. |
| Initial audit harness | **2 passed / 2 failed**: terminal dependency URLs used forward slashes against the Windows ASAR API, so `Terminal` was undefined. Fixed only the audit asset server's path normalization; terminal rerun **2 passed**, final combined **4 passed**. Retained `audit.log` and `terminal-debug.log`. This was a fixture setup failure, not an OffGrid runtime failure. |
| Visual inspection | Current/proposed terminal and recovery light/dark, narrow recovery and short terminal region. Terminal window/sidebar embedding is not simulated. Directory line redesign was rejected and replaced by the original card. |
| Installed asset inspection | Read-only ASAR/preload/UI identity checks. No app shutdown, upgrade, native process creation or recovery operation. `installed-assets.json`. |

Full Go/API/build/renderer/desktop CI was **not rerun for this documentation/prototype-only review**. The earlier implementation's checks remain historical evidence for that increment. Full fresh-install/update, macOS/Linux, real-model quality, physical low-memory hardware, all-palette ANSI contrast, native 200% zoom, assistive technology, translator and participant acceptance remain unverified here.

**Archive assessment:** one critical completeness gate (29 unresolved task entries), four confirmed UI warnings (R1/R2/T1/T2), and zero optional suggestions in this focused review. Do not archive. Full requirement/scenario mapping and the skipped CI, platform, hardware, model and human checks listed above remain unverified because this was a focused read-only audit, not a whole-product qualification run. Resolve the defects, qualify the matched installed result, and close the remaining gates with actual evidence before describing the supplied prompt as completed.
