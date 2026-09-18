# OffGrid LLM v0.4.7

This release replaces the unqualified v0.4.6 tag. It contains the durable chat,
model discovery, knowledge recovery, and installer improvements prepared for
v0.4.6, plus the Windows qualification fixes required before publication.

## Durable conversations and model workflows

- Streaming chat turns survive navigation and reconnect without duplicate
  submission. Completed turns are committed atomically; interrupted output is
  labelled and kept out of completed conversational context.
- The web, desktop, and CLI clients can search public GGUF repositories, choose
  exact files, and resume service-owned downloads safely.
- Knowledge setup, document management, and model download state remain visible
  across navigation and service restart, with explicit recovery actions.

## Windows installation and repair

- Setup defers application launch until its native Finish window has closed,
  asks a running desktop to quit normally during repair, and preserves workspace
  and model data across reinstall and uninstall.
- The isolated installed-app qualification now passes its profile to the
  graceful-quit helper, waits for the NSIS Finish page to settle, and treats only
  a sustained five-second message-loop failure as an unresponsive wizard.
- Release diagnostics identify the exact failed installer stage instead of
  reporting only a generic process exit code.

## Packages, containers, and trust

The release workflow builds Windows x64, macOS Intel and Apple Silicon, Linux x64
desktop, Linux x64/ARM64 CLI bundles, AMD64/ARM64 CPU containers, and the NVIDIA
AMD64 container. CPU images use `takuphilchan/offgrid-llm:0.4.7`; NVIDIA uses
`takuphilchan/offgrid-llm:0.4.7-gpu`.

Verify downloads with `checksums-v0.4.7.sha256`. Desktop packages remain unsigned
and not notarized until verified signing identities are configured. Windows
SmartScreen and macOS Gatekeeper warnings therefore remain expected; do not
disable operating-system protections.

## Verification and remaining limits

The Windows package is exercised through clean installation, installed desktop
startup, Finish launch, active-app repair, silent refusal, interactive reinstall,
workspace preservation, and uninstall. Cross-platform Go, web, Electron, browser,
macOS, archive, and container gates remain required in the unified release.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.5...v0.4.7)
