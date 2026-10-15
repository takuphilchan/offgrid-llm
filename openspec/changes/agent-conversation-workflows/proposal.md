# Proposal

## Why

Current task-wide access couples permission to computer-work intent: even a greeting can acquire an application-opening check. Recent installed-model runs also substituted service files for native documents, continued with unrelated tools after verified app opening, and asked for manual outcomes after a proven pre-write rejection. OffGrid needs a conversational agent product, not more tool-specific screens or prompts layered onto these defects.

## What Changes

- Make Chat the primary conversation/work surface. Make Agents the management home for reusable, versioned agent profiles; retain detailed canonical-job inspection as a secondary work view. The user confirmed this product direction on 2026-10-13.
- Support ordinary answers, clarification and tool proposals in the same conversation. Enabled tools and remembered Full access are available authority, never a requirement to act. Do not use English keyword routing or application-specific task scripts.
- Route tool-enabled conversation turns through the existing durable runner, including turns that use no tools. Link each turn to one canonical job with crash-safe admission and final-response bookkeeping; preserve ordinary text-only Chat's existing path.
- Add actor-owned profile configuration for instructions, installed model preference, permitted Knowledge references, tool/connection attachments, budgets and optional read-only delegation. Pin a profile revision per turn; instructions cannot grant permissions or replace non-configurable execution safeguards.
- Compose authorized capabilities with profile/conversation/task restrictions, then present bounded relevant tools. Add requested-app lookup across the full host catalog, bounded MCP result projection and explicit service/host locations. Do not hide MCP by attaching a computer session.
- Separate user intent, plans, action preconditions and verification evidence. Treat clean pre-dispatch rejection, completed action, partial effects and unknown effects distinctly. Use safe observation/journal recovery before manual reconciliation; never auto-replay an uncertain mutation.
- Render conversation, concise activity and verified results as the primary reading flow. Put payloads, context statistics and full evidence in expandable details; maintain persistent control/Stop independently of navigation. Manual outcomes are exceptional recovery, not a second prompt.
- Align CLI and Python clients with the same profiles, conversation turns and jobs. Preserve existing commands/endpoints as adapters; do not introduce another orchestrator or CLI-local agent executor.

### Acceptance

1. With a profile, MCP tools and Full access enabled, `hello` yields an answer with zero host actions, tool executions or required verification declarations.
2. `Open OBS` performs one authorized launch, verifies OBS and answers, without calling calculator or unrelated documentation tools. `Open Settings` resolves Settings beyond the first catalog page; ambiguity asks the user instead of substituting Access.
3. The exact unsaved multilingual Notepad workflow uses native document tools, not an artifact or service file. Missing native capability is reported before input; actual Windows completion still requires the separate native qualification gate.
4. A trusted protected-path/argument rejection known to precede effects gets bounded corrective feedback, not an outcome textarea. A lost acknowledgment after input remains uncertain and is never replayed.
5. A tool-enabled turn and its canonical job survive failure at every persistence boundary. Reconnect, navigation, profile editing and opening details create no second job, approval or action.
6. Users can configure a profile, attach authorized connections, select it in Chat, converse, run work, stop, inspect the result and change profiles for future turns. Narrow/short screens, both themes, keyboard/RTL/IME and all nine locales work coherently.

### Boundaries

This change owns conversational coordination and profile configuration, not a replacement native driver, speech engine, downloader, credential system or universal automation framework. `native-document-workflows` retains document-provider implementation and native platform qualification; its unfinished tasks are not completed by this change. Existing access, prohibited operations, stop authority and actor policy remain enforced. Existing read-only delegation is extended, not replaced or broadened to concurrent desktop controllers. No new hosted inference, model downloads, arbitrary command-entry UI or automatic package installation.

Profiles reference only connections the actor is entitled to use. Existing administrator-configured workspace connections must be labelled as such; attaching one does not establish new personal OAuth isolation or authorize connection administration. Connection lifecycle/secret-store gaps remain separate requirements.

## Capabilities

### New Capabilities

- `agent-profiles`: Actor-owned immutable profile revisions, tool/connection attachments, instruction composition and bounded delegation settings.
- `conversation-job-coordination`: Crash-safe conversation/turn/job linkage, history, steering and deletion semantics through the existing runner.
- `intent-aware-capability-planning`: Answer/clarify/act decisions, task-relevant capability discovery, full-catalog lookup and bounded context.
- `effects-aware-task-recovery`: Trusted effects classification, safe correction/observation, truthful completion and exceptional manual reconciliation.
- `conversational-agent-experience`: Chat work presentation, profile management, compact activity, accessible controls and client parity.

### Modified Capabilities

None. `openspec list --specs --json` reports no published main specifications. This change references existing in-flight contracts rather than recreating native/speech requirements.

## Impact

Existing `internal/agents`, `internal/server`, conversation persistence under `internal/sessions`, capability broker/registry, OpenAPI/SDK types, current React Chat/Agents surfaces, CLI/Python clients and regression fixtures. Add an actor-scoped profile store and additive job-origin/turn linkage; do not introduce another execution authority. Review persistence migrations and old-client adapters explicitly. No new production dependency is assumed.

Record test/latency/model/platform evidence in `docs/advanced/product-reliability-plan.md`; `docs/advanced/production-readiness.md` continues to govern release claims. Planning does not deploy, reinstall, commit, push or publish. A later authorized deployment must back up and replace the existing service, retain exactly one running OffGrid service and deliver matching desktop assets when host changes require them.
