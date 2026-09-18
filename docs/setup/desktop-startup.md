# Desktop installation, startup, and recovery

The Windows, macOS, and Linux desktop packages use the same startup controller
and the same product UI. Installation and operating-system trust are separate
from connecting to an OffGrid service.

## Opening a workspace

The desktop opens a lightweight, monochrome window before checking the service.
It shows the desktop version, detected service version, local address and actual
connection state. It does not show an invented installation percentage.

The compatibility check requires matching product versions, supported API
contracts and UI build identity. A healthy older service is not necessarily
compatible with a newer desktop. For example, desktop `0.4.4` cannot attach to
service `0.4.3-history-dev`. Reinstalling the same desktop will not update that
Docker container. Even equal version numbers can contain different UI builds.

When the versions differ, choose one of these actions:

- **Open existing web workspace:** continue using the identified service in your
  normal browser, with its existing models and history. This does not bypass the
  desktop's compatibility check or stop the service.
- **Retry connection:** after explicitly backing up and updating the external
  service, check it again. Repeated clicks never start duplicate child processes.
- **Use a separate desktop workspace:** start the bundled matching service on
  another loopback port. This is a different workspace with separate models and
  history; it does not import, upgrade or replace the existing workspace.

The separate-workspace choice is remembered. To change it, choose **File →
Connection on next launch** (press Alt to reveal the menu on Windows/Linux), or
use the same submenu on the tray icon. Select **Configured local service** to
return to the configured port. Changes apply after quitting and reopening the
app; they do not stop work in progress or delete either workspace.

If the configured port is free, the desktop can start its bundled service using
the normal desktop workspace. An occupied, unresponsive or unidentified port is
never treated as permission to replace that service. Startup checks are bounded;
after a timeout, retry reconnects to an already-started child rather than
spawning another one.

For a container running in WSL, also check `wsl --list --verbose` in PowerShell.
A stopped distribution cannot serve the Windows desktop. Verify the URL from
Windows as well as from inside Ubuntu: success inside WSL alone does not prove
Windows localhost forwarding works. Keep your externally managed WSL/container
environment running. Desktop startup does not silently launch, reconfigure or
upgrade it; the native bundled-workspace option does not depend on WSL.

## Where work is stored

For a desktop-managed service:

| Mode | Workspace roots |
| --- | --- |
| Normal desktop workspace | `~/.offgrid-llm/data`, `~/.offgrid-llm/models` |
| Separate desktop workspace | `~/.offgrid-llm/desktop-workspace/data`, `~/.offgrid-llm/desktop-workspace/models` |

On Windows, `~` means your user profile, normally `C:\Users\<name>`. An external
service uses its own storage settings; Docker volumes are not Windows desktop
folders. Settings identifies the connected backend. Never connect two services
to the same writable workspace directory. See [workspace backup and
recovery](../advanced/workspace-recovery.md) before upgrading or moving data.

Closing a successfully connected window keeps the app in the tray where
available. Use **Quit OffGrid** to exit. Quitting stops only a child started by
this desktop; it never stops an externally managed container/service. Failed
first launches are not silently hidden in the tray. A stopped child returns the
window to recovery rather than leaving a blank page.

For isolated testing, set `OFFGRID_DESKTOP_HOME` to an absolute, empty test
directory before launching. This also isolates Electron's profile. Set
`OFFGRID_PORT` to the external test service's port. Do not point qualification
tests at your working workspace. Product language selection remains in the
shared UI; the native startup/recovery text currently uses English.

## Installer appearance and OS warnings

Windows Setup retains native, accessible installation controls, per-user install
by default, and the option to change the install location/scope. Its branding is
monochrome and uses Segoe UI. The redundant license-acceptance page is removed;
the MIT license is still shipped inside the application resources. Native
installation details remain available: **Show details** expands the actual
installation log, including extraction, file copying and registration. Setup now
restores those log messages instead of showing an empty panel. A failure leaves
recovery guidance; it must not be read as a successful installation.

Setup declares system DPI awareness and uses 4× monochrome source artwork, native
Segoe UI text and a monochrome progress bar. It retains Windows focus indicators
and contrast handling. Moving between monitors with different scales and every
accessibility configuration still require separate qualification.

The Finish callback records whether you chose to launch; app activation runs after
the wizard closes. Setup no longer waits for application startup with a frozen
Finish window. Decompression, antivirus inspection and disk performance still
affect installation time; no artificial progress/acceleration is claimed.

Reinstalling a running app asks you to finish active work and approve a normal
close. It does not use repeated PowerShell process scans or force-kill processes.
Older versions without the quit-request handler must be exited with **File → Quit**
or **Quit OffGrid** in the tray before Retry. Silent installation exits 2 if the
app is running; it does not silently interrupt tasks. An externally managed
OffGrid service or Docker container is not stopped by this process.

### Windows SmartScreen

The blue **Windows protected your PC** / **More info** prompt belongs to Microsoft
Defender SmartScreen. It is not an OffGrid dialog or a blue-screen crash. Changing
the installer design cannot suppress it safely. Verify the downloaded installer
itself, not just an apparently successful signing step in a build log:

```powershell
Get-AuthenticodeSignature -LiteralPath 'C:\path\to\OffGrid-Setup.exe' |
  Select-Object Status, StatusMessage, SignerCertificate
```

Direct downloads need a verified signing identity and consistently signed,
timestamped executables/installers. New signed files can still receive reputation
warnings; an EV certificate is not an automatic exemption. Self-signing is not a
substitute. Microsoft's Store-distributed, Microsoft-signed packages avoid the
SmartScreen download warning; merely listing a direct-download EXE is not that
distribution model. See Microsoft's [SmartScreen guidance](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation)
and [signing options](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options).

Do not disable SmartScreen, Smart App Control, antivirus or organizational policy
to make the installer appear trustworthy. Signing requires account/identity
validation and release credentials; it cannot be manufactured by a source change.
Unsigned preview packages must be labelled accordingly.

### macOS and Linux

macOS releases need Developer ID signing and Apple notarization, including the
nested Electron helpers and bundled runtime, followed by signature/Gatekeeper
verification and stapling the notarization ticket where applicable. Ad-hoc
signing or `gatekeeperAssess: false` is not notarization. Do not remove quarantine
attributes as a substitute. See [Electron's macOS signing guidance](https://www.electronjs.org/docs/latest/tutorial/code-signing#signing--notarizing-macos-builds).

Linux has no Windows SmartScreen dialog. Keep package checksums/provenance,
executable permissions and platform dependency checks. Linux AppImage/DEB,
macOS Intel/Apple Silicon, and Windows Setup/portable remain separate packaging
targets; a passing Windows test does not qualify every other package.

## Verification and limits

`desktop/test` covers version/build identity, hostile/occupied ports, cancellation,
single-flight startup, recovery states, workspace separation and IPC trust.
`dev/scripts/test-desktop-startup.mjs` launches the real packaged Electron app
against disposable data and a local fixture, then checks the bundled Go runtime,
native workspace preference, relaunch, keyboard recovery and external-service
ownership. CI runs it on Windows, Linux and both Mac architectures.

The separate Windows installer smoke builds a test-only application identity,
installs into a temporary directory, exercises the installed app, reinstalls the
same version and uninstalls while checking its workspace fixture is preserved.
It exercises the real Finish button with launch checked/unchecked, populated Show
details, DPI awareness, silent running-app refusal, and consent-driven reinstall.
It does not replace a user's real desktop installation. See the
[desktop development guide](../../desktop/README.md) for the exact commands.

These checks are not qualification of every historical upgrade, elevated install,
SmartScreen reputation, notarization, or model performance.
The [reliability log](../advanced/product-reliability-plan.md) records
which checks actually ran and which still require native hardware or signing
identities. No existing installation is updated simply by editing this source.
