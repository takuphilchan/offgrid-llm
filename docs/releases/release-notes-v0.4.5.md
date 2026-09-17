# OffGrid LLM v0.4.5

This patch improves desktop installation and startup recovery across Windows,
macOS and Linux. It does not replace the shared product UI or change workspace
storage formats. The v0.4.4 release and its published artifacts are unchanged.

## Fixes

- The desktop window opens before service discovery. One bounded, cancellable
  controller replaces duplicate readiness loops and blocking error dialogs.
- A mismatched or unavailable service now has visible recovery choices: retry,
  open an identified existing workspace in the browser, or explicitly start a
  separate desktop workspace with its own models/history and local port.
- Separate-workspace selection survives restart. **File → Connection on next
  launch** and the tray menu let you switch back without interrupting current
  work. External services and containers are never stopped or replaced by Electron.
- Release-built services now report `0.4.5` rather than the Git tag label
  `v0.4.5`, matching Electron's package identity. This fixes a release-only false
  mismatch. Actual version, API capability and UI-build checks remain strict.
- Missing runtime files, child exits and timeouts show actionable recovery.
  Startup-only IPC remains restricted to the trusted local startup page.
- Window-state writes are asynchronous/debounced. Windows service launches hide
  the console window. Dark mode, reduced motion, keyboard focus and narrow-window
  recovery receive regression coverage.
- Windows Setup uses monochrome artwork and Segoe UI while retaining native
  installation location/scope controls. The redundant MIT acceptance page is
  removed; the license still ships inside the application resources.

## Upgrade and existing workspaces

Back up the stopped workspace before upgrading and preserve its model volumes,
configuration and credentials. Never run two service versions against the same
writable data directory. Upgrade the desktop and external service together if
you want to keep using that service inside Electron. Reinstalling the desktop
does not update Docker or WSL.

The separate-desktop option is a different workspace, not an automatic migration.
It leaves the existing workspace untouched and requires models to be installed
separately. To keep using an older service without upgrading yet, choose **Open
existing web workspace**. For WSL containers, verify that Ubuntu is running and
that the service is reachable from Windows, not just from inside Ubuntu.

See the [startup and recovery guide](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.5/docs/setup/desktop-startup.md).

## Packages and trust limitations

Existing Windows x64, macOS Intel/Apple Silicon, Linux x64 desktop and x64/ARM64
CLI targets remain. Existing CPU, Vulkan and Metal runtime variants retain their
platform-specific limitations. CPU containers use
`takuphilchan/offgrid-llm:0.4.5`; NVIDIA Linux AMD64 uses
`takuphilchan/offgrid-llm:0.4.5-gpu`. Pulling an image does not replace an existing
container. See the [Docker guide](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.5/docs/setup/docker.md)
for volume-preserving deployment.

Verify downloads with `checksums-v0.4.5.sha256`. Desktop packages remain
unsigned/not notarized previews: Windows SmartScreen and macOS Gatekeeper warnings
are not fixed by installer styling. Verified signing identities and Apple
notarization remain external requirements. Do not disable OS protections.
Signing itself does not promise immediate SmartScreen reputation.

## Verification and remaining limits

Local validation passed the full Windows Go suite, Linux CLI/version tests,
15 desktop Node tests on both platforms, UI type/contract checks, and workflow
lint. An isolated Windows installer identity passed clean install, real installed
Electron/Go startup, same-version reinstall, uninstall and workspace-fixture
preservation. The user's actual application and container were not replaced.

CI now checks real packaged Electron startup on Windows, Linux and both Mac
architectures, including mismatches, hung ports, workspace relaunch and IPC
denial. It also runs the isolated Windows installer smoke. Native job results,
not successful packaging alone, determine those checks' outcomes.

This is an incremental reliability patch, not full production certification.
Interactive installer accessibility, all historical/elevated upgrade paths,
native-shell localization (currently English), signing/notarization, offline
installation and broader performance/hardware qualification remain outstanding.
No measured installation-speed or inference-speed improvement is claimed.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.4...v0.4.5)
