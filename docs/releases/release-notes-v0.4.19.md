# OffGrid LLM v0.4.19

This release reorganizes the repository and strengthens the local execution
foundation while preserving the existing GGUF/llama.cpp path. The browser,
desktop, CLI and API remain one service-backed workspace; saved data and model
volumes are not migrated by this release.

## Changes

- Applications, the Python SDK and worker, developer scripts, release tooling,
  and maintained documentation now have explicit homes. Existing public API,
  CLI and installer identities remain compatible.
- The service owns a provider-aware inference scheduler, supervised local worker
  contract, measured capability records and bounded model-routing policy.
  Existing GGUF models continue to use llama.cpp. MLX and Transformers text
  providers remain explicitly gated pending real model and hardware checks;
  their metadata is not a claim that their runtimes are bundled or qualified.
- Agent work has stronger typed action evidence, bounded recovery, collaboration
  budgets and prevention of blind repeats after uncertain effects. Browser and
  native-document workflows still report what was actually verified.
- A service-startup regression in which the GGUF transport appeared loaded
  before scheduler ownership was fixed. The packaged desktop and browser test
  fixtures now cover the composed service and current Models response shape.
- An availability test now waits for its simulated storage failure before
  clearing it, avoiding an intermittent CI timeout.
- Hermes setup chooses an installed chat model using its admitted context, and
  readiness checks the model and context actually saved in Hermes. A configured
  65K value no longer masks a smaller OffGrid allocation.
- OpenClaw setup preserves existing OffGrid model entries when adding another
  model. Its provider discovers installed chat models and reported contexts
  from the running service instead of relying on a baked-in model name or
  invented context size. The newly selected model becomes OffGrid's preferred
  OpenClaw model without changing OpenClaw's global default.

## Verification and limits

The Go suite, affected race tests, API/type/build checks, 363 isolated browser
tests (10 environment-gated skips), 71 desktop tests, 73 companion tests,
74 Python client/worker tests, packaged Windows startup/browser/presentation
fixtures, release-script fixtures and maintained-document checks passed during
release preparation. These checks do not establish universal native-app success.

For the integration correction, the Go suite, API/type/web build, 71 desktop
tests, documentation and OpenSpec checks passed locally. A CPU-only container
loaded Meta-Llama at a 65,536-token slot with q4_0 KV cache and generated a
reply. OpenClaw's provider smoke test returned a model response, and both
integration readiness checks passed. A full Hermes agent reply and sustained
OpenClaw agent workflow were not established by these smoke tests; concurrent
CPU-model tests caused a canceled load, so test full workflows one at a time.

The full native workflow matrix, installed macOS/Linux permission behavior,
Apple Silicon MLX and local PyTorch model qualification remain unverified. On
the preparation host, NVIDIA container passthrough stalled in the host runtime
hook; a CPU-mode smoke test of the dual-runtime image does not qualify GPU
inference. Windows packages built locally are unsigned. Consult the
[quality evidence](../quality/README.md) and
[production-readiness gates](../quality/production-readiness.md) before enabling
preview capabilities or making platform-specific claims.

## Upgrade

Stop active work and back up the workspace before replacing a container or
desktop installation. Preserve the existing data and model volumes; install a
desktop package matching the service version. Do not resume an uncertain task
through refresh or automatic retry.

CPU image: `takuphilchan/offgrid-llm:0.4.19` (AMD64/ARM64).
NVIDIA image: `takuphilchan/offgrid-llm:0.4.19-gpu` (AMD64), subject to host
NVIDIA runtime compatibility. Verify published assets with
`checksums-v0.4.19.sha256` before installation. The image and checksum names
are release targets, not proof of publication until the release workflow
finishes and verifies them.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.18...v0.4.19)
