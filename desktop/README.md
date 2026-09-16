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
go build -trimpath -o build/linux/offgrid ./cmd/offgrid

cd desktop
npm ci
npm run dev
```

Use `build/windows/offgrid.exe` on Windows and an architecture-specific binary
under `build/macos` on macOS. Set `OFFGRID_PORT` before starting Electron to use
another local port. Invalid port values fall back to `11611`.

Electron checks `/api/v2/system` before attaching: product, API, supported
contracts, matching product version, and UI build identity must agree. A 200
health response alone is not enough. Rebuild the UI/runtime together; explicitly
replace an older container/service before trying the new desktop. Packaged apps
can start their bundled runtime when the port is free. An occupied incompatible
or unresponsive service is left untouched.

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
required. Code signing and notarization credentials should be provided by the
release environment; development packages are unsigned.

## Runtime behavior

- The application keeps models in `~/.offgrid-llm/models` and all other state in
  `~/.offgrid-llm/data`.
- Closing the window keeps the app in the tray on Windows and Linux; **Quit**
  stops the runtime owned by Electron.
- If a compatible Docker or other local service owns the configured port, Electron
  connects without stopping it on exit. Settings shows the active backend and UI
  build. Desktop-local paths are shown only for a desktop-managed runtime.
- Window geometry is persisted in `~/.offgrid-llm/window-state.json`.
- The loading screen uses the product visual system and reports startup failure
  without exposing raw backend internals.

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
```

CI also builds the React application and its generated API types before the
Electron package, preventing stale or missing UI assets from shipping.
The Node tests exercise handshake/timeout/URL/IPC policy. They are not installed
Electron smoke tests or proof that signing/update qualification has passed.
