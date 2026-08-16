# Container layout

OffGrid's container files are separated by responsibility:

| File | Purpose |
| --- | --- |
| `Dockerfile` | Portable Linux AMD64/ARM64 CPU release image |
| `Dockerfile.gpu` | Linux AMD64 NVIDIA CUDA release image |
| `docker-compose.yml` | Pull-first, localhost-only CPU deployment |
| `docker-compose.dev.yml` | Overlay that builds the current checkout |
| `docker-compose.gpu.yml` | Pull-first NVIDIA deployment |
| `docker-compose.prod.yml` | Authenticated TLS stack with optional monitoring |
| `docker-build.sh` | Maintainer local/multi-platform build helper |
| `validate-docker.sh` | Configuration and optional image smoke tests |
| `.env.example` | Supported Compose image, version, port, and secret inputs |

The release image runs as UID/GID 1000, drops Linux capabilities in Compose,
uses an init process, includes a health check, and stores mutable state only in
`/var/lib/offgrid/models` and `/var/lib/offgrid/data`.

Start with [DOCKER_README.md](DOCKER_README.md) for commands or see the
[complete deployment guide](../docs/setup/docker.md).

## Maintainer builds

From the repository root:

```bash
# Local CPU image
bash ./docker/docker-build.sh

# CPU AMD64/ARM64 manifest
PUSH=true bash ./docker/docker-build.sh 0.3.0

# CPU manifest plus Linux AMD64 CUDA image
PUSH=true BUILD_GPU=true bash ./docker/docker-build.sh 0.3.0
```

Override `IMAGE` and `PLATFORMS` when publishing to another registry. Normal
users should pull the prebuilt images and do not need a compiler or Node.js.
