#!/bin/bash
# Validate Docker setup for OffGrid LLM

set -e

echo "🐳 OffGrid LLM - Docker Validation"
echo "=================================="
echo ""

# Check Docker
echo "✓ Checking Docker installation..."
if ! command -v docker &> /dev/null; then
    echo "✗ Docker is not installed"
    exit 1
fi
docker --version

# Check Docker Compose
echo "✓ Checking Docker Compose..."
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null 2>&1; then
    echo "✗ Docker Compose is not installed"
    exit 1
fi
docker-compose --version 2>/dev/null || docker compose version

# Validate Dockerfile
echo "✓ Validating Dockerfile..."
if [ ! -f "Dockerfile" ]; then
    echo "✗ Dockerfile not found"
    exit 1
fi
echo "  - Dockerfile exists"

# Validate docker-compose files
echo "✓ Validating docker-compose files..."
for file in docker-compose.yml docker-compose.gpu.yml docker-compose.prod.yml; do
    if [ ! -f "$file" ]; then
        echo "✗ $file not found"
        exit 1
    fi
    docker-compose -f "$file" config > /dev/null 2>&1 || docker compose -f "$file" config > /dev/null 2>&1
    echo "  - $file is valid"
done

# Check required files
echo "✓ Checking documentation..."
for file in DOCKER_README.md docs/DOCKER.md docs/QUICKSTART.md; do
    if [ ! -f "$file" ]; then
        echo "✗ $file not found"
        exit 1
    fi
    echo "  - $file exists"
done

# Check Go source
echo "✓ Checking source files..."
if [ ! -f "cmd/offgrid/main.go" ]; then
    echo "✗ main.go not found"
    exit 1
fi
echo "  - Go source files present"

# Check web UI
echo "✓ Checking web UI..."
if [ ! -d "web/ui" ]; then
    echo "✗ web/ui directory not found"
    exit 1
fi
echo "  - Web UI files present"

echo ""
echo "✅ All validations passed!"
echo ""
echo "Next steps:"
echo "  1. Build: docker build -t offgrid-llm ."
echo "  2. Run: docker-compose up -d"
echo "  3. Visit: http://localhost:11611/ui/"
echo ""
echo "For GPU support:"
echo "  docker-compose -f docker-compose.gpu.yml up -d"
echo ""
echo "For production:"
echo "  docker-compose -f docker-compose.prod.yml up -d"
