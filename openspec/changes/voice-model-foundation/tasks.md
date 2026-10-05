# Tasks

The six completed tasks below record the tested import-only foundation. They do
not complete the revised change. Groups 4-8 are required before this change can
be marked implemented or archived; their checks have not run merely because this
plan was validated. Runtime packs, actual speech inference, and voice workflows
remain subsequent required stages of the full program, not acquisition features.

## 1. Model package core

- [x] 1.1 Implement strict versioned manifests and architecture layouts; validate valid and adversarial fixtures with `go test ./internal/models` and document the manifest contract.
- [x] 1.2 Implement confined atomic imports, verification, inventory, leases, and guarded removal; test corruption, cancellation, conflicts, links, and concurrent lifecycle operations with `go test -race ./internal/models` and document recovery behavior.
- [x] 1.3 Exclude packages from legacy scanning and replace non-embedding chat assumptions in server, CLI, and shared UI; run model/server and UI regression tests demonstrating legacy compatibility.

## 2. Model management integration

- [x] 2.1 Add permission-gated typed v2 package inventory and bounded import/verify/removal APIs; test authorization, limits, safe errors, and actual temporary-directory storage using `go test ./internal/server` and update API documentation/contracts.
- [x] 2.2 Integrate package inventory/import/verify/removal in Models with explicit readiness and no speech entries in chat selection; run `npm.cmd --prefix web/app run api:check`, `npm.cmd --prefix web/app run check`, and `npm.cmd --prefix web/app run build` with regression coverage and user guidance.

## 3. Foundation integration evidence

- [x] 3.1 Run applicable Go/race, desktop, Python, documentation and isolated real-service UI checks from the workflow guide; record actual results, unrun checks, and remaining voice stages in the existing reliability plan without production claims.

## 4. Shared catalog and complete package resolution

- [x] 4.1 Generalize catalog/inventory contracts with typed legacy-file and package targets, capability categories, variants, provenance, and independent readiness; preserve legacy IDs/projections and update model/API guidance. Verify `go test ./internal/models ./internal/server ./pkg/api` covers speech exclusion and unchanged language/embedding selection.
- [x] 4.2 Extend the existing Hugging Face discovery client with bounded category search and immutable-revision resolution for complete Whisper, Piper, streaming Zipformer, Kokoro, and reviewed Qwen recipes; generate manifests without user dependency assembly. Add missing/ambiguous-layout, incomplete-listing, changed-revision, and source-identity fixtures; run `go test ./internal/models` and document supported discovery in `docs/guides/model-discovery.md`.
- [x] 4.3 Add at least one real curated package variant per supported recipe to the shared catalog with pinned source/runtime requirements, complete artifact identities/sizes, licenses, and explicit unqualified runtime status. Validate metadata/provenance evidence without automatic weight downloads, run `go test ./internal/models`, and record sources and limitations in the existing reliability plan; placeholders or synthetic entries do not satisfy this task.
- [x] 4.4 Implement resolution preflight for complete transfer/staging/recovery space, missing runtime requirements, public-access limitations, safe HTTPS sources/redirects, and bounded metadata/artifacts without changing proxy/VPN settings. Cover insufficient disk, unsafe hosts/paths, credential leakage, missing digest identity, and incomplete dependencies with `go test ./internal/models ./internal/server`; document actionable preflight failures in the model guide.
- [x] 4.5 Add reviewed Hugging Face package layouts for the official Qwen3-ASR 0.6B/1.7B and Qwen3-TTS 0.6B/1.7B families. Detect their safetensors/config/tokenizer/codec assets without executing repository code, generate immutable manifests with explicit `qwen3-asr`/`qwen3-tts` adapters, expose unsupported forced-aligner and community conversions as informative results, and cover complete/incomplete Qwen repositories with model-resolution tests and documentation.

## 5. Shared durable acquisition and recovery

- [ ] 5.1 Extend the existing download coordinator and one versioned operation store for legacy files and package targets; stage/back up/validate legacy history migration, preserve partial bytes, and reject corrupt/unknown versions without dual writing. Run `go test ./internal/server` against interrupted-migration, legacy-resume, full-disk, and recovery fixtures; update download and workspace-recovery documentation.
- [ ] 5.2 Implement bounded multi-artifact transfer using shared HTTP/resume primitives, package staging, aggregate progress, per-artifact size/identity checks, and atomic complete-package publication; keep long transfers/hashing outside global lifecycle locks. Run `go test ./internal/models ./internal/server` for range 200/206/416, truncated/corrupt bodies, timeout, size limits, and concurrent ordinary downloads; document transfer versus verification states.
- [ ] 5.3 Persist actor/request/source binding, acceptance, artifact checkpoints, activation intent/receipt, and terminal state before acknowledgment; deduplicate identical retries and reject changed payloads or competing package writers. Add crash/lost-ack/full-disk fixtures before and after activation, run `go test -race ./internal/models ./internal/server` in the supported race environment, and document exact recovery semantics.
- [ ] 5.4 Implement cancel settlement, explicit resume after restart, revalidation of retained artifacts, and visible retained-byte/discard state without automatic restart or revision drift. Run `go test -race ./internal/models ./internal/server` for cancel/resume races, changed upstream revisions, stale partial files, and interruption after individual artifacts; update `docs/guides/model-discovery.md`.
- [ ] 5.5 Implement exact-revision staged repair, recoverable publication on supported filesystems, prior-data preservation, and lease-aware activation; offer re-import for local packages without verified sources. Run `go test ./internal/models ./internal/server` with Windows and Linux coverage for sharing failures, missing configuration, failed verification, restart between publication steps, and active leases; document repair/rollback limitations.
- [ ] 5.6 Coordinate installation/repair with confirmed package removal and explicit staging discard; prevent deleted packages reappearing and remove only the selected managed data. Run `go test -race ./internal/models ./internal/server` for removal/activation races, orphaned staging, links/path replacement, and preservation of other packages/documents; document retained data and cleanup behavior.

## 6. Authorized API and service-backed clients

- [ ] 6.1 Add v2 typed inventory/catalog, resolution/preflight, and durable model-operation endpoints with model permissions, actor/admin ownership, persist-before-202, conflict detection, and coded errors; keep v1 and existing package APIs compatible. Update OpenAPI/generated types and API docs; run `go test ./internal/server ./pkg/api` and `npm.cmd --prefix web/app run api:check`, including cross-user access and legacy-client fixtures.
- [ ] 6.2 Extend existing CLI model commands for capability-filtered discovery and package install/status/cancel/resume/repair/removal through the authenticated service client, not a direct speech downloader. Verify `go test ./cmd/offgrid ./internal/serviceclient` covers valid JSON, stderr progress, cancellation/exit codes, source-conflict errors, and old command behavior; update CLI/model guidance.
- [ ] 6.3 Add matching typed Python model-operation helpers using existing authentication and request infrastructure, preserving legacy model/audio methods. Verify `PYTHONPATH=. python3 -m pytest tests/test_client.py` from `python/` uses isolated HTTP fixtures with accidental live sockets blocked, including operation polling/cancellation/errors; update Python/API usage documentation.

## 7. Unified web and desktop Models experience

- [ ] 7.1 Replace the import-first speech section with shared installed/discovery cards and Language, Embeddings, Speech recognition, and Speech generation filters; support explicit Hugging Face search, variant selection, complete-package preview, and Download without file collection or manifest editing. Move compatible local import into collapsed advanced/offline controls; run `npm.cmd --prefix web/app run check` and isolated real-service Playwright tests, and update model/discovery guides.
- [ ] 7.2 Integrate durable aggregate/per-artifact progress, cancellation, resume, repair, verified deletion, and partial-data discard into existing Models interactions; navigation/reconnect must restore work without resubmission. Show source/license/space and missing runtime requirements before Download, with truthful installed readiness afterward. Run isolated real-service UI tests for all supported package layouts and recovery paths; update screenshots/guidance only from actual behavior.
- [ ] 7.3 Preserve monochrome styling, all nine locales, RTL, keyboard/focus behavior, accessible status announcements, and responsive web/matching-desktop layouts. Run `npm.cmd --prefix web/app run check`, `npm.cmd --prefix web/app run build`, `npm.cmd --prefix desktop test`, and isolated browser accessibility/layout checks in both themes; record actual evidence and any unrun installed-desktop checks in the reliability plan.

## 8. Revised-change integration gates

- [ ] 8.1 Run `go test ./...`, applicable CI race coverage including models/audio/server, `npm.cmd --prefix web/app run api:check`, UI check/build, desktop tests, and isolated Python checks after integration. Record results in the existing reliability plan; earlier foundation passes do not substitute for reruns on the changed download/operation code.
- [ ] 8.2 Run `node dev/scripts/test-web-workspace.mjs <fresh-built-binary>` against isolated state with synthetic Hugging Face fixtures exercising download, verify, cancel/resume, restart recovery, repair, deletion, and legacy chat/embedding regression. Separately record approved real-source installation checks and installed-platform evidence when available; do not count synthetic bytes as inference qualification. Run documentation checks and `openspec validate voice-model-foundation --strict --no-interactive`; retain unresolved acquisition defects as incomplete work.

Implementation checks do not authorize real model-weight downloads, microphone
capture, replacement of the current container/desktop, commits, pushes, or releases.
When this acquisition change passes, continue the required runtime/adapters and
voice-interface stages; it must not be announced as the complete Voice program.

## 9. User-requested speech runtime corrections (2026-10-05)

The user separately authorized these corrections and declined new model downloads.
They do not complete the pending acquisition gates or the full Voice program.

- [x] 9.1 Route installed packages by architecture/capability, retain legacy paths, refuse unsupported profiles and explicit mismatches, and hold model leases for worker lifetime. Verify `go test ./internal/audio ./internal/server` plus Linux executable Whisper/Piper protocol fixtures; document unavailable adapters without claiming real-model qualification.
- [x] 9.2 Isolate Python/library stdout from the worker protocol, bound/cancel owned inference, remove implicit reference-voice cloning, and use private output files. Run embedded-worker noisy-import/cancellation/Base-rejection fixtures on Windows and Linux.
- [x] 9.3 Fix page CSP for owned audio, expose preparation/errors/stop, bound first speech chunks and look-ahead work, and stop on navigation. Run UI check/build and `node dev/scripts/test-web-workspace.mjs bin/offgrid-voice-test.exe voice-playback.spec.ts` against isolated state.
- [x] 9.4 Record final regression and deployment results, package support boundaries, and unrun real-voice/latency checks in the existing reliability plan. Preserve installed models; no microphone capture, commits, or publication. Necessary compatible-model downloads were subsequently authorized.

## 10. Streamed response speech (user authorized 2026-10-05)

- [x] 10.1 Add explicit session-only Speak responses opt-in, incremental prose buffering, bounded playback work, readiness feedback before synthesis, and cancellation on generation failure/stop/navigation. Keep text streaming independent, do not replay recovered turns, and label provisional speech. Test streaming before completion, final-tail delivery without duplicates, unsupported models, markup/reasoning exclusion, queue overload, and cancellation with isolated browser fixtures; run UI checks/build.
- [x] 10.2 Install a compatible direct-voice model through the existing verified model service under the user's subsequent download approval. Preserve Base/ASR packages, verify real synthesis and cold/warm timings, and record limitations and deployment evidence in the reliability plan. This does not authorize microphone capture or voice cloning.

Section 10 uses the verified Piper voice. Qwen CustomVoice remains a retained,
resumable partial download after a network-unreachable failure; its actual
synthesis is not claimed as tested. See the reliability plan for measurements.

## 11. Approved pre-release review corrections (2026-10-05)

- [x] 11.1 Cancel microphone acquisition/transcription on context exit, release late permission results, preserve concurrent draft edits and keep Stop usable during generation. Verify isolated browser regressions without recording a user microphone.
- [x] 11.2 Add shared browser-scoped ASR/TTS and declared-voice selectors beside the composer; pin the selection per recording/answer, reject unavailable explicit profiles, and forward validated managed Piper speaker IDs. Verify browser selection/chunk tests and Linux executable adapter fixtures, without claiming model quality.
- [x] 11.3 Remove duplicate package-review markup, show download conditions inline, retain meaningful multi-variant selection, query Hub keyword results instead of Qwen shortcuts, and stop interpreting parameter counts as bytes. Update acquisition regressions and documentation; verify Go and isolated acquisition tests.
- [x] 11.4 Complete integrated regression checks, document release blockers and unrun installed-platform tests in the reliability plan. Do not treat this correction as full Voice qualification or deploy/commit without a separate request.
