# OffGrid LLM v0.4.23

This maintenance release improves workspace organization across web and desktop,
unifies file selection, and corrects overly broad memory-pressure admission.

## Changes

- Chat groups occasional response, retrieval and voice preferences in **Context
  & response**, while image attachment, microphone input and active Stop controls
  remain directly available. Preference menus are bounded and scrollable.
- Agent setup puts identity and model before purpose and optional capabilities.
  Save actions remain reachable on tall desktop layouts; narrow layouts retain
  ordinary document flow.
- Models offers explicit selection separately from verification/removal.
  Selection alone does not load or download a model. Knowledge groups search and
  question actions, preserving explicit selected-document scope.
- Activity separates saved tasks from schedules and retains an unsaved schedule
  while switching views. Long artifact names have compact summaries and an exact
  filename disclosure; integrity checks and downloaded names remain unchanged.
- Management pages share consistent alignment. File inputs use shared accessible
  controls, and preferences no longer stretch unnecessarily across wide screens.
- Desktop startup leads with connection progress and recovery actions. Versions,
  address and elapsed time remain available under **Technical details**.
- Bounded inference requests are no longer rejected solely by the broad
  memory-pressure gate. Model/context admission and safety reserves still apply;
  the service does not silently change models, reduce context or retry work.

## Verification and limits

The final local renderer suite passed **689 tests with 28 existing skips**.
Desktop tests passed **92 with one existing skip**. API/type/build, documentation,
layout, hygiene and OpenSpec checks passed. Earlier failed runs and their fixes
are retained in the [completion audit](../quality/evidence/workspace-completion-audit-2026-11-01.md).
Memory-admission changes have separate Go and race-test evidence in that audit.
Release packaging requires passing CI for the exact tagged source revision.

These results are not a claim of completed human usability or native-model
qualification. The narrowest composer remains dense; novice comprehension,
manual screen-reader behavior, native-speaker review, real vision interpretation,
and the complete native platform/model matrix remain open. Existing experimental
capabilities retain their restrictions. No competitor-parity claim is made.

## Upgrade

Finish or stop active work, back up the workspace, and install matching service
and desktop components. Preserve models and configuration; inspect interrupted
work before explicitly resuming it. Native Windows/macOS desktop does not require
Docker or WSL. This update does not migrate a Docker workspace automatically,
download models, grant permissions or change your chosen service.
