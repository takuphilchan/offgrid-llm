#!/usr/bin/env bash
# Validate OffGrid's container definitions. Set BUILD_IMAGE=true for a full
# local image build and startup smoke test.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "${SCRIPT_DIR}")"
BUILD_IMAGE="${BUILD_IMAGE:-false}"
VALIDATION_IMAGE="${VALIDATION_IMAGE:-offgrid-llm:validation}"
VALIDATION_CONTAINER="offgrid-validation"

cd "${ROOT_DIR}"

command -v docker >/dev/null || { echo "Docker is not installed." >&2; exit 1; }
docker info >/dev/null
docker compose version
docker buildx version

required_files=(
  docker/Dockerfile
  docker/Dockerfile.gpu
  docker/docker-compose.yml
  docker/docker-compose.dev.yml
  docker/docker-compose.gpu.yml
  docker/docker-compose.prod.yml
  web/app/package.json
  cmd/offgrid/main.go
)

for file in "${required_files[@]}"; do
  [[ -e "${file}" ]] || { echo "Missing required file: ${file}" >&2; exit 1; }
done

docker compose -f docker/docker-compose.yml config --quiet
docker compose \
  -f docker/docker-compose.yml \
  -f docker/docker-compose.dev.yml \
  config --quiet
docker compose -f docker/docker-compose.gpu.yml config --quiet
GRAFANA_ADMIN_PASSWORD=validation-only \
  docker compose -f docker/docker-compose.prod.yml config --quiet
bash -n docker/docker-build.sh

echo "Container definitions are valid."

if [[ "${BUILD_IMAGE}" != "true" ]]; then
  echo "Set BUILD_IMAGE=true to build and smoke-test ${VALIDATION_IMAGE}."
  exit 0
fi

cleanup() {
  docker rm -f "${VALIDATION_CONTAINER}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker buildx build \
  --file docker/Dockerfile \
  --load \
  --build-arg VERSION=validation \
  --tag "${VALIDATION_IMAGE}" .
docker run --rm "${VALIDATION_IMAGE}" version
[[ "$(docker run --rm --entrypoint id "${VALIDATION_IMAGE}" -u)" == "1000" ]] || {
  echo "Image does not run as the expected non-root UID 1000." >&2
  exit 1
}
docker run -d \
  --name "${VALIDATION_CONTAINER}" \
  --publish 127.0.0.1:11612:11611 \
  "${VALIDATION_IMAGE}" >/dev/null

for _ in $(seq 1 30); do
  if curl --fail --silent http://127.0.0.1:11612/health >/dev/null &&
     curl --fail --silent http://127.0.0.1:11612/ui/ >/dev/null; then
    echo "Image build and startup smoke test passed."
    exit 0
  fi
  sleep 2
done

docker logs "${VALIDATION_CONTAINER}" >&2
echo "Container did not become healthy." >&2
exit 1
