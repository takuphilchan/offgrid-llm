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

In development Electron uses an existing healthy local server when available.
Packaged applications can start the bundled runtime themselves.

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
- If Docker or another local service already owns the configured port, Electron
  connects to it and does not stop it on exit.
- Window geometry is persisted in `~/.offgrid-llm/window-state.json`.
- The loading screen uses the product visual system and reports startup failure
  without exposing raw backend internals.

## Security boundary

The renderer has Node integration disabled, context isolation and Chromium
sandboxing enabled, and only a small preload API. New windows are denied;
trusted HTTPS links open in the operating-system browser. Do not add generic
filesystem or command-execution functions to the preload bridge.

## Validation

```bash
node --check main.js
node --check preload.js
npx electron-builder --dir --linux
```

CI also builds the React application and its generated API types before the
Electron package, preventing stale or missing UI assets from shipping.
