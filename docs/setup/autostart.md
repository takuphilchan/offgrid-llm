# Start OffGrid automatically

Autostart should supervise the OffGrid service, which manages its own inference workers. Do not start a second `llama-server` just because an old guide mentions one. Choose the supervisor that owns your existing workspace.

## Desktop installations

Launching OffGrid Desktop connects to a compatible local service or starts its bundled service when the port is free. It does not install a background Windows service or update Docker. Closing the window may leave it in the tray; **Quit OffGrid** stops only its owned child service.

This guide does not claim a built-in login-start toggle or system-wide daemon for desktop. Use [desktop startup](desktop-startup.md) to understand ownership before configuring OS login items. Native computer control still requires an interactive desktop and local consent.

## Docker installations

The documented container profiles use `restart: unless-stopped`. The Docker engine must itself be running. A container you deliberately stop stays stopped until you start it.

For the named container in the [quickstart](docker.md):

```bash
docker ps -a --filter name=offgrid
docker start offgrid
docker logs --tail 100 offgrid
```

For Compose, run commands in the deployment directory with the same project and Compose files used to create it. The CPU service is `offgrid`; the GPU service is `offgrid-gpu`. Do not start both against the same data volumes or port.

## Existing Linux systemd installations

The repository's `scripts/install.sh` can install an `offgrid@.service` template. First inspect the unit that actually exists; do not assume every installation used this script:

```bash
systemctl list-unit-files 'offgrid*' 'llama-server*'
systemctl cat "offgrid@$USER.service"
```

Check its executable, user, home directory, data/model paths, and bind address. If this is your intended service, use:

```bash
sudo systemctl enable "offgrid@$USER.service"
sudo systemctl start "offgrid@$USER.service"
systemctl status "offgrid@$USER.service"
journalctl -u "offgrid@$USER.service" -n 100 --no-pager
```

Enabling starts it on later boots; starting runs it now. Stop active tasks before restarting or stopping the unit. If a legacy `llama-server@...` unit is present, inspect its ownership and purpose first; it may belong to another application. Do not disable unrelated services or kill processes by port.

A systemd unit on a server does not provide a signed-in user's desktop permissions. [Computer tasks](../guides/computer-tasks.md) still need the host application.

## Verify after reboot

```bash
curl --max-time 5 http://127.0.0.1:11611/health
curl --max-time 5 http://127.0.0.1:11611/api/v2/system
```

On Windows use `curl.exe`. Check version, workspace identity, models, and saved history. If desktop reports a mismatch, update the separately managed service rather than starting a second workspace. With WSL, confirm reachability from Windows and WSL; do not automatically restart WSL or alter a required VPN.

For a custom service account, use [deployment](../advanced/DEPLOYMENT.md) and [workspace recovery](../advanced/workspace-recovery.md). Never let two supervisors own the same writable data directory.
