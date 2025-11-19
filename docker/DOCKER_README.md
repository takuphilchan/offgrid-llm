# Docker Quick Start

Run OffGrid LLM in Docker in 2 minutes.

## Quick Start

```bash
# 1. Clone and navigate to docker directory
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm/docker

# 2. Start the container
docker-compose up -d

# 3. Open your browser
open http://localhost:11611/ui/

# 4. Download a model (from terminal or UI)
docker exec offgrid-llm offgrid download qwen2.5:0.5b-instruct-q4_k_m
```

That's it! OffGrid LLM is running.

## Common Commands

```bash
# View logs
docker-compose logs -f offgrid

# Stop the container
docker-compose down

# Restart the container
docker-compose restart

# Update to latest version
docker-compose pull
docker-compose up -d

# Access shell inside container
docker exec -it offgrid-llm sh

# Check status
docker-compose ps
```

## GPU Support

### NVIDIA GPU

**Prerequisites:** Install NVIDIA Container Toolkit first (see [docs/DOCKER.md](../docs/DOCKER.md) for details).

**Start with GPU support:**
```bash
cd docker
docker-compose -f docker-compose.gpu.yml up -d
curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | \
  sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
  sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list

sudo apt-get update
sudo apt-get install -y nvidia-container-toolkit
sudo systemctl restart docker
```

**2. Start with GPU:**
```bash
docker-compose -f docker-compose.gpu.yml up -d
```

## Volumes

OffGrid uses Docker volumes to persist data:

```bash
# Backup models
docker run --rm \
  -v offgrid-llm_offgrid-models:/source \
  -v $(pwd)/backup:/backup \
  alpine tar czf /backup/models.tar.gz -C /source .

# Restore models
docker run --rm \
  -v offgrid-llm_offgrid-models:/target \
  -v $(pwd)/backup:/backup \
  alpine tar xzf /backup/models.tar.gz -C /target

# List volumes
docker volume ls | grep offgrid

# Remove volumes (DELETES ALL DATA)
docker-compose down -v
```

## Environment Variables

Configure via `.env` file or `docker-compose.yml`:

```bash
OFFGRID_PORT=11611           # Server port
OFFGRID_GPU_LAYERS=35        # GPU layers (0 = CPU only)
OFFGRID_MODELS_DIR=/models   # Models directory
```

## Troubleshooting

**Container won't start:**
```bash
docker-compose logs offgrid
```

**Port already in use:**
```bash
sudo lsof -i :11611
# Or change port in docker-compose.yml
```

**Out of memory:**
```bash
# Increase Docker memory limit in Docker Desktop settings
# Or add to docker-compose.yml:
services:
  offgrid:
    mem_limit: 8g
```

**GPU not detected:**
```bash
# Test GPU access
docker run --rm --gpus all nvidia/cuda:11.8.0-base-ubuntu22.04 nvidia-smi
```

## Complete Documentation

For advanced configuration, production deployment, SSL setup, monitoring, and more:

📖 **[docs/DOCKER.md](docs/DOCKER.md)** - Complete Docker guide

---

**Need help?** [Open an issue](https://github.com/takuphilchan/offgrid-llm/issues)
