# OffGrid LLM Desktop - Windows Installer
# PowerShell script to install OffGrid on Windows

$ErrorActionPreference = "Stop"

$CYAN = "$([char]27)[36m"
$GREEN = "$([char]27)[32m"
$YELLOW = "$([char]27)[33m"
$RED = "$([char]27)[31m"
$NC = "$([char]27)[0m"

function Print-Banner { Write-Host "OffGrid Desktop Installer" }

function Print-Step { param($msg) Write-Host "${CYAN}[INSTALL]${NC} $msg" }
function Print-Success { param($msg) Write-Host "${GREEN}[OK]${NC} $msg" }
function Print-Error { param($msg) Write-Host "${RED}[ERROR]${NC} $msg" -ForegroundColor Red }
function Print-Warning { param($msg) Write-Host "${YELLOW}[WARN]${NC} $msg" -ForegroundColor Yellow }

function Assert-ReleaseChecksum {
    param([string]$File, [string]$AssetName, [string]$Manifest)
    $entries = @(Get-Content -LiteralPath $Manifest | ForEach-Object {
        if ($_ -cmatch '^([a-fA-F0-9]{64}) [ *](.+)$' -and $Matches[2] -ceq $AssetName) {
            $Matches[1]
        }
    })
    if ($entries.Count -ne 1) { throw "Expected exactly one checksum for $AssetName" }
    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $File).Hash -ine $entries[0]) {
        throw "Checksum mismatch for $AssetName. Installation stopped."
    }
}

function Get-CliInstallTarget {
    param([bool]$Administrator)
    if ($Administrator) { return @{ Directory = (Join-Path $env:ProgramFiles 'OffGrid'); Scope = 'Machine' } }
    return @{ Directory = (Join-Path $env:LOCALAPPDATA 'Programs\OffGrid CLI'); Scope = 'User' }
}

function Assert-CliStopped {
    $active = @(Get-Process -ErrorAction Stop | Where-Object { $_.ProcessName -in @('offgrid', 'llama-server') })
    if ($active.Count -gt 0) {
        throw 'OffGrid or llama-server is running. Finish active work and close its owning service normally before retrying. No processes were stopped.'
    }
}

function Copy-CliPayload {
    param([string]$Source, [string]$Destination)
    foreach ($required in @('offgrid.exe', 'llama-server.exe')) {
        if (-not (Test-Path -LiteralPath (Join-Path $Source $required) -PathType Leaf)) {
            throw "Missing $required in CLI bundle"
        }
    }
    New-Item -ItemType Directory -Path $Destination -Force | Out-Null
    Get-ChildItem -LiteralPath $Source -Force | ForEach-Object {
        Copy-Item -LiteralPath $_.FullName -Destination $Destination -Recurse -Force -ErrorAction Stop
    }
}

function Get-NativeArchitecture {
    # CIM reports the native processor even when PowerShell runs under emulation.
    $architectures = @(Get-CimInstance Win32_Processor -ErrorAction Stop | Select-Object -ExpandProperty Architecture -Unique)
    if ($architectures.Count -ne 1 -or $architectures[0] -ne 9) {
        throw 'These Windows packages support native x64 only. No Windows ARM64 or x86 package is published.'
    }
    return 'amd64'
}

function Assert-CliHardware {
    if (-not ('OffGridInstallerCpu' -as [type])) {
        Add-Type -TypeDefinition @'
using System.Runtime.InteropServices;
public static class OffGridInstallerCpu {
    [DllImport("kernel32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool IsProcessorFeaturePresent(uint feature);
}
'@
    }
    # PF_AVX2_INSTRUCTIONS_AVAILABLE checks OS-visible support, not CPU branding.
    if (-not [OffGridInstallerCpu]::IsProcessorFeaturePresent(40)) {
        throw 'The Windows CLI runtime requires AVX2. This OS/CPU does not report support. Use a compatible host or a custom runtime; SSE4.2 alone is insufficient.'
    }
}

Print-Banner

# Check if running as Administrator
$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
$isAdmin = $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Print-Step "CLI installation will use your local application directory and user PATH."
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

# Reject unsupported hosts before downloads or changes to PATH/application files.
$null = Get-NativeArchitecture
if ($INSTALL_CLI) { Assert-CliHardware }

# GitHub release info
$GITHUB_REPO = "takuphilchan/offgrid-llm"

# Fetch latest release version from GitHub API
Print-Step "Fetching latest release version..."
try {
    $apiResponse = Invoke-RestMethod -Uri "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" -UseBasicParsing
    $VERSION = $apiResponse.tag_name -replace '^v', ''
    if ($VERSION -notmatch '^\d+\.\d+\.\d+([-.][A-Za-z0-9.-]+)?$') {
        throw "Invalid release version"
    }
    Print-Success "Latest version: v${VERSION}"
} catch {
    Print-Error "Failed to fetch latest version from GitHub: $_"
    exit 1
}

$RELEASE_URL = "https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}"

# Temporary directory
$TMP_DIR = Join-Path $env:TEMP "offgrid-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TMP_DIR | Out-Null

try {
    $MANIFEST = Join-Path $TMP_DIR 'checksums.sha256'
    Invoke-WebRequest -Uri "${RELEASE_URL}/checksums-v${VERSION}.sha256" -OutFile $MANIFEST -UseBasicParsing
    # Install CLI
    if ($INSTALL_CLI) {
        Assert-CliStopped
        Print-Step "Installing CLI..."
        
        $CLI_BUNDLE = "offgrid-v${VERSION}-windows-amd64-cpu-avx2.zip"
        $CLI_URL = "${RELEASE_URL}/${CLI_BUNDLE}"
        $CLI_ZIP = Join-Path $TMP_DIR $CLI_BUNDLE
        
        Print-Step "Downloading CLI bundle..."
        try {
            Invoke-WebRequest -Uri $CLI_URL -OutFile $CLI_ZIP -UseBasicParsing
        } catch {
            Print-Error "Failed to download CLI bundle: $_"
            exit 1
        }
        
        Assert-ReleaseChecksum $CLI_ZIP $CLI_BUNDLE $MANIFEST
        Print-Step "Extracting bundle..."
        $EXTRACT_DIR = Join-Path $TMP_DIR 'cli'
        Expand-Archive -Path $CLI_ZIP -DestinationPath $EXTRACT_DIR -Force
        
        # Find the offgrid.exe in extracted files
        $cliCandidates = @(Get-ChildItem -Path $EXTRACT_DIR -Filter "offgrid.exe" -File -Recurse)
        if ($cliCandidates.Count -ne 1) { throw 'Expected exactly one offgrid.exe in CLI bundle' }
        $CLI_PATH = $cliCandidates[0].FullName
        if (-not $CLI_PATH) {
            Print-Error "Could not find offgrid.exe in bundle"
            exit 1
        }
        
        $target = Get-CliInstallTarget $isAdmin
        $INSTALL_DIR = $target.Directory
        if (-not (Test-Path $INSTALL_DIR)) {
            New-Item -ItemType Directory -Path $INSTALL_DIR -Force | Out-Null
        }
        
        Print-Step "Installing to $INSTALL_DIR..."
        Assert-CliStopped
        Copy-CliPayload (Split-Path -Parent $CLI_PATH) $INSTALL_DIR
        
        # Add to PATH
        $currentPath = [Environment]::GetEnvironmentVariable("Path", $target.Scope)
        if (($currentPath -split ';').TrimEnd('\') -notcontains $INSTALL_DIR.TrimEnd('\')) {
            Print-Step "Adding to $($target.Scope) PATH..."
            [Environment]::SetEnvironmentVariable("Path", "$currentPath;$INSTALL_DIR", $target.Scope)
        }
        $env:Path = "$INSTALL_DIR;$env:Path"
        
        Print-Step "Location: $INSTALL_DIR\offgrid.exe"
        
        # Verify installation
        $versionOutput = & "$INSTALL_DIR\offgrid.exe" --version 2>&1
        if ($LASTEXITCODE -ne 0) { throw "CLI verification failed: $versionOutput" }
        Print-Success "Verification: $versionOutput"
        Print-Success "CLI installed successfully"
        Write-Host ""
    }
    
    # Install Desktop
    if ($INSTALL_DESKTOP) {
        Print-Step "Installing Desktop application..."
        
        $coreVersion = [version](($VERSION -split '-')[0])
        $DESKTOP_FILE = if ($coreVersion -ge [version]'0.4.27') {
            "OffGrid-${VERSION}-windows-x64-setup.exe"
        } else {
            "OffGrid.LLM.Desktop-Setup-${VERSION}.exe"
        }
        $DESKTOP_URL = "${RELEASE_URL}/${DESKTOP_FILE}"
        $INSTALLER_PATH = Join-Path $TMP_DIR "offgrid-desktop-setup.exe"
        
        Print-Step "Downloading desktop installer..."
        try {
            Invoke-WebRequest -Uri $DESKTOP_URL -OutFile $INSTALLER_PATH -UseBasicParsing
        } catch {
            Print-Error "Failed to download desktop installer: $_"
            exit 1
        }
        
        Assert-ReleaseChecksum $INSTALLER_PATH $DESKTOP_FILE $MANIFEST
        Print-Step "Running desktop installer..."
        Print-Warning "Please follow the installation wizard..."
        
        $setup = Start-Process -FilePath $INSTALLER_PATH -Wait -PassThru
        if ($setup.ExitCode -ne 0) { throw "Desktop installer exited with code $($setup.ExitCode)" }
        
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
    Write-Host ""
    
    if ($INSTALL_CLI) {
        Write-Host "CLI installed:"
        Write-Host "  - Run: offgrid --help"
        Write-Host "  - Location: $INSTALL_DIR\offgrid.exe"
        Write-Host "  - NOTE: Restart your terminal to use 'offgrid' command"
        Write-Host "  - For the bundled web UI, set OFFGRID_UI_DIR to $INSTALL_DIR\web\ui before offgrid serve."
        Write-Host ""
    }
    
    if ($INSTALL_DESKTOP) {
        Write-Host "Desktop app installed:"
        Write-Host "  - Launch from Start Menu: OffGrid"
        Write-Host "  - Or from Desktop shortcut"
        Write-Host ""
    }
    
    Write-Host "Next steps:"
    Write-Host "  1. Open the desktop app, or run offgrid serve and open http://127.0.0.1:11611/ui/"
    Write-Host "     Download a chat model from Models."
    Write-Host ""
    Write-Host "  2. Start using:"
    if ($INSTALL_DESKTOP) {
        Write-Host "     - Launch the desktop app from Start Menu"
    } else {
        Write-Host "     - Open Chat in the web UI and send a prompt."
    }
    Write-Host ""
    Write-Host "Documentation: https://github.com/${GITHUB_REPO}"
    Write-Host ""
    
} finally {
    # Cleanup
    $tempRoot = [IO.Path]::GetFullPath($env:TEMP).TrimEnd('\') + '\'
    $cleanupPath = [IO.Path]::GetFullPath($TMP_DIR)
    if ($cleanupPath.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -and
        (Split-Path -Leaf $cleanupPath) -like 'offgrid-install-*' -and
        (Test-Path -LiteralPath $cleanupPath)) {
        Remove-Item -LiteralPath $cleanupPath -Recurse -Force -ErrorAction SilentlyContinue
    }
}
