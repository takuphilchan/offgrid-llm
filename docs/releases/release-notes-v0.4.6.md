# OffGrid LLM v0.4.6

This release makes conversations, model acquisition, knowledge setup, and the
Windows installation path recoverable. It preserves existing workspace and model
storage formats and supersedes v0.4.5 without changing that published release.

## Durable conversations and clearer activity

- Streaming chat turns are admitted and persisted before inference. Leaving the
  page or losing the stream no longer cancels model work; returning reconnects to
  the same turn without submitting it twice.
- Completed turns are committed atomically. Failed, cancelled, or interrupted
  partial output remains visible for review but does not enter completed context.
- Stop targets the exact turn ID, so a delayed request cannot cancel newer work.
- Activity ignores late responses from a previously selected run. Onboarding
  retains keyboard focus, and bounded request failures expose a recovery action.

## Model discovery, downloads, and knowledge

- The web, desktop, and CLI clients can search public GGUF repositories and choose
  an exact file/quantization instead of being limited to the curated catalog.
- Model listing and downloads now use the authoritative OffGrid service. CLI JSON
  remains machine-readable; invalid usage exits 2, operational failures exit 1,
  and user cancellation exits 130.
- Download identity, progress, partial bytes, and knowledge-setup intent survive
  service restart. Resume cannot reuse partial bytes from another source, and
  completion is published only after registry discovery and optional knowledge
  activation succeed.
- Knowledge setup continues after navigation. Documents remain manageable while
  retrieval is disabled, retained extracted text can be inspected, and deletion
  requires confirmation. Model deletion is blocked while runtime or knowledge use
  makes removal unsafe.

## Windows desktop installation

- Setup uses high-resolution monochrome artwork, system-DPI-aware native controls,
  populated installation details, and actionable failure guidance.
- Finish closes before application startup, avoiding a frozen or Not Responding
  wizard. Launching is deferred until the native installer UI has closed.
- Reinstall asks the running app to shut down normally and never force-kills it.
  Silent reinstall exits with code 2 when the app is active.

## Packages, containers, and trust

The release workflow builds Windows x64, macOS Intel and Apple Silicon, Linux x64
desktop, Linux x64/ARM64 CLI bundles, AMD64/ARM64 CPU containers, and the NVIDIA
AMD64 container. CPU images use `takuphilchan/offgrid-llm:0.4.6`; NVIDIA uses
`takuphilchan/offgrid-llm:0.4.6-gpu`.

Verify downloads with `checksums-v0.4.6.sha256`. Desktop packages remain unsigned
and not notarized until verified signing identities are configured. Windows
SmartScreen and macOS Gatekeeper warnings therefore remain expected; do not
disable operating-system protections.

## Verification and remaining limits

Go tests pass on Windows and under the WSL race detector. The production renderer
passes contract/type checks; 49 real-service browser tests pass on both Windows
and Linux; all 18 Electron lifecycle tests pass. These checks do not replace
native macOS execution, signing/notarization, representative-hardware benchmarks,
the planned SQLite workspace migration, soak testing, security review, or a user
pilot.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.5...v0.4.6)
