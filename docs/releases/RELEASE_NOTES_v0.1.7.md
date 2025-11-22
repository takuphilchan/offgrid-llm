# OffGrid LLM v0.1.9 Release Notes

**Release Date:** November 19, 2025

This release focuses on repository organization, Docker deployment infrastructure, and improved developer experience.

---

## What's New

### Production-Ready Docker Support

Complete Docker deployment infrastructure with three deployment modes:

**Basic Deployment**
- Single-command setup with docker-compose
- Alpine-based image (~100MB)
- Persistent volumes for models and data
- Health monitoring and auto-restart

**GPU-Optimized Deployment**
- NVIDIA GPU support with CUDA passthrough
- Configurable GPU layers and resource limits
- Optimized for inference performance

**Production Stack**
- Nginx reverse proxy with SSL/TLS ready
- Prometheus monitoring and metrics
- Grafana dashboards for visualization
- Network isolation and security hardening

### Repository Organization

Cleaner, more maintainable repository structure:

**New Directory Structure:**
- `scripts/` - All build and installation scripts
- `docker/` - Complete Docker deployment files
- `docs/releases/` - Release notes archive
- `desktop/` - Desktop app with documentation

**Benefits:**
- Cleaner root directory
- Logical file grouping
- Better discoverability
- Each directory includes README

### Documentation Improvements

**New Documentation:**
- `docker/README.md` - Comprehensive Docker guide
- `docker/DOCKER_README.md` - Quick start (2 minutes)
- `docs/DOCKER.md` - Complete deployment documentation
- `docs/QUICKSTART.md` - 5-minute getting started guide
- `scripts/README.md` - Script documentation

**Enhanced Guides:**
- Docker-first installation approach
- Progressive disclosure (beginner to expert)
- Task-oriented structure
- Multiple entry points for different user levels

### UI/UX Enhancements

**Chat Interface:**
- Auto-scroll to latest messages
- No manual scrolling needed during conversations
- Smooth scrolling behavior

**USB Model Management:**
- Enhanced import/export functionality
- File browser for path selection
- Manifest generation with metadata
- SHA256 verification

---

## Installation

### Docker (Recommended)

```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm/docker
docker-compose up -d
```

Access UI: http://localhost:11611/ui/

### CLI Installation

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/scripts/install.sh | bash
```

### Desktop App

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.ps1 | iex
```

---

## File Changes

### Moved Files

**Scripts:**
- `install.sh` → `scripts/install.sh`
- `build-all.sh` → `scripts/build-all.sh`
- `start-server.sh` → `scripts/start-server.sh`

**Docker:**
- `Dockerfile` → `docker/Dockerfile`
- `docker-compose.yml` → `docker/docker-compose.yml`
- `docker-compose.gpu.yml` → `docker/docker-compose.gpu.yml`
- `docker-compose.prod.yml` → `docker/docker-compose.prod.yml`
- `docker-build.sh` → `docker/docker-build.sh`
- `validate-docker.sh` → `docker/validate-docker.sh`
- `nginx.conf.example` → `docker/nginx.conf.example`
- `DOCKER_README.md` → `docker/DOCKER_README.md`

**Documentation:**
- `DESKTOP_INSTALL.md` → `desktop/DESKTOP_INSTALL.md`
- `RELEASE_NOTES_v0.1.6_UPDATED.md` → `docs/releases/RELEASE_NOTES_v0.1.6_UPDATED.md`

### New Files

**Docker Infrastructure (7 files):**
- `docker/Dockerfile` - Production Docker image
- `docker/docker-compose.yml` - Basic deployment
- `docker/docker-compose.gpu.yml` - GPU support
- `docker/docker-compose.prod.yml` - Production stack
- `docker/docker-build.sh` - Build automation
- `docker/nginx.conf.example` - Reverse proxy config
- `docker/validate-docker.sh` - Validation script

**Documentation (5 files):**
- `docker/README.md` - Docker documentation hub
- `docker/DOCKER_README.md` - Quick start guide
- `docs/DOCKER.md` - Comprehensive Docker guide
- `docs/QUICKSTART.md` - Complete quick start
- `scripts/README.md` - Scripts documentation

**Backend (2 files):**
- `internal/models/usb_exporter.go` - USB model export
- `internal/models/usb_utils.go` - Shared USB utilities

**Configuration (1 file):**
- `.gitattributes` - Line ending configuration

### Modified Files

**Core:**
- `README.md` - Docker-first approach, updated paths
- `web/ui/index.html` - Auto-scroll, version 0.1.7
- `internal/server/server.go` - Version 0.1.7, file browser API
- `scripts/build-all.sh` - Version 0.1.7

**Documentation:**
- `docs/README.md` - Documentation hub update
- `docs/QUICKSTART.md` - Updated script paths
- `docs/INSTALLATION.md` - Updated references
- `internal/models/usb_importer.go` - Enhanced with manifest

---

## Technical Details

### Docker Architecture

**Production Image (docker/Dockerfile):**
- Alpine-based (~100MB final size)
- Multi-stage build
- Non-root user (uid 1000)
- Health checks included
- Optimized for production

**vs Development Image (dev/Dockerfile):**
- Ubuntu-based (~1GB)
- Builds llama.cpp from source
- Full build toolchain
- Development and testing focus

### Version Management

Version updated to 0.1.7 in:
- `web/ui/index.html` - UI display
- `internal/server/server.go` - API responses
- `scripts/build-all.sh` - Build version

---

## Upgrading from v0.1.6

### Docker Users

```bash
cd docker
docker-compose pull
docker-compose up -d
```

### CLI Users

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/scripts/install.sh | bash
```

### Manual Installation

Note: Script paths have changed. Update any custom automation:
- `install.sh` is now `scripts/install.sh`
- `build-all.sh` is now `scripts/build-all.sh`
- Docker files are in `docker/` directory

---

## Docker Quick Reference

### Basic Commands

```bash
# Start
cd docker && docker-compose up -d

# View logs
docker-compose logs -f offgrid

# Stop
docker-compose down

# Update
docker-compose pull && docker-compose up -d
```

### GPU Support

```bash
cd docker
docker-compose -f docker-compose.gpu.yml up -d
```

### Production Stack

```bash
cd docker
docker-compose -f docker-compose.prod.yml up -d
```

Access:
- OffGrid UI: http://localhost (via Nginx)
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000

---

## Breaking Changes

### Script Paths

Scripts have moved to organized directories. Update any automation:

**Before:**
```bash
./install.sh
./build-all.sh
```

**After:**
```bash
./scripts/install.sh
./scripts/build-all.sh
```

### Docker Files

Docker files moved to `docker/` directory:

**Before:**
```bash
docker-compose up -d
```

**After:**
```bash
cd docker
docker-compose up -d
```

---

## Bug Fixes

- Fixed chat interface scroll behavior
- Cleaned up repository structure
- Removed log files from repository
- Updated documentation paths

---

## Security

- Non-root Docker user (uid 1000)
- Production security hardening in docker-compose.prod.yml
- SSL/TLS ready with nginx.conf.example
- Network isolation in production stack

---

## Performance

- Alpine-based Docker image (90% size reduction vs Ubuntu)
- Multi-stage Docker builds for optimization
- GPU-optimized deployment configuration
- Resource limits in production stack

---

## Documentation

All documentation updated with new paths:
- Installation guides
- Docker deployment
- Quick start guides
- API references
- Build instructions

---

## Contributors

Thank you to everyone who contributed to this release!

---

## Full Changelog

https://github.com/takuphilchan/offgrid-llm/compare/v0.1.6...v0.1.9

---

## Support

- **Issues:** https://github.com/takuphilchan/offgrid-llm/issues
- **Documentation:** https://github.com/takuphilchan/offgrid-llm/tree/main/docs
- **Docker Guide:** https://github.com/takuphilchan/offgrid-llm/blob/main/docker/README.md
