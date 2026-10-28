# OffGrid desktop

The desktop application is a secure Electron host for the same React UI and Go
runtime used by the browser edition. It does not maintain a second frontend.

The current task UI displays versioned outcome checks from the canonical job,
including partial and unavailable verification. Saved-control discovery uses the
same service implementation for UIA, Accessibility and AT-SPI observations; it
does not enable additional capture/input or whole-desktop capabilities on those
platforms. Matching newer hosts supply evidence-only control identities. Older
hosts keep their existing permissions and report unavailable cross-observation
checks. Full native workflows still require separate installed-platform evidence
in the [quality evidence](../../docs/quality/README.md).

## Development

### Native agent workspace access (preview)

With a matching service advertising `native-execution-preview-v1`, Chat can select
an enrolled host or ask main to choose a local folder. The only renderer input is
the read/edit/trusted access enum. Native main chooses the path and obtains local
confirmation, checks the current account/workspace, then requests a short-lived
pairing ticket through its authenticated session. Only that ticket goes over
private framed stdin to the digest-verified executor supervisor. User cookies and
API credentials are not forwarded. The supervisor creates private grants/state
using native ownership/ACL checks and enrolls the same worker used by the CLI.

Trusted commands use the signed-in OS account and are not sandboxed. This is
separate from computer-use consent. Main/preload expose no generic spawn, command,
path or service method. Stop is available from the Workspace control and native
File/tray menus even without model progress. Lock/suspend, account/workspace change,
renderer failure and quit revoke owned access. A stop timeout is uncertain, not
confirmed termination. Restart cannot restore a saved permission.

Services advertising `execution-access-request-v1` let conversations start
without a folder. When local work is requested, Chat offers attachment to that
same waiting job. The native folder review has an unchecked **Remember** option
for that exact folder identity, account, host, service installation and permission
level. It uses OS-protected encrypted storage outside the approved folder; an
unavailable or plaintext-fallback keyring leaves session-only approval available.
Remembered trust creates a fresh checked host session on use, not a restored
grant. It is separate from desktop Full access. **Manage remembered folders** in
File/tray lets the user inspect and forget entries; forgetting affects future
approval, while **Stop workspace access** revokes an already connected host.

The build hook stages a matching `execution-runtime` pack and verifies the copied
binary/version/platform/digest after packaging. For source development, build it
with `go run ./tools/executor-pack --output ABSOLUTE_BUILD_DIRECTORY --version VERSION`
at the repository root; use `build/execution-runtime/win-x64`, `mac-x64`,
`mac-arm64`, or `linux-x64` as appropriate. This builds local assets only; it does
not install or replace the desktop app. Native dialog usability, signed packages
and model workflows still need separate installed-platform qualification.

Build the UI first:

```bash
cd apps/web
npm ci
npm run api:check
npm run build
```

Then make an OffGrid server available at `127.0.0.1:11611`. It can be the local
development binary or the Docker container. From the repository root:

```bash
go build -trimpath -ldflags "-X main.Version=0.4.19" -o build/linux/offgrid ./cmd/offgrid

cd apps/desktop
npm ci
npm run dev
```

Use `build/windows/offgrid.exe` on Windows and an architecture-specific binary
under `build/macos` on macOS. Set `OFFGRID_PORT` before starting Electron to use
another local port. Invalid port values fall back to `11611`.

Electron checks `/api/v2/system` before attaching: product, API, and the served
renderer/desktop bridge contract must agree. Compatible external services need
not have the same release number or UI fingerprint. An owned bundled child must
also match the exact package version and UI build. A 200 health response alone
is not enough. Rebuild the bundled UI/runtime together with the version in
`apps/desktop/package.json`. The startup window offers retry, opening
an identified existing workspace in the browser, or explicitly starting a separate
desktop workspace. An occupied incompatible or unresponsive service is left
untouched. See [desktop startup and recovery](../../docs/setup/desktop-startup.md).

The renderer build produces `desktop-compatibility.json`; the after-pack hook
verifies the actual copied UI. Protocol 1 describes the existing bounded
main/preload bridge. Keep old protocol tests when retaining compatibility, and
change the protocol when argument, result, or permission semantics break.
Only exact reviewed legacy identities in `legacy-compatibility.json` can omit
this declaration. Invalid declarations never fall back to that table.

## Packaging

### Windows task access

Chat's computer-access control proposes whole-desktop scope and an approval policy.
Selecting **Full task access** requires one explicit local confirmation and remembers
authorization only for the live desktop session, service/workspace and actor.
Subsequent tasks get distinct bounded grants without another consent popup.
For a live Chat submission with desktop scope, both an initial desktop request
and a later model-requested application attach through this same session path;
valid Full access does not require another Allow click. Reopening saved work or
recovering a lost session remains explicit and never silently replays actions.
Quit, lock/suspend, account/workspace change, or Stop/revocation clear it. Stored
UI defaults and page reloads do not create permission. Older per-task and limited
window clients remain compatible without broadening their grants.
The policy row is the authorization entry point, not a checked default followed
by a separate Enable button. It is checked only after the host confirms access;
denial or revocation leaves it unchecked. The in-app Stop and narrower policy
selection revoke it.

Windows consent uses an owned native OffGrid dialog rather than the generic
Yes/No MessageBox. The session confirmation starts at 440 × 330 logical pixels,
clamped to the available work area. Scope, task limits, restricted-action summary
and revocation remain visible; **Details** expands the full exclusions and workspace
identity. Segoe UI typography and a single solid rounded keyboard-focus border
replace stock-font/dotted button styling. Its monochrome action row stays visible
while expanded details scroll, including on short/high-DPI screens. **Enable access / Cancel**
are explicit; initial Enter, Escape, and closing the window deny access. Keyboard
users can Tab to Enable access and activate it intentionally. The dialog follows
the Windows app light/dark preference and system high-contrast colors (not the
renderer theme override); native consent text remains English. A renderer or
service message still cannot supply an approval flag. Rebuild the native worker
and matching desktop package to deliver this UI; a container-only update cannot.

Desktop-owned Windows workers no longer show an extra floating Stop window.
The persistent app control, File menu and tray stop locally without waiting for
the service; the independent native emergency shortcut remains registered.
Renderer crash/unresponsiveness revokes control. Keep the worker, private
companion transport, preload/main bridge and renderer packaged together—updating
only the service container cannot change native consent/Stop behavior.

### Build artifacts

```bash
cd apps/desktop
npm ci --prefix ../computer --ignore-scripts
npm run build:win
npm run build:mac
npm run build:linux
```

Build on the target operating system. Release automation supplies the matching
runtime and the prebuilt `apps/web/dist` bundle, then creates:

- Windows x64 NSIS and portable packages;
- macOS x64 and ARM64 zip packages;
- Linux x64 AppImage and Debian packages.

Packaging bundles pinned Playwright and its matching Chromium build. The first
build downloads that browser; end users do not need Node, npm, or terminal
pairing in the matching desktop application. In Chat,
describe the task and choose its computer-access policy. Task-wide access on a
supported host does not require choosing an application first. The optional
selected-window/managed-browser flow remains a narrower access mode; it is not
the retired setup page. Web users hand off to the matching installed desktop
application; a container alone cannot supply a host runtime.

Page addresses retain paths, queries and fragments within the selected site.
For a trusted VPN using fake DNS, Network settings offers explicit trusted-VPN
routing with a local confirmation. Direct routing remains the default; failures
never silently broaden network access. TLS checks remain enabled, and redirects
and resources outside the selected origin stay blocked. Host dispatch results
are journaled before acknowledgment so a lost reply cannot repeat an input.

Native selected-window control is wired through the matching host pack for
structured observation, text replacement, explicit checkbox state, control
activation and a fixed set of approved application shortcuts. Browser file
transfers are bounded and verified. Browser viewport vision is exposed only
after the installed model/projector passes a synthetic image and governed
tool-call check in the current service session.
This is not yet qualified general desktop or vision control; package manifests
remain explicit about that boundary.
The browser uses a separate profile, verifies its runtime manifest before starting,
and requires exact-action approval for changes. Stop is available in the workspace,
File menu, and tray. If local shutdown cannot be confirmed, the app reports that
instead of claiming success. See [browser setup and limitations](../computer/README.md).

Windows uses a per-user install by default so administrator access is not
required. `npm run installer:branding` regenerates the checked-in monochrome NSIS
artwork at 4× source resolution. Setup declares system DPI awareness, uses native
Segoe UI controls, and retains scope/directory choices;
the MIT license is included in resources without a redundant acceptance page.
Code signing and notarization require verified release identities; successful
packaging alone does not mean a package is signed. See the recovery guide for
SmartScreen, Gatekeeper and verification limitations.

## Runtime behavior

- The application keeps models in `~/.offgrid-llm/models` and all other state in
  `~/.offgrid-llm/data`.
- The optional separate workspace uses `~/.offgrid-llm/desktop-workspace` instead.
  It is an explicit session-only recovery choice. The next launch reconnects to
  the configured service; neither choice deletes data.
- Closing a successfully connected window keeps the app in the tray on Windows
  and Linux when available; **Quit** stops the runtime owned by Electron. Failed
  launches remain visible and can be closed normally.
- If a compatible Docker or other local service owns the configured port, Electron
  connects without stopping it on exit. Settings shows the active backend and UI
  build. Desktop-local paths are shown only for a desktop-managed runtime.
- Window geometry is persisted in `~/.offgrid-llm/window-state.json`.
- The startup screen renders before probing/spawning the service. One bounded,
  cancellable controller owns startup; renderer reads use cached state rather than
  launching another probe loop. Window-state writes are asynchronous and debounced.
- Startup/recovery uses the monochrome system, dark/reduced-motion support,
  keyboard controls and safe errors. Startup and custom menus share the nine
  interface locales; native OS menu roles follow platform localization.

## Security boundary

The renderer has Node integration disabled, context isolation and Chromium
sandboxing enabled, and only a small preload API. New windows are denied;
trusted HTTPS links open in the operating-system browser. Do not add generic
filesystem or command-execution functions to the preload bridge.
Navigation checks compare exact origins, not URL prefixes. IPC is accepted only
from this window's trusted main frame, never a subframe or another window.
Compatibility metadata is not a substitute for application signing.
The main process also remembers successful external ownership per local endpoint
under `desktop-backends` in the desktop configuration directory. Relaunch while
that endpoint is offline stays in recovery rather than spawning a bundled
workspace. This local record grants no access and never replaces the live
compatibility handshake. Corrupt/unwritable records fail closed; explicitly
selected isolated recovery does not overwrite the external ownership hint.

## Validation

```bash
node --check main.js
node --check preload.js
npm test
npx electron-builder --dir --linux
xvfb-run -a node ../../scripts/test/test-desktop-startup.mjs dist/linux-unpacked/offgrid-llm-desktop
xvfb-run -a node ../../scripts/test/test-packaged-browser.mjs dist/linux-unpacked/offgrid-llm-desktop
```

CI also builds the React application and its generated API types before the
Electron package, preventing stale or missing UI assets from shipping.
The Windows job also runs `test-desktop-access.mjs` against that package: real
preload/main access status, no consent from defaults, and keyboard scrolling at
100%/200% Electron zoom. It uses a disposable fixture/profile and never enables
control or submits a task.
The Node tests exercise handshake/timeout/URL/IPC and lifecycle policy. CI also
launches native unpacked Electron on Windows, Linux, macOS Intel and Apple Silicon
against isolated fixture services and the bundled Go binary. On Windows, pass
`dist/win-unpacked/OffGrid LLM Desktop.exe` to the smoke script. On Mac, pass the
executable inside `dist/mac[-arm64]/OffGrid LLM Desktop.app/Contents/MacOS`.
The script creates its own temporary profile, never modifies the installed app,
and leaves screenshots/test data as evidence. It does not perform installation,
uninstallation, signing, notarization, upgrades or real-model qualification.

On Windows, the separate installer smoke uses its own application identity and
temporary installation path, with shortcuts/elevation disabled. The test explicitly
exercises the interactive Finish checkbox as well as silent installation:

```powershell
cd apps/desktop
npx electron-builder --config installer-test.cjs --win nsis --x64 --publish never
cd ../..
$version = (Get-Content apps/desktop/package.json | ConvertFrom-Json).version
./scripts/release/test-windows-installer.ps1 -InstallerPath "build/windows-installer-smoke/OffGrid Desktop Install Test-Setup-$version.exe"
```

It checks clean installation, real installed-app startup, Show details with real
log entries, DPI awareness, Finish closing within two seconds, both launch checkbox
states, refusal to silently replace a running app, consent-driven same-version
reinstall, uninstall and workspace-fixture preservation. It refuses normal release installers
and existing test registrations. It removes only its own test application and
retains evidence; it does not qualify all historical upgrades or elevated installs.
The native wizard test needs an interactive Windows desktop. Its screenshot capture
is limited to the test window; it never captures the whole desktop. Clear the
development-only `ELECTRON_RUN_AS_NODE` variable before launching an installed
Electron application; the test runner does this for its Finish-launch check.

The installer watchdog records timestamped probes and Win32 errors, and counts
only consecutive failed observations toward its five-second responsiveness gate.
Long gaps in monitoring reset that streak; the overall 180-second installer
deadline still applies. Screenshots run in owned helpers with a five-second
deadline, so a blocked `PrintWindow` cannot block monitoring. Failed captures
are reported as missing diagnostics, not successful screenshots. The separate
`scripts/release/test-windows-installer-probe.ps1` regression suite checks timing gaps,
recovery, real frozen/healthy fixture windows, and bounded capture. On failure,
a separate bounded helper records the isolated installer's thread wait chains
and control states, without collecting process memory or named lock objects.
CI preserves
installer `diagnostics/` and probe evidence on failure, excluding installer
binaries and installed application directories.
