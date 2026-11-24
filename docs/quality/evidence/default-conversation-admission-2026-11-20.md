# Default conversation admission and request parity

Date: 2026-11-20. Scope: the uncommitted shared worktree in `D:\offgrid-llm`,
continuing `agent-conversation-workflows` 10.1–10.3. These tasks remain open.
The roadmap and master tracker retain the broader transformation scope.

## Implemented contract

| Requirement | Implementation | Acceptance evidence |
|---|---|---|
| Default assistant entry without a required profile | `internal/config/config.go` enables canonical admission for omitted configuration; existing Chat/CLI capability routing uses the same service inbox and runner | `TestConversationAdmissionDefaultAndExplicitDisable`; `TestConversationInputPublicServiceJourneyWithoutProfile` exercises actual HTTP/service coordination, direct answer and ongoing work with injected inference |
| Explicit disable preserves accepted work | Input-format capability remains distinct from new-admission capability in `internal/server/system_identity.go`; matching Chat/CLI do not route new default text requests through legacy generation when admission is disabled | `TestDefaultConversationOptionsPinBeforeQualificationAndSurviveDisable`; `TestConversationCLIDisabledDefaultAdmissionCannotFallBack`; renderer disabled-admission test |
| Response settings reach the qualified execution context | `pkg/api/conversation_inbox.go`, generated TypeScript schema, `internal/server/conversation_preferences.go`, preparation paths and `canonical-input.ts` retain `response_profile` and `max_tokens` in the durable input; context selection precedes model policy/qualification | Server test checks every qualification lookup against interactive/extended/omitted context and verifies the actual prepared configuration; CLI/renderer/Python tests inspect transported fields |
| Exact explicit retry | Existing input digest includes nonzero response options; the browser retains the full envelope and compares options before configuration writes | Changed options under the same request ID conflict; accepted retries work after disable; renderer reload/lost-receipt test confirms exactly one accepted input and identical envelopes |
| Source-only ordinary messages | `conversation_inbox.go` allows empty ordinary captions only with images/files; `conversation_turn_admission.go` permits an empty prompt only through canonical foreground admission with resolved sources; `conversation_decision.go` prevents inferred work creation | `TestConversationInputOptionsAndSourceOnlyBoundaries`; `TestDefaultConversationSourceOnlyPreservesCaptionAndAnswer` runs file/image turns, checks source evidence and restarts persistence with the empty caption retained |
| Capability and legacy compatibility | New `conversation-input-options-v1` format capability; CLI rejects explicit unsupported response options; old canonical Chat sends its prior fields; explicit legacy generation routes are unchanged | CLI capability rejection and no-mutation tests; existing canonical/legacy server/client regressions; renderer old-service document-only rejection |
| Existing source and coordination authority | Source attachment/model checks remain in `conversation_sources.go`; ordinary empty jobs and textless work/corrections/answers remain invalid; native coordination guard in `tool_qualification.go` is unchanged | New boundary tests plus existing `conversation_sources_test.go`, `conversation_selection_test.go` and `conversation_decision_test.go` qualification regression |

## Environment and checks

Windows source checkout, Go module/runtime already present, Node 24.18.0 and
existing npm dependencies. Playwright launches isolated fixture browser contexts;
the Vite fixture mode disables fall-through to the installed service. Go tests use
temporary directories and local HTTP fixtures. The image test uses a synthetic
projector file and injected inference; it does not load or qualify a vision model.
No model weights, installed state or personal browser sessions were used.

| Command | Actual result |
|---|---|
| `go test ./internal/config ./internal/agents ./internal/server -run 'Test(DefaultConversation\|ConversationAdmissionDefault\|ConversationInputOptions)' -count=1` | Passed: config 1.712s, agents 0.750s, server 4.143s |
| `go test ./internal/server -run 'TestConversationInputPublicServiceJourney' -count=1` (same pattern in combined focused run) | Passed: server 4.133s |
| `go test ./internal/cli -run 'TestConversationCLI(ResponseOptions\|DisabledDefault)' -count=1` | Passed: 0.709s |
| `go test ./internal/server ./internal/agents ./internal/config ./internal/cli ./internal/serviceclient ./pkg/api -count=1` | Passed: server 317.247s, agents 235.354s, config 1.944s, CLI 4.326s, serviceclient 1.236s, pkg/api 1.800s |
| `npm.cmd --prefix apps/web run test:e2e -- tests/canonical-conversation.spec.ts --grep 'default canonical\|document-only\|disabled default' --workers=1` | 4 passed, 36.9s |
| `npm.cmd --prefix apps/web run test:e2e -- tests/canonical-conversation.spec.ts --workers=1` | 30 passed, 1 existing opt-in visual evidence test skipped, 4.0m |
| `npm.cmd --prefix apps/web run api:check` | Passed; regenerated contract retains the other increment's Ollama/vLLM engine values |
| `npm.cmd --prefix apps/web run check` | Passed |
| `npm.cmd --prefix apps/web run build` | Passed; existing Vite import/startup asset/large-chunk warnings remain |
| `wsl.exe -- python3 -m pytest /mnt/d/offgrid-llm/sdk/python/tests/test_conversations.py -q` | 13 passed, 2.02s |
| `openspec.cmd validate agent-conversation-workflows --strict --no-interactive` | Passed; validates specification structure only |
| `node scripts/maintenance/check-source-size.mjs` | Passed |
| `node scripts/maintenance/check-docs.mjs` | Passed: 440 maintained files at this checkpoint |

The full affected Go packages and canonical renderer suite completed with no code
changes during those runs. Full server race is coordinated separately by the root
agent; no passing race result for this increment is claimed here.

Initial new CLI fixtures omitted mandatory session timestamps and failed before
the tested operation. Those fixtures now satisfy the existing session contract.
An old-service assertion expected the capability string, while the existing client
returns its generic upgrade-required message; it now checks rejection and zero
submissions. The renderer assertion initially expected absent `profile_id`, while
configuration writes explicitly save an empty profile; it now checks that no
profile is selected. These fixes changed test assumptions, not application guards.
Windows `python` resolves to an unavailable Store alias; the SDK test was actually
executed with the already available WSL Python runtime.

## Unfinished requirements and qualification

This increment does not complete 10.1: foreground discovery currently intersects
with executable capabilities, which canonical foreground admission clears.
Metadata-only progressive discovery needs a separate, scope-filtered contract that
cannot invoke tools or acquire host authority. Native `conversation_decide`
qualification remains enforced, so enabling default admission does not claim every
installed local model can coordinate work.

Follow-up: the separate [foreground metadata increment](foreground-tool-metadata-2026-11-20.md)
now implements that discovery boundary with focused regression evidence. Its
integration checks and limitations are recorded separately; it does not change
the earlier results or imply that skill/provider execution is available in a
foreground turn.

10.2–10.3 remain open for full interactive CLI source migration, all cross-client
steering/correction/control flows, accessibility/locales/voice and end-to-end
configuration inspection. Older canonical services retain their previous Chat
response defaults; the new settings contract requires the new capability. Source
history migration remains explicitly limited rather than dropping sources.

The tests prove service/client contracts with controlled inference, not real model
accuracy, ongoing-task usefulness, hardware scheduling, a soak, packaged platform
behavior or human usability. Full CI/race and independent review of this increment
remain integration gates; the broader transformation and model/hardware/user
qualification remain incomplete.
