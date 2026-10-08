# OffGrid LLM v0.4.15

This release improves the shared web and desktop workspace: clearer controls,
task-focused results, model discovery, and recovery without losing drafts or
repeating accepted work. It preserves the monochrome light/dark themes and
existing conversations, models, permissions, and task history.

## Workspace and everyday use

- A compact shared header and collapsible navigation keep the current work in
  focus. Workspace options hold appearance and language controls; Settings is
  the canonical place for service details. The shared Refresh action replaces
  duplicate page-level controls while retaining contextual recovery actions.
- Consistent outlined buttons, anchored popovers, keyboard focus, and narrow/RTL
  layouts replace scattered link-like actions. Popovers dismiss outside or with
  Escape. All nine existing interface locales remain available.
- Chat keeps voice and send controls beside the composer. Response details and
  performance measurements belong to the answer, not the input area. Stopped or
  unsaved output remains visibly distinct from a saved response.
- Agents emphasizes the requested outcome, current activity, pending input,
  and results. Task continuation and artifact inspection stay with the selected
  task; approvals still bind to the exact authorized operation.
- Models separates Installed from Discover models across language, embedding,
  ASR, and TTS categories. Search/filter state, package sizes, readiness, and a
  single anchored download review help users choose without scrolling between
  unrelated configuration blocks.
- Knowledge, Activity, and task history provide clearer selection, filtering,
  empty states, and deletion controls. Active work and failed deletions remain
  inspectable rather than disappearing from the interface.

## Recovery and maintenance

- Drafts are scoped to their actor and workspace. Restoring legacy drafts is an
  explicit choice and cannot overwrite newer work or restore tool authority.
- Request recovery retains stable identities and distinguishes a failed read
  from an uncertain mutation. Reconnecting or remounting a view does not submit
  another task or automatically repeat an uncertain action.
- Reused-runtime container builds now explicitly serve the matching packaged
  UI rather than an older renderer inherited from the base image.
- Electron is updated to 43.7.9, alongside compatible build-dependency security
  updates. The web dependency audit is clear; the desktop audit has no high or
  critical findings. Eight moderate findings remain in the desktop build chain.

## Availability and known limitations

This is a workspace and recovery release, not completion of the Voice or Computer
Tasks programs. The [voice-preview limitations](release-notes-v0.4.13.md#voice-availability-and-limitations)
still apply: speech weights need a compatible runtime, full Talk mode is not
complete, and speech quality/latency requires model- and hardware-specific
qualification. The recorded Qwen ASR limitation for explicitly supplied `en`
is unchanged. Computer Tasks remains a preview; this release does not establish
general native/vision reliability or authorize additional access.

Automated renderer and isolated Windows package checks cover recovery, themes,
locales, keyboard behavior, and real 200% zoom. They do not replace manual
screen-reader testing, the pending five-person usability study, exact pre-change
latency comparison, or complete installed-platform qualification. Signing,
notarization, real-model quality, and soak/pilot completion are not claimed.
Evidence and remaining gates are recorded in the
[product reliability plan](../advanced/product-reliability-plan.md).

## Upgrade

Back up workspace data and models and stop active work before upgrading. Update
desktop and service together; the Windows installer does not replace an external
Docker container. Preserve existing data/model volumes and speech runtimes.
This release introduces no service storage migration. Existing legacy local
drafts can be reviewed and restored explicitly into the scoped draft store.

CPU image: `takuphilchan/offgrid-llm:0.4.15` (AMD64/ARM64).
NVIDIA image: `takuphilchan/offgrid-llm:0.4.15-gpu` (AMD64).
Verify release assets with `checksums-v0.4.15.sha256`.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.14...v0.4.15)
