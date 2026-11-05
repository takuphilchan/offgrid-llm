# OffGrid LLM v0.4.26

This release improves everyday setup, source inspection and workspace navigation.
Existing conversations, models, profiles, grants and workspace data are preserved.

## Changes

- Document answers expose readable source controls for validated retained
  passages. Opening a reference checks its saved source identity and revision;
  ambiguous or unavailable sources do not silently open a different document.
- Navigation groups work and library destinations. Activity separates work
  needing attention from completed history, and retained downloads take less
  space above installed models. The interface explains the distinction between
  conversation attachments and the reusable document library.
- Model selection distinguishes selected, runtime-ready and runtime-unavailable
  models. A known unavailable runtime cannot be selected as usable. Unknown
  readiness from an older service is not presented as qualification.
- First-use setup offers an explicit offline model-file path on compatible
  services. Administrators can import one complete GGUF file after reviewing
  its name and size. Imports preserve existing files and model selection across
  refresh and reload. Selecting a usable model leads back to the retained Chat
  draft without sending it automatically.
- Secondary screens load on demand, with shared control styles available to
  Chat immediately. Model-picker focus stays inside the scrolling surface.
  Floating controls and setup dialogs keep text opaque during entrance motion.
- The Windows desktop explains its existing hide-to-tray behavior with a quiet
  notification on the first close per launch. Use Quit to stop the app and its
  owned processes; closing a window continues to preserve active work.
- CLI first-use help leads with installed models, conversation and readiness.
  The full help catalog retains every registered command family and aliases.
- Windows installer checks identify mismatched bundled service versions before
  startup and retain more detail when the Finish window does not close promptly.

## Verification and limits

The pre-release renderer run passed 1,038 tests with 31 documented skips. The
subsequent overlay/material/setup checks passed 32 tests, and the desktop unit
suite passed 129 with one platform skip. A separate throttled rendering fixture
passed. Go, API/type/build, documentation and source-size checks have passed on
the contributing changes. Exact-release-source CI and packaged artifact checks
must pass before publication.

Offline import checks exact bytes, a digest and bounded GGUF metadata. It is not
tensor validation, runtime qualification or a model-quality test. The single-file
flow does not accept split models or vision projectors; complete supported vision
bundles retain their separate acquisition path. No model or companion is
downloaded automatically. Imports go to the connected service, which may be on
another computer; a lost response requires inspection before another attempt.

Automated checks do not establish broad document-answer accuracy, interpretation
of unread PDF figures, enterprise retention qualification, human usability,
manual screen-reader/translation quality or every native hardware combination.
The Windows tray notice may be suppressed by OS notification settings. Existing
experimental capability boundaries remain in effect.

## Upgrade

Finish active work and quit OffGrid before installing matching desktop and
service builds. Back up the workspace and preserve its model/data directories.
This update does not clear history, migrate Docker data or change service
ownership. Older services remain usable without advertising the new import API.
