# Deployment

OffGrid supports three deployment shapes. Use one authoritative data directory
and one authoritative models directory in every shape.

## Container

Docker is the simplest repeatable server deployment. Follow the complete
[Docker guide](../setup/docker.md) for the current `edge` image, persistent
volumes, authentication, TLS, CPU/GPU variants, validation, and publishing.

Bind the unauthenticated quick start to `127.0.0.1`. A remote deployment must
enable authentication and terminate TLS at a trusted reverse proxy.

## Native service

Build the React bundle and Go binary, then install both the binary and generated
UI assets:

```bash
cd web/app
npm ci
npm run api:check
npm run build
cd ../..

go build -trimpath -o offgrid ./cmd/offgrid
sudo install -m 0755 offgrid /usr/local/bin/offgrid
sudo install -d -o offgrid -g offgrid /var/lib/offgrid/models
sudo install -d -o offgrid -g offgrid /var/lib/offgrid/data
sudo install -d -o offgrid -g offgrid /var/lib/offgrid/web/ui
sudo cp -R web/dist/. /var/lib/offgrid/web/ui/
```

Configure the service with:

```text
OFFGRID_HOST=127.0.0.1
OFFGRID_PORT=11611
OFFGRID_MODELS_DIR=/var/lib/offgrid/models
OFFGRID_DATA_DIR=/var/lib/offgrid/data
OFFGRID_UI_DIR=/var/lib/offgrid/web/ui
```

Use the systemd unit supplied by the project as the starting point. The service
account needs write access to model and data roots, but not to the executable or
UI files.

## Desktop

Electron is the supported local graphical distribution for Windows, macOS, and
Linux. It packages the same generated React bundle and starts a loopback-only
runtime when one is not already available. See [desktop/README.md](../../desktop/README.md).

Desktop packages are for one interactive user; do not use Electron as a server
or shared-network deployment.

## Air-gapped installation

Prepare and checksum the following on a connected machine:

- the platform OffGrid and `llama-server` binaries or a saved container image;
- the generated `web/dist` files when installing natively;
- selected GGUF models and any required embedding/audio models;
- configuration and service files.

Transfer them using approved media, verify checksums before installation, and
keep model files outside the application image so upgrades do not replace user
data.

## Upgrade and rollback

1. Back up `OFFGRID_DATA_DIR` and record model-file checksums.
2. Stop the owned runtime cleanly.
3. Replace only application binaries/UI assets or the disposable container.
4. Start the new version and check `/livez`, `/readyz`, models, and a saved
   session.
5. Roll back the application artifact if validation fails; do not delete data.

Legacy data migration copies recognized files without overwriting current
state, so the source remains available for recovery.
