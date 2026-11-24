# Primary-source implementation patterns — 2026-11-20

Scope: a focused comparison of upstream implementation contracts and operating
patterns relevant to the approved OffGrid program. Sources were read on
2026-11-20. These are upstream statements, not independently run benchmarks or
a single universal industry standard. The concrete OffGrid implications below
are engineering inferences and remain subject to tests in the
[requirement tracker](../TRANSFORMATION_TRACKER.md).

| Reference and observed behavior | OffGrid implication and acceptance |
|---|---|
| [Playwright MCP](https://github.com/microsoft/playwright-mcp) documents isolated/persistent contexts and an extension for existing Chrome/Edge tabs, including session tab groups. It explicitly says origin lists and the MCP server are not security boundaries. | M2 needs both managed and explicitly attached browsing, clear ownership/visibility and local takeover. Test redirect/private-network/subresource enforcement independently of model-facing scope filters; do not copy an allowlist and call it a sandbox. |
| [Browser Use](https://github.com/browser-use/browser-use) distinguishes cloud, fresh local browser and existing-browser modes; it offers typed results, uploads/downloads, follow-ups and action visibility. Its README documents limitations of domain/secret guards and local debugging ports. | M2 acceptance must cover usable complete tasks, files, follow-ups and visible control. Preserve OffGrid's explicit local default and authenticated selected-tab bridge; do not inherit cloud defaults or assume redacted text removes screenshot/JS/transport exposure. A model-based final judge is useful feedback, not independent success proof. |
| [LangGraph persistence](https://docs.langchain.com/oss/python/langgraph/persistence) separates per-thread checkpoints from cross-thread stores. [Fault tolerance](https://docs.langchain.com/oss/python/langgraph/fault-tolerance) explains that retries re-execute nodes and external side effects need durable idempotency or reconciliation; missing acknowledgement is unknown. | M1/M3 should reuse OffGrid's existing durable runner and distinct memory scopes. Test persisted intent before dispatch, lost acknowledgements, restart, human interruption and no uncertain replay. Adding a checkpoint library does not provide exactly-once external effects. |
| [OpenHands SDK](https://github.com/OpenHands/software-agent-sdk) places agents/tools/conversations/workspaces/events in a canonical SDK/Agent Server, generates typed API access and separates automation dispatch from conversation execution. It supports local or ephemeral workspaces. | M3/M6 should keep one service-owned executor and generated client contracts while making bounded worker workspaces independently admitted. Test manifests, ownership, sealed changes, cancellation and integration against actual files; do not move authority into UI adapters. |
| [Ollama API](https://docs.ollama.com/api/introduction) exposes local and cloud base URLs and several compatibility surfaces. It also allows cloud models through a local server. | M5 must distinguish transport location from inference location. Require declared provider/model provenance and remote authorization, truthful capability discovery and pinned contract tests. A loopback endpoint alone is insufficient for an offline/privacy claim. |
| [Nativ](https://github.com/Neywa-Labs/nativ) documents an Apple Silicon/macOS 26+ application around NativServerKit and MLX-VLM, including model controls, artifacts, voice/extensions, scheduled tasks and OpenAI/Anthropic routes. Network access is still needed for initial model/build dependencies. | M5–M7 must preserve local image generation, developer compatibility, cohesive controls and Apple Silicon qualification as real deliverables. Compare complete workflows on matched hardware/model and recommended setups. Do not use a README, stars or different hardware to award a quality/performance win. |
| [MCP security guidance](https://modelcontextprotocol.io/docs/2025-11-25/tutorials/security/security_best_practices) covers confused deputies, token passthrough, SSRF, session hijacking, local server compromise and scope minimization. | M0/M1/M6 threat review must trace client/actor/token/resource binding, consent, redirect/URL validation and revocation. Add malicious-server and cross-user tests at OffGrid's actual dispatch boundaries; protocol compliance alone does not prove safe execution. |

## Resulting comparison protocol

Freeze a versioned test set before comparing products. Record OS, hardware,
memory, accelerator, application build, runtime, model/quantization/context,
permissions, network policy and cold/warm state. Measure both a matched setup
and each product's recommended setup; keep those result groups separate.

Run installation/first answer, model discovery/load/unload, chat/continuity,
browser research/forms/files/takeover, coding read/edit/test, grounded documents,
voice, image generation, interruption/recovery and offline first use. Record
completion against an independent task oracle, wrong/unauthorized actions,
false completion, approval interruptions, recovery success, user effort,
latency and peak resource usage. Preserve failures and unsupported capabilities.

The planned >=30 browser and >=30 agent cases and repeated model/profile runs
are still required. Physical Mac access, supported installed builds and human
usability review were not available as evidence in this research pass. No
comparative superiority, real-world safety or performance conclusion is drawn.
