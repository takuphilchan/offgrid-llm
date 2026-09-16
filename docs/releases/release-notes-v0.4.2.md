# OffGrid LLM v0.4.2

OffGrid LLM v0.4.2 completes the 0.4 workspace release across the web,
Electron desktop application, command line, and container images. It includes
the redesigned product experience, nine interface languages, and the corrected
release pipeline for all supported native platforms.

## Highlights

- A refined monochrome workspace shared by the browser and Electron, with
  responsive navigation, system-aware light and dark themes, locally bundled
  IBM Plex fonts, and a consistent application identity.
- A command palette, dedicated Activity and Settings pages, durable agent
  routes, and clearer separation between work, library, and system areas.
- Rich chat responses with GitHub-flavored Markdown, code and response copy
  actions, a growing composer, and privacy-preserving remote-image handling.
- Outcome-based onboarding that completes only after a successful local model
  response, with clearer model and knowledge setup guidance.
- A unified terminal experience for interactive chat, agent tasks, server
  startup, automation, light and dark terminals, and ASCII-only environments.

## Language support

The browser and Electron workspace include complete typed locale packs for
English, French, Spanish, German, Arabic, Kiswahili, ChiShona, isiNdebele
(Northern Ndebele), and isiZulu. Language selection persists across restarts.

## Reliability and distribution

- Fixed Apple Silicon packaging for the Bash 3.2 environment on GitHub macOS
  runners.
- Fixed AVX-512 Linux packaging so build-time llama.cpp helpers remain runnable
  on standard runners while GGML receives the intended optimization flags.
- Refined command-palette focus and scrollbar presentation for Chromium and
  Electron without removing keyboard accessibility.
- Added bounded retries for transient npm and Go module download failures in
  container builds.
- Release publishing preserves existing assets on retries and verifies the full
  native and desktop artifact set before publishing.
- CPU images support Linux AMD64 and ARM64; the NVIDIA CUDA image supports Linux
  AMD64.

## Container quick start

```bash
docker pull takuphilchan/offgrid-llm:0.4.2

docker run -d \
  --name offgrid \
  --init \
  --restart unless-stopped \
  --security-opt no-new-privileges=true \
  --cap-drop ALL \
  -p 127.0.0.1:11611:11611 \
  -v offgrid-models:/var/lib/offgrid/models \
  -v offgrid-data:/var/lib/offgrid/data \
  takuphilchan/offgrid-llm:0.4.2
```

Open <http://localhost:11611/ui/> after the health check becomes ready.

## Desktop and CLI packages

Release assets include Linux x64 and ARM64 CLI bundles, Linux x64 desktop
packages, macOS Apple Silicon and Intel desktop/CLI packages, and Windows x64
installer, portable desktop, and CLI packages.

Desktop packages are currently unsigned. Windows SmartScreen and macOS
Gatekeeper may therefore require explicit approval.

## Verify downloads

Download `checksums-v0.4.2.sha256` with the desired assets, then run:

```bash
sha256sum -c checksums-v0.4.2.sha256
```

See the [installation guide](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.2/docs/setup/installation.md),
[Docker guide](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.2/docs/setup/docker.md), and
[full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.3.1...v0.4.2).
