# OffGrid LLM v0.4.11

This update adds a supervised browser-task preview, packages its browser runtime
with the desktop application, and improves agent persistence and chat recovery.

## Changes

- Start browser assistance from Agents in the matching desktop application,
  with a local consent prompt and a separate browser profile. End users do not
  need Node, npm, or terminal pairing for this desktop path.
- Inspect pages, navigate within one permitted origin, fill text, select native
  dropdown options, set checkboxes, click, and verify results using typed tools.
  Changes require exact-action approval; stale observations are rejected.
- Show readable actions and approval summaries in the shared web/desktop UI.
  Stop attempts local shutdown and service revocation independently. An
  unconfirmed local shutdown is reported honestly and blocks another session.
- Persist agent tasks, checkpoints, approvals and events transactionally. Legacy
  imports validate records and retain originals rather than silently discarding
  malformed tasks or dual-writing the old store.
- Preserve chat selection and unsent drafts when older history requests finish
  late. Regression tests cover the production renderer as well as development.

## Upgrade and first use

Back up the workspace and quit OffGrid before updating. Keep the desktop and
service versions together; an externally managed Docker/WSL service must be
updated separately. Models do not need to be deleted or downloaded again.

The desktop package is larger because it includes pinned Chromium. Start with
the practice page in Agents, approve local consent, then describe the desired
result. Browser tasks need a model/runtime that passes the tool-call preflight;
that check is not a guarantee of task success. Inspect the proposed actions.

Web/container users still need the matching host desktop or companion: the
container does not control the host by itself. See [browser setup and limits](../../computer/README.md).

## Preview boundaries

This is **browser control, not general native desktop or vision control**.
One permitted origin, bounded sessions, and supervised changes remain enforced.
Personal browser profiles, credential automation, arbitrary downloads, native
applications, custom widgets and multi-select controls are not supported by this
preview. VPN fake-DNS destinations remain blocked; use the isolated practice
page when a public origin cannot safely be verified.

Local Windows packaged browser/startup checks and the documented Go, UI,
companion and desktop suites passed. Full platform/model qualification, signing,
optional offline automation packs and the broader computer-use program remain
incomplete. This release does not certify general computer-use reliability or
eliminate Windows/macOS reputation warnings.

CPU container: `takuphilchan/offgrid-llm:0.4.11` (AMD64/ARM64). NVIDIA container:
`takuphilchan/offgrid-llm:0.4.11-gpu` (AMD64). Verify release downloads against
`checksums-v0.4.11.sha256`. Read the [reliability evidence](../advanced/product-reliability-plan.md)
for qualification boundaries and migration details.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.10...v0.4.11)
