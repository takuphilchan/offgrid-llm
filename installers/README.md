# OffGrid installers

For a personal computer, use the **Windows Setup, macOS ZIP, or Linux desktop
package** in the [installation guide](../docs/setup/installation.md#download-and-verify-a-release).
Desktop includes the OffGrid service; Go, Node, Docker and WSL are not required.
Models and the main inference runtime may still require separate downloads.
See the [platform matrix](../docs/setup/installation.md#platform-capabilities)
before planning GPU, speech, terminal, or offline workflows.

## Optional script workflows

Download and inspect a script before running it. These are advanced alternatives
to the desktop package, not a prerequisite for using OffGrid.

- [`desktop.ps1`](desktop.ps1) offers CLI and/or desktop installation on Windows.
  The CLI installs the entire release payload. Without elevation it uses
  `%LOCALAPPDATA%\Programs\OffGrid CLI` and user PATH; with elevation it uses
  `%ProgramFiles%\OffGrid` and machine PATH. Restart terminals after installation.
  CLI replacement refuses running OffGrid/inference processes. Finish active
  work and close their owning service before retrying.
  Current builds discover the bundled CLI web UI automatically; older releases
  need `OFFGRID_UI_DIR` as described in [CLI setup](../docs/setup/installation.md#install-the-cli).
- [`desktop.sh`](desktop.sh) offers CLI and/or desktop installation on Linux/macOS.
  CLI installation writes to `/usr/local/bin` and may require sudo. It refuses
  replacement while OffGrid is running: finish active work and close the owning
  service normally, then retry. It never stops running OffGrid processes.
- Both desktop scripts download the same release's `checksums-vX.Y.Z.sha256`
  and verify each payload before extraction or execution. Missing, ambiguous,
  or mismatched checksums stop installation. Releases without a manifest are
  not accepted. Checksums detect corruption; they do not establish publisher
  identity. See [OS trust guidance](../docs/setup/desktop-startup.md#installer-appearance-and-os-warnings).

Example after saving and inspecting the script:

```powershell
powershell -NoProfile -File .\desktop.ps1
```

```bash
bash ./desktop.sh
```

The separate root [`install.sh`](../install.sh) is the maintained Unix release
installer for CLI/service users. It selects a platform bundle; systemd autostart
is opt-in through `AUTOSTART`. The published [`scripts/install.sh`](../scripts/install.sh)
URL remains a compatibility wrapper with its legacy GPU selection and
`AUTOSTART=ask` default. Installed unit identity remains `offgrid@.service`;
unit sources are under [`systemd/`](systemd/).

The root and compatibility entry points now require release archive verification
before extraction, including after a CPU/GPU fallback. Missing release manifests
stop installation. They refuse replacement while `offgrid` or `llama-server`
is running, or process inspection fails; close the owning service normally and
retry. They do not terminate processes or stop an existing systemd service.

For the older source/GPU/systemd path, see the
[source-build guide](../docs/advanced/BUILDING.md) and
[`install-from-source.sh`](../scripts/build/install-from-source.sh). That helper
builds dependencies and changes system services.

## After installation

Open desktop, or start `offgrid serve` and visit <http://127.0.0.1:11611/ui/>.
Follow the [quickstart](../docs/setup/quickstart.md) to install a model and verify
an actual reply. A successful installer exit does not confirm inference or GPU
readiness. Windows CPU bundles do not become CUDA-enabled by installing a toolkit.

Updates are manual; follow the [upgrade guide](../docs/setup/installation.md#update-an-existing-installation).
To uninstall, remove only the identified application using its OS uninstaller
or remove its extracted application directory after quitting it. Preserve
`~/.offgrid-llm`, model/data roots and backups. Do not delete workspaces or stop
unrelated services as routine uninstall or repair steps.
