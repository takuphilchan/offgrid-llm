# OffGrid desktop

The desktop application is a secure Electron host for the same React UI and Go
runtime used by the browser edition. It does not maintain a second frontend.

## Development

Build the UI first:

```bash
cd web/app
npm ci
npm run api:check
npm run build
```

Then make an OffGrid server available at `127.0.0.1:11611`. It can be the local
development binary or the Docker container. From the repository root:

```bash
go build -trimpath -ldflags "-X main.Version=0.4.4" -o build/linux/offgrid ./cmd/offgrid

cd desktop
npm ci
npm run dev
```

Use `build/windows/offgrid.exe` on Windows and an architecture-specific binary
under `build/macos` on macOS. Set `OFFGRID_PORT` before starting Electron to use
another local port. Invalid port values fall back to `11611`.

Electron checks `/api/v2/system` before attaching: product, API, supported
contracts, matching product version, and UI build identity must agree. A 200
health response alone is not enough. Rebuild the UI/runtime together with the
version in `desktop/package.json`. The startup window now offers retry, opening
an identified existing workspace in the browser, or explicitly starting a separate
desktop workspace. An occupied incompatible or unresponsive service is left
untouched. See [desktop startup and recovery](../docs/setup/desktop-startup.md).

## Packaging

```bash
cd desktop
npm run build:win
npm run build:mac
npm run build:linux
```

Build on the target operating system. Release automation supplies the matching
runtime and the prebuilt `web/dist` bundle, then creates:

- Windows x64 NSIS and portable packages;
- macOS x64 and ARM64 zip packages;
- Linux x64 AppImage and Debian packages.

Windows uses a per-user install by default so administrator access is not
required. `npm run installer:branding` regenerates the checked-in monochrome NSIS
artwork. Setup uses native Segoe UI controls and retains scope/directory choices;
the MIT license is included in resources without a redundant acceptance page.
Code signing and notarization require verified release identities; successful
packaging alone does not mean a package is signed. See the recovery guide for
SmartScreen, Gatekeeper and verification limitations.

## Runtime behavior

- The application keeps models in `~/.offgrid-llm/models` and all other state in
  `~/.offgrid-llm/data`.
- The optional separate workspace uses `~/.offgrid-llm/desktop-workspace` instead.
  Change the remembered choice using **File → Connection on next launch** or the
  tray menu. Neither choice deletes data; it takes effect after quitting/reopening.
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
  keyboard controls and safe errors. The native shell text is currently English.

## Security boundary

The renderer has Node integration disabled, context isolation and Chromium
sandboxing enabled, and only a small preload API. New windows are denied;
trusted HTTPS links open in the operating-system browser. Do not add generic
filesystem or command-execution functions to the preload bridge.
Navigation checks compare exact origins, not URL prefixes. IPC is accepted only
from this window's trusted main frame, never a subframe or another window.
Compatibility metadata is not a substitute for application signing.

## Validation

```bash
node --check main.js
node --check preload.js
npm test
npx electron-builder --dir --linux
xvfb-run -a node ../dev/scripts/test-desktop-startup.mjs dist/linux-unpacked/offgrid-llm-desktop
```

CI also builds the React application and its generated API types before the
Electron package, preventing stale or missing UI assets from shipping.
The Node tests exercise handshake/timeout/URL/IPC and lifecycle policy. CI also
launches native unpacked Electron on Windows, Linux, macOS Intel and Apple Silicon
against isolated fixture services and the bundled Go binary. On Windows, pass
`dist/win-unpacked/OffGrid LLM Desktop.exe` to the smoke script. On Mac, pass the
executable inside `dist/mac[-arm64]/OffGrid LLM Desktop.app/Contents/MacOS`.
The script creates its own temporary profile, never modifies the installed app,
and leaves screenshots/test data as evidence. It does not perform installation,
uninstallation, signing, notarization, upgrades or real-model qualification.

On Windows, the separate installer smoke uses its own application identity and
temporary installation path, with shortcuts/elevation/automatic launch disabled:

```powershell
cd desktop
npx electron-builder --config installer-test.cjs --win nsis --x64 --publish never
cd ..
$version = (Get-Content desktop/package.json | ConvertFrom-Json).version
./dev/scripts/test-windows-installer.ps1 -InstallerPath "build/windows-installer-smoke/OffGrid Desktop Install Test-Setup-$version.exe"
```

It checks clean installation, real installed-app startup, same-version reinstall,
uninstall and workspace-fixture preservation. It refuses normal release installers
and existing test registrations. It removes only its own test application and
retains evidence; it does not qualify all historical upgrades or elevated installs.
