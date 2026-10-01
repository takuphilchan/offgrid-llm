# Deployment and upgrades

Choose one service owner: a container, a native service supervisor, or the desktop
application. Each workspace has one writer. Browser and CLI clients connect to
that service; they do not need another server using the same data.

## Choose a deployment

| Deployment | Start here | Responsibility |
| --- | --- | --- |
| Personal desktop | [Installation](../setup/installation.md) | Desktop starts its bundled service only when the configured port is available |
| Docker CPU or NVIDIA | [Docker deployment](../setup/docker.md) | Docker manages the service and persistent volumes |
| Native CLI/service | [Build guide](BUILDING.md), [autostart](../setup/autostart.md) | You provision a matching binary, UI, inference runtime, and supervisor |
| Network workspace | [Multi-user access](../guides/multi-user.md) | Administrator configures authentication, permissions, TLS, and backups |

Keep unauthenticated installations bound to loopback. Computer input requires a
companion running as the interactive desktop user; a server container is not a
host-control service. See [Computer Tasks](../guides/computer-tasks.md).

## Native service layout

Build using the [versioned build commands](BUILDING.md). Deploy the executable,
generated `web/dist` directory, and matching native inference runtime together.
If using a dedicated service account, create it through your operating system's
administration tools first. It needs write access to data/models, not to binaries
or UI assets.

An example layout, not an instruction to overwrite an existing installation:

```text
OFFGRID_HOST=127.0.0.1
OFFGRID_PORT=11611
OFFGRID_MODELS_DIR=/var/lib/offgrid/models
OFFGRID_DATA_DIR=/var/lib/offgrid/data
OFFGRID_UI_DIR=/var/lib/offgrid/web/ui
OFFGRID_LLAMA_SERVER_PATH=/opt/offgrid/bin/llama-server
```

Verify these paths exist and belong to the intended service account. Do not mix
this dedicated-account layout with a per-user installer without explicitly
migrating the configuration and storage.

## Upgrade an existing workspace

1. Identify the running service, version, supervisor, data/model paths, and
   installation method. Record environment settings and retain the old application
   artifacts. Protect configuration and credentials.
2. Download and verify the replacement before downtime. Read its release notes
   for schema, API, runtime, and desktop compatibility.
3. Finish or stop active tasks and computer sessions. Stop the service through
   its owner; do not kill a process simply because it uses the desired port.
4. [Back up and verify the stopped workspace](workspace-recovery.md). Separately
   retain model/runtime files and configuration outside its data directory.
5. Replace application artifacts or recreate the container with the same intended
   storage. Never start two services against that writable workspace.
6. Start the new service. Check `/health`, `/readyz`, and `/api/v2/system`;
   then inspect a saved conversation, models, knowledge status, task history,
   and authentication. Run a small model request: health alone is not inference.
7. Update desktop/companion components to the matching build before testing
   computer tasks. Reconnecting an old companion is not an upgrade.

A failed schema migration or compatibility check is a reason to stop and inspect,
not to delete the workspace. Keep logs private and remove secrets before sharing.

## Roll back safely

Do not point an older executable at data already migrated by a newer release.
Use the matching application version and a verified pre-upgrade backup restored
into a **new directory**. Follow [restore instructions](workspace-recovery.md);
do not merge it over newer data. Changes made after that backup will not exist
in the restored workspace, so preserve the newer copy.

Computer dispatch journals stay on the host and must not be rolled back with a
service snapshot. Restored tasks need new local consent; uncertain effects
must be inspected, never automatically repeated.

## Prepare an offline installation

On a connected machine, collect matching application/UI/runtime artifacts,
checksums, licenses, selected GGUF models, and any required embedding/projector
files. Docker users can transfer a saved, verified image separately from volumes.
A model alone is not a complete installation.

Verify artifacts after transfer. Test first use with networking disabled,
including inference and any intended knowledge workflow. Model discovery,
downloads, public websites, and external MCP servers require their own network
access. Do not interpret local inference as a guarantee that every optional
integration is offline.

See [qualification gates](production-readiness.md) and the
[reliability evidence log](product-reliability-plan.md). Successful deployment
does not establish platform, model, signing, or security qualification.
