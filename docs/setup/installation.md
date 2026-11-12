# Install OffGrid

Choose one service installation and connect your clients to it. This guide covers desktop, CLI, Docker, and source builds. Installing a client does not migrate an existing workspace or update a separately managed service.

## Choose a distribution

| Your goal | Distribution | Next step |
| --- | --- | --- |
| Use OffGrid on a personal computer | Desktop package | Install the OS-specific package below |
| Keep a service in Docker or WSL | CPU or NVIDIA container | [Docker deployment](docker.md) |
| Use terminal commands or an API server | CLI/runtime archive | Extract it and run `offgrid serve` |
| Change the code | Source checkout | [Build guide](../advanced/BUILDING.md) |
| Call an existing service from Python | Python client | [Python guide](../../sdk/python/README.md); this does not install the service |

Check memory, storage, CPU instructions, and model requirements before downloading. The project targets lightweight qualified workflows on 8 GB systems and recommends 16 GB; this is not a guarantee for every model, context, or vision workflow. See [CPU support](../advanced/cpu-support.md) and the [quality evidence](../quality/README.md).

## Download and verify a release

Use the [official releases](https://github.com/takuphilchan/offgrid-llm/releases). Download the package and `checksums-vX.Y.Z.sha256` from the same release. Do not mix packages from different revisions to resolve a compatibility warning.

The newest published version is [0.4.30, an unsigned preview](https://github.com/takuphilchan/offgrid-llm/releases/tag/v0.4.30).
GitHub's **Latest** badge and `/releases/latest` select the stable channel
(currently 0.4.28), so use the versioned preview link to get 0.4.30.
A Git tag alone does not mean installable packages were published: 0.4.29 was
superseded by 0.4.30 after its release build failed.

| Platform | Desktop package |
| --- | --- |
| Windows x64 | `OffGrid-X.Y.Z-windows-x64-setup.exe` (recommended) |
| macOS Intel | `OffGrid-X.Y.Z-macos-x64.zip` |
| macOS Apple Silicon | `OffGrid-X.Y.Z-macos-arm64.zip` |
| Linux x64 | `OffGrid-X.Y.Z-linux-amd64.deb` for Debian/Ubuntu; `OffGrid-X.Y.Z-linux-x86_64.AppImage` for other distributions |

These desktop names apply from 0.4.27 onward. Earlier releases retain their
original names. The application and shortcuts are named **OffGrid**; repository
URLs, application IDs and existing data directories retain their established
identities. CLI/runtime archives are separate downloads, not desktop installers.

CLI/runtime archives also cover Linux ARM64. That does not imply a Linux ARM64 desktop package. Check the release's actual assets; do not assume Windows ARM64, RPM, or universal macOS packages exist.

### Other downloads

Windows portable (`OffGrid-X.Y.Z-windows-x64-portable.exe`), CLI CPU/Vulkan
archives, containers, and source builds are alternatives for specific workflows.
Most desktop users need only the package in the table above. Match Intel/AMD x64
and ARM64 carefully. AVX2 and AVX512 CLI archives require those CPU instructions;
AVX512 is not a general upgrade for every x64 machine. See
[CPU support](../advanced/cpu-support.md). OS compatibility and tested hardware
vary by release: consult its notes and evidence instead of assuming that an
available asset qualifies every version of the OS.

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

## Platform capabilities

This matrix describes packaged components and prerequisites, not qualification
of every OS version or model. An external service uses its own runtimes.

| Workflow | Windows x64 | macOS Intel / Apple Silicon | Linux x64 |
| --- | --- | --- | --- |
| Local chat | Model required; CPU runtime fallback download | Model required; CPU / Metal runtime fallback download | Model required; CPU runtime fallback download |
| GPU acceleration | Compatible GPU runtime must be installed separately | Intel CPU default / Apple Silicon Metal | Compatible runtime and drivers required; GPU containers are another option |
| Dictation | Whisper runtime bundled; model download required | Whisper runtime bundled; model download required | Whisper runtime bundled; model download required |
| Speech playback | Piper bundled; voice download required | Piper excluded; separately configured compatible runtime required | Piper bundled; voice download required |
| Documents | PDF/OCR workers bundled; OCR language download and embedding model as needed | Same prerequisites | Same prerequisites |
| Integrated terminal | Available on x64 | Not currently available | Not currently available |
| Computer access | Preview, local consent and supported controls required | Preview, Accessibility permissions and supported controls required | Preview, AT-SPI and supported desktop session required; Wayland limitations apply |

The standard Windows CLI bundle is CPU/AVX2. Installing the CUDA Toolkit alone
does not turn that binary into a GPU runtime. Supply a compatible GPU build with
`OFFGRID_LLAMA_SERVER_PATH`; see [inference performance](../advanced/PERFORMANCE.md).
First-use runtime downloads are separate from model downloads. For offline use,
prepare the model and all runtimes, obtain a real reply, then repeat the intended
workflow with networking disabled. See [voice setup](../guides/voice.md) and
[computer tasks](../guides/computer-tasks.md) for additional limitations.

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

Current source builds discover `web/ui`, the sibling inference executable, and
`audio/whisper` and `audio/piper` relative to the CLI executable, independent of
the terminal's current directory. Keep the complete archive layout. Explicit
`OFFGRID_UI_DIR` and `OFFGRID_LLAMA_SERVER_PATH` settings still take precedence.
Older releases such as 0.4.28 need `OFFGRID_UI_DIR` set to the extracted `web/ui`
directory before `offgrid serve`; rebuilding source does not update that release.

Repository installer scripts are optional, separate workflows. Read [installer documentation](../../installers/README.md) and the script before running one; they may install dependencies or services. Running a remote script as administrator is not required for desktop first use.

## Update an existing installation

Finish active work, identify the service and data roots, and make a [verified backup](../advanced/workspace-recovery.md).

| Component | What updating it changes | What it does not update |
| --- | --- | --- |
| Desktop installer | Shell, bundled UI, service binary, host components | Running container or separately installed service |
| Container replacement | Application, UI, inference runtime inside it | Desktop app or host companion |
| CLI archive replacement | Files in that installation directory | Running process, another CLI on `PATH`, or a container |
| Source edit or build | Checkout and generated local outputs | Installed app or published release |

Desktop updates are manual: download and verify the new package, close active
work, install it, and reopen the existing workspace. No integrated updater is
provided. For desktop plus Docker, preserve the same volumes and update any
component rejected by the compatibility check, then choose **Retry connection**.
Compatible external versions can differ. See [Docker upgrades](docker.md#upgrade-an-existing-workspace).
Use an isolated workspace only if you deliberately want separate data.

For locally rebuilt or preview packages, record the revision and UI build from
`/api/v2/system` along with the version. A local build sharing a public version
number is not evidence that it is the same artifact as that release.

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
