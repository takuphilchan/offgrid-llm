# OffGrid 0.4.29 - Unsigned preview

This is an **unsigned preview**. Windows installers do not have a verified
publisher signature, and the macOS apps are not Developer ID signed or notarized.
Windows may show an unknown-publisher warning; macOS may block opening the app.
Checksums verify downloaded bytes, not publisher trust. This release does not
claim a warning-free first installation or signed-release qualification.

## Changes

- Setup offers explicit inference-engine preparation, reports CPU/GPU mode and
  download progress, and preserves recoverable first-run failures. First-answer
  setup completes only after a successful model response in that workspace.
- Network installers verify the selected release archive before extraction.
  Installer helpers retain complete runtime payloads, and CLI installations
  discover their packaged resources independently of the launch directory.
- Windows Setup refuses replacement while a separately running OffGrid service
  or inference worker remains active, and stops if process inspection fails.
  The existing desktop quit flow and saved-work preservation remain in place.
- Release automation checks actual downloaded artifacts and records native
  trust results. This explicitly selected preview retains CI and integrity checks,
  publishes trust reports, and does not promote stable download/container aliases.
  The default signed-release path still requires publisher verification.
- Settings, appearance controls and desktop terminal interactions are refined.
  All 29 registered languages have authored text for the 1,356-message catalog;
  preview language labels remain until the required human reviews are recorded.
  This does not imply translation of arbitrary CLI output, OS dialogs, custom
  service messages or user content.
- Delegated agent work preserves selected profile revisions and request facts,
  recovers completed work without replaying it, and presents ordered specialist
  results before the final answer. Knowledge generation rechecks source access
  and document queue admission rejects expired work.

## Verification and limits

The combined candidate passed the full Go suite, API contract validation,
TypeScript and production web build, translation structure/completeness/
consistency checks, and 202 desktop tests with one platform skip. Translation
regression tests passed 11 cases; installer/trust/preview fixtures passed 15.
The web build retains its existing large-chunk advisory.

After correcting document-worker packaging in the test launch and outdated test
assumptions, a subsequent full CI browser run passed 1,574 tests with 32 skips.
Linux desktop tests passed 203, and both macOS native CI jobs passed. Each CI
result applies to its recorded revision; publication also requires a passing
full CI run for the exact tagged source.

The installed Windows upgrade matched all 1,186 package files. The 576 workspace
files retained metadata, and files at most 16 MiB retained checked hashes.
Using the installed executable with isolated test data, NVIDIA engine preparation,
a real Qwen2.5 3B reply, saved-response restart and image-draft preservation passed.
Unsupported vision was refused; this is not successful vision inference evidence.
The rollback installer remains available locally.

See the [distribution evidence](../../openspec/changes/distribution-first-run-readiness/evidence.md)
and [publisher credential requirements](../development/distribution-builds.md).
The local installer is unsigned and was built before the final commits.
Exact-revision CI, container qualification and public checksum verification
remain required for this preview. Signing and notarization are unavailable;
the attached trust reports record that limitation. Native-language
review, successful CPU-model inference and broader native-platform/hardware
qualification are not implied by the local Windows checks.

## Downloads and upgrades

Windows desktop users should choose
`OffGrid-0.4.29-windows-x64-setup.exe` and verify it against the release checksum
manifest. Choose other packages using the [platform guide](../setup/installation.md).
Finish active tasks, quit OffGrid and stop independently managed OffGrid services
before upgrading. Existing models, conversations and workspace locations remain
compatible. Docker and WSL deployments are separate from the desktop installation.
