# Presentation and usability review — 2026-10-30

## Follow-up: order and presentation (2026-11-01)

Reviewing populated screens revealed that consistent buttons did not establish
consistent hierarchy. Composer preferences competed with input; model maintenance
was more explicit than model selection; profile save actions required scrolling;
Knowledge search and question entry occupied separate sections; Activity mixed
task browsing and scheduling; management pages had different leading edges;
startup repeated technical metadata; and long artifact names could consume ten
lines while passing overflow tests. These are workflow and composition findings,
not evidence that a different gradient or smaller border alone would help.

Group 19 of `workspace-outcome-experience` tracks the corrections and stronger
realistic-content tests. Shared management alignment, grouped preference/library
controls, separate Activity views and bounded artifact names reuse existing
controllers. No presentation change supplies authorization or verification.
Human usability, assistive-technology and unavailable native-platform gates stay
open. Browser screenshots and automated journeys cannot establish competitor parity.

Remaining presentation concerns after this pass:

- At 320px, the composer still spends substantial height on active agent,
  workspace, model and tool context. Grouping preferences helps, but this is not
  yet a compact mobile composition. Hiding active authority or Stop is not an
  acceptable density fix.
- Agent creation remains a configuration form with technical concepts. The
  reordered identity/purpose fields and persistent wide-screen save action reduce
  searching; they do not establish that a first-time user understands resources.
- Empty libraries and advanced model metadata still have uneven density. A
  readable sparse screen is preferable to decorative filler, but populated,
  empty and unavailable states need continued comparison in user testing.
- Shared controls and alignment now have regression coverage. Native-speaker
  wording, actual screen-reader announcements and physical display behavior do
  not have equivalent evidence and remain release-qualification limitations.

This is a heuristic review and cognitive walkthrough, not a user study. No
participant success rate or competitor parity is claimed. Source baseline:
`7357acae431e40092fd40a5458207d3649d503e8`. Live read-only inspection covered the
six installed web screens. Disposable renderer fixtures cover absent/present
models, roles, service failure and recovery; they do not qualify native effects.

## Personas and tasks

- **Field operator:** needs a useful answer and document lookup, limited technical
  vocabulary, unreliable internet, must recognize when work requires help.
- **Small-team administrator:** installs models, prepares document search,
  configures agents and accounts, checks readiness and handles recovery.
- **Developer:** switches models, reviews tool evidence, configures connections,
  uses shortcuts and needs exact technical detail without constant clutter.

All three perspectives were applied to the same paths below. Counts are
deterministic navigation/activation steps from the named starting state, excluding
typing, network wait, credentials and native permission dialogs. They describe
the inspected path, not observed human performance. A dash means the path depends
on unavailable hardware/downloads and no comparable count is claimed.

| Task and starting state | Main persona | Before | After | Evidence/limitation |
| --- | --- | ---: | ---: | --- |
| First reply, ready Chat: type then Send | Operator | 1 | 1 | Existing streaming/setup fixtures |
| First reply, no model | Operator/admin | — | — | Download and real inference require separate evidence |
| Review a model download from Installed | Admin | 2 | 2 | Discover, Review; download remains explicit |
| Switch model in ordinary Chat | Developer | 1 | 1 | Existing select; profile selection differs |
| Continue a saved conversation | All | 1 | 1 | History selection; narrow drawer adds one |
| Add own document from Knowledge | Operator/admin | 1 | 1 | Add document; indexing is asynchronous |
| Ask using ready Knowledge | Operator | 2 | 2 | Ask using Knowledge, Send |
| Run an existing agent from Agents | Operator | 3 | 3 | Chat, Agent, select profile; Send additional |
| Inspect what a task did from Chat | All | 1 | 1 | Inspect work; details disclosure additional |
| Approve or deny a pending call | Operator/admin | 1 | 1 | Exact request review required; no grant shortcut |
| Change language from a main page | Operator | 2 | 2 | Workspace menu, language select |
| Change theme from a main page | All | 2 | 2 | Workspace menu, appearance select |
| Check an interrupted task | All | 1 | 1 | Refresh/check state; resume is separate |
| Find offline model import from Models | Admin | 1 | 1 | Existing advanced disclosure; import is separate |
| Find Settings via keyboard palette | Developer | 2 | 2 | Open palette, select Settings |

## Prioritized findings

### Follow-up from installed-app feedback

The user found controls too large, the options popup cumbersome and agent
configuration too dependent on disclosures. The follow-up introduces 32px
fine-pointer secondary actions and 36px primary/form controls; coarse-pointer
targets remain at least 44px. Workspace options uses aligned label/control rows
and compact menu actions, with stable pressed/open feedback. Chat history has a
reserved thin scrollbar lane, neutral thumb and native forced-color fallback.

Agent identity/model/instructions and resources now occupy adjacent columns when
the content width permits. Tools and Knowledge are visible immediately; Skills
and collaborators no longer sit inside Advanced. Budget and workspace execution
policy remain advanced. Save remains explicit, with its existing draft/conflict
and permission protections.

In the deterministic create-with-tool/Knowledge/collaborator fixture, three
disclosure activations are removed (Tools, Knowledge, Advanced). Opening Skills
requires one disclosure rather than two. These are path counts, not participant
performance measurements. Native dropdown keyboard behavior is retained.

Validation: 444 renderer tests passed, 27 skipped; API/type/build, palette
inventory, source-size and OpenSpec checks passed. Six added tests cover the
editor/options at 390/1440px in English/Arabic, history scrolling without footer
movement, and coarse-pointer targets. Existing model selection, resource save,
revision conflict, dirty draft and permission tests remain active. Two older
fixtures were corrected to wait for completed entrance animation before exact
geometry measurements and to use a real outside target for popup dismissal;
their spacing, dismissal and request-identity assertions were retained.

The existing palette is unchanged. Seventeen explicit additive declarations for
the menu, scrollbar and editor are recorded in
`scripts/test/ui-compact-color-allowlist.json`; the palette audit accepts only
those exact entries with `--compact`, and still rejects baseline modifications.
The heavy-stream CPU-throttling limitation and unrun native-speaker/screen-reader
checks from the earlier pass remain open.

Severity weighs potential confusion and frequency. Effort is relative (S/M/L).
Protected means a proposed resolution could change authority, persistence,
first-run completion or execution semantics. Initial status is deferred until
implementation and checks are recorded below.

| ID | Screen/task | Finding and evidence | Severity | Proposed resolution | Effort | Protected | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| U01 | Agents → task | Configuration and execution span three destinations; current prose offers no direct handoff. `AgentPage.tsx` | Major | Explain profile/task distinction and link to existing Chat/Activity | S | No | Fixed (automated/heuristic evidence) |
| U02 | Models | Category terms assume knowledge of embeddings and speech engines. `ModelsPage.tsx` | Major | Localized category purpose/prerequisite guidance | S | No | Fixed (automated/heuristic evidence) |
| U03 | Discover/offline | Service connectivity and internet connectivity are different. `ModelSearch.tsx` | Major | Explain online search and local Installed fallback without claiming internet detection | S | No | Fixed (automated/heuristic evidence) |
| U04 | Knowledge | Added, indexed and used in an answer can be confused. `KnowledgePage.tsx` | Major | Explain the sequence next to the existing document actions | S | No | Fixed (automated/heuristic evidence) |
| U05 | Footer/Settings | Menu is labelled with product/account; language and appearance live behind it. `WorkspaceHeader.tsx` | Minor | Use explicit workspace-options label; retain account identity inside | S | No | Fixed (automated/heuristic evidence) |
| U06 | First run | “Private” is broader than verified service reachability, and model purpose is assumed. `Onboarding`/locale resources | Major | Explain local model and connected-service storage without a privacy certification | M | No | Fixed (automated/heuristic evidence) |
| U07 | Failed/empty states | Generic empty statements provide uneven next-step guidance. `WorkspacePresentation.tsx` | Major | Add scoped guidance using existing actions; preserve distinct states | M | No | Deferred: targeted improvements made; wider audit remains |
| U08 | Chat | Slow first output can look stalled; existing phase labels are brief. `ChatPage.tsx` | Major | Explain cold loading without fabricated progress or speed guarantees | S | No | Fixed (automated/heuristic evidence) |
| U09 | Approvals | Exact invocation, account and expiry cannot be simplified into general approval. `TaskAccess`/approval components | Blocker if weakened | Retain exact review; improve surrounding explanatory copy only | M | Yes | Proposed-as-contract-change: broad approval excluded |
| U10 | Task titles/history | Technical auto-generated names reduce recognition. Saved session identities are persistent. | Minor | Separate future display titles from IDs only in a dedicated contract change | L | Yes | Proposed-as-contract-change |
| V01 | Controls | `.text-button` geometry is repeated in `styles.css`, `workspace.css`, `controls.css` | Polish | One final control owner; drain superseded legacy geometry | M | No | Fixed (automated/heuristic evidence) |
| V02 | Typography | Mixed 8–16px labels and nonstandard weights across legacy/new rules | Polish | Add semantic type scale and 400/500/600 weights | M | No | Deferred: targeted improvements made; wider audit remains |
| V03 | Sections | Headings, lists and settings have inconsistent vertical rhythm | Polish | Token-based section and list spacing | M | No | Fixed (automated/heuristic evidence) |
| V04 | Chat composer | Multiple controls have equal visual mass; narrow layouts need careful wrapping | Major | Refine geometry/grouping, retain every active setting and Stop | M | No | Fixed (automated/heuristic evidence) |
| V05 | Code | Copy floats above code without a language header | Minor | Safe language metadata/header; identical copied source | S | No | Fixed (automated/heuristic evidence) |
| V06 | Empty states | Broad blank areas with small unstructured text | Minor | Shared spacing/measure and contextual guidance | S | No | Fixed (automated/heuristic evidence) |
| V07 | Model rows | Names, readiness chips, size and maintenance compete | Minor | Improve title/metadata rhythm and tabular numbers | M | No | Fixed (automated/heuristic evidence) |
| V08 | Disclosures | Details/summary spacing varies across features | Polish | Shared disclosure rhythm and focus retention | S | No | Deferred: targeted improvements made; wider audit remains |
| V09 | Overlays | Popover/palette entry abrupt; onboarding uses backdrop blur | Polish | Short transform/opacity entrance; remove blur | S | No | Fixed (automated/heuristic evidence) |
| V10 | Press feedback | Hover is present but active feedback is uneven | Polish | Small transform-only press response with reduced-motion opt-out | S | No | Fixed (automated/heuristic evidence) |
| V11 | Startup | Desktop startup uses separate layout metrics | Polish | Align rhythm and motion; retain native font and frozen palette | S | No | Fixed (automated/heuristic evidence) |
| V12 | CSS ownership | Superseded geometry and unlayered legacy rules make refinement fragile | Major for maintenance | Migrate affected geometry only; retain frozen color owners | M | No | Deferred: targeted improvements made; wider audit remains |

The review covers Nielsen's visibility, real-world language, user control,
consistency, error prevention, recognition, efficiency, minimalism, recovery and
help principles. Added dimensions are learnability, information scent, progressive
disclosure and offline/role truthfulness. Every cosmetic issue is a heuristic
judgment; test outcomes are reported separately.

## Protected-contract proposals

The existing workspace OpenSpec change records these as excluded follow-ups,
not implementations: persistent friendly conversation titles (identity/migration
review required), global dismissal of historical attention (must not hide pending
effects), and broader remembered approvals (exact invocation/account/expiry must
remain enforced). No source behavior is changed to achieve these ideas.

## Evidence and acceptance

- Baseline visual matrix: 224 PNGs under `build/ui-craft/before`, 16 Playwright
  cases, 14 scenes each, 390/768/1024/1440, light/dark, English/Arabic.
- Baseline colors: `build/ui-craft/colors-before.json`, 976 entries in 172 files.
- Before captures include Chat with saved code and streaming text, all six pages,
  model discovery, palette, popover, onboarding, empty/loading/error states.
- Screen-reader output and installed macOS/Linux window behavior require real
  assistive-technology/platform review; DOM accessibility tests are not substitutes.
- Existing workspace tasks 6.2/6.3/7.2/8.4 retain their original evidence gates.

## Results before the additional gradient phase

The final isolated renderer run passed **420**, skipped **26**, failed **0**.
The opt-in visual matrix separately passed **16**, producing 224 after images.
The first broad run found that extra model guidance pushed search below the
compact-layout gate; contextual guidance fixed this and the original assertion
passed. Geometry assertions were updated only for the explicitly changed
composer radius and visible profile scrollbar.

API generation check, TypeScript, production build, maintained-doc checks and
source-size ratchet passed. Desktop tests passed 85 with one platform skip;
main/preload syntax checks passed. The build retains its existing >500 kB chunk
advisory. No dependencies were added. All 976 baseline color entries remain
unchanged, including the original palette and legacy color rules.

Step counts are unchanged for the listed paths: this pass improves recognition
and explanations rather than eliminating authority checks. From Settings itself,
language selection is now one action rather than two. Authentication, restricted
roles, offline failures, draft preservation, exact approvals and recovery remain
covered by existing fixtures; no real-user timing or fresh model qualification
is claimed. The native installed app was not driven through every persona path.

Representative direct screenshot review covered Chat/code, Agents, Models,
Settings and onboarding in light/dark and Arabic narrow layouts. All other matrix
captures are available for review; automated capture is not human review of each.
Keyboard, IME, reduced-motion, contrast, 320px/short-window and 200%-equivalent
reflow fixtures passed. Physical display banding, actual browser zoom and spoken
screen-reader output still need manual verification. The fixture latency gate
passed: history p95 34.6 ms, slowest measured warm route 148 ms.

## Top five remaining usability risks

1. Native-speaker review of new explanations and setup labels is outstanding.
2. Real screen-reader speech, physical zoom and platform window behavior need
   assistive-technology and installed Windows/macOS/Linux checks.
3. Heuristic clarity is not demonstrated field usability; retain the five-person
   usability gate rather than claiming the interface is universally intuitive.
4. Runtime/model readiness and unknown external effects remain inherently complex;
   copy cannot substitute for real hardware qualification or effect verification.
5. Legacy metadata density and generic server errors still vary across advanced
   screens. These are deferred, not hidden by the new presentation scale.

The later user-authorized gradient phase has its own palette-token proof and
performance report. Screenshots use synthetic content and remain build artifacts.

## Installed-build correction ? 2026-11-01

The owner rejected the installed v0.4.23 presentation as too incremental. The
release's passing fixtures did not establish satisfactory whole-product design.
The subsequent review found concrete defects:

- A single history action inherited the same 340px padded popup as configuration.
- Ordinary deletion exposed a three-option active-work policy before any conflict.
- Agent profile headers repeated Chat/Activity navigation already in the sidebar.
- A two-option workspace dropdown used technical host-ceiling language.
- Activity's selected view had no visible selected fill, and the actual work list
  and diagnostics retained a second centered content edge.
- Quit could skip later cleanup on an earlier rejection and remain permanently
  marked as quitting; forced service shutdown discarded ownership without exit proof.

The corrective pass uses compact menu rows, conditional typed-conflict deletion
choices, direct limits, consistent Activity alignment and retryable owned cleanup.
A seven-process installed snapshot consisted of one Electron parent, three Electron
helpers, one service and separate chat/embedding workers; no orphan was established
by that snapshot. Isolated ownership tests are separate evidence.

These corrections do not establish competitor parity or complete the broader
composition review. Native-speaker, screen-reader and participant checks remain
unavailable. Keep focus indicators and consequential confirmations; do not remove
them merely to make screenshots look quieter.
