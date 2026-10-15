# Design

## Context

See [proposal.md](proposal.md) for motivation and acceptance outcomes. The current React Chat uses persisted session turns; agent work uses a separate durable runner and SQLite store. `internal/server/agent_jobs.go` supplies task-first instructions, and `configureDesktopTask` in `desktop_computer_tools.go` replaces those instructions with a one-tool-per-response computer protocol. Workflow completion in `internal/agents/workflow_completion.go` is enabled by computer-session configuration, not evidence that the user requested an external outcome. The inference response format already permits messages as well as tool calls.

The registry composes native and MCP capabilities, but the model can still see irrelevant tools. Application context initially projects 40 entries; the host can report more. Existing history retrieval is available, but does not make requested-app lookup an explicit discovery operation. Ordinary built-in errors can occur after an executing marker is persisted, including path validation before any write. The generic failure path can consequently classify a known rejection as uncertain.

`TaskWorkspace.tsx` is the current detailed work UI. `ChatPage.tsx` already owns saved conversation streaming, Knowledge, voice and drafts. The retired in-memory coordinator and old agent renderer must remain retired. Native document providers, platform qualification and unfinished work remain owned by `native-document-workflows`. Use the architecture, client-contract and product-reliability documents as the ongoing authoritative guides.

## Goals / Non-Goals

**Goals:** one execution authority; durable links between different persistence stores; configurable behavior without configurable security bypasses; bounded model context; useful recovery based on evidence; a shared conversation projection usable by browser, desktop, CLI and Python.

**Non-Goals:** new native drivers, universal application support, replacing inference or speech engines, automatic model acquisition, new OAuth/secret-storage infrastructure, arbitrary command configuration, parallel desktop controllers, or asserting native-platform qualification from contract tests.

## Decisions

### 1. Conversation is a first-class runner result

Retain the existing runner and its answer/tool response protocol. Do not introduce a separate intent-classification model call on every turn or an English keyword router. The same inference step can answer, ask a clarification or propose an authorized action. Native actions remain serialized; the protocol must say that an action response contains at most one native action, rather than requiring every response to contain an action.

Separate available desktop authority from a task's requested execution scope. A Chat profile can enable computer discovery without attaching a desktop session before the model identifies relevant host work. Existing remembered Full access can satisfy the local grant step under its current identity and lifetime rules. Availability alone cannot create an application expectation or open an access-needed card.

Track intended external outcomes separately from grants. For a substantial task, persist a short plan and checks derived from the user request and subsequent corrections. These are model interpretations, not proof of authority or complete understanding. Pure conversation can finish without checks when there is no outstanding requested external outcome or unresolved dispatched work. Existing edit preconditions and required verification continue to apply once host work is proposed. A final answer must not clear unresolved effects.

After a verified final step, provide a compact completion state and a message option. Do not add a calculator call to express a sentence. Enforce explicit requested application constraints when resolving an observed identity. A conflicting app choice receives corrective feedback before dispatch. Test multilingual and mixed conversational/action requests rather than hardcoding the observed English prompts.

Alternative considered: forcing all tool-enabled turns through computer workflow checks. Rejected because permission is not task intent and the current greeting failure demonstrates the consequence.

### 2. Profiles configure behavior and resource restrictions

Add actor/workspace-scoped profiles alongside the durable agent store, using additive schema migrations. Logical fields are profile ID, owner, workspace, display name, description, archive status and current revision. Immutable revision records contain instructions, installed-model reference, authorized Knowledge references, tool/connection references, execution budgets, optional delegation-profile references, creation metadata and a digest. Writes use an expected revision for conflict detection.

At admission, resolve and pin the entire effective configuration, including child-profile revisions. Do not depend on a mutable profile row while a task runs. Archived profiles disappear from new selections but remain available through authorized historical work. Profile exports contain references and instructions, never credentials; imported references require local resolution and authority validation.

Compose instructions from versioned runner protocol, bounded profile instructions, user conversation and the current task stage. Keep policy enforcement in the broker and host. Replace brittle string replacement of protocol paragraphs with explicit composable sections and tested feature variants. Operators can edit persona/workflow instructions through profiles; they cannot disable grants, effects checks or actor isolation through text.

Effective capability authority remains the intersection of workspace configuration, actor permission, profile restrictions, conversation/task restrictions, current grants and model/runtime readiness. Stable capability identity includes provider, tool ID, schema revision/digest, execution location and reviewed policy classification. Recheck at execution. MCP annotations remain hints. A schema change invalidates affected approval identities; it does not silently widen a saved tool-set grant.

Keep personal profile ownership separate from connection ownership. The first implementation attaches only currently authorized connections and explicitly labels existing workspace-admin connections. It does not claim that those legacy credentials are personal OAuth credentials. New connection management uses the existing management authorization rather than giving Chat users admin rights.

Alternative considered: storing all configuration in a custom system-prompt string. Rejected because it cannot reliably express resource restrictions, revision history or credential ownership.

### 3. Coordinate existing session turns with held canonical jobs

Extend the existing turn submission contract with optional profile revision and restricted resource references. Plain text-only Chat retains its current path. A tool-enabled turn creates a canonical durable job even if it ends in an ordinary answer.

The coordinator uses these persisted stages:

1. Validate actor, profile and input. Persist a turn intent with an actor-scoped request ID and digest of normalized payload, including session, selected revision, model override, Knowledge and resource restrictions.
2. Create or retrieve a held job using that same identity. Held jobs cannot dispatch.
3. Persist the conversation/turn/job association.
4. Record admission complete and release the held job idempotently.
5. Project canonical job events into conversation activity.
6. Commit final conversation output once using the turn/job identity.

Use a unique job origin key and compare the payload digest on every retry. Same identity and different payload is a conflict. Persist enough journal state to distinguish accepted admission from an abandoned draft. Recovery can complete bookkeeping for a durably accepted submission, but cannot infer approval, replay an executing action or resume an interrupted job. Merely observing a page invokes no admission or release operation.

Session JSON and agent SQLite cannot share one transaction. Use atomic session replacement and an idempotent coordinator/outbox record, with fault injection before and after every write. If the final conversation write fails, the job remains the result authority; a recovery operation attaches its existing result without calling inference or tools again.

Conversation deletion consults linked active jobs. The user chooses stop-and-delete, waiting for confirmed stop, or retain work in the secondary inspector. Keep content-minimal deduplication tombstones independently of visible history. Retention must preserve existing workspace policy; deleting a title is never authorization to rerun its task.

Bound conversation history supplied to the runner: user messages, assistant outputs, selected authorized Knowledge and compact prior work summaries. Full native payloads and arbitrary MCP results do not become permanent system instructions. Pin model and profile per turn. A running-job correction is a persisted steering event applied at the next safe boundary; subsequent messages can start a new turn only when explicitly treated as new work. Voice uses the same submission/steering API.

Alternative considered: Chat creates jobs and immediately starts them, then saves the link. Rejected because a failed link write could leave unseen active work or duplicate submissions.

### 4. Discover relevant capabilities within the actual context budget

Authorize before relevance ranking. Present a bounded discovery summary and relevant tool schemas, retaining the ability to discover other authorized tools. Computer access adds capabilities and never removes MCP. Relevance is advisory; execution still requires the same policy checks. Keep provider and service/host execution location in the structured descriptor, not only prose.

Add requested-app lookup over the full host-issued catalog with bounded ranked matches and pagination. Match registered display names and aliases, including localized names. Do not synthesize executable paths or substitute a nearby alphabetic entry. Ask when multiple distinct applications genuinely match. Cache catalog results by host/catalog generation and invalidate on changes or restart.

Budget the actual model request, including tool schemas and the private tool-call envelope, rather than just visible conversation tokens. Reserve space for results and the response. Bound large MCP/native projections with explicit truncation, provenance, digest and authorized history retrieval. Preserve current user constraints, pinned configuration, outstanding action state and latest relevant evidence during compaction. If a viable request cannot fit, return a specific context limitation before dispatch.

Service artifacts and native documents are separate output types. Native document tasks discover negotiated document capabilities and use their preconditions and read-back. The existence or digest of an artifact does not prove that Notepad contains the requested text. Unsupported capability is a useful reported limit, not a reason to write an invented container path.

Alternative considered: exposing every tool and increasing context for every model. Rejected because small local models and hardware need bounded inputs, and larger context does not correct wrong execution locations or intent.

### 5. Classify effects at the trusted execution boundary

Retain durable intent/dispatch/result records. Return typed failures for validation that is proven to precede all effects: argument parsing, protected-path checks, authorization and unsupported operation checks. In particular, a path rejection before directory creation is not an uncertain write. Errors after directory creation, partial file writes, native input or remote dispatch require the appropriate partial/unknown classification. Do not mechanically convert every error into no effects.

Feed eligible pre-effect failures into the existing bounded proposal repair mechanism, up to three consecutive attempts. Unrelated reads do not reset the budget. Explain the actual argument, location or unavailable capability. On exhaustion, retain completed steps and an actionable failure rather than repeatedly prompting for the same access.

For uncertainty, query a trusted dispatch journal or perform permitted read-only observation before requiring manual reconciliation. A verification result must bind the relevant action, target and desired condition. A visible success-looking page alone cannot prove exactly-once submission. No recovery path may repeat an uncertain mutation to learn whether it succeeded.

Manual statements remain explicitly human-reported evidence with actor and timestamp. Preserve the original call arguments and unresolved checks. Free text such as done cannot automatically verify a document or an unrelated task outcome. The UI offers inspect, take over, stop and, only when needed, record-observation recovery. Resuming execution remains explicit at the existing boundary.

Alternative considered: hiding the outcome form or relaxing uncertain-action handling. Rejected because neither establishes what happened and both conceal useful evidence.

### 6. One conversation presentation, a profile library and a secondary inspector

Evolve the current `ChatPage` and `AgentPage`; do not restore the retired renderer. Chat's composer gets a compact agent selector beside existing model, Knowledge, Tools and voice controls. Show profile identity and current access in the composer without another configuration wall above the conversation. No profile selected preserves existing text-only Chat.

Chat renders user text and assistant output as messages. Active work uses one compact activity row with a concise current step, elapsed state and optional progress disclosure. Completed work shows the result and useful attachments. Technical payloads, context counts and detailed evidence are collapsed. Do not repeat the full prompt as an oversized task heading.

Agents becomes a profile library with create, edit, duplicate and archive actions. Profile editing groups identity/instructions, model/Knowledge, connections/tools and advanced budgets/delegation. Show effective capability location and unavailable reasons. Reuse connection-management components and their permissions from Settings; avoid copying a second connection administration implementation.

Keep a secondary Work inspector reachable from Chat, known work and old job links. Unlinked historical tasks remain inspectable there; navigation never creates a conversation or resubmits them. A linked job opens its conversation and focused activity when appropriate. Keep legacy APIs as adapters but never use a legacy page as a missing-feature fallback.

Persistent Pause, Take over and Stop controls remain in the shared app shell during computer control, including after navigating to profiles. Requested stopping and confirmed stopped remain visually different. Preserve local host stopping if the service is disconnected. No new consent dialog is added by profile selection.

Use the existing monochrome tokens, typography, border treatment and real button components. Draw default, running, approval, unavailable, failed, uncertain, partial and complete states before implementing the final composition. Verify desktop and narrow/short layouts, both themes, all nine locales, RTL, IME, keyboard order, focus restoration and concise screen-reader announcements. Spoken output remains parent-facing and does not narrate nested tool payloads.

Alternative considered: restyling the existing full task inspector as the primary Chat surface. Rejected because its repeated headings, forms and execution details would retain the current reading burden.

### 7. Extend delegation and clients through the same service

Use `internal/agents/coordination.go` and canonical child jobs. Resolve approved child profiles to immutable revisions, intersect their requested capabilities with the parent's authority and reviewed read-only delegation set, reject cycles and bound depth, child count, aggregate iterations and context consumption. Children cannot grant access, create overlapping desktop leases or speak independently. Aggregate their results and failures into the parent's compact activity.

Expose profile CRUD and optional turn/job origin fields in the service OpenAPI contract; generate TypeScript types and update Python methods. Extend existing `offgrid agent run/status/...` commands and add the explicit `offgrid agent profiles` namespace, avoiding the legacy template listing semantics. Add profile selection to the existing conversation client path. CLI profile flags are service references; never launch a CLI-local executor.

Interactive terminal output shows assistant text and compact progress, with an explicit details option. Structured non-interactive output represents pending access, failed, partial and uncertain states with documented exit semantics. Inspection and reconnect commands cannot approve or resume work. Unsupported server capability produces a compatibility explanation, never fallback to the retired orchestrator.

## Risks / Trade-offs

- Model semantic mistakes remain possible -> freeze the observed failures as regression cases and run installed-model evaluations separately from deterministic contracts. Typed arguments alone do not demonstrate useful agency.
- Two persistence stores can diverge -> held admission, unique origin identity, atomic session updates and fault injection at every transition; preserve result authority in the job.
- Profiles can appear to promise unavailable tools -> validate references on save and admission, show health separately from authority, and recheck immediately before execution.
- Large catalogs compete with conversation context -> authorize then bound discovery and results; measure actual request size and compaction, including tools.
- Changing the main work surface can hide active control -> shared-shell controls, legacy-link integration tests and observation-only navigation.
- Configurable instructions can contradict safeguards -> compose bounded instructions, record revisions and enforce authority outside model text.
- Native tests require actual supported desktops -> keep unrun Windows/macOS/Linux workflow checks explicitly unverified; synthetic hosts and compilation do not qualify native control.

## Migration Plan

1. Add profile and job-origin schemas with reversible/additive migrations and disposable upgrade fixtures. Existing jobs and conversations remain valid without profile or origin fields.
2. Land effects/intent/discovery corrections and shared contracts before changing the default navigation. Add feature negotiation so mismatched clients give a useful message.
3. Introduce profile management and linked tool-enabled turns behind one coherent service capability. Avoid exposing a partially functional profile picker against an older server.
4. Switch the current Agents entry to profiles and retain stable work-inspector routes. Preserve drafts and saved selections by actor/workspace; do not reinterpret legacy templates or silently expand grants.
5. Run the specified suites and record real model, UI and available platform evidence in `docs/advanced/product-reliability-plan.md`. Do not mark dependent native qualification complete here.
6. Deployment is a separately authorized operation: back up affected stores, build verified current UI assets, replace the existing container with one running service and update matching desktop assets if required. Verify image/build identity, health and a read-only profile/conversation smoke check.

Rollback disables new admissions while retaining canonical work and data. A previous binary must refuse or safely ignore unsupported records, not delete them. If its schema compatibility cannot be established, restore the pre-deployment backup only after confirming no newer user work would be discarded; otherwise retain the current data and fix forward. Active or uncertain jobs are never automatically resumed during either direction of migration.
