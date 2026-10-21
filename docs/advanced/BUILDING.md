# Build OffGrid from source

Build the Go service and shared React UI together. These instructions are for contributors; ordinary users should use [release packages](../setup/installation.md). A build in this checkout does not replace an installed app or running container.

## Prerequisites

Use the toolchain declared in [go.mod](../../go.mod), Node.js 22, npm, and Git. CI currently uses Go 1.26.6. Dependencies and first-use runtime downloads need network access unless already provisioned.

Desktop packaging also needs the native build tools for its target OS. macOS native components need the macOS SDK; Linux native components need the development libraries listed in [desktop packaging](../../apps/desktop/README.md). Do not assume a Go cross-compile qualifies native desktop behavior.

The root [Makefile](../../Makefile) offers `build`, `test`, `test-go`,
`test-web`, `test-desktop`, `test-computer`, `test-python`, `check-docs`, and
`lint` as contributor shortcuts. These targets do not install, deploy, start a
service, or run live desktop control. On Windows without Make, run the direct
commands below using `npm.cmd` and the installed Python launcher.

## Build the shared UI

From the repository root:

```bash
npm ci --prefix apps/web
npm run api:check --prefix apps/web
npm run check --prefix apps/web
npm run build --prefix apps/web
```

This generates `apps/web/dist`, used by the browser service and packaged desktop. Do not edit generated assets or maintain a separate desktop UI.

## Build and start the service

Bash:

```bash
version=$(tr -d '\r\n' < VERSION)
go build -trimpath -ldflags "-X main.Version=$version" -o bin/offgrid ./cmd/offgrid
./bin/offgrid serve
```

PowerShell:

```powershell
$buildVersion = (Get-Content -Raw VERSION).Trim()
go build -trimpath -ldflags "-X main.Version=$buildVersion" -o bin/offgrid.exe ./cmd/offgrid
.\bin\offgrid.exe serve
```

Open <http://127.0.0.1:11611/ui/>. Build from the root so relative UI lookup can find `apps/web/dist`, or set `OFFGRID_UI_DIR` to its absolute path. Stop here if another service owns the port or workspace; do not kill it by port number. For isolated development, set separate `OFFGRID_PORT`, `OFFGRID_DATA_DIR`, and `OFFGRID_MODELS_DIR` before starting.

The service supervises a native `llama-server`. `OFFGRID_LLAMA_SERVER_PATH` selects an explicit runtime executable. A compatible executable on `PATH` or the native fallback installer may otherwise be used; Docker bundles one. See [llama.cpp integration](llama-cpp.md) before assuming a custom runtime supports embeddings or vision.

## Develop the UI and desktop

With the service running, `npm run dev --prefix apps/web` starts Vite. Its API proxy defaults to port 11611: do not run destructive fixture tests against your real workspace.

For Electron development:

```bash
npm ci --prefix apps/desktop
npm run dev --prefix apps/desktop
```

A separately running service must pass version, API, and UI-build compatibility checks. If desktop needs to launch its own service, put the binary at the platform-specific path in [apps/desktop/README.md](../../apps/desktop/README.md). Both components must come from the same build; setting the version alone does not make unrelated UI assets compatible.

## Package desktop

Follow [desktop packaging](../../apps/desktop/README.md) on the target OS. The build needs the matching Go binary, generated UI, locked computer dependencies, and bundled browser/native workers. The packaging hooks prepare and verify those components. End users do not install Node, Go, or Playwright.

The current desktop targets are Windows x64 Setup/portable, macOS x64/arm64 ZIP archives, and Linux x64 AppImage/DEB. The authoritative configuration is [apps/desktop/package.json](../../apps/desktop/package.json). CPU/GPU CLI archives and container images are separate release products.

For containers, follow [Docker development](../setup/docker.md#local-development-image). Local builds do not authorize replacing a live container, pushing images, or publishing packages.

## Validate before review

From the root:

```bash
node --test scripts/maintenance/check-docs.test.mjs
node scripts/maintenance/check-docs.mjs
go test ./...
npm test --prefix apps/desktop
npm ci --prefix apps/computer --ignore-scripts
npm run browser:install --prefix apps/computer
npm test --prefix apps/computer
PYTHONPATH=sdk/python python3 -m pytest -q sdk/python/tests/test_client.py
python3 -m pytest -q runtimes/python/tests
```

PowerShell equivalents for the Python checks are:

```powershell
$env:PYTHONPATH = (Resolve-Path sdk/python).Path
py -3 -m pytest -q sdk/python/tests/test_client.py
py -3 -m pytest -q runtimes/python/tests
Remove-Item Env:PYTHONPATH
```

The `offgrid` client distribution lives in [sdk/python](../../sdk/python/README.md)
and retains its independent version. The supervised Python provider and audio
workers live in `runtimes/python`; they are not part of the SDK wheel. The
client fixtures and worker protocol fixtures use synthetic state. Do not run
`sdk/python/tests/test_live.py` against a personal service as an ordinary check.
The older source/GPU/systemd installer is
[`scripts/build/install-from-source.sh`](../../scripts/build/install-from-source.sh);
it has installation side effects and is never invoked by `make test`.

The computer suite requires its browser dependencies. Native/installed-package tests have separate OS and consent requirements; see [computer development](../../apps/computer/README.md).

After building the UI and service, install the test browser once and run the isolated web wrapper:

```bash
cd apps/web
npx playwright install chromium
cd ../..
node scripts/test/test-web-workspace.mjs bin/offgrid
```

Use `bin/offgrid.exe` on Windows. The wrapper creates disposable state and a separate port. Do not point `OFFGRID_E2E_URL` at a real workspace: the full browser suite creates and deletes fixtures.

API edits require `npm run api:generate --prefix apps/web`, then `api:check`. Review generated changes with the source contract. See [contributing](../../CONTRIBUTING.md), [CI](../../.github/workflows/ci.yml), and [release gates](releasing.md).

## Diagnose build problems

| Symptom | Check |
| --- | --- |
| Web build or contract check fails | Install from the lockfile with `npm ci`; inspect type/schema errors rather than bypassing checks. |
| Desktop reports another UI build | Rebuild and package the same `apps/web/dist` as the service uses. Do not weaken the compatibility check. |
| Native worker build fails | Build on the required OS with its SDK/libraries; do not substitute an empty worker. |
| A test changes real history | Stop the test and restore from your backup if necessary. Use isolated fixtures next time. |
| Package is unsigned | Signing/notarization require real publisher identities. A successful build does not supply them. |

Release only through the documented [release process](releasing.md). Never move a published tag to hide a failed build.
