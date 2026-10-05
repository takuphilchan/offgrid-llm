# Design

## Context

See proposal.md for the corrected user outcome. Package manifests, local import,
verification, leases, and chat-model exclusion already exist; network acquisition
and its normal-user workflow do not. These completed primitives remain useful,
but the import-only Models section must not be the primary speech experience.

Current integration constraints found in the implementation:

- `internal/models/discovery.go` discovers standalone GGUF files, not speech
  packages. Hugging Face search metadata alone cannot establish compatibility.
- `internal/server/download_state.go` persists single-file download identities in
  `downloads.json`. Recovery and validation currently assume model filenames.
- `ModelsPage.tsx` tracks catalog operations using `.gguf` keys and renders the
  speech importer separately. Merely adding speech cards would misroute downloads.
- `PackageStore` already validates immutable data-only manifests and atomically
  activates imports. Its lease guards must also protect download/repair activation.

The existing [architecture](../../../docs/advanced/ARCHITECTURE.md),
[model discovery guide](../../../docs/guides/model-discovery.md), and
[reliability plan](../../../docs/advanced/product-reliability-plan.md) remain the
authoritative architecture, user guidance, and evidence references respectively.

## Goals / Non-Goals

**Goals:** one model-management surface and one transfer authority for existing
single-file models and speech packages; complete, resumable, identity-bound package
acquisition; explicit readiness; preserved legacy IDs, files, and client behavior.

**Non-goals:** downloading arbitrary repository code, new hosted inference,
automatic runtime execution, or treating this change as working voice. Runtime
pack setup/adapters, actual inference, live speech/recordings, offline pack
distribution, and qualification remain required subsequent program stages.

## Decisions

### 1. One typed catalog and Models workflow

Extend the shared catalog projection with capabilities and an installation target:
legacy file or versioned package. Keep existing catalogs/IDs authoritative instead
of adding an independent Whisper/Piper list. The new v2 projection combines both;
v1 remains a language/embedding projection for clients with single-file assumptions.

The shared renderer uses Language, Embeddings, Speech recognition, and Speech
generation filters across installed models, curated discovery, and Hugging Face
search. Supported variants show language, voice, size, license, source, and runtime
requirements in the familiar model cards. Choosing a variant resolves the package;
users never choose every dependency or supply a manifest in the download flow.

Keep advanced local import available inside Models, collapsed by default. Replace
the standalone import-first block and oversized refresh control with existing
model-card, action, and progress components. Preserve monochrome styling, all nine
locales, keyboard/focus behavior, screen-reader status, RTL, and responsive layout.

**Alternative rejected:** a separate speech catalog/importer with its own progress
state recreates the current confusing workflow and duplicates lifecycle behavior.

### 2. Data-only package resolution

Extend the existing Hugging Face client and discovery service rather than adding
a second network client. Search remains an explicit user action. Apply the chosen
capability filter without assuming every compatible model is GGUF. Tags help find
repositories; only a reviewed architecture recipe can resolve an installable model.

Recipes cover Whisper/whisper.cpp, Piper, streaming Zipformer/sherpa-onnx, and
Kokoro/sherpa-onnx. Each recipe declares complete data roles and layout constraints,
including configuration, tokens, speaker/voice tables, and any required model data.
Multiple languages/voices/quantizations become explicit variants. Ambiguous or
missing required artifacts block resolution with a specific reason; no silent
best-guess selection. Adding a new architecture still requires reviewed code.

Resolve a selected repository revision to a full immutable commit before transfer.
Generate the validated package manifest in the service; bind all artifact paths,
sizes, expected digests, recipe version, and runtime requirements to that revision.
Curated entries pin reviewed digests and licenses. Advanced repository discovery
can use metadata verified against the pinned source. For small configuration or
token data, bounded resolution can fetch and check the pinned repository object
identity before calculating the manifest's SHA-256; record that provenance rather
than claiming an independently reviewed checksum. If verifiable source identity
is absent, do not offer an unverified normal download. Never silently compute a
digest after a weight transfer and present it as an independently supplied check.
Local import retains its explicitly untrusted provenance distinction.

Expose only public repositories under the current Hugging Face authentication
policy. Gated/private results explain that limitation, never prompt for a token in
task text or put credentials in a URL. Bound metadata size, pagination, file count,
individual/aggregate size, and timeouts; incomplete listings cannot produce a
complete manifest. Accept repository identities, not arbitrary source URLs or host
paths. Limit artifact redirects to reviewed HTTPS Hugging Face delivery hosts;
reject unexpected hosts, insecure redirects, credential leakage, and path escapes.
Keep existing configured proxy behavior without changing the VPN or network policy.

**Alternative rejected:** extension/tag-based compatibility or repository execution
would accept incomplete models and introduce an executable installation authority.
No model scripts, pickle loading, package installation, or remote code execution.

### 3. Shared durable operations, not another downloader

Extend the existing download coordinator, HTTP transfer/resume primitives, catalog,
and progress persistence to support typed targets. Represent package installation
as one operation with ordered artifact checkpoints and aggregate progress. Do not
issue unrelated legacy file downloads and later guess whether they form a package.

Persist actor, request ID, immutable source/manifest digest, operation kind, target,
state, artifact progress, verification, and safe recovery information before
acknowledging acceptance. Identical retries return the existing operation; changed
payloads with the same request ID conflict. Concurrent requests for the same
package coordinate on one writer rather than racing two activations.

Use states for queued, downloading, verifying, activating, complete, cancelling,
cancelled, interrupted, and failed work. Aggregate transferred bytes and verification
are distinct: 100% transfer is not completion. Persist terminal results before
reporting success. A lost acknowledgment after activation resolves against the
immutable installation receipt; it does not start a second transfer or replace data.

Generalize `downloads.json` into one versioned durable operation store owned by the
same service. Stage, back up, and validate migration from legacy records before
activation; retain originals, reject corrupt/unknown versions, and never dual-write
two authorities. Continue exposing existing progress/recovery endpoints as legacy
projections. Preserve legacy `.tmp` bytes and source identity without treating them
as speech packages or guessing an unrecorded revision.

On restart, active transfers become interrupted and offer explicit Resume. Revalidate
authorization, pinned identity, partial sizes, and verified artifacts before
continuing. No automatic switch to a newer revision. Cancel waits for owned workers
to stop writing before Resume/Discard can run. Keep partial data with visible size
and an explicit discard action; discard touches only that operation's staging.

**Alternative rejected:** a speech-only queue/history would split authorization,
recovery, resource bounds, and user-visible progress from ordinary model downloads.

### 4. Confined staging, activation, and repair

Keep package storage registry-owned under `.packages`, excluded from legacy scans.
Reuse manifest validation and confined filesystem roots; reject traversal, links,
special files, platform path aliases, unknown schema fields, and executable data.
Network operations have private persistent staging indexed by operation identity.
Existing bounded multipart/local imports remain compatible advanced entry points.

Preflight known transfer bytes, available disk, staging/activation overhead, and
retained recovery revisions. Unknown requirements are labelled, not zero estimates;
failure to establish required artifact identity or size blocks the normal download.
Continue enforcing limits during transfer even after preflight. Reuse validated
Range handling, including servers returning 200 or 416. Never append bytes from a
changed source. Verify every artifact before a flushed atomic package activation.

Keep network/hash work out of long-held global lifecycle locks. Coordinate package
writers and leases during publication/removal. An active runtime lease prevents
replacement/removal; future runtime adapters must lease before loading. Deletion
cannot race a transfer into recreating the package: settle or cancel its operation
before confirmed removal.

Repair is a staged operation for the exact installed revision and trusted receipt.
Recheck artifacts, retain valid data, replace missing/corrupt data, and verify the
whole staged package before publishing. Preserve the original until the new
installation and durable receipt commit; journal any platform-required rename
sequence so restart can deterministically retain or complete one revision. Failure
never deletes the prior package to force success. New revisions are side-by-side,
not implicit overwrites. An imported package without a verifiable network source
offers re-import, not a fabricated Repair download. Orphaned staging is reported
as recoverable/discardable, never silently counted as installed.

### 5. API and client cutover

Add typed catalog and inventory projections under `/api/v2/models` and `/catalog`,
a resolution/preflight contract, and `/api/v2/models/operations` for durable package
installation, inspection, cancellation, resume, repair, and staged-data discard.
Freeze their concrete shapes in OpenAPI before clients consume them. Return `202`
only after persistence, conflicts as `409`, and safe coded errors with retryability.
Use the current model-read/model-management authorization and actor/admin operation
ownership; discovery never grants mutation permission.

Keep `/api/v2/models/packages` import/verify/remove compatible and route overlapping
mutations through the same package coordination. Retain legacy aliases and v1
model/download projections. Speech cannot enter chat selectors, onboarding choices,
CLI language-model loading, or llama.cpp admission through either API version.

Generate TypeScript contracts; expose matching service-backed CLI and Python model
operations rather than separate direct-to-disk speech installers. Reuse existing
authentication/cancellation and JSON conventions. Shared React changes apply to web
and matching desktop builds; reconnect reads durable progress instead of restarting
downloads when a page remounts.

### 6. Acquisition is not runtime readiness

Show installed, integrity checked, runtime compatible, smoke-tested, and qualified
as independent states. Resolution explains absent/incompatible runtime components
before a user spends bandwidth; they can explicitly download supported weights for
later use with that limitation visible. Afterward show what is missing and a real
available setup/update action, or state that integration is not yet available. No
dead repair button, terminal prerequisite, or claimed speech readiness.

Runtime packs/adapters and real ASR/TTS qualification remain the next required
program stage. This change records their requirements and setup handoff, but does
not implement their execution, activate a microphone, or mark them qualified.

## Risks / Trade-offs

- Repository metadata drift -> pin revision/receipt before dispatch; do not resume
  against `main`. Version recipes and regression-test bounded upstream fixtures.
- Publisher digests, licenses, or layouts missing -> explain incompatibility rather
  than invent provenance or dependencies. Curated profiles need recorded evidence.
- Large models, hashing, and repair space -> bounded workers, cancellable hashing,
  honest preflight estimates, and retained partial data with explicit cleanup.
- Persistence/activation races or Windows sharing failures -> durable intent,
  package writer/lease coordination, bounded retries, and crash-injection tests.
- Downloaded models confused with usable voice -> separate readiness in discovery,
  operation completion, installed inventory, and all capability selectors.
- Malicious local administrator -> not an isolation guarantee; confined storage
  protects service operations, not arbitrary host compromise.

## Migration Plan

### Authorized corrective runtime increment (2026-10-05)

The user requested fixes to response playback and architecture-independent speech
routing after exercising downloaded packages, and declined further model-weight
downloads. This increment is not completion of the full Voice program.

`internal/audio/package_runtime.go` owns architecture/capability selection and
PackageStore leases. Its single active engine invokes the existing Whisper/Piper
adapters or the supervised Qwen worker. Unknown/unimplemented adapters remain
explicitly unavailable. It never guesses an adapter from a file extension and
never silently substitutes a requested package. Base/VoiceDesign cannot masquerade
as Qwen CustomVoice. Status inspection is independent of worker lifetime.

The worker isolates protocol output from library diagnostics, bounds responses,
supports cancellation, uses private output files, and closes before architecture
switching. Read-aloud uses bounded text chunks, one look-ahead request, immediate
preparation/stop feedback, and cancellation on navigation. The page and service
CSP both allow owned blob audio without enabling remote media or scripts.

Acceptance: fixtures exercise Whisper/Piper paths, unsupported architectures,
noisy Python imports, cancellation, leases, visible failures, chunk bounds,
navigation cleanup, and real-browser CSP enforcement. No new weights or voice
cloning are authorized. Real direct-voice quality/latency and sherpa runtime
implementation remain unverified/incomplete.

The user subsequently authorized necessary compatible-model downloads and speech
during response generation. Use the shared acquisition service, preserving existing
packages. A session-only Speak responses control opts into speech for new turns;
restored turns never replay automatically. Incremental playback consumes complete
prose sentences (or bounded clauses), excludes reasoning/code, and flushes the last
fragment only after the turn commits. It has one synthesis in flight and a bounded
text backlog; overload stops speech visibly without blocking text generation.
Generation failure, cancellation, selection changes, navigation and manual Stop
discard pending audio immediately. Provisional speech is labelled, not announced
as verified task completion. This is chunked response speech, not native audio
token streaming or complete Talk mode.

### Acquisition migration

1. Preserve current package manifests and legacy model files/IDs. Add typed v2
   projections and resolver behind the existing authorization boundary.
2. Migrate download records with staged validation and a private recovery copy.
   Fixture tests must cover interrupted/failed records and legacy partial bytes.
3. Switch the shared Models UI to typed catalog/operation clients; retain the
   legacy API projection and move local import to advanced controls.
4. Verify install/cancel/resume/repair/remove against isolated HTTP and storage
   fixtures for all four layouts, plus legacy download and chat regressions.
5. Record real-source checks and actual evidence in the reliability plan. Live
   model downloads need separate approval; synthetic data proves lifecycle, not
   inference quality. No changes to conversation, agent, or Knowledge stores.
6. Deployment is separate. Back up before applying the new operation schema; an
   older service must not open an incompatible store. Roll back with matched
   application/data, following the existing workspace recovery procedure.

### Approved review follow-up: voice selection and lifecycle

Use the existing actor/workspace-scoped browser draft store for shared ASR/TTS
preferences, without persisting capture permission or autoplay. Resolve a model
revision and voice once per recording/answer; explicit unavailable preferences
fail visibly instead of falling back. Keep selectors in a collapsed panel beside
the composer, not in the primary model selector. This does not replace the later
server-side audio preference/session APIs.

Microphone work owns its abort controller, tracks, recorder, timer and composer
identity. Cleanup invalidates ownership before stopping devices; late permission
results release their tracks. Transcription uses the latest draft callback only
in the original context. Parent generation must not disable Stop recording.

Acquisition review is inline: one supported variant resolves automatically;
multiple supported variants require selection. Conditions stay visible before
Download. Keyword search uses the Hub rather than a Qwen shortcut. Search size
must use byte metadata, never the safetensors parameter count.
