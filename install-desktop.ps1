# OffGrid LLM Desktop - Windows Installer
# PowerShell script to install OffGrid LLM on Windows

$ErrorActionPreference = "Stop"

$CYAN = "`e[36m"
$GREEN = "`e[32m"
$YELLOW = "`e[33m"
$RED = "`e[31m"
$NC = "`e[0m"

function Print-Banner {
    Write-Host "${CYAN}" -NoNewline
    Write-Host @"
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║     ██████╗ ███████╗███████╗ ██████╗ ██████╗ ██╗██████╗      ║
║    ██╔═══██╗██╔════╝██╔════╝██╔════╝ ██╔══██╗██║██╔══██╗     ║
║    ██║   ██║█████╗  █████╗  ██║  ███╗██████╔╝██║██║  ██║     ║
║    ██║   ██║██╔══╝  ██╔══╝  ██║   ██║██╔══██╗██║██║  ██║     ║
║    ╚██████╔╝██║     ██║     ╚██████╔╝██║  ██║██║██████╔╝     ║
║     ╚═════╝ ╚═╝     ╚═╝      ╚═════╝ ╚═╝  ╚═╝╚═╝╚═════╝      ║
║                                                               ║
║               DESKTOP INSTALLER v0.1.3                        ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
"@
    Write-Host "${NC}"
}

function Print-Step { param($msg) Write-Host "${CYAN}[INSTALL]${NC} $msg" }
function Print-Success { param($msg) Write-Host "${GREEN}[OK]${NC} $msg" }
function Print-Error { param($msg) Write-Host "${RED}[ERROR]${NC} $msg" -ForegroundColor Red }
function Print-Warning { param($msg) Write-Host "${YELLOW}[WARN]${NC} $msg" -ForegroundColor Yellow }

Print-Banner

# Check if running as Administrator
$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
$isAdmin = $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Print-Warning "This script should be run as Administrator for full installation."
    $response = Read-Host "Continue anyway? (y/N)"
    if ($response -ne "y" -and $response -ne "Y") {
        exit 1
    }
}

# Installation options
Write-Host ""
Write-Host "What would you like to install?"
Write-Host "  1) CLI only (command-line tool)"
Write-Host "  2) Desktop app only (GUI application)"
Write-Host "  3) Both CLI and Desktop (recommended)"
Write-Host ""
$choice = Read-Host "Enter your choice [1-3]"

$INSTALL_CLI = $false
$INSTALL_DESKTOP = $false

switch ($choice) {
    "1" {
        $INSTALL_CLI = $true
    }
    "2" {
        $INSTALL_DESKTOP = $true
    }
    default {
        $INSTALL_CLI = $true
        $INSTALL_DESKTOP = $true
    }
}

# GitHub release info
$GITHUB_REPO = "takuphilchan/offgrid-llm"
$VERSION = "0.1.3"
$RELEASE_URL = "https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}"

# Temporary directory
$TMP_DIR = Join-Path $env:TEMP "offgrid-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TMP_DIR | Out-Null

try {
    # Install CLI
    if ($INSTALL_CLI) {
        Print-Step "Installing CLI..."
        
        $CLI_FILE = "offgrid-windows-amd64.exe"
        $CLI_URL = "${RELEASE_URL}/${CLI_FILE}"
        $CLI_PATH = Join-Path $TMP_DIR "offgrid.exe"
        
        Print-Step "Downloading CLI binary..."
        try {
            Invoke-WebRequest -Uri $CLI_URL -OutFile $CLI_PATH -UseBasicParsing
        } catch {
            Print-Error "Failed to download CLI binary: $_"
            exit 1
        }
        
        # Install to Program Files
        $INSTALL_DIR = "${env:ProgramFiles}\OffGrid"
        if (-not (Test-Path $INSTALL_DIR)) {
            New-Item -ItemType Directory -Path $INSTALL_DIR -Force | Out-Null
        }
        
        Print-Step "Installing to $INSTALL_DIR..."
        Copy-Item $CLI_PATH "$INSTALL_DIR\offgrid.exe" -Force
        
        # Add to PATH
        $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
        if ($currentPath -notlike "*$INSTALL_DIR*") {
            Print-Step "Adding to system PATH..."
            [Environment]::SetEnvironmentVariable("Path", "$currentPath;$INSTALL_DIR", "Machine")
            $env:Path += ";$INSTALL_DIR"
        }
        
        Print-Success "CLI installed successfully"
        Print-Step "Location: $INSTALL_DIR\offgrid.exe"
        
        # Verify installation
        try {
            $versionOutput = & "$INSTALL_DIR\offgrid.exe" --version 2>&1
            Print-Success "Verification: $versionOutput"
        } catch {
            Print-Warning "Could not verify installation"
        }
        Write-Host ""
    }
    
    # Install Desktop
    if ($INSTALL_DESKTOP) {
        Print-Step "Installing Desktop application..."
        
        $DESKTOP_FILE = "OffGrid-LLM-Desktop-Setup-${VERSION}.exe"
        $DESKTOP_URL = "${RELEASE_URL}/${DESKTOP_FILE}"
        $INSTALLER_PATH = Join-Path $TMP_DIR "offgrid-desktop-setup.exe"
        
        Print-Step "Downloading desktop installer..."
        try {
            Invoke-WebRequest -Uri $DESKTOP_URL -OutFile $INSTALLER_PATH -UseBasicParsing
        } catch {
            Print-Error "Failed to download desktop installer: $_"
            exit 1
        }
        
        Print-Step "Running desktop installer..."
        Print-Warning "Please follow the installation wizard..."
        
        Start-Process -FilePath $INSTALLER_PATH -Wait
        
        Print-Success "Desktop app installer completed"
        Write-Host ""
    }
    
    # Create config directory
    $CONFIG_DIR = Join-Path $env:USERPROFILE ".offgrid-llm"
    $MODELS_DIR = Join-Path $CONFIG_DIR "models"
    $DATA_DIR = Join-Path $CONFIG_DIR "data"
    
    Print-Step "Creating configuration directory..."
    New-Item -ItemType Directory -Path $MODELS_DIR -Force | Out-Null
    New-Item -ItemType Directory -Path $DATA_DIR -Force | Out-Null
    Print-Success "Config directory: $CONFIG_DIR"
    Write-Host ""
    
    # Print success message
    Print-Success "Installation complete!"
    Write-Host ""
    Write-Host "${GREEN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    Write-Host "${GREEN}║                                                               ║${NC}"
    Write-Host "${GREEN}║  OffGrid LLM has been installed successfully!                 ║${NC}"
    Write-Host "${GREEN}║                                                               ║${NC}"
    Write-Host "${GREEN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    Write-Host ""
    
    if ($INSTALL_CLI) {
        Write-Host "CLI installed:"
        Write-Host "  - Run: offgrid --help"
        Write-Host "  - Location: ${env:ProgramFiles}\OffGrid\offgrid.exe"
        Write-Host "  - NOTE: Restart your terminal to use 'offgrid' command"
        Write-Host ""
    }
    
    if ($INSTALL_DESKTOP) {
        Write-Host "Desktop app installed:"
        Write-Host "  - Launch from Start Menu: OffGrid LLM Desktop"
        Write-Host "  - Or from Desktop shortcut"
        Write-Host ""
    }
    
    Write-Host "Next steps:"
    Write-Host "  1. Download a model:"
    Write-Host "     offgrid download llama-2-7b-chat"
    Write-Host ""
    Write-Host "  2. Start using:"
    if ($INSTALL_DESKTOP) {
        Write-Host "     - Launch the desktop app from Start Menu"
    } else {
        Write-Host "     - Run: offgrid chat"
    }
    Write-Host ""
    Write-Host "Documentation: https://github.com/${GITHUB_REPO}"
    Write-Host ""
    
} finally {
    # Cleanup
    if (Test-Path $TMP_DIR) {
        Remove-Item -Path $TMP_DIR -Recurse -Force -ErrorAction SilentlyContinue
    }
}
