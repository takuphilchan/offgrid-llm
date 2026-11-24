# Local model policy admission — 2026-11-20

This records source/contract implementation in the shared worktree. It is not
real model, hardware, automatic switching or release qualification. The owning
change is [model-adaptive-execution](../../../openspec/changes/model-adaptive-execution/),
task 4.1; conversation-agent tasks 12.2 and 12.3 remain open as whole requirements.

## Existing scheduling and budget behavior inspected

`internal/inference/generation_admission.go` already supplies a bounded 3:1
foreground/background admission queue with actor/work rotation and cancellation
cleanup. Admission precedes family/runtime acquisition. `internal/agents/scheduling.go`
derives scope from accepted work; `coordination_budget.go` reserves the greater
of two calls or ten percent for canonical parent verification/response.
`worker_budget.go` only refunds settled unused child capacity. These were inspected
and relevant tests rerun; this increment did not create those earlier mechanisms.

## New implementation

- Immutable optional version 1 profile policy: guided/adaptive intent, at most
  two distinct local fallback models and a separate perception reference. Absent
  policy leaves old serialized profile digests unchanged.
- Profile readiness and admission resolve actual installed models, planned
  context, and matching retained workflow evidence through the existing trusted
  installation policy. Models, runtime/configuration bytes, hardware and context
  must match; a name, metadata flag or synthetic smoke pass is insufficient.
- Primary stays selected by default. A job may explicitly select an approved
  qualified fallback. The task pins candidates, evidence and selection reason;
  the existing under-lease native adapter validation prevents stale inference.
- Perception additionally requires full workflow qualification and measured
  projector/preprocessing identities. Text-only or scoped evidence cannot supply it.
- Accepted configuration is detached before insertion into the live task map.
  A failing regression exposed that saving a detached snapshot alone allowed
  caller-owned pointers to mutate live policy; `run_prepare.go` now copies all
  persisted configuration data before validation/publication. Explicit empty
  Knowledge/tool ceilings are canonicalized on the detached copy so JSON null
  cannot turn a restriction into inheritance; the caller is not mutated.
- API schema/generated web types, CLI JSON parsing and Python forwarding retain
  the new policy. Foreign profile reads and unknown fields remain denied.
- Conversation admission now finalizes its response profile before readiness and
  policy qualification. Default/interactive and extended context are pinned by
  the shared resolver, with no later independent adapter overwrite.

## Checks

- Focused Windows Go tests for profiles, model policy, selected evidence, scope,
  generation fairness, cancellation and CLI parsing passed after the admission
  alias fix. The first new service test failed on that alias; it was not waived.
- API generation/check passed. Web TypeScript/check and build passed after the
  parallel browser locale changes completed. An earlier check during those edits
  failed on missing browser locale keys; it is not presented as a clean baseline.
- Python profile request/inspection contract: one passed, 36 deselected, using
  Ubuntu WSL and `PYTHONPATH=sdk/python`.
- Source-size ratchet passed without increasing limits.
- `go test ./internal/agents ./internal/inference ./internal/server ./internal/cli -count=1`
  passed (150.517 s / 24.625 s / 202.673 s / 3.231 s). This run preceded the final
  empty-selection normalization; its focused test subsequently passed.
- Ubuntu WSL `CGO_ENABLED=1 go test -race ./internal/agents ./internal/server ./internal/cli -run TestProfile -count=1`
  passed (5.391 s / 9.044 s / 1.054 s). Latest `-run TestProfileModel` on agents/server
  passed (1.418 s / 2.713 s); `-run TestRunAdmissionDetaches` on agents passed
  (1.055 s), both with `CGO_ENABLED=1 go test -race` in Ubuntu WSL.
- `node scripts/maintenance/check-docs.mjs` checked 428 maintained Markdown files
  with zero problems. `openspec validate model-adaptive-execution --strict` and
  `git diff --check` passed.

Logs: `build/local-model-policy-go.log`, `build/local-model-policy-race.log` and
the final `build/local-model-policy-*-race.log` files. Test fixtures include
restart preserving selection/evidence without dispatch, stale selected-runtime
rejection, unchanged tool authority, legacy profile history and explicit Chat
override semantics. The original tests were corrected to compare against the
existing allowed capability baseline rather than assuming that baseline was empty.

Independent review reproduced two context defects with isolated service tests:
an interactive-only qualified fallback was rejected against the default agent
window, while a default-window-only fallback could be durably admitted at an
unqualified interactive window with its adapter cleared. Both failures were
reproduced before correction. The actual session HTTP/admission regression now
covers omitted/interactive and extended response profiles plus rejection when
the final context lacks qualification, inspecting the saved intent before any
inference. `go test ./internal/server -run
'Test(ProfileModelPolicy|ConversationHistoryBudgets|ConversationIntentPins)'
-count=1` passed (9.018 s). A proposed simultaneous-two-window fixture was
corrected: the existing evidence store selects the newest per-model/check record
and treats an older different-context record as stale; this increment does not
change that evidence policy.

The reviewer reran the original independent two-case Go overlay unchanged after
the correction; both cases passed (3.346 s), and review reported no further
blocking findings in the sampled policy/configuration code. Ubuntu WSL
`CGO_ENABLED=1 go test -race ./internal/server -run TestProfileModelPolicy
-count=1` passed (4.937 s); log `build/local-model-policy-context-race.log`.
Source-size, strict OpenSpec validation, diff checks and documentation validation
(430 maintained files, zero problems) passed on the corrected source.

## Unfinished work

The current candidate path uses the existing measured native-agent policy for
installed GGUF models. Generalized constrained/native-text pool qualification,
automatic safe-boundary runtime switching and perception dispatch are unfinished.
Delegated child policy pinning/admission remains part of open task 4.3; the new
selection snapshot applies to root job and conversation admission.
`adaptive` stores intent and validates candidates; it does not currently cause
automatic switching. Tasks 4.2–4.4, broader scheduling 12.2–12.3 and hardware tests
remain open. No model weights, live installations or personal state were used.
All qualification records in tests are synthetic records in disposable stores.
