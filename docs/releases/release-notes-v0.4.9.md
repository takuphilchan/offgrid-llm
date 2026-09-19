# OffGrid LLM v0.4.9

This update improves knowledge correctness, workspace recovery, and consistency
across the shared web and desktop interface. It is not a claim that every
production-readiness milestone or hardware profile is qualified.

## Important: existing knowledge indexes

Earlier default builds could create hash-derived placeholder vectors rather
than semantic embeddings. OffGrid now uses a supervised llama.cpp embedding
runtime and refuses to search indexes without verified model/runtime identity.
Your retained documents and source text are preserved, but an older knowledge
index may require an explicit rebuild before document retrieval works again.

Back up your workspace before upgrading. Stop the service and follow the
[knowledge recovery instructions](../advanced/workspace-recovery.md) with your
installed embedding model and runtime. Rebuilding stages work resumably and
publishes a complete replacement atomically; missing retained sources block the
rebuild rather than silently dropping documents. This maintenance operation is
currently a CLI workflow, not an in-app automatic migration.

## Runtime and task safety

- Validate embedding dimensions, normalization, response ordering and pipeline
  identity; fail honestly when a model or runtime cannot load.
- Preserve explicit seeds and stop sequences; use the model template instead of
  guessing stop tokens from filenames. Reject unsupported research controls.
- Correct batch duration units and distinguish prompt from completion tokens.
- Never kill an unrelated process merely because it occupies an inference port.
- Allow authorized deletion of interrupted legacy tasks without recovery state.
  Active work, resumable tasks and uncertain tool outcomes remain protected.

## Web and desktop experience

- Consistent monochrome buttons, inputs, focus states and responsive layouts.
- A task-first agent workspace with more room for results and stable controls
  while output grows; readable Markdown results and expandable technical details.
- Improved command-palette sizing, keyboard focus, RTL and composition behavior.
- Clear installed/download states and knowledge readiness; loading and failed
  requests are no longer presented as empty histories or disabled features.
- Preserve filters, model search and connector drafts across navigation; refresh
  the current workspace without discarding drafts or resubmitting work.
- Role-aware controls, safer bulk history deletion and shared nine-language
  desktop startup/appearance preferences. Translation coverage is not speaker review.

## Distribution and verification

The existing Windows, macOS Intel/Apple Silicon, Linux desktop/CLI and container
editions are retained. CPU containers use `takuphilchan/offgrid-llm:0.4.9`
(AMD64/ARM64); NVIDIA uses `takuphilchan/offgrid-llm:0.4.9-gpu` (AMD64).
Verify release downloads against `checksums-v0.4.9.sha256`.

Desktop packages remain unsigned and not notarized until signing identities are
configured. Platform security/reputation warnings may still appear. Full
workspace migration, project sharing and research experiments remain unfinished;
see the [reliability plan](../advanced/product-reliability-plan.md) for evidence
and qualification limits.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.8...v0.4.9)
