# Design

## Context

See proposal.md and the preceding audit. The root installer bypasses archive verification and stops name-matched processes. Desktop's afterPack checks are stronger than archive installation checks. CLI speech/UI layouts differ from discovery defaults; first inference can provision an engine implicitly. Existing onboarding already tracks first replies and must be extended rather than replaced. The inference-provider-runtimes change owns provider adapters; this change owns distribution preparation and first-use visibility only.

## Goals / Non-Goals

Deliver the requested safe installation and first-response workflow on this Windows machine, with reproducible tests and truthful release/platform status. Preserve workspace roots, external-service ownership, auth and IPC. The subsequent user instruction authorizes commits and publication through the maintained release gates. No credential invention, security bypass or claim of unperformed OS qualification.

## Decisions

- Verify a uniquely named archive against its release manifest before extraction. Keep root and streamed entry points self-contained, fail closed, and refuse replacement while active processes exist. Never infer permission to terminate an unrelated service.
- Retain complete app-local runtime layouts and extend discovery using executable-relative paths, not the current directory. Preserve explicitly configured paths and legacy workspace runtime resolution.
- Reuse the pinned inference runtime manager behind a preparation/readiness API with bounded downloads, state/progress and cancellation. Reuse existing onboarding first-reply tracking. CPU/GPU suitability and runtime availability remain distinct.
- Release trust verification checks actual packaged files; unsigned preview builds are explicitly labelled and release trust checks must not pass without real evidence. Signing credentials are an external prerequisite.
- The user's later release-without-credentials instruction authorizes a manual
  unsigned-preview channel. Native trust inspection still runs and records
  failure to establish publisher trust; it is not converted into a passing
  signed-release check. Publish as a prerelease without stable alias promotion,
  retain source/channel markers, reject cross-channel asset reuse, and require
  all normal source, payload and container checks. Future signed publication
  remains separate qualification with a new release version.
- On 2026-11-12, the user explicitly requested that the published 0.4.30 become
  GitHub's Latest release. This authorizes clearing its GitHub prerelease flag
  and selecting it as latest, while retaining the unsigned-preview title,
  source/channel identity and actual trust reports. It does not authorize
  changing Docker aliases or weakening future signed-release checks.
- Build a distinguishable preview from the current workspace after project checks, test installation with disposable identity/data, then upgrade the identified user installation. Back up application metadata/config and verify workspace preservation; do not move or erase model stores.

## Risks / Trade-offs

- Live unrelated UI edits: inspect diffs before touching overlapping files; use focused patches and test the actual final build.
- Installation interrupts the owned desktop: inspect current processes, finish or safely close the app, preserve independent services and verify startup afterward.
- No cross-platform host or signing identity: record incomplete qualification explicitly and continue all independent implementation work.
- Large models/downloads: use an existing suitable model for local response verification and expose download costs before optional provisioning.

## Migration Plan

Keep application identity and data paths. Preserve prior installer and build metadata for rollback; do not call binary replacement a data-schema rollback. New distribution identity must be visible alongside revision. Update manual upgrade guidance and maintain an evidence record with unrun gates.
