# Deployment Guide

This guide covers various deployment scenarios for OffGrid LLM in production environments.

## Table of Contents

- [Docker Deployment](#docker-deployment)
- [Systemd Service](#systemd-service)
- [Kubernetes](#kubernetes)
- [Air-Gapped Deployment](#air-gapped-deployment)
- [Multi-Node Setup](#multi-node-setup)
- [Production Best Practices](#production-best-practices)

## Docker Deployment

### Quick Start

```bash
# Build image (mock mode)
docker build -t offgrid-llm:latest .

# Run container
docker run -d \
  --name offgrid-llm \
  -p 11611:11611 \
  -p 8081:8081 \
  -v $(pwd)/models:/root/.offgrid/models \
  offgrid-llm:latest
```

### Using Docker Compose

```bash
# Start service
docker-compose up -d

# View logs
docker-compose logs -f offgrid-llm

# Stop service
docker-compose down

# Rebuild after changes
docker-compose up -d --build
```

### Build with llama.cpp Support

```bash
# Build image with real inference
docker build \
  --build-arg BUILD_MODE=llama \
  -t offgrid-llm:llama \
  .

# Run with GPU support (NVIDIA)
docker run -d \
  --name offgrid-llm \
  --gpus all \
  -p 11611:11611 \
  -v $(pwd)/models:/root/.offgrid/models \
  offgrid-llm:llama
```

### Configuration

Create `config.yaml` and mount it:

```yaml
server:
  port: 11611
  host: "0.0.0.0"

models:
  directory: "/root/.offgrid/models"
  auto_load: true

inference:
  num_threads: 4
  context_size: 4096
  enable_gpu: false

p2p:
  enabled: true
  discovery_port: 8081
```

Mount configuration:

```bash
docker run -d \
  -v $(pwd)/config.yaml:/root/.offgrid/config.yaml:ro \
  offgrid-llm:latest
```

## Systemd Service

For native Linux deployment without Docker.

### Installation

```bash
# Build binary
make build

# Copy to system location
sudo cp offgrid /usr/local/bin/
sudo chmod +x /usr/local/bin/offgrid

# Create system user
sudo useradd -r -s /bin/false -d /var/lib/offgrid offgrid

# Create directories
sudo mkdir -p /var/lib/offgrid/models
sudo mkdir -p /etc/offgrid
sudo chown -R offgrid:offgrid /var/lib/offgrid
```

### Service File

Create `/etc/systemd/system/offgrid.service`:

```ini
[Unit]
Description=OffGrid LLM - Offline AI Orchestrator
Documentation=https://github.com/takuphilchan/offgrid-llm
After=network.target

[Service]
Type=simple
User=offgrid
Group=offgrid
WorkingDirectory=/var/lib/offgrid

# Binary and configuration
ExecStart=/usr/local/bin/offgrid serve
Environment="OFFGRID_CONFIG=/etc/offgrid/config.yaml"
Environment="OFFGRID_MODELS_DIR=/var/lib/offgrid/models"

# Resource limits
MemoryLimit=8G
CPUQuota=400%

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/offgrid

# Restart policy
Restart=on-failure
RestartSec=10s
StartLimitInterval=5min
StartLimitBurst=3

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=offgrid

[Install]
WantedBy=multi-user.target
```

### Configuration File

Create `/etc/offgrid/config.yaml`:

```yaml
server:
  port: 11611
  host: "0.0.0.0"

models:
  directory: "/var/lib/offgrid/models"

inference:
  num_threads: 4
  context_size: 4096

p2p:
  enabled: true
  discovery_port: 8081
```

### Service Management

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable service (start on boot)
sudo systemctl enable offgrid

# Start service
sudo systemctl start offgrid

# Check status
sudo systemctl status offgrid

# View logs
sudo journalctl -u offgrid -f

# Stop service
sudo systemctl stop offgrid

# Restart service
sudo systemctl restart offgrid
```

## Kubernetes

Deploy OffGrid LLM in a Kubernetes cluster.

### Deployment Manifest

Create `k8s-deployment.yaml`:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: offgrid

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: offgrid-config
  namespace: offgrid
data:
  config.yaml: |
    server:
      port: 11611
      host: "0.0.0.0"
    models:
      directory: "/models"
    inference:
      num_threads: 4
      context_size: 4096

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: offgrid-models
  namespace: offgrid
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: offgrid-llm
  namespace: offgrid
spec:
  replicas: 1
  selector:
    matchLabels:
      app: offgrid-llm
  template:
    metadata:
      labels:
        app: offgrid-llm
    spec:
      containers:
      - name: offgrid
        image: offgrid-llm:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 11611
          name: http
        - containerPort: 8081
          name: p2p
        env:
        - name: OFFGRID_PORT
          value: "11611"
        - name: OFFGRID_MODELS_DIR
          value: "/models"
        volumeMounts:
        - name: models
          mountPath: /models
        - name: config
          mountPath: /root/.offgrid/config.yaml
          subPath: config.yaml
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
          limits:
            memory: "8Gi"
            cpu: "4"
        livenessProbe:
          httpGet:
            path: /health
            port: 11611
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 11611
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: models
        persistentVolumeClaim:
          claimName: offgrid-models
      - name: config
        configMap:
          name: offgrid-config

---
apiVersion: v1
kind: Service
metadata:
  name: offgrid-llm
  namespace: offgrid
spec:
  type: LoadBalancer
  selector:
    app: offgrid-llm
  ports:
  - name: http
    port: 11611
    targetPort: 11611
  - name: p2p
    port: 8081
    targetPort: 8081
```

### Deploy to Kubernetes

```bash
# Apply manifest
kubectl apply -f k8s-deployment.yaml

# Check status
kubectl get pods -n offgrid
kubectl get svc -n offgrid

# View logs
kubectl logs -f -n offgrid deployment/offgrid-llm

# Port forward for local testing
kubectl port-forward -n offgrid svc/offgrid-llm 11611:11611

# Delete deployment
kubectl delete -f k8s-deployment.yaml
```

## Air-Gapped Deployment

Deploy in environments with no internet access.

### Preparation (On Internet-Connected Machine)

```bash
# 1. Build binary
make build

# 2. Download models
./offgrid download tinyllama-1.1b-chat Q4_K_M
./offgrid download llama-2-7b-chat Q4_K_M

# 3. Create deployment package
mkdir -p offgrid-package/{bin,models,config,docs}

# Copy binary
cp offgrid offgrid-package/bin/

# Copy models
cp -r models/* offgrid-package/models/

# Create config
cat > offgrid-package/config/config.yaml <<EOF
server:
  port: 11611
  host: "0.0.0.0"
models:
  directory: "./models"
inference:
  num_threads: 4
  context_size: 4096
p2p:
  enabled: true
EOF

# Copy documentation
cp README.md offgrid-package/docs/
cp -r docs/* offgrid-package/docs/

# Create install script
cat > offgrid-package/install.sh <<'EOF'
#!/bin/bash
set -e

echo "Installing OffGrid LLM..."

# Copy binary
sudo cp bin/offgrid /usr/local/bin/
sudo chmod +x /usr/local/bin/offgrid

# Create directories
mkdir -p ~/.offgrid/models

# Copy models
cp -r models/* ~/.offgrid/models/

# Copy config
cp config/config.yaml ~/.offgrid/

echo "Installation complete!"
echo "Run: offgrid serve"
EOF

chmod +x offgrid-package/install.sh

# 4. Create archive
tar -czf offgrid-airgap.tar.gz offgrid-package/

# 5. Transfer to USB/SD card
cp offgrid-airgap.tar.gz /media/usb/
```

### Installation (On Air-Gapped Machine)

```bash
# Extract package
tar -xzf offgrid-airgap.tar.gz
cd offgrid-package

# Run installer
./install.sh

# Verify installation
offgrid list

# Start server
offgrid serve
```

## Multi-Node Setup

Deploy across multiple nodes with P2P model sharing.

### Node 1 (Primary)

```bash
# Configure with models
cat > ~/.offgrid/config.yaml <<EOF
server:
  port: 11611
models:
  directory: "./models"
p2p:
  enabled: true
  discovery_port: 8081
EOF

# Download models
./offgrid download tinyllama-1.1b-chat Q4_K_M
./offgrid download llama-2-7b-chat Q4_K_M

# Start server
./offgrid serve
```

### Node 2 (Secondary)

```bash
# Configure without models initially
cat > ~/.offgrid/config.yaml <<EOF
server:
  port: 11611
p2p:
  enabled: true
  discovery_port: 8081
EOF

# Start server (will discover Node 1)
./offgrid serve
```

### Verify P2P Discovery

```bash
# On any node, check discovered peers
curl http://localhost:11611/peers

# Check available models across network
curl http://localhost:11611/v1/models
```

## Production Best Practices

### Security

```bash
# Run as non-root user
sudo useradd -r -s /bin/false offgrid

# Restrict file permissions
chmod 600 ~/.offgrid/config.yaml
chmod 700 ~/.offgrid/models

# Use firewall
sudo ufw allow 11611/tcp
sudo ufw allow 8081/udp  # P2P only
sudo ufw enable
```

### Monitoring

```bash
# Add health check monitoring
*/5 * * * * curl -f http://localhost:11611/health || systemctl restart offgrid

# Log rotation (systemd handles this automatically)
# For manual logs:
sudo logrotate /etc/logrotate.d/offgrid
```

### Backup

```bash
# Backup models directory
tar -czf models-backup-$(date +%Y%m%d).tar.gz ~/.offgrid/models/

# Backup configuration
cp ~/.offgrid/config.yaml ~/.offgrid/config.yaml.backup
```

### Resource Limits

```bash
# Set memory limits (systemd)
MemoryLimit=8G

# Set CPU limits (systemd)
CPUQuota=400%  # 4 cores

# Monitor resource usage
watch -n 1 'curl -s http://localhost:11611/health | jq .resources'
```

### Scaling

**Horizontal Scaling:**
- Deploy multiple instances across nodes
- Enable P2P for model sharing
- Use load balancer for API requests

**Vertical Scaling:**
- Increase memory for larger models
- Add GPU for faster inference
- Use higher quantization (Q5_K_M, Q8_0)

### Troubleshooting

```bash
# Check service status
systemctl status offgrid

# View real-time logs
journalctl -u offgrid -f

# Check resource usage
docker stats offgrid-llm  # Docker
htop  # Native

# Test API
curl http://localhost:11611/health
curl http://localhost:11611/v1/models

# Validate configuration
./offgrid config validate
```

## Next Steps

- [API Documentation](API.md)
- [Model Setup Guide](MODEL_SETUP.md)
- [llama.cpp Integration](LLAMA_CPP_SETUP.md)
