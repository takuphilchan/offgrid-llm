# OffGrid container quick start

The published CPU image contains the OffGrid server, CLI, React UI, and a
pinned llama.cpp runtime. Models and application data remain outside the image
in named Docker volumes.

## Pull and run

```bash
docker pull takuphilchan/offgrid-llm:latest
docker run -d \
  --name offgrid \
  --init \
  --restart unless-stopped \
  --security-opt no-new-privileges=true \
  --cap-drop ALL \
  -p 127.0.0.1:11611:11611 \
  -v offgrid-models:/var/lib/offgrid/models \
  -v offgrid-data:/var/lib/offgrid/data \
  takuphilchan/offgrid-llm:latest
```

Open <http://localhost:11611/ui/>. The loopback port binding is intentional:
the default container is unauthenticated and must not be exposed publicly.

```bash
# Inspect server health
curl http://127.0.0.1:11611/health

# Use the CLI inside the running container
docker exec -it offgrid offgrid version
docker exec -it offgrid offgrid download tinyllama-1.1b-chat --yes

# Follow logs and stop without deleting data
docker logs -f offgrid
docker stop offgrid
docker rm offgrid
```

## Compose

From this directory:

```bash
cp .env.example .env
docker compose pull
docker compose up -d
docker compose ps
docker compose logs -f offgrid
```

Update without deleting the `offgrid-models` or `offgrid-data` volumes:

```bash
docker compose pull
docker compose up -d
```

Build the current checkout for development:

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.dev.yml \
  up -d --build
```

## NVIDIA image

The GPU image is Linux AMD64 only and requires NVIDIA Container Toolkit:

```bash
docker compose -f docker-compose.gpu.yml pull
docker compose -f docker-compose.gpu.yml up -d
```

Release tags use `<version>-gpu`, and the stable GPU tag is `latest-gpu`.

## Production

The production definition enables OffGrid authentication, terminates TLS at
Nginx, and keeps the application port on an internal network. Supply TLS files
and bootstrap an administrator before starting it:

```bash
cp /path/to/fullchain.pem certs/cert.pem
cp /path/to/private-key.pem certs/key.pem
cp .env.example .env

docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml run --rm offgrid users create admin admin
docker compose -f docker-compose.prod.yml up -d
```

Enable the optional Prometheus and Grafana services with:

```bash
docker compose -f docker-compose.prod.yml --profile monitoring up -d
```

## Validation

```bash
bash ./validate-docker.sh
BUILD_IMAGE=true bash ./validate-docker.sh
```

The second command performs a complete CPU image build, checks the embedded CLI,
starts the server on `127.0.0.1:11612`, and probes its health endpoint.

Removing a Compose project with `docker compose down` preserves named volumes.
Using `docker compose down --volumes` permanently deletes models and user data.
