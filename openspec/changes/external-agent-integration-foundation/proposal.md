# Proposal

## Why

Hermes and OpenClaw duplicate setup, selection, readiness and process-launch logic. Dynamic context changes and partially configured external runtimes consequently produce contradictory readiness and reinstall advice. A shared lifecycle is needed before adding more integrations or qualifying an external-agent demo.

## What Changes

- Use the supported hook-initialized OpenClaw `agent --local` launch specifically for explicit verification contracts. Give each invocation private session/config-overlay state and its declared workspace, preserve the selected model and configured tool/permission policy, and keep ordinary `agent exec` runs unchanged. Verify actual installed CLI lifecycle delivery, not only hook-dispatcher fixtures.
- Add shared host-side outcome verification through supported external-agent hooks. Use explicit expected outcomes, scoped independent file reads and correlated tool events; never infer an exact-copy requirement from arbitrary prose, rewrite tool results or turn the inference service into an executor. Missing evidence or contradictory files must not become success merely because the external process exits zero. Preserve ordinary runs; the approved per-run `--verify-contract <file.json>` interface opts into bounded exact-copy or unchanged-workspace checks without installing or enabling plugins silently.
- Extend the opt-in pinned template adapter with versioned tool-result interpretation guidance: distinguish documented display decoration from actual content, preserve Unicode and genuine numbered text, and distinguish a write receipt from independent outcome verification. Keep original messages/results and unknown templates untouched. Add frozen synthetic and isolated real tests for truncation, mismatches, abstention and false completion; prompt guidance alone is not an enforcement or qualification claim.
- Introduce a shared host integration manager with typed adapters, one authenticated service client, explicit model precedence, idempotent configuration and inspectable recovery.
- Preserve existing CLI aliases and provider-only setup while adding generic managed status, configure, test and run commands.
- Separate service compatibility, host runtime/plugin/configuration checks, smoke evidence and workflow qualification. Expose additive API fields and generic localized UI presentation.
- Preserve admissible context independent of battery and transient pressure, retain resource admission, and compare configured context as a safe bound instead of demanding exact equality with changing capacity. Reuse compatible resident allocations and confirm runtime capacity.
- Repair interactive terminal ownership and truthful cancellation, and forward validated structured response formats on supported inference backends.
- Start accepted streaming requests before slow local loading/prefill completes, expose truthful bounded progress, classify cancellation/timeouts as operational outcomes, and preserve instructions and tool access when reducing request-local overhead.
- Separate external native tool contracts from the explicitly selected internal single-step protocol. Enforce complete, schema-valid decisions without hardcoded model identities; preserve ordinary answer streaming and reject incomplete executable calls.
- Add a versioned compatibility adapter for positively identified tool templates whose instruction framing forces tool behavior for ordinary conversation. Match verified runtime/template metadata, not display names; preserve model, user instructions, tools and native call/result semantics. Unknown templates retain the existing path. Test tool abstention, sequential results and independent file read-back before claiming workflow qualification.
- Repair the pinned bundled llama.cpp native parser's per-call repetition so explicit sequential requests cannot generate several executable calls. Apply a reviewed, revision-bound build patch with parser regression tests, not a modification of installed upstream source. First-party provider hooks default to sequential calls while preserving explicit caller controls and the complete native tool catalog. Verify greetings, dependent tool/result turns and immutable exact-file outcomes before committing; retain failures separately.
- Prevent short-lived auxiliary requests from repeatedly cancelling shared model preparation needed by live requests; preserve bounded cancellation, exclusive loading and resource admission. Remove implicitly capability-reducing launch flags and measure cold/warm latency and real workflows for both integrations across eligible installed models.
- Preserve installed models, external config entries, secrets and external defaults during per-run model selection. Trusted bundled plugins remain explicitly installed; external agents retain their own tool authority.
- Verify configuration preservation, authentication, model changes, failure recovery, an additional fixture adapter, and isolated real-agent workflows where the installed environment permits them.

## Capabilities

### New Capabilities

- `external-agent-integrations`: shared lifecycle, compatibility, model discovery, context bounds, trusted configuration and honest readiness for external inference clients.

### Modified Capabilities

None. This change consumes the context/runtime projections of `model-adaptive-execution`; it does not redesign the inference providers or governed desktop runner.

## Impact

`internal/integrations`, `internal/cli`, `cmd/offgrid`, integration HTTP projections, power policy, generated API types, Agents integration presentation and maintained external-agent documentation. No deployment, desktop replacement, Git publication or model downloads are part of implementation. Runtime installation still requires an explicit install command and consent. Real external-agent tool tests use isolated homes and disposable files; unavailable platform/model qualification remains recorded as unverified.
