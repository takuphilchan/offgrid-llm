# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned
- llama.cpp integration for actual inference
- P2P file transfer implementation
- Multi-model parallel serving
- Prometheus metrics endpoint
- Shell completion scripts

## [0.2.0-alpha] - 2025-11-10

### Added
- **Prompt Templates** - 10 built-in templates for common tasks
  - code-review, summarize, translate, explain, brainstorm
  - debug, document, refactor, test, cli
  - Commands: `template list`, `template show`, `template apply`
- **Response Caching** - LRU cache with configurable TTL
  - 1000 entry capacity, 1 hour default TTL
  - Automatic cleanup every 15 minutes
  - API endpoints: `GET /cache/stats`, `POST /cache/clear`
- **Batch Processing** - Parallel JSONL processing
  - Configurable concurrency (default: 4 workers)
  - Per-request metrics (duration, tokens/sec)
  - Command: `batch process <input.jsonl> <output.jsonl>`
- **Model Aliases** - Friendly names for models
  - Commands: `alias set`, `alias list`, `alias remove`
  - Persistent storage in `~/.offgrid/aliases.json`
- **Favorites System** - Star frequently used models
  - Commands: `favorite add`, `favorite list`, `favorite remove`
  - Visual indicators in CLI output

### Changed
- Updated README with new features documentation
- Enhanced PROJECT_STATUS with completion tracking
- Expanded CLI help text to include new commands

## [0.1.0-alpha] - 2024-01-XX

### Added
- Initial release with core functionality
- OpenAI-compatible HTTP API (`/v1/chat/completions`, `/v1/completions`, `/v1/models`)
- Model registry with automatic filesystem scanning
- Mock inference engine for testing
- Resource monitoring (CPU, memory, GPU detection)
- Configuration via environment variables
- P2P discovery skeleton (UDP broadcast)
- Model catalog with 4 curated models:
  - TinyLlama 1.1B
  - Llama 2 7B
  - Mistral 7B
  - Phi-2
- Download manager with:
  - Resume support (Range requests)
  - SHA256 verification
  - Progress tracking
  - Multi-source fallback
- CLI commands:
  - `serve` - Start HTTP server
  - `download <model>` - Download models from catalog
  - `list` - List installed models
  - `catalog` - Browse available models
  - `info`/`status` - System information
  - `help` - Command help
- USB distribution package creator script
- Comprehensive documentation:
  - README.md - Project overview
  - API.md - API reference
  - MODEL_SETUP.md - Model download guide
  - ARCHITECTURE.md - Multi-tier distribution design
  - CONTRIBUTING.md - Contribution guidelines
- Example clients (Bash, Python)
- Unit tests for server handlers (9 tests)
- Makefile with build/test/run targets
- GitHub repository setup (private)

### Technical Details
- Go 1.21.5
- OpenAI-compatible API design
- Multi-tier distribution architecture:
  - Tier 1: Online from HuggingFace
  - Tier 2: P2P local network
  - Tier 3: USB/SD card
  - Tier 4: Sneakernet updates
- Zero hosting costs (uses HuggingFace CDN)
- Graceful shutdown with signal handling
- Logging middleware
- CORS support for web clients

### Known Limitations
- Mock inference only (no actual LLM execution yet)
- No streaming responses yet
- P2P discovery only (no file transfer)
- No authentication/authorization
- Single-user only

## [0.0.1] - 2024-01-XX

### Added
- Project scaffolding
- Basic HTTP server structure
- Initial Go module setup

---

## Release Notes Format

### Added
For new features.

### Changed
For changes in existing functionality.

### Deprecated
For soon-to-be removed features.

### Removed
For now removed features.

### Fixed
For any bug fixes.

### Security
In case of vulnerabilities.
