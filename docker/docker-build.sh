#!/bin/bash
# Build and push OffGrid LLM Docker images

set -e

# Read version from VERSION file if not provided
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
DEFAULT_VERSION=$(cat "$ROOT_DIR/VERSION" 2>/dev/null | tr -d '\n\r ' || echo "latest")

VERSION=${1:-$DEFAULT_VERSION}
REGISTRY=${REGISTRY:-docker.io/offgrid}

echo "Building OffGrid LLM Docker image..."
echo "Version: $VERSION"
echo "Registry: $REGISTRY"

# Build for multiple platforms
docker buildx create --use --name offgrid-builder 2>/dev/null || docker buildx use offgrid-builder

# Build and push multi-arch image
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t ${REGISTRY}/offgrid-llm:${VERSION} \
  -t ${REGISTRY}/offgrid-llm:latest \
  --push \
  .

echo "✓ Successfully built and pushed:"
echo "  ${REGISTRY}/offgrid-llm:${VERSION}"
echo "  ${REGISTRY}/offgrid-llm:latest"

# Cleanup
docker buildx rm offgrid-builder

echo ""
echo "To run the image:"
echo "  docker run -d -p 11611:11611 ${REGISTRY}/offgrid-llm:${VERSION}"
