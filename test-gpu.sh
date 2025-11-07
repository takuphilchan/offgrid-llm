#!/bin/bash
# Quick GPU detection test script

# Color codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color
DIM='\033[2m'

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  OffGrid LLM - GPU Detection Test${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

GPU_TYPE="none"

# Method 1: Check nvidia-smi
echo -e "${YELLOW}Testing Method 1: nvidia-smi${NC}"
if command -v nvidia-smi &> /dev/null && nvidia-smi &> /dev/null; then
    GPU_TYPE="nvidia"
    GPU_NAME=$(nvidia-smi --query-gpu=name --format=csv,noheader 2>/dev/null | head -n1)
    DRIVER=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader 2>/dev/null | head -n1)
    COMPUTE=$(nvidia-smi --query-gpu=compute_cap --format=csv,noheader 2>/dev/null | head -n1)
    echo -e "${GREEN}✓ NVIDIA GPU detected: $GPU_NAME${NC}"
    echo -e "${GREEN}  Driver: $DRIVER | Compute Capability: $COMPUTE${NC}"
else
    echo -e "${DIM}  nvidia-smi not available or not working${NC}"
fi

# Method 2: Check lspci for NVIDIA
echo ""
echo -e "${YELLOW}Testing Method 2: lspci (NVIDIA)${NC}"
if lspci 2>/dev/null | grep -i 'vga.*nvidia\|3d.*nvidia\|display.*nvidia' &> /dev/null; then
    GPU_INFO=$(lspci | grep -i 'vga.*nvidia\|3d.*nvidia\|display.*nvidia' | head -n1)
    echo -e "${GREEN}✓ Found in lspci: $GPU_INFO${NC}"
    if [ "$GPU_TYPE" = "none" ]; then
        GPU_TYPE="nvidia"
        echo -e "${YELLOW}  Note: nvidia-smi not working but GPU is present${NC}"
    fi
else
    echo -e "${DIM}  No NVIDIA GPU found in lspci${NC}"
fi

# Method 3: Check lspci for AMD
echo ""
echo -e "${YELLOW}Testing Method 3: lspci (AMD)${NC}"
if lspci 2>/dev/null | grep -i 'vga.*amd\|vga.*ati\|3d.*amd' &> /dev/null; then
    GPU_INFO=$(lspci | grep -i 'vga.*amd\|vga.*ati\|3d.*amd' | head -n1)
    echo -e "${GREEN}✓ Found in lspci: $GPU_INFO${NC}"
    if [ "$GPU_TYPE" = "none" ]; then
        GPU_TYPE="amd"
    fi
else
    echo -e "${DIM}  No AMD GPU found in lspci${NC}"
fi

# Method 4: Check /proc/driver/nvidia (WSL)
echo ""
echo -e "${YELLOW}Testing Method 4: /proc/driver/nvidia (WSL)${NC}"
if [ -d "/proc/driver/nvidia" ]; then
    echo -e "${GREEN}✓ NVIDIA driver directory found${NC}"
    if [ "$GPU_TYPE" = "none" ]; then
        GPU_TYPE="nvidia"
        echo -e "${YELLOW}  Likely running in WSL or container${NC}"
    fi
else
    echo -e "${DIM}  /proc/driver/nvidia not found${NC}"
fi

# CUDA Detection
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  CUDA Toolkit Detection${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

CUDA_FOUND=false

# Check nvcc in PATH
echo -e "${YELLOW}Checking nvcc in PATH:${NC}"
if command -v nvcc &> /dev/null; then
    CUDA_VERSION=$(nvcc --version 2>/dev/null | grep "release" | awk '{print $5}' | tr -d ',')
    CUDA_PATH=$(which nvcc | sed 's|/bin/nvcc||')
    echo -e "${GREEN}✓ Found: $CUDA_VERSION at $CUDA_PATH${NC}"
    CUDA_FOUND=true
else
    echo -e "${DIM}  nvcc not in PATH${NC}"
fi

# Check /usr/local/cuda
echo ""
echo -e "${YELLOW}Checking /usr/local/cuda:${NC}"
if [ -d "/usr/local/cuda" ] && [ -f "/usr/local/cuda/bin/nvcc" ]; then
    CUDA_VERSION=$(/usr/local/cuda/bin/nvcc --version 2>/dev/null | grep "release" | awk '{print $5}' | tr -d ',')
    echo -e "${GREEN}✓ Found: $CUDA_VERSION at /usr/local/cuda${NC}"
    CUDA_FOUND=true
else
    echo -e "${DIM}  /usr/local/cuda not found or no nvcc${NC}"
fi

# Check /usr/lib/cuda
echo ""
echo -e "${YELLOW}Checking /usr/lib/cuda:${NC}"
if [ -d "/usr/lib/cuda" ] && [ -f "/usr/lib/cuda/bin/nvcc" ]; then
    CUDA_VERSION=$(/usr/lib/cuda/bin/nvcc --version 2>/dev/null | grep "release" | awk '{print $5}' | tr -d ',')
    echo -e "${GREEN}✓ Found: $CUDA_VERSION at /usr/lib/cuda${NC}"
    CUDA_FOUND=true
else
    echo -e "${DIM}  /usr/lib/cuda not found or no nvcc${NC}"
fi

# Summary
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Summary${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "GPU Type: ${GREEN}$GPU_TYPE${NC}"
if [ "$GPU_TYPE" = "nvidia" ]; then
    if [ "$CUDA_FOUND" = true ]; then
        echo -e "Build Mode: ${GREEN}GPU Accelerated (CUDA)${NC}"
    else
        echo -e "Build Mode: ${YELLOW}CPU Only (CUDA toolkit not installed)${NC}"
        echo -e "${DIM}Install CUDA: https://developer.nvidia.com/cuda-downloads${NC}"
    fi
elif [ "$GPU_TYPE" = "amd" ]; then
    echo -e "Build Mode: ${GREEN}GPU Accelerated (ROCm)${NC}"
else
    echo -e "Build Mode: ${YELLOW}CPU Only${NC}"
fi
echo ""
