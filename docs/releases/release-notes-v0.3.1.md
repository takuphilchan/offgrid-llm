# OffGrid LLM v0.3.1

OffGrid LLM v0.3.1 is the first supported release of the new production
foundation. It supersedes the incomplete v0.3.0 publishing attempt, which did
not finish every runtime bundle.

## Highlights

- A redesigned React web application shared by browser and Electron, with
  durable navigation and English, French, Spanish, Swahili, and Arabic
  localization foundations.
- Stateful chat, model, knowledge, and agent workspaces backed by real server
  APIs rather than demonstration-only UI state.
- OpenAI-compatible API surfaces for connecting external agent clients and
  local-AI tooling.
- A durable agent run/event store, official MCP SDK integration, capability
  discovery, sandbox controls, and guarded computer-use foundations.
- A persistent SQLite RAG store with index metadata, lifecycle checks,
  evaluation support, and clearer embedding-model handling.
- Signed P2P manifests and stricter transfer validation for local model
  sharing.
- Resumable model downloads with range validation, recovery from stale partial
  files, and protection against duplicate downloads.
- A polished interactive CLI and terminal chat experience aligned with the web
  and desktop applications.
- Hardened container defaults, persistent volumes, CPU and NVIDIA images, and
  reproducible Docker build metadata.

## Release reliability fixes

- Pin `llama.cpp` to the same reviewed commit used by the container images.
- Build Apple Silicon on `macos-15` and Intel macOS on
  `macos-15-intel`.
- Build Linux ARM64 on a native ARM64 runner.
- Refuse to package a runtime when the runner CPU does not match the advertised
  artifact architecture.
- Make the Docker Hub account deterministic and fail early with a useful error
  if its publishing token is missing.

## Container quick start

```bash
docker pull takuphilchan/offgrid-llm:0.3.1

docker run -d \
  --name offgrid \
  --init \
  --security-opt no-new-privileges=true \
  --cap-drop ALL \
  -p 127.0.0.1:11611:11611 \
  -v offgrid-models:/var/lib/offgrid/models \
  -v offgrid-data:/var/lib/offgrid/data \
  takuphilchan/offgrid-llm:0.3.1
```

Open <http://localhost:11611/ui/> after the health check becomes ready.

## Desktop and CLI packages

Release assets include:

- Linux x64 desktop packages and CPU/Vulkan CLI bundles.
- Linux ARM64 CLI bundles.
- macOS Apple Silicon and Intel desktop/CLI packages.
- Windows x64 installer, portable desktop package, and CLI bundle.

Desktop packages are currently unsigned. Windows SmartScreen and macOS
Gatekeeper may therefore require explicit approval from the user. Code signing
and notarization remain required before recommending unattended enterprise
deployment.

## Verify downloads

Download `checksums-v0.3.1.sha256` with the desired assets, then run:

```bash
sha256sum -c checksums-v0.3.1.sha256
```

See the [installation guide](https://github.com/takuphilchan/offgrid-llm/blob/v0.3.1/docs/setup/installation.md),
[Docker guide](https://github.com/takuphilchan/offgrid-llm/blob/v0.3.1/docs/setup/docker.md), and
[full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.2.12...v0.3.1).
