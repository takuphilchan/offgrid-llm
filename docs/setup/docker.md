# Docker deployment

OffGrid publishes a portable CPU image for Linux AMD64 and ARM64. The image
contains the server, CLI, web UI, and llama.cpp runtime; downloaded models and
application state live in persistent Docker volumes.

## Quick start

```bash
docker pull takuphilchan/offgrid-llm:edge
docker run -d \
  --name offgrid \
  --init \
  --restart unless-stopped \
  --security-opt no-new-privileges=true \
  --cap-drop ALL \
  -p 127.0.0.1:11611:11611 \
  -v offgrid-models:/var/lib/offgrid/models \
  -v offgrid-data:/var/lib/offgrid/data \
  takuphilchan/offgrid-llm:edge
```

Open <http://localhost:11611/ui/> and download a model from the Models page, or
run:

```bash
docker exec -it offgrid offgrid download tinyllama-1.1b-chat --yes
```

`127.0.0.1` keeps the unauthenticated quick-start API on the Docker host. Never
publish this configuration on `0.0.0.0`; use the authenticated production stack
and TLS for remote access.

## Image tags

| Tag | Meaning |
| --- | --- |
| `edge` | Current development image; update deliberately |
| `latest` | Most recent stable CPU release after the first stable publication |
| `<version>` | Immutable semantic-version release, for example `1.0.0` |
| `<major>.<minor>` | Most recent patch in a stable minor release |
| `sha-<commit>` | Image built from an exact source revision |
| `latest-gpu` | Most recent stable NVIDIA release (Linux AMD64) |
| `<version>-gpu` | Versioned NVIDIA release (Linux AMD64) |

Use `edge` for the current pre-release build. Pin a full version or digest in
production once a stable release is published.

## Compose deployments

Clone the repository only when using its Compose definitions:

```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm/docker
cp .env.example .env
docker compose pull
docker compose up -d
```

Useful commands:

```bash
docker compose ps
docker compose logs -f offgrid
docker compose exec offgrid offgrid version
docker compose exec offgrid offgrid download tinyllama-1.1b-chat --yes
docker compose down
```

`docker compose down` preserves the named volumes. Do not add `--volumes`
unless you intentionally want to erase all downloaded models and application
data.

### Local development image

The default Compose file is pull-first. Add the developer overlay to build the
current checkout:

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.dev.yml \
  up -d --build
```

### NVIDIA GPU

Install [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html)
for the Docker host and confirm GPU passthrough first:

```bash
docker run --rm --gpus all nvidia/cuda:12.8.1-base-ubuntu24.04 nvidia-smi
```

Then run:

```bash
docker compose -f docker-compose.gpu.yml pull
docker compose -f docker-compose.gpu.yml up -d
```

The GPU image is deliberately separate because it is Linux AMD64-only and much
larger than the portable CPU image. It uses CUDA 12.8 and includes Blackwell
`sm_120` support. `OFFGRID_GPU_LAYERS=0` selects automatic VRAM-aware offload;
`OFFGRID_ENABLE_GPU=true` is set by the image and Compose profile. The model
must actually load before GPU use can be confirmed: check `docker logs` for
CUDA and offloaded layers, and watch `nvidia-smi` during inference.

For Hermes, set a real 65,536-token context and choose a model that can fit its
weights **and** KV cache in available RAM/VRAM. On a machine with 8 GB VRAM,
Phi-3.5 Mini's 64K KV cache may need partial CPU offload; a smaller-cache
long-context model is preferable. Example Compose overrides:

```bash
OFFGRID_MAX_CONTEXT=65536 OFFGRID_ADAPTIVE_CONTEXT=false \
  OFFGRID_KV_CACHE_TYPE=q4_0 \
  docker compose -f docker-compose.gpu.yml up -d
```

Do not run the CPU and GPU profiles on the same host port simultaneously.
Compose creates project-scoped volumes, while the `docker run` example above
uses volumes named exactly `offgrid-models` and `offgrid-data`; switching
between those deployment methods does not automatically move models or data.
Back up and deliberately migrate volumes before changing deployment methods.

On WSL 2, `.wslconfig` sets a **maximum** VM memory limit; Docker GPU access
does not remove the need for host RAM. Consult [Microsoft's WSL resource
settings](https://learn.microsoft.com/en-us/windows/wsl/wsl-config) and keep
enough memory for Windows. Applying a changed limit requires `wsl --shutdown`,
which also stops running WSL containers until Docker restarts.

## Persistent storage

| Volume | Contents |
| --- | --- |
| `offgrid-models` | Downloaded GGUF model files |
| `offgrid-data` | Sessions, users, settings, indexes, runs, and artifacts |

Back up volumes with your normal Docker volume backup tooling before upgrades.
The image itself is disposable and should never contain model or user data.

## Production stack

The production Compose file enables OffGrid authentication, exposes the app
only to Nginx, and terminates TLS on ports 80/443.

```bash
cd docker
cp /path/to/fullchain.pem certs/cert.pem
cp /path/to/private-key.pem certs/key.pem
cp .env.example .env
# Edit .env and set a strong GRAFANA_ADMIN_PASSWORD.

docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml run --rm offgrid users create admin admin
docker compose -f docker-compose.prod.yml up -d
```

Save the one-time password and API key printed during administrator creation.
Enable the optional metrics stack only when required:

```bash
docker compose -f docker-compose.prod.yml --profile monitoring up -d
```

## Building and publishing

Maintainers can build locally with:

```bash
bash ./docker/docker-build.sh
```

`PUSH=true` uses Buildx and publishes the configured `IMAGE` for AMD64 and
ARM64. `BUILD_GPU=true` additionally publishes the AMD64 CUDA tag.

Git tags matching `v*` trigger `.github/workflows/docker-publish.yml`. Configure
the GitHub Actions variable `DOCKERHUB_USERNAME` and secret `DOCKERHUB_TOKEN`
with a Docker Hub access token. The workflow builds and smoke-tests the image,
publishes version and source-revision tags, generates provenance and an SBOM,
and creates an image attestation.

## Validation and troubleshooting

```bash
cd docker
bash ./validate-docker.sh
BUILD_IMAGE=true bash ./validate-docker.sh
```

If startup fails, inspect `docker compose ps`, `docker compose logs offgrid`,
and `docker inspect` health output. Check for host-port conflicts with
`docker ps --format 'table {{.Names}}\t{{.Ports}}'` and change `OFFGRID_PORT`
in `.env` when necessary.
