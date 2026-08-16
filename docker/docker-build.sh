#!/usr/bin/env bash
# Build (and optionally publish) OffGrid container images.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "${SCRIPT_DIR}")"
DEFAULT_VERSION="$(tr -d '\n\r ' < "${ROOT_DIR}/VERSION")"

VERSION="${1:-${DEFAULT_VERSION}}"
IMAGE="${IMAGE:-takuphilchan/offgrid-llm}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"
PUSH="${PUSH:-false}"
BUILD_GPU="${BUILD_GPU:-false}"
VCS_REF="$(git -C "${ROOT_DIR}" rev-parse HEAD 2>/dev/null || printf 'unknown')"
BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"

cd "${ROOT_DIR}"

echo "Building ${IMAGE}:${VERSION}"
if [[ "${PUSH}" == "true" ]]; then
  docker buildx build \
    --file docker/Dockerfile \
    --platform "${PLATFORMS}" \
    --build-arg "VERSION=${VERSION}" \
    --build-arg "VCS_REF=${VCS_REF}" \
    --build-arg "BUILD_DATE=${BUILD_DATE}" \
    --tag "${IMAGE}:${VERSION}" \
    --push .
  echo "Published ${IMAGE}:${VERSION} for ${PLATFORMS}"
else
  docker buildx build \
    --file docker/Dockerfile \
    --load \
    --build-arg "VERSION=${VERSION}" \
    --build-arg "VCS_REF=${VCS_REF}" \
    --build-arg "BUILD_DATE=${BUILD_DATE}" \
    --tag "${IMAGE}:${VERSION}" .
  echo "Built ${IMAGE}:${VERSION} for the local architecture"
fi

if [[ "${BUILD_GPU}" == "true" ]]; then
  if [[ "${PUSH}" == "true" ]]; then
    docker buildx build \
      --file docker/Dockerfile.gpu \
      --platform linux/amd64 \
      --build-arg "VERSION=${VERSION}" \
      --build-arg "VCS_REF=${VCS_REF}" \
      --build-arg "BUILD_DATE=${BUILD_DATE}" \
      --tag "${IMAGE}:${VERSION}-gpu" \
      --push .
    echo "Published ${IMAGE}:${VERSION}-gpu"
  else
    docker buildx build \
      --file docker/Dockerfile.gpu \
      --load \
      --build-arg "VERSION=${VERSION}" \
      --build-arg "VCS_REF=${VCS_REF}" \
      --build-arg "BUILD_DATE=${BUILD_DATE}" \
      --tag "${IMAGE}:${VERSION}-gpu" .
    echo "Built ${IMAGE}:${VERSION}-gpu"
  fi
fi

echo "Run locally:"
echo "  docker run -d --name offgrid -p 127.0.0.1:11611:11611 -v offgrid-models:/var/lib/offgrid/models -v offgrid-data:/var/lib/offgrid/data ${IMAGE}:${VERSION}"
