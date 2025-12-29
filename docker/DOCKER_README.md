# Docker Quick Start

Run OffGrid LLM in Docker in 2 minutes.

## Quick Start (CPU)

```bash
# 1. Clone the repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# 2. Build and start
cd docker
docker-compose up -d --build

# 3. Open your browser
open http://localhost:11611/ui/

# 4. Download a model (from terminal or UI)
docker exec offgrid-llm offgrid download qwen2.5:0.5b-instruct-q4_k_m
```

That's it! OffGrid LLM is running.

## Quick Start (GPU - NVIDIA)

```bash
# 1. Install NVIDIA Container Toolkit (one-time setup)
distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
curl -s -L https://nvidia.github.io/libnvidia-container/gpgkey | sudo apt-key add -
curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | \
  sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit
sudo systemctl restart docker

# 2. Build and start with GPU
cd docker
docker-compose -f docker-compose.gpu.yml up -d --build

# 3. Verify GPU is detected
docker exec offgrid-llm-gpu nvidia-smi
```

## Common Commands

```bash
# View logs
docker-compose logs -f offgrid

# Stop the container
docker-compose down

# Restart the container
docker-compose restart

# Update to latest version
git pull
docker-compose up -d --build

# Access shell inside container
docker exec -it offgrid-llm sh

# Check status
docker-compose ps

# Download a model
docker exec offgrid-llm offgrid download llama3.2
```

## Volumes

OffGrid uses Docker volumes to persist data:

| Volume | Purpose |
|--------|---------|
| `offgrid-models` | Downloaded GGUF models |
| `offgrid-data` | Sessions, cache, settings |

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

# Remove volumes (⚠️ DELETES ALL DATA)
docker-compose down -v
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OFFGRID_PORT` | 11611 | Server port |
| `OFFGRID_GPU_LAYERS` | 0 | GPU layers (0=CPU, 99=all GPU) |
| `OFFGRID_MODELS_DIR` | /var/lib/offgrid/models | Models directory |

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
# Add memory limit to docker-compose.yml:
services:
  offgrid:
    deploy:
      resources:
        limits:
          memory: 8G
```

**GPU not detected:**
```bash
# Test GPU access
docker run --rm --gpus all nvidia/cuda:12.2.0-base-ubuntu22.04 nvidia-smi
```

## Production Deployment

For production with SSL and monitoring:
```bash
docker-compose -f docker-compose.prod.yml up -d --build
```

See [docs/setup/docker.md](../docs/setup/docker.md) for complete production guide.

---

**Need help?** [Open an issue](https://github.com/takuphilchan/offgrid-llm/issues)
