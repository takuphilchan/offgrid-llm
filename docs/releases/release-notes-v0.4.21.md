# OffGrid LLM v0.4.21

This release adds a governed local workspace path to OffGrid's native agent and
improves its terminal and desktop experience. Local file and process work now
uses an enrolled executor with explicit folder authority, durable receipts and
independent result checks.

## Changes

- `offgrid agent` opens a persistent conversation with a compact command menu,
  visible progress and streamed replies. One-shot terminal runs follow work to
  completion by default; explicit detached and JSON use remain available.
- File and process capabilities use a selected host and folder. Conversation
  requests do not ask for folder access. A terminal can remember approval for
  the exact folder, host, service identity and permission level and can revoke
  that trust.
- File writes are checked against host receipts and read-back. A service task
  artifact is identified as an artifact, not reported as a file in the user's
  current directory. Exact text selected from a unique user-request span is
  preserved as UTF-8 bytes through the typed write path.
- Native tool use is admitted by measured model/runtime capability and by the
  current task's authority. Rejected or uncertain actions cannot be relabelled
  as completed work; process results retain their actual exit and output.
- Reusable agent skills, opt-in scoped memory and calendar schedules use owned,
  revision-bound records. These resources do not grant desktop or folder access.
- Chat, Agents and Activity expose the new work and access state while retaining
  the current conversation and saved-run identities.

## Verification and limitations

Go, desktop, API generation, web build, documentation and OpenSpec checks pass
for this source. Isolated packaged-model testing verified six exact local-file
creations across Qwen2.5 7B and Meta-Llama 3.1 8B, three reviewed skill runs,
and a repeated conversation/workspace suite; its final result and limitations
are recorded in the [dated evidence](../quality/evidence/native-agent-execution-2026-10-25.md).

Those isolated checks do not qualify arbitrary requests, every model, or every
platform. They use disposable data and an explicitly seeded evaluation-only
capability record; no such record is copied into installed user state. Native
macOS/Wayland permission behavior, unattended schedules on real hardware and
subjective answer quality require separate qualification. The existing
execution and model readiness gates continue to apply.

## Upgrade

Stop active work before replacing a service or desktop package. Preserve the
model and data volumes, and install the matching executor bundle where local
folder work is required. Review any interrupted saved job before resuming;
refresh, reconnect and replacement do not replay uncertain actions.
