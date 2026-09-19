# OffGrid LLM v0.4.10

This patch fixes chat model loading failures caused by occupied inference ports
and improves Windows native-runtime lifecycle management.

## Fixes

- Select and reserve an available loopback port for each model instead of relying
  on fixed ports. An older runtime or another workspace no longer blocks loading
  merely by occupying port 42382. Unrelated listeners are never terminated.
- Use Windows-native process liveness checks so healthy model processes can be
  reused instead of being unnecessarily restarted.
- Attach Windows chat and embedding runtimes to a service-owned Job Object so
  they terminate when their owning service exits, including a forced exit.
- Release failed-start tracking and wait for each unloading process only once.
- Add native regression tests for occupied ports, single/multiple model caches,
  runtime reuse, failed startup and Windows forced-exit cleanup.

## Upgrading

Quit OffGrid before installing the matching desktop update. Your models and
history do not need to be deleted or downloaded again. The desktop still checks
service version compatibility; update an externally managed service separately
or explicitly choose the separate desktop workspace.

This update does not terminate orphaned processes left by older versions. If
those remain, restart Windows after saving your work, or have an administrator
verify and stop only the orphaned processes. New model loads choose free ports.

Back up your workspace before upgrading. If upgrading from before v0.4.9, read
its [knowledge-index recovery notice](release-notes-v0.4.9.md).

## Verification and limits

Windows Go tests and Linux inference/server race tests passed locally. Real
TinyLlama and Morena models returned streamed responses through an isolated
patched Windows service while another inference listener remained active.
Repeated requests reused runtime processes; forced service termination removed
its children without stopping the other workspace. This verifies loading and
stream transport, not answer quality, GPU performance or all hardware profiles.

The existing CLI, desktop and container editions are retained. CPU container:
`takuphilchan/offgrid-llm:0.4.10` (AMD64/ARM64). NVIDIA container:
`takuphilchan/offgrid-llm:0.4.10-gpu` (AMD64). Verify downloaded packages against
`checksums-v0.4.10.sha256`. Signing/notarization limitations remain unchanged.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.9...v0.4.10)
