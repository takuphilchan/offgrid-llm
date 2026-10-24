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

Both CPU and CUDA builds pin llama.cpp revision
`b95502ba9aa0eb73a2f4fc8878d7fbe6a847a0b9` (upstream `b10516`), matching the
portable runtime contract. CPU installations do not need NVIDIA support. GPU
installations require a CUDA runtime and device reservation; enabling a flag on
a CPU-only image is not GPU support. Cached portable binaries must not shadow
an installed compatible GPU runtime. `OFFGRID_ENABLE_GPU=false` retains explicit
CPU operation; neither edition changes installed models or user data. The CUDA
image includes a separate CPU-only worker at the same pinned revision, so CPU
mode and device-free operation do not need a first-use runtime download. The
effective configuration (including configuration-file values) controls runtime
selection; embedding placement is independent of Chat's GPU preference.

Source builds apply the reviewed [native sequencing patch](../runtimes/llama-cpp/README.md)
at that exact upstream pin. Its `-offgrid-seq1` runtime identity distinguishes it
from an unpatched worker. Patch mismatch fails the build; installed binaries and
model files are not silently rewritten. Parser tests are not workflow qualification.

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
