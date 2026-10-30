# UI craft local installation — 2026-10-30

## Follow-up: compact controls and direct agent configuration

The user's installed-app feedback, including chat-history scrollbars, was
implemented in `008577f` and installed in both editions. Current container image:
`offgrid-llm:local-ui-compact-20261030` (`b39866e32ffc`). Current shared UI identity:
`e3b59b7d617bce87cfb3cde0d2fd704b8e837843d52a7b4be37ce9e72133dbfe`.

The final renderer run passed **444 tests, 27 skipped**. Existing API/type/build,
palette, source-size, docs and OpenSpec checks passed. New captures cover compact
options/editor layouts at 390/1440px in English/Arabic and long chat history.
The installed Windows executable passed the isolated theme, saved-preference,
nine-option and actual 200% zoom checks again. Evidence:
`build/ui-craft/compact-release-check.log`, `compact-final-focus`,
`compact-installed-theme.log`, `compact-container-install.log` and
`%TEMP%/offgrid-desktop-theme-5MJwRl`.

The source draft/authority contracts are unchanged. Fine-pointer controls are
compact; coarse-pointer targets remain at least 44px. The existing palette audit
allows only the 17 exact additive declarations reviewed for this follow-up.
The prior throttled-stream and unrun human/platform gates below remain open.

Before replacement, the service and desktop were idle. A stopped-data backup and
clone validation preceded activation. The same five models, 282 task IDs and
workspace identity were independently compared; the served UI matches the
desktop. The previous container remains stopped as
`offgrid-before-computer-1793351765`. New rollback copies are retained at
`/home/phil/offgrid-computer-backup-F6AEPTC9` and
`%LOCALAPPDATA%/OffGrid-install-backups/ui-compact-20261030-171413`.

The earlier installation record follows; its image and UI hashes are historical.

This records the user's authorized container and desktop replacement for local
testing. It is not a published release or completion of the broader usability
and platform qualification gates. Source UI revision: `bb35479` on
`ui/presentation-craft-v0421`; application version remains `0.4.21`.

## Installed identity and preservation

- Container: `offgrid-llm:local-ui-craft-20261030-bb35479`, image prefix
  `866bfda73c75`, container `bc840f2e1df5`.
- Shared UI build:
  `b0bedfb346835b54a2f24642096bd1e4e768fd44164098ae1eeb23d1f1affcd9`.
- The UI layer uses the exact prior local GPU image. Backend/runtime binaries,
  volume mappings, GPU admission, environment and loopback host port are preserved.
- Windows installation: existing OffGrid LLM Desktop, with its application
  archive and bundled UI replaced. Executor and computer pack digests passed;
  existing Electron, backend and companion binaries were retained.
- One extra Playwright build-registry file under `computer/browsers/.links`
  pointed to the source checkout and failed exact pack inventory validation.
  All manifest-listed file digests matched. The extra file was moved into the
  installation backup; the unchanged pack then passed its normal verifier.
- Idle checks found no active/queued inference, running/queued tasks, connected
  execution workspaces or computer sessions before replacement. Saved interrupted,
  uncertain and waiting tasks were left untouched.
- The stopped workspace was copied, archived and checksum-verified. A clone was
  started and checked before live activation. The previous container remains
  stopped as `offgrid-before-computer-1793349406`.
- Workspace identity, all **5 models** and **282 task IDs** matched before/after.
  The post-install session endpoint returned **78 conversations**; its identity
  set was not captured pre-install, so no independent before/after conversation
  comparison is claimed. Exactly one OffGrid service container is running.

Private rollback locations:

- WSL: `/home/phil/offgrid-computer-backup-xExvwXB7`.
- Windows: `%LOCALAPPDATA%/OffGrid-install-backups/ui-craft-20261030-163447`.

Preserve current data before any rollback. The previous image and its matching
workspace backup must stay paired; do not replay uncertain work.

## Verification

Production API/type/build, palette freeze exceptions, generator, source-size,
documentation and OpenSpec checks passed. The renderer suite passed **430** tests
with **27** skipped; the separate final visual matrix passed **16** cases. Desktop
unit tests passed **85**, with one skip. The subsequent locale/startup-focused run
passed **20** cases. Full Go/race, macOS and Linux installed checks were not rerun
for this presentation-only change; this is not an assertion of full CI readiness.

The actual updated Windows executable passed:

```powershell
node scripts/test/test-desktop-startup.mjs "$env:LOCALAPPDATA/Programs/OffGrid LLM Desktop/OffGrid LLM Desktop.exe"
node scripts/test/test-desktop-theme.mjs "$env:LOCALAPPDATA/Programs/OffGrid LLM Desktop/OffGrid LLM Desktop.exe"
```

These use isolated profiles and HTTP fixtures. Startup first-window time was
1,770 ms, recovery 2,037 ms. Theme checks cover saved/live light, dark and system
appearance, nine readable options and actual Electron 200% zoom on seven routes.
Representative native screenshots were inspected. This qualifies those tested
Windows presentation paths, not native computer actions or screen-reader use.

Live `/health`, `/readyz`, system, model, session and access endpoints passed.
The served UI identity matched the installed desktop UI. A bounded Qwen 7B
request returned `OK` in **17.79 seconds**, including cold preparation; no tools
or saved agent turn were created. Startup/inference logs contained no fatal or
inference error. The existing authentication-disabled warning remains because
the service binds `0.0.0.0` inside Docker; host publication remains restricted
to `127.0.0.1:11611`. Authentication policy was not changed.

The first image-build attempt failed during registry lookup before stopping the
old service. Retrying against an explicitly tagged existing local image with the
legacy builder succeeded. The legacy-builder deprecation and Vite chunk-size
advisories remain. Temporary connection resets during startup polling resolved
before readiness checks passed.

Private logs and screenshots are under `build/ui-craft` and the isolated
`%TEMP%/offgrid-desktop-startup-OgAwCA` and
`%TEMP%/offgrid-desktop-theme-XBUb4g` directories.

## Remaining limits

The [gradient evidence](../../ux/gradients.md) retains three failed 250 ms Stop
assertions under 4x/6x CPU throttling, including flat mode. This is an unresolved
heavy-stream responsiveness limit. Native translation review, screen-reader
testing, physical-display banding review, participant usability testing and
unavailable installed operating systems remain unverified. No publication or
release qualification is claimed by this installation.
