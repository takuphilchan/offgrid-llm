# OffGrid LLM v0.4.20

This release improves Hermes and OpenClaw integration, local inference startup,
native tool-call sequencing and explicit file-outcome verification.

## Changes

- A shared integration manager owns inspection, configuration, per-run model
  selection, preparation, cancellation and recovery. Existing command aliases
  remain available. Model catalogs and context values come from the local service.
- Model context is independent of battery percentage. Memory admission remains
  enforced, and a compatible resident allocation can serve smaller requests.
- Streaming starts before slow model loading/prefill finishes and reports bounded
  waiting progress. Cancellation stops owned inference and external processes.
- External agents retain native tool-call contracts. Complete calls are validated
  before dispatch; the bundled llama.cpp parser enforces sequential cardinality.
- Experimental `--verify-contract` runs independently check exact copies or an
  unchanged workspace. Supported hooks preserve genuine source bytes and reject
  incorrect proposals, incomplete evidence and false completion claims.
- Hermes distinguishes display line numbers from source content in contracted
  workflows. OpenClaw contracted runs initialize lifecycle hooks through its
  supported local launch with private session state and read-only configuration
  composition. Ordinary runs retain their existing launch behavior.
- Integration UI readiness distinguishes service compatibility, host inspection
  and workflow evidence across the supported locales.

## Verification and limitations

Real contracted greeting and Unicode exact-copy tests passed with Hermes
0.21.2/f364c197 using Meta-Llama 3.1 8B, and OpenClaw 2026.10.4/383c124 using
Qwen2.5 7B. Independent byte comparison, read-back and workspace preservation
checks passed after the local container/CLI update. Automated tests additionally
cover rejected content, genuine numeric prefixes, truncation, missing hooks,
cancellation and fabricated completion.

These narrow results do not qualify every model or arbitrary uncontracted agent
work. File contracts require the reviewed hook revisions and explicit plugin
enablement/permissions; they are not an OS sandbox. The optional template adapters
remain opt-in. The measured Meta-Llama case used the explicit
`OFFGRID_TOOL_TEMPLATE_ADAPTER=llama31-optional-tools-v1` setting; its result must
not be attributed to an unconfigured native template or another runtime revision.
OpenClaw may emit upstream SQLite cleanup/provenance warnings; Hermes may report
its optional security scanner unavailable.

The broad native workflow matrix, real MLX/PyTorch hardware qualification and
installed macOS/Linux permission behavior remain incomplete. See the
[integration guide](../guides/external-agents.md) and
[dated evidence](../quality/evidence/external-integrations-2026-10-22.md).

## Upgrade

Back up workspace data and stop active work before replacing service/desktop
packages. Preserve existing model and data volumes. Install matching first-party
provider bundles through the managed commands. Review the guide before explicitly
enabling file-contract hooks; no model download, cloud fallback or additional
desktop authority is granted by this release. Inspect uncertain saved work before
retrying it.
