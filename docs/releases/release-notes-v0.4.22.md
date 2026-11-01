# OffGrid LLM v0.4.22

This release improves the shared web and desktop workspace, adds image
attachments to ordinary Chat and the CLI, and fixes native agent response
termination and Windows inference-worker packaging.

## Changes

- Chat has quieter controls, grouped model/workspace/input choices, improved
  history scrolling and accessible neutral surface materials. Long Markdown
  parsing runs off the main UI thread. You can navigate between conversations
  while accepted work continues without resubmitting it.
- Ordinary Chat accepts bounded PNG/JPEG attachments through file selection,
  paste or drop. CLI `offgrid run` supports repeated `--image` and interactive
  `/image`, `/images` and `/remove-image`. Failed sends retain attachments.
  Inference requires an installed compatible local vision model and projector;
  agent image execution remains unavailable.
- Model selection is searchable, catalog comparisons distinguish estimates
  from measured readiness, and agent setup groups optional capabilities around
  the intended task. Skill selection preserves reviewed revisions.
- Knowledge questions can select specific documents, with service-enforced
  retrieval scope and inspectable supporting passages. Quick actions searches
  authorized conversation names and root task requests, not private transcripts.
- Results offer integrity-checked text previews, compact specialist activity and
  recovery guidance based on saved state. Scheduling includes a service-computed
  calendar preview; previewing never starts a task.
- Desktop waits for a delayed external service and reconnects without replacing
  the workspace. Standalone Windows/macOS desktop continues to use its native
  bundled service; Docker, Ubuntu and WSL are not prerequisites.
- Windows runtime packaging verifies app-local compiler/OpenMP dependencies.
  Governed tool generation avoids a native regex-grammar defect while retaining
  full argument validation, and keeps stable prompt prefixes across tool results.
- Memory-admission failures retain a typed code across streaming and saved
  history. Chat explains recovery in the selected language, keeps technical
  details expandable, preserves the draft when opening Models, and stops showing
  confirmed failed work as running. This does not reduce memory safety reserves
  or automatically retry or change models.

## Verification and limits

The final local renderer run passed 678 tests with 28 opt-in/environment skips;
the earlier visual-enabled run passed 682. Desktop, companion, Python protocol,
Go, API and structural checks
are recorded with skips and initial failures in the
[verification evidence](../quality/evidence/native-agent-termination-2026-10-31.md).
Three isolated packaged Windows CPU agent journeys independently verified exact
artifact bytes, preview and saved-state restart. They do not establish all-model
accuracy or latency guarantees.

Those native passes used candidate binaries. A subsequent downloaded Windows CPU
desktop attempt was rejected by memory admission before its first reply; this
failure remains recorded in the [completion audit](../quality/evidence/workspace-completion-audit-2026-11-01.md).
The package's digest and worker startup were verified, but that attempt does not
qualify inference. Available memory, model size and context still constrain use.

Real vision interpretation, manual screen-reader testing, participant usability
studies and the complete native platform/model matrix remain unqualified. Image
transport tests do not prove visual accuracy. See the
[whole-product evidence](../quality/evidence/workspace-experience-2026-10-30.md)
for measured responsiveness, retained failures and outstanding gates. This release
does not claim completion of every production-readiness milestone.

## Upgrade

Finish or stop active work before replacing service, desktop or executor files.
Back up the existing workspace and retain its models and configuration. Install
matching components where local execution is used. This release does not migrate
Docker data to a native desktop workspace automatically, download models or grant
new permissions. Inspect interrupted work before explicitly resuming it.
