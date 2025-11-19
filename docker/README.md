# Docker Deployment

This directory contains all Docker-related files for containerized deployment of OffGrid LLM.

## Quick Start

```bash
cd docker
docker-compose up -d
```

Access the UI at http://localhost:11611/ui/

## Files Overview

### Core Files

- **`Dockerfile`** - Production-ready multi-stage Docker image
  - Alpine-based (~100MB final image)
  - Non-root user (uid 1000)
  - Health checks included
  - Optimized for production

- **`DOCKER_README.md`** - Quick start guide (2 minutes)
  - Basic commands
  - GPU setup
  - Common troubleshooting

### Deployment Configurations

- **`docker-compose.yml`** - Basic deployment
  - Single container setup
  - Volume persistence for models and data
  - Health monitoring
  - Auto-restart enabled

- **`docker-compose.gpu.yml`** - GPU-optimized deployment
  - NVIDIA GPU support
  - CUDA device passthrough
  - GPU resource limits
  - Performance-tuned settings

- **`docker-compose.prod.yml`** - Production stack
  - Nginx reverse proxy with SSL/TLS ready
  - Prometheus monitoring
  - Grafana dashboards
  - Network isolation
  - Production security hardening

### Configuration & Build

- **`nginx.conf.example`** - Nginx reverse proxy configuration
  - SSL/TLS termination template
  - WebSocket support
  - Rate limiting
  - Security headers

- **`docker-build.sh`** - Multi-architecture build automation
  - Builds for AMD64 and ARM64
  - Pushes to registry
  - Tags versions automatically

- **`validate-docker.sh`** - Automated validation
  - Checks Docker/Compose installation
  - Validates configuration files
  - Tests container startup
  - Verifies health endpoints

## Usage

### Basic Deployment

```bash
cd docker
docker-compose up -d
```

### GPU Deployment (NVIDIA)

**Prerequisites:** Install NVIDIA Container Toolkit first.

```bash
cd docker
docker-compose -f docker-compose.gpu.yml up -d
```

### Production Deployment

**With monitoring stack:**
```bash
cd docker
docker-compose -f docker-compose.prod.yml up -d
```

Access:
- OffGrid UI: http://localhost (via Nginx)
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000

### Validation

```bash
cd docker
./validate-docker.sh
```

## Common Commands

```bash
# View logs
docker-compose logs -f offgrid

# Stop containers
docker-compose down

# Update to latest
docker-compose pull
docker-compose up -d

# Shell access
docker exec -it offgrid-llm sh

# Check status
docker-compose ps
```

## Configuration

Create a `.env` file in this directory for custom configuration:

```env
# Server settings
OFFGRID_PORT=11611
OFFGRID_HOST=0.0.0.0

# GPU settings (for GPU deployments)
GPU_LAYERS=35
CUDA_VISIBLE_DEVICES=0

# Resource limits
MEMORY_LIMIT=4g
CPU_COUNT=4
```

## Volume Persistence

Data is persisted in Docker volumes:

- `offgrid_models` - Downloaded AI models
- `offgrid_data` - User data and configuration
- `prometheus_data` - Metrics (production stack)
- `grafana_data` - Dashboards (production stack)

**Backup volumes:**
```bash
docker run --rm -v offgrid_models:/models -v $(pwd):/backup alpine tar czf /backup/models-backup.tar.gz -C / models
```

**Restore volumes:**
```bash
docker run --rm -v offgrid_models:/models -v $(pwd):/backup alpine tar xzf /backup/models-backup.tar.gz -C /
```

## Architecture Differences

This production Docker setup differs from `../dev/Dockerfile`:

**Production (`docker/Dockerfile`):**
- Alpine-based (~100MB)
- Go binary only
- Optimized for deployment
- Minimal attack surface

**Development (`dev/Dockerfile`):**
- Ubuntu-based (~1GB)
- Builds llama.cpp from source
- Full build toolchain
- Development and testing focus

## Documentation

- **Quick Start:** [DOCKER_README.md](DOCKER_README.md)
- **Complete Guide:** [../docs/DOCKER.md](../docs/DOCKER.md)
- **General Docs:** [../docs/README.md](../docs/README.md)

## Troubleshooting

**Container won't start:**
```bash
./validate-docker.sh
docker-compose logs offgrid
```

**GPU not detected:**
```bash
docker run --rm --gpus all nvidia/cuda:12.0-base nvidia-smi
```

**Port already in use:**
```bash
# Change port in .env or docker-compose.yml
OFFGRID_PORT=11612
```

**Permission issues:**
```bash
# Fix volume permissions
docker-compose down
docker volume rm offgrid_models offgrid_data
docker-compose up -d
```

## Security Notes

For production deployments:

1. **Enable SSL/TLS** - Use `nginx.conf.example` as template
2. **Set strong passwords** - Configure Grafana admin password
3. **Network isolation** - Use production compose file's network setup
4. **Regular updates** - Keep base images updated
5. **Volume backups** - Implement regular backup strategy

## Support

- Issues: https://github.com/takuphilchan/offgrid-llm/issues
- Documentation: [../docs/](../docs/)
- Development: [../dev/README.md](../dev/README.md)
