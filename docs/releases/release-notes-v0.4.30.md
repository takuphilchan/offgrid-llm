# OffGrid 0.4.30 - Unsigned preview

This is an **unsigned preview**. Windows installers do not have a verified
publisher signature, and the macOS apps are not Developer ID signed or notarized.
Windows may show an unknown-publisher warning; macOS may block opening the app.
Checksums verify downloaded bytes, not publisher trust. The attached native trust
reports record the actual artifact status.

Version 0.4.29 remained unpublished after its container build exposed missing
shared UI assets. This version includes those assets in both Docker build
contexts and adds CI builds of their renderer stages before release tagging.

## Changes

- Setup offers explicit inference-engine preparation, reports CPU/GPU mode and
  download progress, and preserves recoverable failures. Setup completes only
  after a successful model response in that workspace.
- Network installers verify the selected release archive before extraction.
  Installer helpers retain complete runtime payloads, and CLI installations
  discover their packaged resources independently of the launch directory.
- Windows Setup refuses replacement while a separately running OffGrid service
  or inference worker remains active, and stops if process inspection fails.
  The existing desktop quit flow and saved-work preservation remain in place.
- Release automation requires passing CI for the exact tagged source, verifies
  downloaded artifacts, and publishes checksums and native trust reports. This
  preview does not promote stable download or container aliases. The default
  signed-release path continues to require publisher verification.
- Settings, appearance controls and desktop terminal interactions are refined.
  Terminal startup preserves newer host visibility and focus instructions.
  All 29 registered languages have authored text for the 1,356-message catalog;
  preview labels remain until the required human reviews are recorded.
- Delegated agent work preserves selected profile revisions and request facts,
  recovers completed work without replaying it, and presents ordered specialist
  results before the final answer. Knowledge generation rechecks source access
  and document queue admission rejects expired work.

## Verification and limits

The preceding 0.4.29 candidate passed all 13 CI jobs, including Go/race checks,
API and UI checks, 1,574 browser tests with 32 skips, packaged checks on Windows,
Linux and both macOS architectures, and Windows install/repair/uninstall checks.
Its installed Windows upgrade preserved checked workspace state and produced a
real Qwen2.5 3B reply with NVIDIA inference using isolated test data. Saved reply
restart and image-draft preservation passed; unsupported vision was refused.
These observations apply to that candidate, not an untested artifact.

Publication of 0.4.30 additionally requires CI for its exact source, both Docker
renderer-context builds, complete native assets, container qualification and
public checksum verification. The web build retains its existing large-chunk
advisory. Publisher signing/notarization, native-language review, successful CPU
model inference, vision inference and broader hardware qualification are not
implied by these checks.

See the [distribution evidence](../../openspec/changes/distribution-first-run-readiness/evidence.md)
and [publisher credential requirements](../development/distribution-builds.md).

## Downloads and upgrades

Windows desktop users should choose `OffGrid-0.4.30-windows-x64-setup.exe` and
verify it against the release checksum manifest. Choose other packages using
the [platform guide](../setup/installation.md). Finish active tasks, quit OffGrid
and stop independently managed OffGrid services before upgrading. Existing
models, conversations and workspace locations remain compatible. Docker and
WSL deployments are separate from the desktop installation.
