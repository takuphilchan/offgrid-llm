# OffGrid LLM v0.4.8

This release supersedes v0.4.7 with a corrected high-DPI Windows installation
progress layout. It also includes the durable chat, model discovery, knowledge
recovery, and installer-repair improvements introduced in the preceding patch.

## Windows installer presentation

- The current-file status area now uses the page background instead of the
  progress-track background. At high DPI, the native installer therefore shows
  one clear monochrome progress bar rather than two visually stacked tracks.
- The installer retains native keyboard behavior, installation details, bounded
  responsiveness checks, graceful running-app repair, and data preservation.
- The native desktop window now appears as soon as Electron is ready, while the
  bundled service continues starting visibly in the background. Clicking
  Finish no longer leaves an unexplained blank wait before the first window.

## Product improvements included

- Streaming chat turns survive navigation and reconnect without duplicate work.
- Web, desktop, and CLI model workflows support public GGUF search, exact-file
  selection, resumable downloads, and explicit recovery states.
- Knowledge setup and document management remain available across navigation and
  service restart, with honest disabled/error states.

## Packages, containers, and trust

The release workflow builds Windows x64, macOS Intel and Apple Silicon, Linux x64
desktop, Linux x64/ARM64 CLI bundles, AMD64/ARM64 CPU containers, and the NVIDIA
AMD64 container. CPU images use `takuphilchan/offgrid-llm:0.4.8`; NVIDIA uses
`takuphilchan/offgrid-llm:0.4.8-gpu`.

Verify downloads with `checksums-v0.4.8.sha256`. Desktop packages remain unsigned
and not notarized until verified signing identities are configured, so Windows
SmartScreen and macOS Gatekeeper warnings remain expected.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.5...v0.4.8)
