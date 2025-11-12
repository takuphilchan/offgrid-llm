# OffGrid LLM Easy Installer for Windows
# Usage: irm https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.ps1 | iex

$ErrorActionPreference = "Stop"

# Banner
Write-Host ""
Write-Host "╔═══════════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║                                                               ║" -ForegroundColor Cyan
Write-Host "║     ██████╗ ███████╗███████╗ ██████╗ ██████╗ ██╗██████╗      ║" -ForegroundColor Cyan
Write-Host "║    ██╔═══██╗██╔════╝██╔════╝██╔════╝ ██╔══██╗██║██╔══██╗     ║" -ForegroundColor Cyan
Write-Host "║    ██║   ██║█████╗  █████╗  ██║  ███╗██████╔╝██║██║  ██║     ║" -ForegroundColor Cyan
Write-Host "║    ██║   ██║██╔══╝  ██╔══╝  ██║   ██║██╔══██╗██║██║  ██║     ║" -ForegroundColor Cyan
Write-Host "║    ╚██████╔╝██║     ██║     ╚██████╔╝██║  ██║██║██████╔╝     ║" -ForegroundColor Cyan
Write-Host "║     ╚═════╝ ╚═╝     ╚═╝      ╚═════╝ ╚═╝  ╚═╝╚═╝╚═════╝      ║" -ForegroundColor Cyan
Write-Host "║                                                               ║" -ForegroundColor Cyan
Write-Host "║               E A S Y   I N S T A L L E R                     ║" -ForegroundColor Cyan
Write-Host "║                                                               ║" -ForegroundColor Cyan
Write-Host "╔═══════════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host ""

function Write-Step {
    param($Message)
    Write-Host "[$(Get-Date -Format 'HH:mm:ss')] " -NoNewline -ForegroundColor Cyan
    Write-Host $Message
}

function Write-Success {
    param($Message)
    Write-Host "✓ " -NoNewline -ForegroundColor Green
    Write-Host $Message
}

function Write-Error {
    param($Message)
    Write-Host "✗ " -NoNewline -ForegroundColor Red
    Write-Host $Message -ForegroundColor Red
}

function Write-Warning {
    param($Message)
    Write-Host "⚠ " -NoNewline -ForegroundColor Yellow
    Write-Host $Message -ForegroundColor Yellow
}

# Detect platform
Write-Host ""
Write-Step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
Write-Step "  STEP 1/3: System Check"
Write-Step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
Write-Host ""

$arch = if ([Environment]::Is64BitOperatingSystem) { "x64" } else { "arm64" }
$gpuInfo = ""
$gpuVariant = "cpu"

# Detect GPU
try {
    $nvidiaCheck = nvidia-smi 2>$null
    if ($LASTEXITCODE -eq 0) {
        $gpuName = (nvidia-smi --query-gpu=name --format=csv,noheader 2>$null) -split "`n" | Select-Object -First 1
        Write-Success "NVIDIA GPU detected: $gpuName"
        
        # Check for CUDA
        if (Test-Path "C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA") {
            $gpuVariant = "cuda-12.4"
            $gpuInfo = " (CUDA GPU)"
            Write-Success "Using CUDA-accelerated binary"
        } else {
            Write-Warning "CUDA not found. Installing CPU-only binary."
            Write-Warning "For GPU acceleration, install CUDA Toolkit:"
            Write-Host "  https://developer.nvidia.com/cuda-downloads"
            $gpuInfo = " (CPU-only - install CUDA for GPU)"
        }
    }
} catch {
    # No NVIDIA GPU
}

Write-Success "Detected: windows-$arch$gpuInfo"

# Install Directory
$installDir = "$env:LOCALAPPDATA\offgrid-llm"
$binDir = "$installDir\bin"

Write-Host ""
Write-Step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
Write-Step "  STEP 2/3: Install llama.cpp (Inference Engine)"
Write-Step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
Write-Host ""

# Check if llama-server already exists
if (Test-Path "$binDir\llama-server.exe") {
    Write-Success "llama.cpp already installed"
} else {
    Write-Step "Installing llama.cpp (inference engine)..."
    
    # Get latest release
    Write-Step "Fetching latest llama.cpp release..."
    $llamaCppRelease = Invoke-RestMethod "https://api.github.com/repos/ggml-org/llama.cpp/releases/latest"
    $llamaCppVersion = $llamaCppRelease.tag_name
    Write-Success "Latest llama.cpp: $llamaCppVersion"
    
    # Build filename
    if ($gpuVariant -eq "cuda-12.4") {
        $llamaCppFile = "llama-$llamaCppVersion-bin-win-cuda-12.4-$arch.zip"
    } else {
        $llamaCppFile = "llama-$llamaCppVersion-bin-win-cpu-$arch.zip"
    }
    
    $llamaCppUrl = "https://github.com/ggml-org/llama.cpp/releases/download/$llamaCppVersion/$llamaCppFile"
    
    # Download
    $tempDir = New-Item -ItemType Directory -Path "$env:TEMP\offgrid-install-$(Get-Random)" -Force
    $llamaCppZip = "$tempDir\llama.zip"
    
    Write-Step "Downloading llama.cpp..."
    try {
        Invoke-WebRequest -Uri $llamaCppUrl -OutFile $llamaCppZip -UseBasicParsing
    } catch {
        if ($gpuVariant -ne "cpu") {
            Write-Warning "GPU binary download failed, trying CPU-only version..."
            $llamaCppFile = "llama-$llamaCppVersion-bin-win-cpu-$arch.zip"
            $llamaCppUrl = "https://github.com/ggml-org/llama.cpp/releases/download/$llamaCppVersion/$llamaCppFile"
            Invoke-WebRequest -Uri $llamaCppUrl -OutFile $llamaCppZip -UseBasicParsing
            $gpuInfo = " (CPU-only - GPU binary unavailable)"
        } else {
            throw
        }
    }
    
    # Extract
    Write-Step "Extracting llama.cpp..."
    Expand-Archive -Path $llamaCppZip -DestinationPath "$tempDir\llama" -Force
    
    # Check if llama-server.exe exists
    if (-not (Test-Path "$tempDir\llama\llama-server.exe")) {
        Write-Error "Could not find llama-server.exe in archive"
        exit 1
    }
    
    # Create install directory
    New-Item -ItemType Directory -Path $binDir -Force | Out-Null
    
    # Copy all binaries and DLLs
    Write-Step "Installing llama-server to $binDir..."
    Copy-Item "$tempDir\llama\*" -Destination $binDir -Recurse -Force
    
    # Cleanup
    Remove-Item -Path $tempDir -Recurse -Force
    
    Write-Success "llama.cpp installed successfully!"
}

# Install OffGrid
Write-Host ""
Write-Step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
Write-Step "  STEP 3/3: Install OffGrid LLM"
Write-Step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
Write-Host ""

Write-Step "Installing OffGrid LLM..."

# Get latest release
Write-Step "Fetching latest OffGrid release..."
$offgridRelease = Invoke-RestMethod "https://api.github.com/repos/takuphilchan/offgrid-llm/releases/latest"
$offgridVersion = $offgridRelease.tag_name
Write-Success "Latest OffGrid: $offgridVersion"

# Download
$offgridFile = if ($arch -eq "x64") { "offgrid-windows-amd64.zip" } else { "offgrid-windows-arm64.zip" }
$offgridUrl = "https://github.com/takuphilchan/offgrid-llm/releases/download/$offgridVersion/$offgridFile"
$offgridZip = "$env:TEMP\offgrid.zip"

Write-Step "Downloading OffGrid..."
try {
    Invoke-WebRequest -Uri $offgridUrl -OutFile $offgridZip -UseBasicParsing
} catch {
    Write-Warning "Version $offgridVersion not available yet, trying v0.1.2..."
    $offgridVersion = "v0.1.2"
    $offgridUrl = "https://github.com/takuphilchan/offgrid-llm/releases/download/$offgridVersion/$offgridFile"
    Invoke-WebRequest -Uri $offgridUrl -OutFile $offgridZip -UseBasicParsing
}

Write-Step "Extracting OffGrid..."
try {
    Expand-Archive -Path $offgridZip -DestinationPath "$env:TEMP\offgrid-extract" -Force
} catch {
    Write-Error "Failed to extract OffGrid archive"
    Write-Warning "This may require administrator privileges"
    Write-Warning "Try running PowerShell as Administrator and run the installer again"
    exit 1
}

# Find and rename the binary
$extractedBinary = Get-ChildItem "$env:TEMP\offgrid-extract" -Filter "offgrid*.exe" -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $extractedBinary) {
    Write-Error "OffGrid binary not found in archive"
    Write-Warning "Download may be corrupted. Please try again."
    exit 1
}

try {
    Copy-Item $extractedBinary.FullName -Destination "$binDir\offgrid.exe" -Force -ErrorAction Stop
} catch {
    Write-Error "Failed to install OffGrid binary"
    Write-Warning "This may require administrator privileges"
    Write-Warning "Try running PowerShell as Administrator and run the installer again"
    exit 1
}
Remove-Item "$env:TEMP\offgrid-extract" -Recurse -Force

# Verify
if (Test-Path "$binDir\offgrid.exe") {
    Write-Success "OffGrid installed successfully!"
    
    $version = & "$binDir\offgrid.exe" version 2>&1 | Select-String "v\d+\.\d+\.\d+" | ForEach-Object { $_.Matches.Value }
    Write-Success "Installation verified: $version"
} else {
    Write-Error "OffGrid installation failed"
    exit 1
}

# Add to PATH
Write-Step "Adding to PATH..."
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$binDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$currentPath;$binDir", "User")
    Write-Success "Added to PATH (restart terminal to use)"
}

# Success banner
Write-Host ""
Write-Host "╔═══════════════════════════════════════════════════════════════╗" -ForegroundColor Green
Write-Host "║                                                               ║" -ForegroundColor Green
Write-Host "║              🎉  INSTALLATION COMPLETE!  🎉                   ║" -ForegroundColor Green
Write-Host "║                                                               ║" -ForegroundColor Green
Write-Host "╚═══════════════════════════════════════════════════════════════╝" -ForegroundColor Green
Write-Host ""

Write-Success "All components installed successfully!"
Write-Host ""
Write-Host "  Installed:"
Write-Host "    • OffGrid LLM     ($binDir\offgrid.exe)"
Write-Host "    • llama.cpp       ($binDir\llama-server.exe)$gpuInfo"
Write-Host ""

if ($gpuVariant -eq "cuda-12.4") {
    Write-Host "  GPU Acceleration:"
    Write-Host "    • CUDA-accelerated inference enabled"
    Write-Host "    • Use --gpu-layers flag to offload layers to GPU"
    Write-Host ""
} elseif (Get-Command nvidia-smi -ErrorAction SilentlyContinue) {
    try {
        $null = nvidia-smi 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Host "  GPU Detected but not enabled:"
            Write-Host "    • Install CUDA Toolkit for GPU acceleration:"
            Write-Host "      https://developer.nvidia.com/cuda-downloads"
            Write-Host "    • Then reinstall this script"
            Write-Host ""
        }
    } catch {
        # Silently ignore nvidia-smi errors
    }
}

Write-Host "  Get Started (restart terminal first):"
Write-Host "    offgrid version           # Check version"
Write-Host "    offgrid server start      # Start API server"
Write-Host "    offgrid chat              # Interactive chat"
Write-Host ""
Write-Host "  Documentation:"
Write-Host "    https://github.com/takuphilchan/offgrid-llm"
Write-Host ""

Write-Warning "⚠ Please restart your terminal for PATH changes to take effect"
Write-Host ""
Write-Host "Press any key to close..." -ForegroundColor Yellow
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
