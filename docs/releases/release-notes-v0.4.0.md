# OffGrid LLM v0.4.0

OffGrid LLM v0.4.0 delivers a unified workspace experience across the web,
desktop application, and command line. This release focuses on making the
system clearer to navigate, easier to operate, and safer to distribute.

## Highlights

- A redesigned monochrome workspace shared by the browser and Electron,
  including responsive navigation, system-aware light and dark themes, and
  locally bundled IBM Plex fonts.
- A command palette, dedicated Activity and Settings pages, durable agent
  workspace routes, and clearer separation between work, library, and system
  areas.
- Rich chat responses with GitHub-flavored Markdown, code and response copy
  actions, a growing composer, and privacy-preserving handling of remote
  images.
- Improved first-run behavior that only marks onboarding complete after a real
  assistant response, plus clearer model recommendations and knowledge-state
  presentation.
- Updated English, French, Spanish, Swahili, and Arabic UI foundations.
- A unified monochrome terminal language for interactive chat, agent tasks,
  server startup, redirected output, light and dark terminals, and ASCII-only
  environments.
- Explicit terminal controls for automation through `--json`, `--no-color`,
  `NO_COLOR`, `OFFGRID_UNICODE`, `OFFGRID_TUI`, and `OFFGRID_PLAIN`.

## Reliability and distribution

- Browser tests can run against an isolated local UI server or the packaged Go
  service used by CI.
- The release workflow now handles sanitized Electron asset names safely on
  retries and verifies every required artifact before publication.
- Windows llama.cpp builds use a reviewed platform-specific revision, while
  Linux, macOS, and container builds remain pinned for reproducibility.
- CPU images are verified for Linux AMD64 and ARM64, and the NVIDIA image is
  verified for Linux AMD64 before a release is finalized.
- Release assets must be uploaded successfully and carry valid SHA-256 digests
  before the release notes and checksum manifest are published.

## Container quick start

```bash
docker pull takuphilchan/offgrid-llm:0.4.0

docker run -d \
  --name offgrid \
  --init \
  --restart unless-stopped \
  --security-opt no-new-privileges=true \
  --cap-drop ALL \
  -p 127.0.0.1:11611:11611 \
  -v offgrid-models:/var/lib/offgrid/models \
  -v offgrid-data:/var/lib/offgrid/data \
  takuphilchan/offgrid-llm:0.4.0
```

Open <http://localhost:11611/ui/> after the health check becomes ready.

## Desktop and CLI packages

Release assets include:

- Linux x64 desktop packages and CPU/Vulkan CLI bundles.
- Linux ARM64 CLI bundles.
- macOS Apple Silicon and Intel desktop/CLI packages.
- Windows x64 installer, portable desktop package, and CLI bundle.

Desktop packages are currently unsigned. Windows SmartScreen and macOS
Gatekeeper may therefore require explicit approval. Code signing and
notarization remain required before unattended enterprise deployment.

## Verify downloads

Download `checksums-v0.4.0.sha256` with the desired assets, then run:

```bash
sha256sum -c checksums-v0.4.0.sha256
```

See the [installation guide](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.0/docs/setup/installation.md),
[Docker guide](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.0/docs/setup/docker.md), and
[full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.3.1...v0.4.0).
