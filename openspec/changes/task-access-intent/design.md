# Design

## Context

See proposal.md. Current job submission persists an AgentConfig but has no proposed policy. TaskAccess chooses policy during a waiting-for-input interruption. Protocol-v2 binds a consent to one exact window; it cannot express desktop authority. The durable runner already has pre-dispatch proposal recovery but accepts a subsequent no-tool response unconditionally.

## Goals / Non-Goals

**Goals:** Make access intent stable and visible before execution, close the demonstrated false-success path, and report text-only results honestly without an English keyword intent classifier.

**Non-Goals:** Changing grant enforcement, automatically launching apps outside the durable intent path, removing the installed host's required window binding, or presenting this slice as the completed task-wide program.

## Decisions

- Add `requested_computer_approval_mode` to persisted AgentConfig and job snapshots, with `computer_approval_mode` in the submission envelope. This is proposed policy, not granted authority. The existing granted-policy field remains unchanged. An omitted field keeps old clients and their request hashes compatible.
- Check attached policy before model preflight, host reservation or runner continuation. New tasks show their recorded policy; legacy tasks retain their old selector. Do not permit a renderer to change the saved policy halfway through the task.
- Include policy in the composer submission record and request digest. Persist the current draft policy within actor/workspace scope. Use the existing utility popover for outside-click, Escape, focus and narrow-screen handling. Keep all nine locales and monochrome controls.
- No-tool completion following an unresolved proposal uses the same persisted repair counter, corrective messages and limit. Do not convert arbitrary provider failures or uncertain effects into retryable work. Rejection history remains evidence of no dispatch, not success.
- Derive a conservative `answer_only` outcome from completed tasks with no executed tool steps, excluding internal plan/input/rejected bookkeeping. Other tasks have no new qualification claim. Keep the existing terminal status for legacy clients; new UI labels answer-only tasks and shows a no-actions notice. Do not mutate historical tasks just to change presentation.
- Prefer this evidence-based outcome over keyword detection of phrases such as "open" or "I cannot". It cannot guarantee model task compliance; actual desktop completion still needs host execution and independent evidence.

## Risks / Trade-offs

- Policy selection can be mistaken for a desktop grant → Explicit localized explanation and retained local consent; no silent downgrade or widened scope.
- Small models may still answer instead of using tools → Honest answer-only outcome; regression fixtures reproduce the exact failure without claiming real-model qualification.
- Old tasks have no proposed policy → Keep compatibility consent selection; never infer an upgrade from their settings.
- Native task-wide access remains unfinished → Record it as a required subsequent host stage in the existing computer guide/reliability plan, not as a completed acceptance scenario.

## Migration Plan

Additive JSON fields require no data rewrite. Existing request envelopes omit empty policy when hashed. Existing sessions stay window-bound. Rollback preserves task data, but older clients may ignore the new outcome classification and policy UI; service enforcement must stay on a matching build for the new workflow.
