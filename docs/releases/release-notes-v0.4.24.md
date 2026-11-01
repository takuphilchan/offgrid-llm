# OffGrid LLM v0.4.24

This maintenance release simplifies Chat configuration and activity navigation,
adds in-app Help, and corrects desktop shutdown and several layout problems.

## Changes

- Agent settings brings the pinned model, profile revision, decision budget and
  existing resource restrictions into one panel. Services supporting on-demand
  access request a folder when needed; active access remains visible. Existing
  profiles and permissions are not expanded.
- Help is available from the header and Quick actions, with examples, setup
  guidance and links for returning to saved work. It preserves drafts, supports
  keyboard dismissal and restores focus.
- Response settings replaces the ambiguous Context & response label. Context
  allocation and response-token limits sit under Advanced, with current values
  summarized. Knowledge and voice controls remain available; defaults do not
  change merely by opening the panel.
- Model pickers have one scrolling list, fixed search controls, readable rows
  and stable placement in short windows. Model Verify/Delete menus and history
  actions use compact rows instead of oversized button panels.
- Activity uses compact task links with secondary status, a bounded attention
  menu and visible cleanup for removable history. Long task titles and Show more
  no longer dominate the page. Existing protected-deletion rules still apply.
- Blocked conversations explain the required next action and link to the
  canonical task. Starting a new chat does not retry or change uncertain work.
  Cancelled-task presentation no longer displays a stray zero or oversized
  continuation controls.
- Folder setup has one access label and concise native-desktop guidance.
  Confirmed undersized labels use shared typography tokens. Help and management
  surfaces support narrow layouts, RTL and 200% zoom in automated checks.
- Desktop shutdown preserves cleanup ownership and allows retry when an owned
  process does not stop successfully, rather than silently abandoning cleanup.

## Verification and limits

The local renderer run passed 724 tests, skipped 28 and identified 12 tests tied
to the former menu placement or translated label. After updating those test
expectations, all 51 tests in their three files passed; application source did
not change between those runs. API/type/build, documentation, source-size,
repository hygiene and OpenSpec checks passed. Packaged Windows presentation
checks covered native dropdowns, all seven routes at 200% zoom, Help, appearance
persistence and restart. The [completion audit](../quality/evidence/workspace-completion-audit-2026-11-01.md)
retains earlier failures and installation evidence. Release packaging requires
passing CI for the exact tagged source revision.

Unknown-outcome tasks are still protected from deletion. This release does not
claim that an interrupted external action succeeded or was undone. A separate
explicit discard-history change remains pending. Existing experimental model,
image and agent capabilities retain their gates. Human usability participation,
manual screen-reader/native-speaker review and full native platform/model
qualification remain open; automated layout checks do not replace them.

## Upgrade

Finish or stop active work and quit the desktop app before installing matching
service and desktop components. Back up the workspace and retain models and
configuration. This update does not clear existing conversations or activity,
grant access, download models, change the selected service or migrate a Docker
workspace. Local cleanup performed at the owner's request is not an upgrade
side effect.
