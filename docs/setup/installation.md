# Install OffGrid

Choose one service installation and connect your clients to it. This guide covers desktop, CLI, Docker, and source builds. Installing a client does not migrate an existing workspace or update a separately managed service.

## Choose a distribution

| Your goal | Distribution | Next step |
| --- | --- | --- |
| Use OffGrid on a personal computer | Desktop package | Install the OS-specific package below |
| Keep a service in Docker or WSL | CPU or NVIDIA container | [Docker deployment](docker.md) |
| Use terminal commands or an API server | CLI/runtime archive | Extract it and run `offgrid serve` |
| Change the code | Source checkout | [Build guide](../advanced/BUILDING.md) |
| Call an existing service from Python | Python client | [Python guide](../../python/README.md); this does not install the service |

Check memory, storage, CPU instructions, and model requirements before downloading. The project targets lightweight qualified workflows on 8 GB systems and recommends 16 GB; this is not a guarantee for every model, context, or vision workflow. See [CPU support](../advanced/cpu-support.md) and the [reliability plan](../advanced/product-reliability-plan.md).

## Download and verify a release

Use the [official releases](https://github.com/takuphilchan/offgrid-llm/releases). Download the package and `checksums-vX.Y.Z.sha256` from the same release. Do not mix packages from different revisions to resolve a compatibility warning.

| Platform | Desktop package |
| --- | --- |
| Windows x64 | Setup `.exe` or portable `.exe` |
| macOS Intel | x64 `.zip` containing the app |
| macOS Apple Silicon | arm64 `.zip` containing the app |
| Linux x64 | `.AppImage` or `.deb` |

CLI/runtime archives also cover Linux ARM64. That does not imply a Linux ARM64 desktop package. Check the release's actual assets; do not assume Windows ARM64, RPM, or universal macOS packages exist.

Calculate the file's SHA-256 and compare the entire digest with its entry in the checksum file:

```powershell
Get-FileHash -Algorithm SHA256 -LiteralPath '.\YOUR_DOWNLOADED_PACKAGE.exe'
```

```bash
# Linux
sha256sum ./YOUR_DOWNLOADED_PACKAGE.tar.gz
# macOS
shasum -a 256 ./YOUR_DOWNLOADED_PACKAGE.zip
```

Checksums detect corruption; they are not a publisher signature. Check the release's signing status and [OS trust guidance](desktop-startup.md#installer-appearance-and-os-warnings). Do not disable security controls to install an untrusted file.

## Install desktop

On Windows, run verified Setup for a registered installation, or launch the portable executable. Close OffGrid normally when Setup requests it. Models and workspace data are stored separately from application files.

On macOS, extract the archive and place the app in Applications. On Linux, use the distribution's package installer for the DEB, or make the verified AppImage executable and launch it. OS permissions and publisher trust may still need attention; a successful build is not proof of signing or notarization.

Open the app. If port 11611 is free, it can start its bundled service. If a compatible service already runs there, it connects to that workspace. Follow the [quickstart](quickstart.md) to download a model; models are not included in the installer.

## Install the CLI

Extract the matching CLI/runtime archive into an application directory. Preserve its bundled runtime and supporting files; consult the archive instructions and [llama.cpp integration](../advanced/llama-cpp.md) for custom runtime selection. Add the CLI directory to `PATH`, or use its full path.

```bash
offgrid version
offgrid --help
offgrid serve
```

On Windows, from the extracted directory:

```powershell
.\offgrid.exe version
.\offgrid.exe serve
```

Open <http://127.0.0.1:11611/ui/>. If the UI is absent or cannot be located, use a matching desktop/container package or build its UI and configure `OFFGRID_UI_DIR`. Do not copy unrelated release assets.

Repository installer scripts are optional, separate workflows. Read [installer documentation](../../installers/README.md) and the script before running one; they may install dependencies or services. Running a remote script as administrator is not required for desktop first use.

## Update an existing installation

Finish active work, identify the service and data roots, and make a [verified backup](../advanced/workspace-recovery.md).

| Component | What updating it changes | What it does not update |
| --- | --- | --- |
| Desktop installer | Shell, bundled UI, service binary, host components | Running container or separately installed service |
| Container replacement | Application, UI, inference runtime inside it | Desktop app or host companion |
| CLI archive replacement | Files in that installation directory | Running process, another CLI on `PATH`, or a container |
| Source edit or build | Checkout and generated local outputs | Installed app or published release |

For desktop plus Docker, update both to matching versions and UI builds, preserve the same volumes, then choose **Retry connection**. See [Docker upgrades](docker.md#upgrade-an-existing-workspace). Use an isolated workspace only if you deliberately want separate data.

Rollback needs compatible application and data snapshots. Replacing only the executable after a schema migration is not safe rollback.

## Remove the application without deleting work

Use the OS uninstaller for registered desktop installations, or remove only the extracted app directory after quitting a portable app. Keep data/model roots and backups unless you separately intend to erase them.

For Docker, stop/remove only the identified container; do not add volume-removal flags. Never delete a workspace as routine startup repair. [Recovery](../advanced/workspace-recovery.md) explains backup coverage.

## Check the connection

```powershell
curl.exe --max-time 5 http://127.0.0.1:11611/health
curl.exe --max-time 5 http://127.0.0.1:11611/api/v2/system
```

In Bash, use `curl`. `/health` reports availability; `/api/v2/system` reports version, revision, UI build, and workspace identity. Keep these identities with issue reports, but omit credentials and private data. With WSL, test from Windows as well as inside WSL.
