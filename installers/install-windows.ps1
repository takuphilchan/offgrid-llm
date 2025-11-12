# OffGrid LLM Windows Installation Script
# PowerShell installer with automatic setup

#Requires -RunAsAdministrator

param(
    [string]$InstallPath = "$env:ProgramFiles\OffGrid",
    [switch]$AddToPath = $true,
    [switch]$CreateShortcuts = $true
)

$ErrorActionPreference = "Stop"

# Colors for output
function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    Write-Host $Message -ForegroundColor $Color
}

function Write-Header {
    param([string]$Text)
    Write-Host ""
    Write-ColorOutput "╭────────────────────────────────────────────────────────────────────╮" "Cyan"
    Write-ColorOutput "│ $Text" "Cyan"
    Write-ColorOutput "╰────────────────────────────────────────────────────────────────────╯" "Cyan"
    Write-Host ""
}

function Write-Success {
    param([string]$Text)
    Write-ColorOutput "✓ $Text" "Green"
}

function Write-Error {
    param([string]$Text)
    Write-ColorOutput "✗ $Text" "Red"
}

function Write-Info {
    param([string]$Text)
    Write-ColorOutput "→ $Text" "Cyan"
}

# Banner
Write-Host ""
Write-ColorOutput @"
    ╔═══════════════════════════════════════════════════════════════╗
    ║                                                               ║
    ║     ██████╗ ███████╗███████╗ ██████╗ ██████╗ ██╗██████╗      ║
    ║    ██╔═══██╗██╔════╝██╔════╝██╔════╝ ██╔══██╗██║██╔══██╗     ║
    ║    ██║   ██║█████╗  █████╗  ██║  ███╗██████╔╝██║██║  ██║     ║
    ║    ██║   ██║██╔══╝  ██╔══╝  ██║   ██║██╔══██╗██║██║  ██║     ║
    ║    ╚██████╔╝██║     ██║     ╚██████╔╝██║  ██║██║██████╔╝     ║
    ║     ╚═════╝ ╚═╝     ╚═╝      ╚═════╝ ╚═╝  ╚═╝╚═╝╚═════╝      ║
    ║                                                               ║
    ║              W I N D O W S   I N S T A L L E R                ║
    ║                                                               ║
    ╚═══════════════════════════════════════════════════════════════╝
"@ "Cyan"
Write-Host ""

Write-Header "Installing OffGrid LLM"

# Check if running as Administrator
if (-NOT ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {
    Write-Error "This script requires Administrator privileges"
    Write-Host "Please run PowerShell as Administrator and try again"
    exit 1
}

# Create installation directory
Write-Info "Creating installation directory..."
if (Test-Path $InstallPath) {
    Write-Info "Installation directory already exists, removing old version..."
    Remove-Item -Path $InstallPath -Recurse -Force
}
New-Item -ItemType Directory -Path $InstallPath -Force | Out-Null
Write-Success "Created: $InstallPath"

# Copy binaries (assuming they're in the current directory)
Write-Info "Installing binaries..."
$CurrentDir = Get-Location

if (Test-Path ".\offgrid.exe") {
    Copy-Item ".\offgrid.exe" -Destination $InstallPath
    Write-Success "Installed: offgrid.exe"
} else {
    Write-Error "offgrid.exe not found in current directory"
    exit 1
}

if (Test-Path ".\llama-server.exe") {
    Copy-Item ".\llama-server.exe" -Destination $InstallPath
    Write-Success "Installed: llama-server.exe"
} else {
    Write-Info "llama-server.exe not found (optional)"
}

# Copy documentation
if (Test-Path ".\README.md") {
    Copy-Item ".\README.md" -Destination $InstallPath
}
if (Test-Path ".\LICENSE") {
    Copy-Item ".\LICENSE" -Destination $InstallPath
}

# Create config directory
$ConfigPath = "$env:APPDATA\OffGrid"
Write-Info "Creating configuration directory..."
if (-not (Test-Path $ConfigPath)) {
    New-Item -ItemType Directory -Path $ConfigPath -Force | Out-Null
    Write-Success "Created: $ConfigPath"
}

# Add to PATH
if ($AddToPath) {
    Write-Info "Adding to system PATH..."
    $CurrentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    
    if ($CurrentPath -notlike "*$InstallPath*") {
        $NewPath = "$CurrentPath;$InstallPath"
        [Environment]::SetEnvironmentVariable("Path", $NewPath, "Machine")
        Write-Success "Added to PATH: $InstallPath"
        Write-ColorOutput "  Note: You may need to restart your terminal" "Yellow"
    } else {
        Write-Success "Already in PATH"
    }
}

# Create Start Menu shortcuts
if ($CreateShortcuts) {
    Write-Info "Creating Start Menu shortcuts..."
    $StartMenuPath = "$env:ProgramData\Microsoft\Windows\Start Menu\Programs\OffGrid LLM"
    
    if (-not (Test-Path $StartMenuPath)) {
        New-Item -ItemType Directory -Path $StartMenuPath -Force | Out-Null
    }
    
    $WshShell = New-Object -ComObject WScript.Shell
    
    # Command Prompt shortcut
    $Shortcut = $WshShell.CreateShortcut("$StartMenuPath\OffGrid Command Prompt.lnk")
    $Shortcut.TargetPath = "cmd.exe"
    $Shortcut.Arguments = "/K offgrid --help"
    $Shortcut.WorkingDirectory = $InstallPath
    $Shortcut.IconLocation = "$InstallPath\offgrid.exe"
    $Shortcut.Save()
    
    # README shortcut
    if (Test-Path "$InstallPath\README.md") {
        $Shortcut = $WshShell.CreateShortcut("$StartMenuPath\README.lnk")
        $Shortcut.TargetPath = "$InstallPath\README.md"
        $Shortcut.Save()
    }
    
    Write-Success "Created Start Menu shortcuts"
}

# Create uninstaller script
Write-Info "Creating uninstaller..."
$UninstallScript = @"
# OffGrid LLM Uninstaller
#Requires -RunAsAdministrator

Write-Host "Uninstalling OffGrid LLM..." -ForegroundColor Cyan

# Remove from PATH
`$CurrentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
`$NewPath = (`$CurrentPath.Split(';') | Where-Object { `$_ -ne "$InstallPath" }) -join ';'
[Environment]::SetEnvironmentVariable("Path", `$NewPath, "Machine")

# Remove files
Remove-Item -Path "$InstallPath" -Recurse -Force -ErrorAction SilentlyContinue

# Remove Start Menu shortcuts
Remove-Item -Path "$StartMenuPath" -Recurse -Force -ErrorAction SilentlyContinue

# Ask about config
`$RemoveConfig = Read-Host "Remove configuration files in $ConfigPath? (y/N)"
if (`$RemoveConfig -eq 'y' -or `$RemoveConfig -eq 'Y') {
    Remove-Item -Path "$ConfigPath" -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host "Configuration removed" -ForegroundColor Green
}

Write-Host "✓ OffGrid LLM has been uninstalled" -ForegroundColor Green
"@

$UninstallScript | Out-File -FilePath "$InstallPath\Uninstall.ps1" -Encoding UTF8
Write-Success "Created uninstaller: $InstallPath\Uninstall.ps1"

Write-Host ""
Write-ColorOutput "╭────────────────────────────────────────────────────────────────────╮" "Green"
Write-ColorOutput "│ Installation Complete!                                              │" "Green"
Write-ColorOutput "╰────────────────────────────────────────────────────────────────────╯" "Green"
Write-Host ""

Write-ColorOutput "Installation Details:" "White"
Write-ColorOutput "  Install Path:  $InstallPath" "Gray"
Write-ColorOutput "  Config Path:   $ConfigPath" "Gray"
Write-Host ""

Write-ColorOutput "Next Steps:" "Cyan"
Write-ColorOutput "  1. Restart your terminal or run:" "White"
Write-ColorOutput "     `$env:Path = [System.Environment]::GetEnvironmentVariable('Path','Machine')" "Gray"
Write-Host ""
Write-ColorOutput "  2. Verify installation:" "White"
Write-ColorOutput "     offgrid --version" "Gray"
Write-Host ""
Write-ColorOutput "  3. Get started:" "White"
Write-ColorOutput "     offgrid --help" "Gray"
Write-ColorOutput "     offgrid server start" "Gray"
Write-Host ""

Write-ColorOutput "To uninstall, run:" "Yellow"
Write-ColorOutput "  powershell -ExecutionPolicy Bypass -File `"$InstallPath\Uninstall.ps1`"" "Gray"
Write-Host ""
