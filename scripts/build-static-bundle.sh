#!/bin/bash
# Build OffGrid LLM as a single static binary with embedded llama.cpp
# This creates a self-contained bundle like Ollama - no external dependencies!
# 
# Usage:
#   ./build-static-bundle.sh              # Auto-detect GPU and build
#   ./build-static-bundle.sh cpu          # Build CPU-only
#   ./build-static-bundle.sh vulkan       # Build with Vulkan GPU support
#   ./build-static-bundle.sh cuda         # Build with CUDA GPU support
#   ./build-static-bundle.sh rocm         # Build with ROCm GPU support
#   ./build-static-bundle.sh all          # Build all variants

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

print_step() { echo -e "${CYAN}▶${NC} $1"; }
print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1" >&2; }
print_warning() { echo -e "${YELLOW}⚠${NC} $1"; }

echo ""
echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║  Building OffGrid LLM Static Bundle (Like Ollama)         ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

BUILD_VARIANT="${1:-auto}"

BUILD_VARIANT="${1:-auto}"

# Check dependencies
print_step "Checking build dependencies..."
MISSING_DEPS=""
for cmd in git cmake make g++ go; do
    if ! command -v $cmd &> /dev/null; then
        MISSING_DEPS="$MISSING_DEPS $cmd"
    fi
done

if [ -n "$MISSING_DEPS" ]; then
    print_error "Missing required tools:$MISSING_DEPS"
    echo "Install with: sudo apt-get install -y git cmake build-essential golang"
    exit 1
fi
print_success "All dependencies found"

# Detect or set GPU variant
detect_gpu_variant() {
    case "$BUILD_VARIANT" in
        cpu)
            GPU_FLAGS=""
            GPU_SUFFIX="-cpu"
            print_success "Building CPU-only variant"
            ;;
        vulkan)
            if ! command -v vulkaninfo &> /dev/null || [ ! -f /usr/lib/x86_64-linux-gnu/libvulkan.so.1 ]; then
                print_error "Vulkan requested but not available"
                echo "Install with: sudo apt-get install -y vulkan-tools libvulkan-dev"
                exit 1
            fi
            GPU_FLAGS="-DGGML_VULKAN=ON"
            GPU_SUFFIX="-vulkan"
            print_success "Building Vulkan GPU variant"
            ;;
        cuda)
            if ! command -v nvcc &> /dev/null; then
                print_error "CUDA requested but nvcc not found"
                echo "Install CUDA toolkit from: https://developer.nvidia.com/cuda-downloads"
                exit 1
            fi
            GPU_FLAGS="-DGGML_CUDA=ON"
            GPU_SUFFIX="-cuda"
            print_success "Building CUDA GPU variant"
            ;;
        rocm)
            if ! command -v hipcc &> /dev/null; then
                print_error "ROCm requested but hipcc not found"
                echo "Install ROCm from: https://rocmdocs.amd.com/"
                exit 1
            fi
            GPU_FLAGS="-DGGML_HIPBLAS=ON"
            GPU_SUFFIX="-rocm"
            print_success "Building ROCm GPU variant"
            ;;
        all)
            print_success "Building all available variants..."
            REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
            
            # Build CPU
            echo ""
            print_step "Building CPU variant..."
            "$0" cpu
            
            # Build Vulkan if available
            if command -v vulkaninfo &> /dev/null && [ -f /usr/lib/x86_64-linux-gnu/libvulkan.so.1 ]; then
                echo ""
                print_step "Building Vulkan variant..."
                "$0" vulkan
            fi
            
            # Build CUDA if available
            if command -v nvcc &> /dev/null; then
                echo ""
                print_step "Building CUDA variant..."
                "$0" cuda
            fi
            
            # Build ROCm if available
            if command -v hipcc &> /dev/null; then
                echo ""
                print_step "Building ROCm variant..."
                "$0" rocm
            fi
            
            echo ""
            echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
            echo -e "${GREEN}║  All variants built successfully!                         ║${NC}"
            echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
            ls -lh "$REPO_ROOT"/offgrid-bundle-* 2>/dev/null || true
            exit 0
            ;;
        auto)
            # Auto-detect best GPU option
            if command -v nvcc &> /dev/null; then
                GPU_FLAGS="-DGGML_CUDA=ON"
                GPU_SUFFIX="-cuda"
                print_success "Auto-detected: CUDA GPU"
            elif command -v vulkaninfo &> /dev/null && [ -f /usr/lib/x86_64-linux-gnu/libvulkan.so.1 ]; then
                GPU_FLAGS="-DGGML_VULKAN=ON"
                GPU_SUFFIX="-vulkan"
                print_success "Auto-detected: Vulkan GPU"
            elif command -v hipcc &> /dev/null; then
                GPU_FLAGS="-DGGML_HIPBLAS=ON"
                GPU_SUFFIX="-rocm"
                print_success "Auto-detected: ROCm GPU"
            else
                GPU_FLAGS=""
                GPU_SUFFIX="-cpu"
                print_success "Auto-detected: CPU-only"
            fi
            ;;
        *)
            print_error "Unknown variant: $BUILD_VARIANT"
            echo "Usage: $0 [cpu|vulkan|cuda|rocm|all|auto]"
            exit 1
            ;;
    esac
}

detect_gpu_variant

detect_gpu_variant

# Get repo root and output name
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
esac
OUTPUT_NAME="offgrid-bundle-${OS}-${ARCH}${GPU_SUFFIX}"

# Clone or update llama.cpp
LLAMACPP_DIR="$HOME/.cache/offgrid-llm-build/llama.cpp"
if [ -d "$LLAMACPP_DIR" ]; then
    print_step "Updating llama.cpp..."
    cd "$LLAMACPP_DIR"
    git pull -q
else
    print_step "Cloning llama.cpp..."
    mkdir -p "$(dirname "$LLAMACPP_DIR")"
    git clone --quiet --depth 1 https://github.com/ggml-org/llama.cpp "$LLAMACPP_DIR"
    cd "$LLAMACPP_DIR"
fi

LLAMA_VERSION=$(git describe --tags --always)
print_success "llama.cpp version: $LLAMA_VERSION"

# Build llama.cpp as static library
print_step "Building llama.cpp with static backends..."
rm -rf build
mkdir -p build && cd build

cmake .. \
    -DBUILD_SHARED_LIBS=OFF \
    -DGGML_STATIC=ON \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
    $GPU_FLAGS

cmake --build . --config Release -j $(nproc)
print_success "llama.cpp built successfully"

# Return to offgrid repo
cd - > /dev/null

# Build offgrid with embedded llama.cpp
print_step "Building OffGrid LLM with embedded llama.cpp..."

# Set CGO flags to link against static llama.cpp
export CGO_ENABLED=1
export CGO_CFLAGS="-I${LLAMACPP_DIR}"
export CGO_LDFLAGS="-L${LLAMACPP_DIR}/build -lllama -lggml -lstdc++ -lm -lpthread"

# Add GPU-specific linker flags
if [[ "$GPU_FLAGS" == *"VULKAN"* ]]; then
    export CGO_LDFLAGS="$CGO_LDFLAGS -lvulkan"
elif [[ "$GPU_FLAGS" == *"CUDA"* ]]; then
    CUDA_PATH="${CUDA_PATH:-/usr/local/cuda}"
    export CGO_LDFLAGS="$CGO_LDFLAGS -L${CUDA_PATH}/lib64 -lcudart -lcublas -lcublasLt"
elif [[ "$GPU_FLAGS" == *"HIPBLAS"* ]]; then
    export CGO_LDFLAGS="$CGO_LDFLAGS -lhipblas -lrocblas"
fi

# Build with llama tag to use native CGO bindings
cd "$REPO_ROOT"

# Get version info
GIT_VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

go build -tags llama \
    -ldflags="-s -w -X main.Version=${GIT_VERSION} -X main.BuildTime=${BUILD_TIME} -X main.LlamaVersion=${LLAMA_VERSION}" \
    -o "$OUTPUT_NAME" \
    ./cmd/offgrid

if [ -f "$OUTPUT_NAME" ]; then
    SIZE=$(ls -lh "$OUTPUT_NAME" | awk '{print $5}')
    print_success "Built: $OUTPUT_NAME ($SIZE)"
    
    # Create SHA256 checksum
    sha256sum "$OUTPUT_NAME" > "$OUTPUT_NAME.sha256"
    
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  Success! Self-contained binary created                   ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Binary:   $REPO_ROOT/$OUTPUT_NAME"
    echo "Size:     $SIZE"
    echo "Variant:  ${BUILD_VARIANT}${GPU_SUFFIX}"
    echo "Checksum: $REPO_ROOT/$OUTPUT_NAME.sha256"
    echo ""
    echo "Install with:"
    echo "  sudo cp $OUTPUT_NAME /usr/local/bin/offgrid"
    echo ""
    echo "Test with:"
    echo "  ./$OUTPUT_NAME --version"
    echo ""
else
    print_error "Build failed"
    exit 1
fi
