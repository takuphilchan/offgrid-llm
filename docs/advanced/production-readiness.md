# Production-readiness delivery contract

Status: approved direction, implementation in progress. No milestone is certified
complete. "9/10" describes the ambition, not a rating earned by passing unit tests.
See the [reliability evidence log](product-reliability-plan.md) for tested changes.

## September 2026 improvement roadmap

The next delivery sequence serves individuals, researchers/students, and small
teams through one product: evidence-backed document answers, verified task
artifacts, and reproducible model comparisons. Deliver staged releases, retain
the current stack and supervised llama.cpp, and qualify 8 GB/16 GB CPU profiles
before promising larger workloads. Evaluation comes before training.

1. **Correctness/runtime repair — in progress.** Replace placeholder embeddings,
   quarantine unverifiable indexes, provide safe staged recovery, validate runtime
   contracts, correct metrics, and report capabilities honestly. The first slice
   implements real HTTP embeddings and offline recovery; this is not a completed
   runtime qualification or a release authorization.
2. **Durable workspace — pending.** Finish transactional migration and shared v2
   projects/conversations/jobs, preserving the existing durable-run behavior.
3. **Evidence and useful tasks — pending.** Private-by-default projects with explicit
   reader/editor membership, inherited collection access, versioned citations,
   strict evidence mode, and bounded member tools producing verified artifacts.
4. **Coherent clients — ongoing.** Project-centred document/task/result workflows,
   consistent recovery and accessibility, and service-authoritative state across
   CLI, web, and Electron. Research controls must not complicate ordinary chat.
5. **Research workspace — pending.** Versioned JSONL/CSV datasets, frozen run
   specifications, per-sample results, raw-versus-assisted comparisons, human
   review, Python access, and inspectable exports, using the same jobs/permissions.
6. **Qualification/distribution — pending.** Verified offline packs, immutable
   release manifests, installed-edition tests, independent review and pilot evidence.

No projects/datasets/experiments v2 API is advertised as implemented. Shell,
arbitrary network/filesystem access, external installation and MCP configuration
remain administrator-controlled. General-model fallback from document mode must
be explicit. Unrestricted computer use, training, distributed inference and
expanded P2P remain outside the production guarantee.

## Product boundary

Dependable local chat, document-grounded answers, and governed agent tasks for
individuals and small teams. One authoritative service owns each workspace; this
is not a multi-tenant SaaS. Retain Go, React/TypeScript, Electron, and the CLI
presentation libraries. CPU-first: 8 GB minimum, 16 GB recommended; larger models
and external-agent contexts need separately measured hardware profiles.

Retain Windows x64; macOS Intel and Apple Silicon; Linux x64 desktop and x64/ARM64
CLI; AMD64/ARM64 CPU containers and AMD64 NVIDIA containers. CPU, Vulkan, and Metal
variants must be qualified independently. Do not imply every accelerator exists
on every OS. Unrestricted computer use, distributed inference, fine-tuning, and
expanded P2P remain experimental and outside the production guarantee.

Updates require explanation and explicit approval, preflight, backup, staged
installation, migration, and health verification. Missing signing credentials,
hardware, independent review, or pilot evidence keeps affected editions in preview.
This plan does not authorize publishing, moving tags, pushing, or replacing a
live installation.

## Architecture decisions

- Conversation, knowledge, job, model, and integration application services own
  authorization, validation, persistence, and lifecycle—not individual clients.
- Ordinary CLI commands use those services through the API, without silent
  direct-file alternatives. Offline maintenance requires exclusive ownership
  while the service is stopped.
- A versioned SQLite workspace database will hold conversations, messages, jobs,
  ordered events, agent checkpoints, collection permissions, and index metadata.
  Large blobs/models stay outside it, referenced by verified digests. Existing
  identity/configuration stores remain behind interfaces.
- Use generated IDs; titles and source filenames are editable/display metadata.
  Configure foreign keys and durability on every connection. Pin a SQLite runtime
  with applicable WAL fixes and use local storage, never a network drive.
- Migration must stage the database, verify ownership/counts/integrity, preserve
  original files plus a recovery manifest, then activate once. No dual-write.
  Unowned sessions/shared knowledge enter an administrator-controlled legacy area;
  never assign them to the first user. Corruption blocks migration with a report.
- `/api/v2/system` identifies the service/build/renderer and supported contracts.
  Remaining v2 operations will expose conversations/turns, jobs/events/cancel,
  and collections/documents/search/source access. The system endpoint is implemented;
  these other v2 APIs are **not yet available**.
- Accepted work must be persisted before `202`. Actor-scoped request IDs return
  existing work for identical retries and `409` for conflicting reuse. Persist
  ordered events before SSE delivery; reconnect supports replay and explicit
  snapshot recovery after cursor expiry. Slow clients cannot block execution.
- Keep deliberate OpenAI-compatible inference endpoints, including
  `/v1/chat/completions`. Retire first-party endpoints only at their coordinated
  client/storage cutover, with an explicit upgrade-required response.
- Generate types and check drift. Errors need stable codes, safe messages,
  retryability, and request IDs. No ambiguous mutation is automatically retried.

## Milestones and gates

### 1. Correctness across clients — in progress

- Standardize CLI success `0`, operational failure `1`, usage `2`, cancellation
  `130`; authenticated bounded requests; JSON results/errors; progress on stderr.
  Knowledge, saved-conversation commands and agent lifecycle controls are migrated. Other commands
  still require conversion; this is not a global exit-code guarantee yet.
- Match desktop/backend identity, API, product version, and UI build; show the
  connected backend. Never stop an externally managed service.
- Cover prompts containing punctuation, paths, Unicode, duplicate titles, protected
  knowledge, and attachment to older backends. Generated conversation IDs belong
  to the staged migration, not a filename-sanitization workaround.

Gate: native Windows/Linux/macOS and container-backed clients behave consistently
for ordinary prompts and authenticated workflows. Packaging alone is insufficient.

### 2. Persistence and recovery — in progress

Implemented foundations: exclusive service ownership, drained agent shutdown,
nonblocking event subscriptions, and offline data-directory backup/verification/
safe restore. See [recovery instructions](workspace-recovery.md). These do not
complete transactional migration, durable chat, or the administrator recovery UI.

- Complete staged migration, single-service ownership, durable submission,
  idempotency, replay, cancellation, and atomic completed turns.
- Separate interrupted/cancelled output from completed conversation context.
  Navigation and transport disconnects do not cancel work; explicit cancellation,
  deadlines, or shutdown do. Persist active jobs in a shared client store and
  reconstruct from the service after reconnect.
- Preserve exact-call approval identity/ownership/expiry and reconciliation.
  On restart, never repeat a side effect with an uncertain outcome.
- Backup/verify/exclusive restore with administrator UI controls; cover the whole
  workspace consistently, not just a copied live SQLite file.

Gate: crash, reconnect, navigation, low disk, and upgrade tests produce no lost
completed turns, duplicated acknowledged work, or falsely successful side effects.

### 3. Trustworthy knowledge and useful agents — pending

- Private collections with reader/editor memberships; enforce access at ingestion,
  retrieval, source opening, export, deletion, and tool execution. Admins own the
  legacy area. Content hashes do not replace independent document identity.
- Store document versions/citation locators with answers; recheck permissions and
  clearly report changed, removed, or inaccessible sources.
- Durable import/reindex, extraction preview, supported-format/OCR limitations,
  atomic replacement indexes preserving the last usable version on failure.
- Distinguish retrieval failure, no evidence, and supported answers. Retrieved
  text is untrusted data, never authority for tool execution.
- Members receive curated calculation and permission-scoped knowledge tools;
  filesystem/shell/network/MCP configuration/install operations remain admin-only.
- Check completion reasons before accepting an agent answer or executing its tools.
  This guard is implemented, but task quality and verified artifacts still need
  qualification. Native model capability is not improved by this guard alone.
- Qualify pinned Hermes/OpenClaw install, configure, repair, remove, and actual
  artifact-producing tasks. Their permissions remain separate from OffGrid approvals.

Gate: useful artifacts and verifiable citations, with no protected-data access or
unauthorized actions in permission, revocation, and prompt-injection tests.

### 4. Clear, responsive clients — pending

Implemented foundation: bounded FIFO inference admission, queue-full errors,
cancelled-waiter removal, and health queue counts. Per-actor/workload scheduling,
indexing coordination, and full client queue/recovery presentation remain pending.

- Preserve monochrome design. Consistent empty/loading/queued/denied/interrupted/
  recovery states, keyboard/focus/screen-reader/responsive and composition behavior.
- Share localization across web, desktop shell, and human CLI messages. Keep
  command/error identifiers stable. All nine languages need speaker review.
- One bounded inference scheduler for chat/agents/indexing, default one active
  generation, fair actor/workload queues, limits and visible cancellation.
- Report actual backend, GPU placement, allocated context and measured timings.
  Ordinary chat remains available without optional agent/knowledge dependencies.
- Redacted support bundles exclude prompts, documents, and credentials by default.

Gate: people can understand, cancel, and recover work without guessing commands
or reading raw traces. Browser preference fallback is not durable workspace state.

### 5. Dependable installation and updates — pending

- A single release manifest ties versions/revisions/requirements/capabilities/
  checksums/evidence to CLI, renderer, desktop, runtime packs, and containers from
  one immutable revision. Stable aliases advance only after all required gates.
- Verify signatures/checksums; sign Windows apps and sign/notarize macOS apps.
- Approved upgrade/backup/migration/health pipeline. Never run an older binary on
  an incompatible migrated DB. Restore a matched application/data snapshot and
  explain rollback consequences. Docker/service upgrades remain admin-controlled;
  do not mount the Docker socket into the application.
- Signed offline packs include requirements, digests and licenses. Reject bad,
  incompatible or untrusted packs; test first use without network after import.
- Retry releases only with artifacts verified against the expected revision and
  manifest, not filenames or a previous successful upload alone.

Gate: clean install, offline use, approved upgrade, failed-update recovery, and
backup restoration on every advertised edition.

## Qualification required before a production claim

- Cross-client contracts, Go race checks, vulnerability/container scans, real
  browser and installed Electron smoke tests across the supported matrix.
- Two-user isolation and revocation for conversations, collections, jobs,
  artifacts and citations; desktop IPC, navigation, credentials and update review.
- Fault injection: disconnect, duplicate submission, crash around persistence and
  tool calls, full disk and corrupt data. Migration/restore fixtures from v0.4.3
  include malformed and unowned records. CLI subprocess coverage includes piping,
  authentication, JSON, exit status and cancellation.
- No unresolved critical/high security findings, P0/P1 bugs or failing safety tests.
- Pinned 8 GB CPU, 16 GB CPU, NVIDIA and Apple Silicon fixtures measure cold load,
  warm first-token/generation, memory, queue delay, cancellation and retrieval at
  increasing corpus sizes. Gate repeatable regressions above 15% on the **same**
  fixture; never equate different model/context profiles.
- At least 100 document questions, including unanswerable cases: at least 90%
  retrieval hit rate and judged answer-support/abstention accuracy on the frozen
  supported-profile set.
- At least 30 useful agent-task cases with repeated execution: at least 90%
  correct outcomes and no unauthorized actions in adversarial cases.
- A 72-hour mixed-workload soak without unexplained crashes, acknowledged-data
  loss, duplicate side effects or unbounded growth.
- A 30-day pilot with at least 15 representative people; at least 90% finish
  onboarding and the core workflow without developer intervention.

Report evidence by model, language, hardware, edition and feature. One successful
profile does not qualify another. Deliver documentation/quickstarts/recovery/
security boundaries with each slice. Rebuild isolated containers at deployment
checkpoints; back up before any approved live replacement and never attach two
services to the same writable workspace.

## References

- [SQLite WAL and WAL-reset fixes](https://www.sqlite.org/wal.html)
- [SQLite online backups](https://www.sqlite.org/backup.html)
- [SSE event IDs and reconnection](https://html.spec.whatwg.org/multipage/server-sent-events.html)
- [Electron signing guidance](https://www.electronjs.org/docs/latest/tutorial/code-signing)
