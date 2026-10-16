# OffGrid LLM v0.4.18

This release refines the shared browser and desktop workspace: a quieter sidebar,
more room for conversations, and one place to inspect agent work. It also includes
Chat attachment-grounding and agent task-storage fixes. Computer use remains
experimental; visual refinement does not change execution permissions.

## A calmer workspace

- One compact, collapsible sidebar combines navigation and Chat history. New chat
  is easy to reach; Settings, appearance, language, refresh and account controls
  live in Workspace options. Narrow windows use a focus-contained drawer.
- Conversation and task history use consistent search and labelled row menus.
  Deletion still requires confirmation and preserves protected active or uncertain
  work. Existing conversation selection and draft isolation remain intact.
- Chat keeps agent/model selection and primary actions close to the editor, with
  detailed voice and response settings in labelled popovers. Follow-latest
  scrolling respects deliberate reading; Latest returns to growing output.
- Copy, Read aloud and Inspect work share an aligned action group. Answers and
  artifacts lead; optional steps and technical details share an activity disclosure.
  Access requests, actionable failures and Stop remain visible.
- Agents configures reusable profiles, tools and connections. Activity owns the
  saved-work list and full-width inspector for the same canonical job. Old agent
  task links remain compatible without restoring a second task composer.
- Shared typography, spacing, controls and surfaces are more restrained across
  all six pages. OffGrid-owned startup, recovery and native access surfaces use
  matching compact proportions without changing trusted consent boundaries.

## Reliability fixes

- Agent task updates reserve SQLite writes before reading snapshots, avoiding
  contention caused by upgrading a read transaction during concurrent updates.
- Chat attachment answers are grounded in retained attachment content, with
  clearer response presentation. Missing evidence is not a verified outcome.
- Navigation, inspection and refresh remain observation-only. They do not submit
  work, approve access, replay speech or retry uncertain mutations.

## Verification and limits

Local verification includes the Go suite, API/type/build checks, 358 renderer
tests (10 opt-in skips), 65 desktop unit tests and documentation checks. Forty-eight
six-page responsive/theme fixtures were reviewed, alongside representative
populated, running and blocked work. The 1,000-row history fixture measured
34.3 ms p95 local feedback; this is not end-to-end model or desktop latency.

Windows startup, recovery, presentation and installed-renderer checks were also
recorded. Manual screen-reader and participant studies, the complete native-app
workflow matrix, and installed macOS/Linux permission behavior remain unverified.
No universal application-control, production-readiness or signing claim is added.
See the [reliability plan](../advanced/product-reliability-plan.md) for evidence
and remaining gates.

Publication requires passing CI for the exact source revision, all required
release assets and checksums, and CPU/GPU container verification. Build success
does not qualify every operating system, model or application workflow.

## Upgrade

Stop active work and back up workspace data before upgrading. Preserve the
existing data/model volumes when replacing the service, and install the matching
desktop package. A container update alone does not replace the desktop companion.
This UI update requires no storage migration; saved work and scoped preferences
remain intact. Previously uncertain work is not automatically resumed.

CPU image: `takuphilchan/offgrid-llm:0.4.18` (AMD64/ARM64).
NVIDIA image: `takuphilchan/offgrid-llm:0.4.18-gpu` (AMD64).
Verify release assets with `checksums-v0.4.18.sha256`.
For installer warnings, see the
[desktop trust guidance](../setup/desktop-startup.md#installer-appearance-and-os-warnings).

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.17...v0.4.18)
