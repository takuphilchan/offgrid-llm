#!/bin/bash
# Build and push OffGrid LLM Docker images

set -e

# Read version from VERSION file if not provided
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
DEFAULT_VERSION=$(cat "$ROOT_DIR/VERSION" 2>/dev/null | tr -d '\n\r ' || echo "latest")

VERSION=${1:-$DEFAULT_VERSION}
REGISTRY=${REGISTRY:-docker.io/offgrid}
BUILD_GPU=${BUILD_GPU:-false}

echo "🐳 Building OffGrid LLM Docker image..."
echo "   Version: $VERSION"
echo "   Registry: $REGISTRY"
echo ""

cd "$ROOT_DIR"

# Build CPU image
echo "📦 Building CPU image..."
docker build \
  -f docker/Dockerfile \
  -t offgrid-llm:${VERSION} \
  -t offgrid-llm:latest \
  .

echo "✓ CPU image built: offgrid-llm:${VERSION}"

# Build GPU image if requested
if [ "$BUILD_GPU" = "true" ]; then
  echo ""
  echo "🎮 Building GPU image (CUDA)..."
  docker build \
    -f docker/Dockerfile.gpu \
    -t offgrid-llm:${VERSION}-gpu \
    -t offgrid-llm:gpu \
    .
  echo "✓ GPU image built: offgrid-llm:${VERSION}-gpu"
fi

echo ""
echo "===================="
echo "Build Complete!"
echo "===================="
echo ""
echo "To run the CPU image:"
echo "  docker run -d -p 11611:11611 -v offgrid-models:/var/lib/offgrid/models offgrid-llm:${VERSION}"
echo ""
if [ "$BUILD_GPU" = "true" ]; then
  echo "To run the GPU image:"
  echo "  docker run -d --gpus all -p 11611:11611 -v offgrid-models:/var/lib/offgrid/models offgrid-llm:gpu"
  echo ""
fi
echo "Or use docker-compose:"
echo "  cd docker && docker-compose up -d"
echo ""
echo "Access the UI at: http://localhost:11611/ui/"
