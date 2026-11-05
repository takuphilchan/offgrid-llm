# OffGrid product experience audit and design plan

**Updated:** 2026-11-05. **Baseline:** the running, installed OffGrid **v0.4.25**, distinguished below from the newer working-tree renderer. Audit and plan only. The old `web/app` and `desktop/` names in the request now refer to `apps/web` and `apps/desktop`. Source locations identify implementation owners; they do not establish that an uninstalled change is already visible to users.

## Executive summary

1. Attached-document answers expose raw byte-range citations. The installed app has a reader, but the answer does not offer a readable, verified route from each claim to its passage.
2. Models gives a cancelled download a large leading card above five installed models; all five say “Runtime unchecked,” and the selected model still says “Select model.”
3. Activity leads with chronological history while the global header says seven items need attention. Finding a result and resolving a blocker need different default views.
4. Knowledge is empty while Chat contains document-backed work. The distinction between a conversation attachment and a reusable library is insufficiently explained at that handoff.
5. Agent setup and first-use guidance still explain system structure before the user's task. The newer source improves grouping and setup, but those improvements are not installed yet.

**Design idea:** make the default experience one clear work surface: ask, add a source, inspect a result. Reveal model, agent, tool, terminal, and recovery controls at the point where they matter, with a persistent, truthful account of where work runs and what can leave the machine.

### Correct installed baseline and evidence limits

The installed executable is `C:/Users/phil/AppData/Local/Programs/OffGrid LLM Desktop/OffGrid LLM Desktop.exe`, product version **0.4.25.0**, with its bundled native `resources/bin/offgrid.exe` running. The running service at `http://127.0.0.1:11611/api/v2/system` reports **0.4.25**, API v2, a ready desktop bridge, and UI build ID:

```text
a7cdabbdfe70ab88a3496163a906fd24b562ec7ecec0f1e92658d1765ec33e7b
```

That matches the installed `resources/ui/index.html` identity. The newly built working-tree UI has a different identity, `7e06acb503d015a9fb93721ef004b39ba080623f4cfed7f749cdeaac9050f31b`. A passing test or screenshot of that build is **not evidence of what is currently installed**.

Following the user's correction to inspect the correct app, opened only the running installation's renderer in a fresh browser context. Allowed same-origin GET/HEAD only; blocked mutations and outbound requests. No models, conversations, profiles, jobs or service preferences were changed. This is a read-only inspection, not a live workflow test. Captures are private, ignored files under `build/ux-audit/`, not publication assets:

| Evidence ID | Capture | Observed state |
| --- | --- | --- |
| I1 | `installed-dark-chat.png` | Existing document answer, raw byte citations, compact composer, saved conversation list |
| I2 | `installed-dark-models.png` | Five installed models, selected model, retained cancelled download |
| I3 | `installed-dark-activity.png` | Populated history, search, clear-removable action, seven attention items in header |
| I4 | `installed-dark-knowledge.png` | Empty indexed library, Add document, disabled Ask using Knowledge |
| I5 | `installed-dark-agents.png` | Existing profile list and unselected detail pane |
| I6 | `installed-dark-settings.png` | Preferences, setup guide, service identity, computer capability and diagnostics |

The disposable browser context uses English/dark to match the installed desktop presentation file. It waits for route-specific ready content and fonts. Earlier captures showing loading placeholders are not the settled-state evidence above. Browser zoom, native chrome and desktop-only bridge behavior are not replicated. In particular, the browser's “No platform driver installed” message is **not a diagnosis of the native desktop companion**. The fresh browser's onboarding prompt also does not prove that onboarding reappears in the user's existing desktop session.

- Read `README.md`, `docs/advanced/workspace-ui.md`, `docs/advanced/cli-experience.md`, `docs/advanced/client-contracts.md`, `openspec/config.yaml`, and `.github/workflows/ci.yml`. Inspected source and the active `workspace-outcome-experience` change; its remaining tasks already cover readiness, document handoff, accessibility, and performance. The plan below should extend that change where it fits, not create a competing implementation plan.
- Confirmed the installed version and UI identity above. The integrated terminal and compact composer already exist. Earlier user screenshots are historical problem examples, not evidence that the old giant composer, missing terminal or oversized menus remain in this installation.
- Ran `npx.cmd playwright test tests/workspace-baseline.spec.ts --grep 'workspace comparison: populated' --workers=1` in `apps/web`: **1 passed**. It produced disposable, API-mocked browser captures at `apps/web/test-results/workspace-baseline-workspace-comparison-populated/`. They establish the fixture renderer only. They **do not** prove the installed v0.4.25 desktop appearance or its terminal. Source findings below are implementation risks to verify in the installed build, not claims that every older visual defect is still present.
- Native window inspection through both available computer-use interfaces failed before capture with `failed to write kernel assets: The system cannot find the path specified`. Consequently, the six installed-renderer captures above establish the actual served UI, **not** the native window, terminal tab, tray or OS dialogs. Direct visual inspection of those native surfaces remains unverified. Screen-reader behavior, macOS/Linux behavior and new real-model latency tests also remain unverified.
- Comparison principles: Apple's [disclosure guidance](https://developer.apple.com/design/human-interface-guidelines/disclosure-controls) prioritizes frequent actions above advanced detail; Raycast's [quickstart](https://manual.raycast.com/quickstart) shows a compact keyboard action path; the [Codex app introduction](https://openai.com/index/introducing-the-codex-app/) frames multiple agents as supervised work; Anthropic's [Projects](https://support.anthropic.com/en/articles/9517075-what-are-projects) and [Artifacts](https://support.anthropic.com/en/articles/9487310-what-are-artifacts-and-how-do-i-use-them) distinguish conversation context from a substantial document view. These are reference principles, not layouts or branding to copy.

## Working-tree follow-up — not installed

The audit above records the installed v0.4.25 baseline. The following earlier changes implement
parts of its direction under `workspace-outcome-experience`; they do not close
the entire redesign or its release gates.

- CLI first-use help now leads with installed models, conversation and readiness.
  `help all` exposes every registered command family, with a regression test that
  compares the catalog against the dispatch registry. It no longer promises an
  automatic model download or relies on fixed model aliases in examples.
- Expanded navigation groups Chat, Activity and Agents under Work, and Knowledge
  and Models under Library. Settings remains in Workspace options. At short
  heights, links remain in a single vertical sequence instead of spreading across
  two columns.
- Setup presents an explicit installed-model choice as the primary action when
  the saved model is unavailable. Service status is visible; implementation
  details and model counts are disclosed on demand. No download, selection or
  grant occurs merely by opening setup.
- The empty Chat heading becomes compact when a document is open, giving the
  source pane priority while keeping the draft and source-coverage notices.

Validation and limits are recorded in
[the follow-up evidence](../quality/evidence/workspace-design-follow-up-2026-11-05.md).
The Windows packaged host was exercised with the new built renderer and isolated
HTTP fixtures. This does not establish the appearance of the user's installed
workspace, model-answer quality, or macOS/Linux native behavior. Human usability,
screen-reader and manual translation reviews remain unverified.

The working-source renderer suite subsequently finished with **992 passed, 31 skipped** (`node scripts/test/test-web-workspace.mjs build/ux-audit/offgrid.exe --workers=2 --reporter=line`). CLI tests, web build/API checks and desktop unit tests also passed. Packaged Windows fixtures exercised terminal input, tabs, cursor preservation, Unicode, search/copy, clipboard isolation and child cleanup. These are regression evidence for the candidate, not proof of usability or qualification of every model. The audit-only request stopped further source edits; this report does not authorize installation or publication.

## Screen and state inventory

The app uses hash routing, not a route library: six top-level pages in `apps/web/src/lib/navigation.ts:1-14`, mounted by `apps/web/src/App.tsx:218-239`. Legacy `#/agents/workspace`, `#/agents/new`, and `#/agents/task/<id>` route to Activity; `#/agents/profiles`, `/tools`, and `/connections` are secondary Agent views. The desktop serves the same React renderer and adds native chrome and a terminal view.

| Current entry and source | Significant states, panels, and dialogs | Proposed primary placement |
| --- | --- | --- |
| `#/chat`: `features/chat/ChatPage.tsx`, `ConversationHistory.tsx`, `DocumentWorkspace.tsx`, `DocumentReader.tsx`, `ComposerFiles.tsx`, `ComposerImages.tsx`, `ComposerOptions.tsx`, `ComposerSpeech.tsx`, `ConversationWork.tsx` | Empty/ready/loading/streaming/interrupted, image/file preparation, unsupported vision, response settings popover, source passages, document reader, blocked job, history deletion | Default **Work** screen. One compact composer; attached document becomes a contextual right pane. Technical model/context controls stay in a labelled details panel. |
| `#/knowledge`: `features/knowledge/KnowledgePage.tsx`, `KnowledgeImport.tsx`, `KnowledgeImportRecovery.tsx`, `OCRLanguageSetup.tsx`, `DocumentSourceDialog.tsx`, `DocumentRetentionDialog.tsx`, `ImportExtractionDialog.tsx` | No model, no documents, preparing, partial/OCR review, ready, failed import, source preview, retention and removal confirmations | **Library → Documents**. First-time “Add a document” stays primary; embedding/OCR/retention management becomes contextual administration. |
| `#/models`: `features/models/ModelsPage.tsx`, `PackageModels.tsx`, `PackageOperation.tsx`, `ModelSearch.tsx`, `LegacyDownloadReview.tsx`, `SpeechPackages.tsx`, `CatalogComparison.tsx` | Installed/discover, four categories, filter, preview/review, downloading/resume/cancel, verify, unavailable runtime, delete confirmation | **Library → Models** for installation and inspection; use a simple recommended path before family/quantization details. |
| `#/agents` and secondary views: `features/agents/AgentPage.tsx`, `AgentProfiles.tsx`, `AgentManagement.tsx`, `SkillLibrary.tsx`, `AgentNavigation.tsx` | Profile list/editor, no profiles, conflict/unsaved draft, capability/resources/advanced, tools, connections, skill administration | **Work → Agents** as optional reusable configurations; profile creation starts with purpose and model. Administration remains one level deeper. |
| `#/activity` and task links: `features/activity/ActivityPage.tsx`, `features/agents/TaskWorkspace.tsx`, `TaskAccess.tsx`, `TaskApproval.tsx`, `TaskResult.tsx`, `WorkRecovery.tsx`, `AgentSchedules.tsx` | Task list/empty/search, schedules, inspector, pending input/approval, interruption/uncertain effects, evidence and diagnostics, delete dialog | **Work → Activity** for active/needs-attention and recent results; full logs, schedules and diagnostics in secondary views. |
| `#/settings`: `features/settings/SettingsPage.tsx` | Preferences, workspace/service identity, computer status/stop, diagnostics, setup guide | **Settings** with human choices first and service details below disclosure. |
| Global: `App.tsx`, `components/shell/*`, `CommandPalette.tsx`, `WorkspaceHelp.tsx`, `HistoryDeleteDialog.tsx`, `ConfirmDialog.tsx`, `UtilityPopover.tsx`, `FilePicker.tsx`, `ModelChoice.tsx` | Login, boot/offline, onboarding, help, command palette, known work, workspace options, history actions, file picker, modal confirmation | Navigation stays compact; only one contextual overlay at a time. Errors and approvals stay next to affected work. |
| Desktop: `apps/desktop/loading.html`, `startup.js`, `main.js`, `terminal.html`, `terminal-renderer.js`, `terminal-dock.js` | Startup checking/starting/ready/incompatible/unavailable, safe recovery, native menus/tray, terminal tabs, local process lifecycle | Same Work surface plus a desktop-only terminal tab. Recovery is a distinct startup state, never a silent switch of workspace. |

**Flow map:** launch → verify service (`App.tsx:146-153`, `apps/desktop/runtime.js:35-132`) → first-use guide (`App.tsx:239-245`) → choose a usable model (`ModelsPage.tsx`) → first reply (`docs/advanced/workspace-ui.md:259-270`) → add document (`KnowledgePage.tsx` or composer) → ask with cited passages (`KnowledgePassages.tsx`, `DocumentReader.tsx`) → optional agent profile/approval (`AgentProfiles.tsx`, `TaskApproval.tsx`) → inspect/recover work (`TaskWorkspace.tsx`). Each handoff currently exposes another section or panel; the proposed path keeps the question and its source visible while setup happens.

### CLI inventory

`internal/cli/registry.go:5-56` is the authoritative root name/alias inventory, but `cmd/offgrid/root.go:53-111` still dispatches several legacy families. Root families are: `recommend`, `doctor`, `appliance`, `init`, `download`, `download-hf`, `search`, `run`, `import`, `remove`, `export`, `chat`, `benchmark`, `compare`, `test`, `list`, `model`, `verify`, `shell-completion`, `export-session`, `quantization`, `quantize`, `info`, `config`, `peers`, `alias`, `favorite`, `template`, `batch`, `session`, `kb`, `workspace`, `completions`, `users`, `audit`, `metrics`, `lora`, `agent`, `integration`, `computer`, `hermes`, `openclaw`, `audio`, `logs`, `serve`, `version`, `help`. The aliases are listed beside them in the registry and should remain compatible.

| CLI area | Actual examples/flags and output contract | Evidence |
| --- | --- | --- |
| Main help | Working source now leads with installed models, agent conversation and doctor; `help all` retains the full catalog. Installed CLI output has not been recaptured in this audit. | `internal/cli/help.go:10-39` |
| Models | `model list/catalog/search/discover/preview/install/status/cancel/resume/discard/repair/verify/remove`, plus older `list/download/download-hf/import/export/verify/quantize`. `--category`, `--revision`, `--variant`, `--architecture`, `--yes`, `--detach`, `--request-id`, `--json`. | `internal/cli/model_package_command.go:83`; `model_download.go:16-18` |
| Chat, knowledge, audio | `run MODEL --image`, `session` create/show/send/chat/attachments, `kb` status/list/add/import review/retry/OCR/source/retention/search/clear, `audio` transcribe/speak/setup. | `internal/cli/session_service.go:18-25`; `knowledge_service.go:18`; `audio_command.go:69,171,328` |
| Agents | Compact `agent help`; `agent help all` contains lifecycle, profiles, skills, memory, schedules, tools, MCP, workspace trust. Run flags include `--model`, `--profile`, `--workspace`, `--workspace-folder`, `--workspace-access`, `--response-profile`, `--wait`, `--detach`, `--style`, `--max-steps`, `--request-id`. | `internal/cli/agent_help.go:28-105`; `agent_session_presentation.go:50-139` |
| Automation/error | Global `--json`, `--no-color`; JSON error `{code,message,retryable,request_id?}`, stdout/stderr split, exit 0/1/2/130; `NO_COLOR`, Unicode and non-TTY fallbacks. Some old paths retain ad hoc usage strings. | `internal/cli/invocation.go:5-23`; `docs/advanced/client-contracts.md:7-38`; `docs/advanced/cli-experience.md:123-142`; `cmd/offgrid/root.go:110-112` |

This is a command-family inventory, not a claim that every dynamic server error string has been enumerated. A release-quality CLI copy audit should extract and snapshot each command's `--help`, invalid-argument output, cancellation, offline service, and JSON form from the executable; the repository currently has tests for subsets in `cmd/offgrid/command_result_test.go`.

## Audit findings

**Severity:** High means a core task is confusing or hard to verify; Medium means recurring friction or implementation risk; Low means bounded polish. No Critical safety/data-loss defect was established by this design inspection. I1–I6 are the installed-renderer captures above. Other rows are source analysis and must not be presented as observed native-window failures.

| Severity · surface | Evidence | Why it hurts users / required correction |
| --- | --- | --- |
| **High · Web/desktop** | **I1**; `internal/sessions/attachment_context.go:148`; `apps/web/src/features/chat/ChatPage.tsx:540,544` | An actual saved answer repeatedly cites `[2610.39302v1.pdf: 0-1178]` and even explains the byte-range citation rule. This is not merely bad model prose: the attachment prompt explicitly requests byte ranges, while Chat renders ordinary Markdown. Keep exact ranges internally; resolve validated references into readable passage/page controls. Never invent page numbers or turn arbitrary model text into an authorized source link. |
| **High · Web** | **I1, I4**; `KnowledgePage.tsx:210-249`; `features/chat/KnowledgePassages.tsx:18-29` | Chat contains document-backed work while Knowledge says no documents are indexed. These are valid distinct stores, but the product does not explain the difference at the handoff. Label “In this conversation” versus “In your document library” and offer an explicit library action. Reuse Knowledge's verified source-opening pattern for attachments; do not imply every upload is indexed or retained identically. |
| **High · Web** | **I2**; `ModelsPage.tsx:193-238` | A cancelled 3.6% download occupies roughly 230 vertical pixels above five installed models and repeats that data is kept. The user's primary task—use an available model—is pushed down. Keep resumability in a compact Downloads section; expand active operations or requested recovery. Do not delete partial data or hide failures. |
| **Medium · Web** | **I2**; `ModelsPage.tsx:207` | The chosen model has a checkmark but still says “Select model.” Every installed model says “Runtime unchecked,” offering no immediate explanation of whether chat can run. Use “Selected” for the active choice; distinguish installed, runnable and workflow-qualified, with one contextual check action. Do not equate successful ordinary chat with verified agent/vision capability. |
| **High · Web** | **I3**; `features/activity/ActivityPage.tsx` | The global header advertises seven attention items, but Activity opens on recent completed/answered history. Put Needs attention / Running / History in one predictable hierarchy and deep-link the header to the same filtered view. Preserve unknown-effect recovery records and explicit deletion semantics. |
| **Medium · Cross-cutting** | **I1–I6**; `components/shell/WorkspaceNavigation.tsx`; `lib/navigation.ts` | Installed navigation gives Chat, Agents, Knowledge, Models and Activity equal standing. Settings is already in Workspace options; it is not a sixth equal sidebar row. Work/Library grouping exists only in the candidate. Validate that grouping with real tasks rather than claiming it already solved the installed experience. |
| **Medium · Web/desktop** | **I1**; `ComposerOptions.tsx`; `workspace-ui.md:200-218` | The installed compact composer is a genuine improvement and should be preserved. Its icon-only response/settings entry and adjacent microphone/Talk actions still need clear accessible names and first-use distinctions. Verify keyboard labels and listening/sending states; do not restore the old all-settings-at-once toolbar. |
| **High · Cross-cutting** | `README.md:214-225`, `workspace-ui.md:238-257`, `SettingsPage.tsx:22-100` | “Service ready” identifies connectivity, not locality or privacy. A Docker/remote endpoint and an external search tool can move data. Show inference location and outbound connection scope separately; make offline status evidence-based. |
| **High · Web** | `TaskApproval.tsx`, `TaskDesktopAccess.tsx`, `TaskWorkspace.tsx`; `README.md:296-303` | The brief's universal 15-minute exact-call approval conflicts with OffGrid's approved full-desktop-session model. Exact-call grants exist for eligible agent calls; desktop Full access covers its session without repeated prompts. UI must show which authority is active, target/effect/expiry, and the stop path without suggesting every click needs approval. |
| **Medium · Web** | `styles.css:3-17,53-65,90-117`, `workspace.css:128,172,199,301,327`, `main.tsx:1-21` | The loaded legacy stylesheet retains blue gradients, glow, 8–11px metadata, and large one-off treatments. Later layers override much of it; this is cascade debt, not proof that blue is currently visible. Remove dead declarations after computed-style comparison, consolidate live values into tokens. |
| **Medium · Web** | `tokens.css:4-80,85-145`, `controls.css:7-34,91-122`, `model-library.css:12-44`, `document-reader.css:8-38` | Semantic tokens exist, but feature rules still introduce local sizes/radii/colors. A quick scan found hundreds of literal geometry/color occurrences across CSS; the token proposal below needs a computed-style inventory before changing them. Preserve explicit focus/forced-colors overrides. |
| **Medium · Web** | `features/chat/DocumentReader.tsx:11-85`, `document-reader.css:23-45`; older PDF screenshots are historical examples only | PDF reading has page/search/zoom and a preparation border, but source coverage and unread figures can still compete with the answer area. Verify the current installed layout, then make the page, coverage, and cited passage the hierarchy if that problem remains. Motion must indicate an actual preparation stage, not a decorative indefinite scan. |
| **Medium · Web** | **I3**; `ActivityPage.tsx`, `TaskWorkspace.tsx` | “Clear removable tasks” exists now; claiming bulk removal is absent would be incorrect. Its eligibility is not obvious from the label, and repeated short task titles make history difficult to scan. Explain the removable count and protected reasons in the review, and add useful result/agent context without exposing technical IDs by default. No live deletion was attempted. |
| **Medium · Web** | **I5**; `AgentPage.tsx:15-56`, `AgentProfiles.tsx:119-176`, `AgentManagement.tsx:123-180` | Agent profiles repeats as tab and heading, followed by a structural explanation and a largely empty detail pane. The list gives name/instructions but little assistance choosing a useful profile. Show purpose, usable capabilities and a clear “Use in Chat” path. Keep editing, resources and revisions secondary. Saved persona text is user-authored; it is not a product-default defect. |
| **Medium · Desktop** | `apps/desktop/main.js:217-277,295-323,442-466`; `loading.html:12-33` | Standard menu roles and tray controls are present. On Windows, closing a healthy window hides it in the tray, which can feel like an app that will not quit. Put clear “Still running in tray” feedback and a distinct Quit path in the product; verify OS appearance and zoom in installed builds. |
| **Medium · Desktop** | `apps/desktop/main.js:304-306,360-366`; `terminal-dock.js:7-67`; `terminal-renderer.js:16-27` | Terminal is real and desktop-only, with tabs and explicit cleanup. Its current header button, View shortcut and tab launch need one consistent label/identity, especially when unavailable on non-Windows builds. A web fixture cannot verify its visual placement. |
| **Medium · CLI** | `internal/cli/help.go:10-39`, `registry.go:5-56`, `cmd/offgrid/root.go:53-111` | The source's compact first-use help is already improved. The remaining risk is consistency across migrated and legacy handlers, not a need to rewrite that help again. Snapshot executable help/error/JSON paths and verify the installed bundled CLI matches the reviewed source before release. Preserve aliases. |
| **Low · CLI** | `internal/cli/agent_help.go:28-105`, `agent_session_presentation.go:50-139`, `docs/advanced/cli-experience.md:98-142` | Compact agent help and `/` discovery already exist. Inconsistent terms—run, turn, job, session, workspace—still leak into ordinary copy. Reserve IDs and protocol terms for inspect/JSON; use “conversation,” “work,” and “folder access” in normal output. |

### Decisions and terms in the first minute

These are **branch counts from inspected UI/source**, not timed usability measurements or measured click counts. Typing the task and picking the source file are task decisions, not configuration. Assume English, a healthy native service and an already installed model unless a row says otherwise. Five non-developer participants are not available, so a claim about completion within 60 seconds remains unverified.

| Journey | Configuration decisions / terms encountered | Target and visible acceptance |
| --- | --- | --- |
| Returning launch with usable saved model | **0 required configuration decisions**; service recovery appears only on failure (`apps/desktop/loading.html`). | Preserve this path. The first message must not require revisiting model, agent or permission setup. |
| First launch with usable installed model | **1 primary setup decision**, start chat; the observed fresh-browser guide also exposes three diagnostic rows: connected service, chat-model count, processing location. | One next action; diagnostics disclosed. Do not confuse fresh-browser onboarding with the user's existing desktop state. |
| No usable model, download path | **At least 4 choices:** installed/discover, model, variant when needed, installation review. Category adds a fifth if changing modality. Terms include runtime, installed size, quantization/variant and context. | Recommend a compatible choice while retaining alternatives and an explicit download confirmation. Runtime checks cannot silently add downloads. |
| Ask about one local document | **2 conceptual choices:** attach to conversation versus library import, then which source. Partial extraction may add a third: retry versus available-text use. | Default to conversation attachment; offer durable library storage separately. A readable PDF should not require understanding embeddings. |
| Verify a cited claim | **No useful direct choice in I1:** citation is a byte string. The user must find/open the document and search manually. | One source control opens the exact verified passage/page; unavailable sources explain why. |
| Use existing agent | **1 configuration choice:** profile. Folder or desktop access is a separate conditional authority decision. Creating a profile adds purpose, model and resource choices. | Keep reusable model/tool/budget defaults in the profile. Ask only for required access, with its duration and target clear. |
| Resolve interrupted work | **At least 2 choices:** locate the run, interpret the state and choose its safe action; unknown effects require inspecting the target. | Attention list leads directly to the relevant run and permitted recovery. Never compress uncertain writes into a generic Retry. |
| Update desktop attached to external service | **At least 2 component decisions:** desktop and service ownership/version. Bundled-service updates should not expose that choice unnecessarily. | Explain what will change, what remains running and which workspace is retained. Test separately from the GUI's normal first-use flow. |

## One design direction: the local workbench

1. **One work surface.** Ask and inspect beside the relevant source; do not make a user switch to Knowledge to understand an attached PDF.
2. **Progressive capability.** Show usable defaults; advanced model, agent and retrieval controls appear when chosen or required. Do not hide a blocked action's repair.
3. **Truthful locality.** Identify the active service, model device and authorized external destination separately. Do not equate `127.0.0.1` with guaranteed privacy.
4. **Evidence before completion.** A citation opens a readable passage/page; an agent result shows what was verified. Do not render raw offsets as user-facing citations.
5. **One primary action per state.** Preparing, blocked, ready, running, and uncertain each have one clear next action. Do not stack generic Refresh buttons beside every notice.
6. **Platform-native restraint.** Use OS menus, focus, and file pickers; use motion to confirm a state transition. Do not use glow, purple/blue hero gradients, or endless decorative scanning.
7. **CLI for precision.** Human output is concise and progressive; JSON and full help retain every detail. Do not make the GUI a visual copy of CLI flags.

**Navigation:** Work (Chat, Activity; Agents after opt-in), Library (Documents, Models), Settings. The command palette searches navigation and saved work; it does not become the only way to find essential setup. A desktop terminal opens as an alternate main-area tab with the selected folder and active session identity, never as an unlabelled floating shell.

**Token direction:** consolidate the existing system rather than introducing another parallel theme. `tokens.css:4-80` already contains most of the intended scale: IBM Plex Sans/Arabic and Mono, type 12/13/14/16/18/24, spacing 4/8/12/16/24/32/48, panel radius 12, editor radius 16, press 120ms and entrance 180ms. Neutral accent roles are already present. The implementation task is to make feature CSS consume these roles, not claim these foundations are missing.

| Role | Light / dark values or rule | Acceptance |
| --- | --- | --- |
| Canvas / working surface | Existing `#f7f7f6` / `#fff`; dark `#101011` / `#181819` | Retain these; a source reader and editor separate through spacing and one boundary, not nested outlines. |
| Text / muted | Existing `#171717` / `#535351`; dark `#f5f5f4` / `#b9b9b7` | Verify computed contrast against actual translucent backgrounds; no opacity on essential text. |
| Accent | Neutral ink/light ink from existing tokens | One primary action, no new branding gradient or glow. Selection also has text/checkmark. |
| Status | Explicit text/icon; retain measured error color | Existing green/amber roles are neutral. Do not promise three different status colors when the palette does not supply them. |
| Typography | 14 controls, 16 prose, 18 sections, 24 page title; 12–13 only metadata | Long answers retain a readable measure; source identifiers never dominate their titles. |
| Geometry | Existing 7px small controls, 10px medium, 12px panels, 16px editor | Do not churn every control from 7px to 8px for cosmetic uniformity. Eliminate unexplained feature-specific variants. |
| Elevation / motion | Existing floating shadow; 120/180ms with existing easing | Elevation only for overlays. Preparation remains animated only during real work; reduced motion gives static stages. |

The CSS source audit is not an exhaustive computed-style census. Live token overrides and legacy declarations need a per-component light/dark/RTL inventory before deletion. An unused blue rule is maintenance debt, not evidence that the installed app visibly has a blue theme.

**Signature components:** (a) a small **Workspace status** indicator stating “On this computer,” “Connected service,” or “External connection,” backed by actual identity; (b) an **Action review** card with target, effect, scope, expiry or session duration, and Stop/Continue; (c) citation chips with document title and page/section that focus the verified passage; (d) capability badges **Core / Optional / Experimental** only when backed by runtime readiness; (e) a document-preparation border that advances on real stages and becomes still when waiting or failed.

**Component rules:** The composer gets one input line, attach, voice and Send; active source/model/agent show as a readable single context row when relevant. A message has clear speaker and content, with Copy/Read aloud on demand. Source chips open the exact source. Model cards lead with name, readiness and fit, then reveal bytes/format/context. Approval cards never obscure the action target. Empty states name the first useful action. Error states preserve drafts and distinguish retryable read, uncertain write and missing capability. Settings start with personal preferences; service paths, protocol and diagnostics live under Advanced. Menus remain bounded, keyboard navigable and dismissible without losing the draft.

**Terminology:** “Model” is acceptable; “installed and ready” beats “runtime unchecked” where the check is available. “Add a document” precedes “embedding/index”; “Read text” precedes “OCR”; “Longer conversation” precedes “context allocation”; “Folder access” precedes “execution workspace.” Keep GGUF, projector, quantization, token budgets, run IDs and provider revisions in inspect/Advanced. Never say an image or figure was understood when only text was extracted.

**CLI language:** `offgrid --help` shows four next steps first, then category links. `offgrid agent` starts conversation; `agent run` is explicitly one task; `agent status` inspects saved work. Progress is one changing line on capable TTY, plain stage lines on dumb/redirected terminals, with `NO_COLOR` and Unicode fallbacks. A final success block shows result plus one inspect command; errors state what happened, what is preserved, and one safe command. Keep existing exit and JSON contracts (`client-contracts.md:7-38`).

## Phased implementation plan

Each phase should extend the existing `workspace-outcome-experience` change, or its owning terminal/document change, with explicit acceptance scenarios before source edits. These are independently reviewable slices; none changes the API by default. Existing saved profiles, models, history, permissions and routes remain readable.

| Phase | Source and concrete work | Hidden/removed from default view; API risk | Tests and non-expert eye check |
| --- | --- | --- | --- |
| **0. Tokens/cascade** | `apps/web/src/tokens.css`, `styles.css`, `workspace.css`, `controls.css`, feature CSS; `apps/desktop/startup.css`. Record computed styles in light/dark/RTL/forced-colors, remove superseded blue/glow and 8–11px live rules, consolidate semantic values. | Remove dead CSS only. No API/type change. | `npm run check/build`, visual fixture at 320/390/1440 and 200% zoom; inspect installed desktop startup. Eye check: a model row, citation, error and menu clearly belong to one system in both themes. |
| **1. Shell/Chat** | `App.tsx`, `lib/navigation.ts`, shell, `ChatPage.tsx`, `ComposerOptions.tsx`, `DocumentWorkspace.tsx`, `ConversationWork.tsx`, history components. Group navigation; make composer and source context legible; keep terminal as optional alternate main tab. | Hide technical settings and duplicate status; retain all routes and controls. No API change. | `workspace-shell`, `chat-streaming`, `chat-files`, `document-reader`, `workspace-handoffs`, keyboard/RTL Playwright. Eye check: a new user can send and attach without opening Settings; a running job's Stop remains visible. |
| **2. First run/Models** | `App.tsx` onboarding, `ModelsPage.tsx`, `ModelChoice.tsx`, model library CSS, readiness copy. Offer one measured eligible path and a truthful offline import route. | Hide formats/variants until inspect; do not auto-download or switch. No API change unless readiness metadata is genuinely missing; if so, add fields additively to OpenAPI and regenerate types. | First-reply and model-acquisition/restart fixtures plus real installed model smoke. Eye check: user knows what will download, memory needed and whether image/voice works. |
| **3a. Documents — highest functional priority** | `internal/sessions/attachment_context.go`, `attachment_selection.go`, `ChatPage.tsx`, `KnowledgePassages.tsx`, `DocumentReader.tsx`, `KnowledgePage.tsx`. Replace the model-facing demand for byte citations with validated source identifiers and a readable rendering path. Preserve exact ranges internally and distinguish conversation attachment from library source. | Existing offset records alone do not guarantee page locations. Determine whether retained extraction units can resolve them. If not, add revision-bound citation locators through the API and regenerate clients explicitly; never guess pages or silently reinterpret old answers. No deletion or automatic library import. | Exact UTF-8 ranges, repeated filenames, forged/out-of-selection citations, inaccessible/deleted sources, old histories, selected excerpts, RTL and keyboard focus. Real PDF answer → citation → independently checked passage, over three follow-ups. Eye check: the reader opens where the claim is supported, with no debug offsets required from the user. |
| **3b. Agents** | `AgentProfiles.tsx`, `AgentManagement.tsx`, `TaskApproval.tsx`, `TaskWorkspace.tsx`, activity list. Purpose-led profile defaults, explicit authority, one next step for paused work. | Tools/connections/budgets under Advanced; never hide Stop, expiry or uncertain outcome. No authority/API change. | Agent profile/approval/permission/recovery fixtures; eye check: reviewer can say which file/app/account and effect an approval covers. |
| **4. Desktop** | `apps/desktop/main.js`, `loading.html`, `startup.js/css`, `terminal-renderer.js`, `terminal-dock.js`, menus/tray. Clarify hide versus quit; align terminal tab/header with native menu; check system theme/zoom and startup recovery. | No new shell auto-start; no IPC contract change without a versioned bridge revision. | `npm --prefix apps/desktop test`, packaged startup/recovery, isolated terminal process cleanup on supported OS. Eye check: close/tray/Quit behavior and active terminal identity are unmistakable. |
| **5. CLI** | `internal/cli/help.go`, `agent_help.go`, `registry.go`, `cmd/offgrid/root.go`, terminal presentation and reference docs. Rewrite first-minute help, unify nouns and errors, snapshot every command family, retain aliases. | Full reference remains available as `help all`; JSON, stdout/stderr, exit codes and command arguments unchanged. | CLI golden/help tests, TTY/plain/NO_COLOR/JSON and offline/cancel paths; eye check: a nontechnical tester can discover a model and get one reply from `--help`. |

**Release gate:** capture the installed v0.4.25 build as the baseline, then repeat packaged UI captures on its successor, including Windows terminal, all six main screens, onboarding, error/empty/loading, approval, model review, document reader and narrow layout. Automated fixtures are necessary but insufficient for installed desktop visual claims. Record a manual keyboard/screen-reader and nine-locale review as unverified until done; do not substitute old screenshots or fixture captures for those checks. Keep current release gates in `docs/advanced/production-readiness.md` authoritative.

## Implementation status — 2026-11-05 candidate

The first implementation increments address navigation/onboarding and CLI
discovery, then the installed-app findings for readable document citations,
selected models, cancelled transfer history, Activity prioritization and the
Knowledge/Chat distinction. Exact retained passages now have an authenticated
API and reader navigation; repeated passages do not receive guessed pages.
Activity filters persist through inspection and reload, and citation focus
returns on reader close.

The full isolated renderer suite passed (1,002 passed, 31 skipped), along with
the affected Go packages and packaged Windows appearance/terminal checks.
See [implementation evidence](../quality/evidence/workspace-design-follow-up-2026-11-05.md)
for commands, earlier failures and their corrections, and qualification limits.
These are candidate results, not a replacement of the installed 0.4.25 baseline.
The complete token inventory, remaining first-use/agent/recovery journeys,
real-model document-answer qualification and human review are not complete.

## Review checklist (answer yes/no for each change)

1. Can a first-time user see one obvious next action without opening a menu?
2. Does the screen say what is happening now in ordinary language?
3. Is the selected model identifiable without exposing its full technical ID everywhere?
4. Is the service/locality claim backed by the actual connected endpoint and provider?
5. Is any outbound destination shown before private content can be sent there?
6. Does an approval show exact target, effect, scope and duration?
7. Is Stop or takeover available while an agent can still act?
8. Does a citation open a readable page or passage, not just an offset string?
9. Are unread figures, scanned pages and unverified answers explicitly marked?
10. Is only one primary action visually dominant in each state?
11. Are secondary controls hidden until relevant while remaining discoverable?
12. Does an error preserve the draft and give one safe recovery step?
13. Are uncertain mutations protected from automatic retry?
14. Do type size and contrast remain readable at 200% zoom and 320px width?
15. Is keyboard focus visible and restored after every menu/dialog?
16. Do reduced-motion and forced-colors modes retain state meaning?
17. Do light, dark, RTL and all nine locales keep the same hierarchy?
18. Is any animation tied to real progress or a meaningful transition?
19. Does desktop close/quit/tray/terminal behavior match what its label promises?
20. Does CLI human output stay concise while `--json` and full help remain stable?
