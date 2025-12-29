# Docker Deployment Guide for OffGrid LLM

Complete guide for deploying OffGrid LLM using Docker and Docker Compose.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Docker Installation](#docker-installation)
- [Building the Image](#building-the-image)
- [Running with Docker Compose](#running-with-docker-compose)
- [GPU Support](#gpu-support)
- [Volume Management](#volume-management)
- [Environment Variables](#environment-variables)
- [Production Deployment](#production-deployment)
- [Troubleshooting](#troubleshooting)

---

## Quick Start

**Get up and running in 2 minutes:**

```bash
# Clone the repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Start with Docker Compose
docker-compose up -d

# Access the UI
open http://localhost:11611/ui/
```

**That's it!** OffGrid LLM is now running in a container.

---

## Docker Installation

### Install Docker

**Linux:**
```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# Log out and back in for group changes
```

**macOS:**
- Download [Docker Desktop for Mac](https://www.docker.com/products/docker-desktop/)

**Windows:**
- Download [Docker Desktop for Windows](https://www.docker.com/products/docker-desktop/)

### Install Docker Compose

Docker Compose is included with Docker Desktop. For Linux:

```bash
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
```

---

## Building the Image

### Build Locally

```bash
# Build the image
docker build -t offgrid-llm:latest .

# Run the container
docker run -d \
  --name offgrid \
  -p 11611:11611 \
  -v offgrid-models:/var/lib/offgrid/models \
  offgrid-llm:latest
```

### Build with Custom Tag

```bash
docker build -t myregistry/offgrid-llm:v0.2.11 .
```

### Multi-Platform Build

```bash
# Build for multiple architectures
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t offgrid-llm:latest \
  --push .
```

---

## Running with Docker Compose

### Basic Usage

```bash
# Start in background
docker-compose up -d

# View logs
docker-compose logs -f

# Stop containers
docker-compose down

# Stop and remove volumes (DELETES ALL DATA)
docker-compose down -v
```

### Custom Configuration

Create a `.env` file in the project root:

```bash
# .env file
OFFGRID_PORT=11611
OFFGRID_GPU_LAYERS=35
OFFGRID_MODELS_DIR=/var/lib/offgrid/models
```

Then reference in `docker-compose.yml`:

```yaml
environment:
  - OFFGRID_PORT=${OFFGRID_PORT}
  - OFFGRID_GPU_LAYERS=${OFFGRID_GPU_LAYERS}
```

---

## GPU Support

### NVIDIA GPU (CUDA)

**Prerequisites:**
- [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html)

**Installation:**
```bash
# Ubuntu/Debian
distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | sudo tee /etc/apt/sources.list.d/nvidia-docker.list

sudo apt-get update
sudo apt-get install -y nvidia-container-toolkit
sudo systemctl restart docker
```

**Docker Compose with GPU:**

Uncomment the GPU section in `docker-compose.yml`:

```yaml
services:
  offgrid:
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
```

**Test GPU Access:**
```bash
docker run --rm --gpus all nvidia/cuda:11.8.0-base-ubuntu22.04 nvidia-smi
```

### AMD GPU (ROCm)

```bash
docker run -d \
  --name offgrid \
  --device=/dev/kfd \
  --device=/dev/dri \
  --group-add video \
  -p 11611:11611 \
  offgrid-llm:latest
```

### Apple Silicon (Metal)

Docker Desktop for Mac automatically provides GPU access. No additional configuration needed.

---

## Volume Management

### Persistent Storage

OffGrid LLM uses Docker volumes to persist data:

```yaml
volumes:
  offgrid-models:     # Model files
  offgrid-data:       # Session data, configs
```

### Inspect Volumes

```bash
# List volumes
docker volume ls

# Inspect a volume
docker volume inspect offgrid-llm_offgrid-models

# View volume data
docker run --rm -v offgrid-llm_offgrid-models:/data alpine ls -lah /data
```

### Backup Models

```bash
# Backup models to local directory
docker run --rm \
  -v offgrid-llm_offgrid-models:/source \
  -v $(pwd)/backup:/backup \
  alpine tar czf /backup/models-backup.tar.gz -C /source .

# Restore from backup
docker run --rm \
  -v offgrid-llm_offgrid-models:/target \
  -v $(pwd)/backup:/backup \
  alpine tar xzf /backup/models-backup.tar.gz -C /target
```

### Mount Local Models Directory

Edit `docker-compose.yml`:

```yaml
volumes:
  - ./models:/var/lib/offgrid/models
```

This mounts your local `./models` directory instead of using a Docker volume.

---

## Environment Variables

### Available Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OFFGRID_PORT` | `11611` | HTTP server port |
| `OFFGRID_HOST` | `0.0.0.0` | Bind address |
| `OFFGRID_MODELS_DIR` | `/var/lib/offgrid/models` | Models directory |
| `OFFGRID_GPU_LAYERS` | `0` | GPU layers (0 = CPU only) |
| `OFFGRID_MOCK_ENGINE` | `false` | Use mock engine for testing |

### Set via Docker Run

```bash
docker run -d \
  -e OFFGRID_GPU_LAYERS=35 \
  -e OFFGRID_PORT=8080 \
  -p 8080:8080 \
  offgrid-llm:latest
```

### Set via Docker Compose

```yaml
environment:
  - OFFGRID_GPU_LAYERS=35
  - OFFGRID_PORT=8080
```

---

## Production Deployment

### With SSL/TLS (Nginx Reverse Proxy)

**1. Create `nginx.conf`:**

```nginx
events {
    worker_connections 1024;
}

http {
    upstream offgrid {
        server offgrid:11611;
    }

    server {
        listen 80;
        server_name your-domain.com;
        return 301 https://$server_name$request_uri;
    }

    server {
        listen 443 ssl http2;
        server_name your-domain.com;

        ssl_certificate /etc/nginx/certs/cert.pem;
        ssl_certificate_key /etc/nginx/certs/key.pem;

        location / {
            proxy_pass http://offgrid;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            
            # WebSocket support for streaming
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection "upgrade";
        }
    }
}
```

**2. Update `docker-compose.yml`:**

```yaml
services:
  nginx:
    image: nginx:alpine
    container_name: offgrid-nginx
    restart: unless-stopped
    ports:
      - "443:443"
      - "80:80"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./certs:/etc/nginx/certs:ro
    depends_on:
      - offgrid
```

**3. Generate SSL certificate:**

```bash
# Self-signed (development)
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout certs/key.pem -out certs/cert.pem

# Let's Encrypt (production)
docker run -it --rm \
  -v $(pwd)/certs:/etc/letsencrypt \
  certbot/certbot certonly --standalone \
  -d your-domain.com
```

### Resource Limits

```yaml
services:
  offgrid:
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G
        reservations:
          cpus: '2'
          memory: 4G
```

### Monitoring with Prometheus

```yaml
services:
  prometheus:
    image: prom/prometheus
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    ports:
      - "9090:9090"
    
  grafana:
    image: grafana/grafana
    volumes:
      - grafana-data:/var/lib/grafana
    ports:
      - "3000:3000"
```

---

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker-compose logs offgrid

# Check if port is in use
sudo lsof -i :11611

# Check container status
docker ps -a
```

### Permission Issues

```bash
# Fix volume permissions
docker run --rm -v offgrid-llm_offgrid-models:/data alpine chown -R 1000:1000 /data
```

### Models Not Persisting

Ensure you're using volumes instead of bind mounts:

```yaml
# Correct - uses named volume
volumes:
  - offgrid-models:/var/lib/offgrid/models

# May have permission issues on some systems
volumes:
  - ./models:/var/lib/offgrid/models
```

### Health Check Failing

```bash
# Test health endpoint
docker exec offgrid curl http://localhost:11611/health

# Check if service is running
docker exec offgrid ps aux
```

### GPU Not Detected

```bash
# Check NVIDIA runtime
docker run --rm --gpus all nvidia/cuda:11.8.0-base-ubuntu22.04 nvidia-smi

# Check container GPU access
docker exec offgrid nvidia-smi
```

### Out of Memory

Increase Docker memory limits:

**Docker Desktop:** Settings → Resources → Memory

**Linux:**
```bash
# Edit daemon.json
sudo nano /etc/docker/daemon.json
```

```json
{
  "default-runtime": "nvidia",
  "default-shm-size": "2G"
}
```

---

## Advanced Usage

### Multi-Container Setup

Run multiple OffGrid instances with load balancing:

```yaml
services:
  offgrid-1:
    image: offgrid-llm:latest
    ports:
      - "11611:11611"
  
  offgrid-2:
    image: offgrid-llm:latest
    ports:
      - "11612:11611"
  
  haproxy:
    image: haproxy:alpine
    volumes:
      - ./haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro
    ports:
      - "80:80"
```

### Custom Entrypoint

```yaml
services:
  offgrid:
    entrypoint: ["/bin/sh", "-c"]
    command:
      - |
        echo "Starting OffGrid LLM..."
        /usr/local/bin/offgrid
```

### Development Mode

```yaml
services:
  offgrid-dev:
    build:
      context: .
      dockerfile: Dockerfile
      target: builder
    volumes:
      - .:/build
    working_dir: /build
    command: go run cmd/offgrid/main.go
```

---

## Next Steps

- [API Documentation](API.md)
- [Model Setup Guide](../guides/models.md)
- [Performance Tuning](../advanced/performance.md)
- [Production Deployment](../advanced/deployment.md)

---

**Need help?** Open an issue on [GitHub](https://github.com/takuphilchan/offgrid-llm/issues).
