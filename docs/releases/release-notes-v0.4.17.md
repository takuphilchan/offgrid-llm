# OffGrid LLM v0.4.17

This release moves reusable agent configuration into Agents and agent work into
Chat, with durable task links, clearer recovery, consistent search controls and
CPU/GPU runtime selection fixes. Computer use remains experimental.

## Agents and conversations

- Create, revise, duplicate and archive actor-owned agent profiles. Configure
  instructions, an installed model, Knowledge, authorized tools and optional
  read-only specialists. Each turn pins its profile revision.
- Select an agent in Chat to converse or run work. Enabling tools or remembered
  desktop access does not require a tool call for an ordinary greeting.
- Chat and the detailed work inspector share one durable job. Persisted request
  identities, held admission and idempotent result attachment prevent navigation,
  reconnect or a lost reply from silently launching duplicate work.
- Send corrections at a safe execution boundary. Child specialists can research
  and draft within the parent's permitted read-only tools and budget; only the
  parent performs authorized edits. This is not parallel desktop control.
- Existing task links remain inspectable without restoring the old task composer.
  CLI and Python clients use the same profile and conversation service contracts.

## Computer workflows and recovery

- Task-wide desktop access supports catalog-backed application discovery and
  launching, with requested-application matching and bounded observations.
- Explicitly enabled Full access can be remembered for the current desktop
  session; it does not bypass prohibited operations or operating-system consent.
- Native document operations distinguish creation, selection, insertion,
  replacement and independent read-back. Unsupported provider operations remain
  unavailable rather than being replaced with a service-side file.
- Proven pre-effect failures receive bounded correction. Lost acknowledgments
  use trusted receipts when available; uncertain mutations are never retried
  automatically, and a human report is not treated as independent verification.

## Interface and runtime

- Unified compact searches across conversation/task history, agent profiles,
  Knowledge, installed models and model discovery, with accessible labels.
- Contained Chat scrolling follows growing output without stealing a deliberate
  scroll-up. Technical activity stays secondary; narrow-screen action buttons
  retain localized accessible names.
- Sequential tool calls use a bounded private response envelope while ordinary
  Chat keeps its existing streaming path. Token accounting includes that envelope.
- A cached CPU runtime no longer shadows an available configured GPU runtime.
  CUDA images also include a separate CPU-only worker. CPU selection does not
  require NVIDIA initialization or replacing installed models.
- Portable fallback, container and bundled-runtime source pins align at llama.cpp
  `b95502ba9aa0eb73a2f4fc8878d7fbe6a847a0b9` (`b10516`). Platform builds are
  independently gated; alignment alone is not hardware qualification.

## Verification and limits

Local checks include the Go suite, the recorded race checks, API/type checks,
348 browser tests (10 opt-in skips), 65 desktop tests and 73 companion tests.
Seven model-acquisition/restart cases passed separately against disposable
services and synthetic packages. These are not speech-quality or native-app
completion evidence. See the [reliability plan](../advanced/product-reliability-plan.md).

Repeated installed-model workflow evaluation, native document acceptance across
Windows/macOS/Linux, manual assistive-technology checks, usability studies and
broader signing/soak/pilot gates remain incomplete. This release does not claim
universal application control or completion of the Voice/Computer programs.
Shared administrator connections do not become personal OAuth credentials.
No model is silently downloaded or replaced by profile selection.

Publication requires passing CI for the exact source revision, all required
release assets and checksums, and CPU/GPU container verification. Publisher
signing and macOS notarization are not claimed; follow the
[desktop trust guidance](../setup/desktop-startup.md#installer-appearance-and-os-warnings).

## Upgrade

Stop active work and back up workspace data before upgrading. Replace the
existing service while keeping its data/model volumes, then install the matching
desktop package. A container rebuild does not update the desktop companion.
Profile and conversation-link storage is additive; existing history is retained.
Do not restore an older backup over newer work or roll back the companion's
duplicate-action journal. Previously uncertain tasks are not automatically resumed.

CPU image: `takuphilchan/offgrid-llm:0.4.17` (AMD64/ARM64).
NVIDIA image: `takuphilchan/offgrid-llm:0.4.17-gpu` (AMD64).
Verify release assets with `checksums-v0.4.17.sha256`.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.16...v0.4.17)
