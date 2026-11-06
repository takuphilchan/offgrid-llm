# Composer, header and model-library follow-up

Source, fixture and installation evidence for the November 6 follow-up.

## Changes

- Chat offers Talk when empty and Send with typed text or attachments. Stop
  remains available during generation; Talk retains its own cancellation.
- Dictation remains separate and does not automatically send. Its orb and
  waveform follow microphone samples; transcription has distinct motion.
  Reduced-motion mode disables that motion and retains the status text.
- Rail labels dismiss on activation and Escape. Pointer clicks no longer leave
  the Chat label covering New chat; keyboard focus can still expose labels.
- CLI profile details sit directly under the selected profile, command choices
  have consistent indentation, and the session heading uses an ASCII separator.
- Header utilities use consistent icon controls with accessible names and
  shortcut titles. Work attention stays a readable, compact status control.
- Models separates Installed/Discover from category navigation. Installed
  package rows use one list grammar; search belongs beside the list heading.
  Readiness errors remain visible. Download guidance is not shown as an
  instruction to users viewing installed speech models.
- Shared controls use bounded 120 ms color/hover transitions, disabled under
  reduced motion. No motion simulates model progress or starts an operation.

## Recorded verification

- `go test ./...` passed before the subsequent presentation-only changes.
- `go test ./cmd/offgrid ./internal/cli` passed after updating session-heading
  expectations; the earlier focused run exposed the old heading assertion.
- Voice, streaming, readiness and Markdown-worker suites: 73 passed. These use
  synthetic media and API fixtures, not real microphone or synthesis evidence.
- Voice/image/shell checks exposed old assumptions that an empty composer must
  always show Send. Those were updated to assert the new draft-dependent action.
  A check also caught hiding Send during concurrent dictation of an existing
  typed draft; that behavior was restored and the recording/generation test passed.
- Model/header checks: 47 passed, seven platform/service-dependent checks
  skipped, and two selection-style expectations failed during development.
  The library now distinguishes tabs with an underline and categories with a
  selected surface; the corrected library/header run passed 21, skipped seven.
- Broader model/import checks passed 41 and skipped six, but the real-service
  import case ran against the fixture-only server and timed out. It now requires
  the explicit disposable-service URL, like other native-service tests; it has
  not been represented as real-service qualification in this pass.
- API generation check and web type/build passed. The existing large-bundle
  warning remains. Light/dark model and narrow dictation screenshots were inspected.
- Final system-presentation run: 83 passed, one skipped. This covers management
  hierarchy, agent layout, interaction contracts, model navigation/import UI and
  all-page reachability at narrow/short sizes and 200% zoom. English/Arabic and
  light/dark fixtures are included. The skipped check requires a disposable
  real service. The measured history fixture reported 37.4 ms p95 local feedback;
  that is not model-generation latency or a performance claim for other hardware.

## Limits

The final Chat/voice rerun passed 52 checks after extracting committed-output
presentation state from ChatPage. The source-size ratchet, diff whitespace and
both affected OpenSpec validations pass. The packaged Windows terminal fixture
also passed with isolated state (`offgrid-desktop-theme-T2IEzk`): interactive CLI,
agent draft and clear, cursor retention, tabs, Unicode, search/copy, 200% zoom,
reload/close/quit child cleanup and service identity. At fixture time, user
installation had not occurred; the later installation is recorded below.

Human usability, screen-reader review and macOS/Linux native qualification remain
unverified. Existing permission, draft, model identity, download confirmation and
uncertain-effect protections are unchanged. Installation evidence is recorded
separately below.

## Installation

After the user quit the app, its remaining model process exited naturally.
The installer completed with exit code zero. All five installed payload files
match the verified candidate; the installation script also verified unchanged
workspace data, model inventory and inference runtimes. No WSL CLI, global PATH
or published release was changed. This remains a local 0.4.26 preview.

- Installer SHA-256: `F1A59447E2BC5D0E22B5C62C140185571206998D2188DC1CBE698AD6E3803F7C`
- Installed UI SHA-256: `EFEA07C22048A17A92367AF8253292C12AB09BFADFD1A2084A4201E1A2ABF664`
- Rollback: `D:/OffGrid-install-backups/v0.4.26-before-system-presentation-20261106-155451`
- Result: `build/desktop-system-presentation-final-20261106/install-result.json`

The app was left closed. Isolated packaged tests do not substitute for a live
user-workspace walkthrough or human usability evaluation.
