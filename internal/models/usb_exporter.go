package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// USBExporter handles exporting models to USB drives or SD cards
type USBExporter struct {
	modelsDir        string
	registry         *Registry
	includeInstaller bool
	binaryPath       string // Path to offgrid binary
}

// NewUSBExporter creates a new USB exporter
func NewUSBExporter(modelsDir string, registry *Registry) *USBExporter {
	return &USBExporter{
		modelsDir:        modelsDir,
		registry:         registry,
		includeInstaller: false,
	}
}

// NewUSBExporterWithInstaller creates a USB exporter that includes the installer
func NewUSBExporterWithInstaller(modelsDir string, registry *Registry, binaryPath string) *USBExporter {
	return &USBExporter{
		modelsDir:        modelsDir,
		registry:         registry,
		includeInstaller: true,
		binaryPath:       binaryPath,
	}
}

// SetIncludeInstaller enables or disables including the installer
func (e *USBExporter) SetIncludeInstaller(include bool, binaryPath string) {
	e.includeInstaller = include
	e.binaryPath = binaryPath
}

// ExportProgress represents export progress
type ExportProgress struct {
	FilePath   string
	FileName   string
	BytesTotal int64
	BytesDone  int64
	Percent    float64
	Status     string // "checking", "copying", "verifying", "complete", "failed"
	Error      error
}

// PackageManifest contains metadata about exported models
type PackageManifest struct {
	CreatedAt   time.Time    `json:"created_at"`
	Version     string       `json:"version"`
	TotalModels int          `json:"total_models"`
	TotalSizeGB float64      `json:"total_size_gb"`
	Models      []ModelEntry `json:"models"`
}

// ModelEntry represents a model in the manifest
type ModelEntry struct {
	FileName     string `json:"file_name"`
	ModelID      string `json:"model_id"`
	Quantization string `json:"quantization"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
}

// ExportAll exports all models from the registry to a USB drive
func (e *USBExporter) ExportAll(usbPath string, onProgress func(ExportProgress)) (int, error) {
	if err := e.registry.ScanModels(); err != nil {
		return 0, fmt.Errorf("failed to scan models: %w", err)
	}

	models := e.registry.ListModels()
	if len(models) == 0 {
		return 0, fmt.Errorf("no models available to export")
	}

	// Check if USB path exists
	if _, err := os.Stat(usbPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("USB path does not exist: %s", usbPath)
	}

	// Calculate total size by getting model metadata
	var totalRequired int64
	for _, model := range models {
		meta, err := e.registry.GetModel(model.ID)
		if err == nil && meta != nil {
			totalRequired += meta.Size
		}
	}

	// Check available disk space
	availableSpace, err := getDiskSpace(usbPath)
	if err != nil {
		return 0, fmt.Errorf("cannot check disk space: %w", err)
	}

	if availableSpace < totalRequired {
		return 0, fmt.Errorf("insufficient disk space: need %s, available %s",
			formatBytes(totalRequired), formatBytes(availableSpace))
	}

	// Create offgrid-models directory on USB
	modelsDir := filepath.Join(usbPath, "offgrid-models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return 0, fmt.Errorf("cannot create models directory: %w", err)
	}

	exported := 0
	var manifestEntries []ModelEntry

	for _, model := range models {
		meta, err := e.registry.GetModel(model.ID)
		if err != nil || meta.Path == "" {
			continue
		}

		// Export the model
		entry, err := e.ExportModel(meta.Path, modelsDir, onProgress)
		if err != nil {
			fmt.Printf("Failed to export %s: %v\n", model.ID, err)
			continue
		}

		manifestEntries = append(manifestEntries, *entry)
		exported++
	}

	// Create manifest file
	if err := e.createManifest(modelsDir, manifestEntries); err != nil {
		fmt.Printf("Warning: failed to create manifest: %v\n", err)
	}

	// Create README
	if err := e.createReadme(usbPath); err != nil {
		fmt.Printf("Warning: failed to create README: %v\n", err)
	}

	// Include installer if requested
	if e.includeInstaller {
		if err := e.includeInstallerFiles(usbPath); err != nil {
			fmt.Printf("Warning: failed to include installer: %v\n", err)
		}
	}

	return exported, nil
}

// ExportModel exports a single model file to USB
func (e *USBExporter) ExportModel(sourcePath, destDir string, onProgress func(ExportProgress)) (*ModelEntry, error) {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("cannot access source file: %w", err)
	}

	fileName := filepath.Base(sourcePath)
	destPath := filepath.Join(destDir, fileName)

	progress := ExportProgress{
		FilePath:   sourcePath,
		FileName:   fileName,
		BytesTotal: sourceInfo.Size(),
		Status:     "checking",
	}

	if onProgress != nil {
		onProgress(progress)
	}

	// Check if file already exists and matches
	if _, err := os.Stat(destPath); err == nil {
		sourceHash, err := calculateFileHash(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("cannot calculate source hash: %w", err)
		}

		destHash, err := calculateFileHash(destPath)
		if err == nil && sourceHash == destHash {
			progress.Status = "complete"
			progress.Percent = 100
			progress.BytesDone = sourceInfo.Size()
			if onProgress != nil {
				onProgress(progress)
			}

			// Return manifest entry
			modelID, quant := extractModelInfo(fileName)
			return &ModelEntry{
				FileName:     fileName,
				ModelID:      modelID,
				Quantization: quant,
				Size:         sourceInfo.Size(),
				SHA256:       sourceHash,
			}, nil
		}
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create destination directory: %w", err)
	}

	// Open source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create temporary file for atomic export
	tempPath := destPath + ".tmp"
	destFile, err := os.Create(tempPath)
	if err != nil {
		return nil, fmt.Errorf("cannot create temporary file: %w", err)
	}
	defer func() {
		destFile.Close()
		os.Remove(tempPath) // Clean up temp file if still exists
	}()

	// Copy with progress tracking
	progress.Status = "copying"
	buffer := make([]byte, 1024*1024) // 1MB buffer
	var bytesCopied int64
	hash := sha256.New()

	for {
		n, err := sourceFile.Read(buffer)
		if n > 0 {
			// Write to destination
			if _, writeErr := destFile.Write(buffer[:n]); writeErr != nil {
				return nil, fmt.Errorf("write error: %w", writeErr)
			}

			// Update hash
			hash.Write(buffer[:n])

			bytesCopied += int64(n)
			progress.BytesDone = bytesCopied
			progress.Percent = float64(bytesCopied) / float64(sourceInfo.Size()) * 100

			if onProgress != nil {
				onProgress(progress)
			}
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}
	}

	// Verify size
	if bytesCopied != sourceInfo.Size() {
		return nil, fmt.Errorf("incomplete copy: expected %d bytes, got %d", sourceInfo.Size(), bytesCopied)
	}

	// Close files before verification
	destFile.Close()
	sourceFile.Close()

	// Verify integrity
	progress.Status = "verifying"
	if onProgress != nil {
		onProgress(progress)
	}

	destHash, err := calculateFileHash(tempPath)
	if err != nil {
		return nil, fmt.Errorf("cannot verify destination: %w", err)
	}

	computedHash := hex.EncodeToString(hash.Sum(nil))
	if destHash != computedHash {
		return nil, fmt.Errorf("verification failed: hash mismatch")
	}

	// Atomic move: rename temp file to final destination
	if err := os.Rename(tempPath, destPath); err != nil {
		return nil, fmt.Errorf("failed to move file to destination: %w", err)
	}

	// Clear the cleanup flag
	tempPath = ""

	progress.Status = "complete"
	progress.Percent = 100
	if onProgress != nil {
		onProgress(progress)
	}

	// Create manifest entry
	modelID, quant := extractModelInfo(fileName)
	return &ModelEntry{
		FileName:     fileName,
		ModelID:      modelID,
		Quantization: quant,
		Size:         sourceInfo.Size(),
		SHA256:       computedHash,
	}, nil
}

// createManifest creates a manifest.json file with package metadata
func (e *USBExporter) createManifest(modelsDir string, entries []ModelEntry) error {
	var totalSize int64
	for _, entry := range entries {
		totalSize += entry.Size
	}

	manifest := PackageManifest{
		CreatedAt:   time.Now(),
		Version:     "1.0",
		TotalModels: len(entries),
		TotalSizeGB: float64(totalSize) / (1024 * 1024 * 1024),
		Models:      entries,
	}

	manifestPath := filepath.Join(modelsDir, "manifest.json")
	file, err := os.Create(manifestPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(manifest)
}

// createReadme creates a README file on the USB drive
func (e *USBExporter) createReadme(usbPath string) error {
	readme := `# OffGrid LLM - Model Package

This USB drive contains OffGrid LLM models for offline use.

## Contents

- offgrid-models/ - GGUF model files
- manifest.json - Package metadata and checksums

## Installation

### Quick Import

1. Mount this USB drive
2. Run the import command:

   Linux/macOS:
   offgrid import /media/usb/offgrid-models

   Windows:
   offgrid import E:\offgrid-models

### Manual Installation

Copy the .gguf files from offgrid-models/ to:
- Linux/macOS: ~/.offgrid-llm/models/
- Windows: %USERPROFILE%\.offgrid-llm\models\

## Verification

The manifest.json file contains SHA256 checksums for all models.
OffGrid will automatically verify integrity during import.

## More Information

Visit: https://github.com/takuphilchan/offgrid-llm
Documentation: https://github.com/takuphilchan/offgrid-llm/tree/main/docs

---
Generated: ` + time.Now().Format("2006-01-02 15:04:05") + `
`

	readmePath := filepath.Join(usbPath, "README.txt")
	return os.WriteFile(readmePath, []byte(readme), 0644)
}

// Helper function to extract model info from filename
func extractModelInfo(filename string) (modelID string, quantization string) {
	name := strings.TrimSuffix(filename, ".gguf")

	quants := []string{"Q2_K", "Q3_K_S", "Q3_K_M", "Q3_K_L", "Q4_0", "Q4_K_S", "Q4_K_M",
		"Q5_0", "Q5_K_S", "Q5_K_M", "Q6_K", "Q8_0", "F16", "F32"}

	for _, q := range quants {
		if strings.HasSuffix(name, "."+q) {
			modelID = strings.TrimSuffix(name, "."+q)
			quantization = q
			return
		}
	}

	modelID = name
	quantization = "unknown"
	return
}

// Helper to calculate file hash
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// includeInstallerFiles copies the offgrid binary and install scripts to the USB
func (e *USBExporter) includeInstallerFiles(usbPath string) error {
	// Create installer directory
	installerDir := filepath.Join(usbPath, "installer")
	if err := os.MkdirAll(installerDir, 0755); err != nil {
		return fmt.Errorf("cannot create installer directory: %w", err)
	}

	// Copy the offgrid binary if available
	if e.binaryPath != "" {
		if _, err := os.Stat(e.binaryPath); err == nil {
			destBinary := filepath.Join(installerDir, filepath.Base(e.binaryPath))
			if err := copyFile(e.binaryPath, destBinary); err != nil {
				return fmt.Errorf("failed to copy binary: %w", err)
			}
			// Make executable
			os.Chmod(destBinary, 0755)
		}
	} else {
		// Try to find the binary in common locations
		binaryPath := findOffgridBinary()
		if binaryPath != "" {
			destBinary := filepath.Join(installerDir, "offgrid")
			if runtime.GOOS == "windows" {
				destBinary += ".exe"
			}
			if err := copyFile(binaryPath, destBinary); err != nil {
				return fmt.Errorf("failed to copy binary: %w", err)
			}
			os.Chmod(destBinary, 0755)
		}
	}

	// Create platform-specific install scripts
	if err := createInstallScript(installerDir, "linux"); err != nil {
		fmt.Printf("Warning: failed to create Linux install script: %v\n", err)
	}
	if err := createInstallScript(installerDir, "darwin"); err != nil {
		fmt.Printf("Warning: failed to create macOS install script: %v\n", err)
	}
	if err := createInstallScript(installerDir, "windows"); err != nil {
		fmt.Printf("Warning: failed to create Windows install script: %v\n", err)
	}

	// Create an INSTALL.txt with instructions
	installInstructions := `# OffGrid LLM - USB Installation

This USB package includes the OffGrid LLM installer for offline installation.

## Quick Install

### Linux / macOS
1. Open a terminal
2. Run: ./installer/install.sh

### Windows
1. Open PowerShell as Administrator
2. Run: .\installer\install.ps1

## Manual Install

1. Copy the 'offgrid' binary from installer/ to your system:
   - Linux/macOS: /usr/local/bin/offgrid
   - Windows: C:\Program Files\OffGrid\offgrid.exe

2. Copy models from offgrid-models/ to:
   - Linux/macOS: ~/.offgrid-llm/models/
   - Windows: %USERPROFILE%\.offgrid-llm\models\

3. Run: offgrid serve

## Verify Installation

Run: offgrid version
Run: offgrid doctor

For more help: offgrid help
`
	instructionsPath := filepath.Join(usbPath, "INSTALL.txt")
	if err := os.WriteFile(instructionsPath, []byte(installInstructions), 0644); err != nil {
		return fmt.Errorf("failed to write INSTALL.txt: %w", err)
	}

	return nil
}

// findOffgridBinary tries to find the offgrid binary
func findOffgridBinary() string {
	// Try common locations
	candidates := []string{
		"./bin/offgrid",
		"./offgrid",
		"/usr/local/bin/offgrid",
		"/usr/bin/offgrid",
	}

	if runtime.GOOS == "windows" {
		candidates = []string{
			".\\bin\\offgrid.exe",
			".\\offgrid.exe",
		}
	}

	// Also try current executable path
	exe, err := os.Executable()
	if err == nil {
		candidates = append([]string{exe}, candidates...)
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// createInstallScript creates a platform-specific install script
func createInstallScript(installerDir, platform string) error {
	var content string
	var filename string

	switch platform {
	case "linux", "darwin":
		filename = "install.sh"
		content = `#!/bin/bash
# OffGrid LLM Installer
set -e

echo "OffGrid LLM USB Installer"
echo "========================="
echo

# Detect USB mount point (parent of this script)
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
USB_ROOT="$(dirname "$SCRIPT_DIR")"

# Check for binary
BINARY="$SCRIPT_DIR/offgrid"
if [ ! -f "$BINARY" ]; then
    echo "Error: offgrid binary not found in installer/"
    echo "Please download offgrid from https://github.com/takuphilchan/offgrid-llm"
    exit 1
fi

# Install binary
echo "Installing offgrid binary..."
if [ -w /usr/local/bin ]; then
    cp "$BINARY" /usr/local/bin/offgrid
    chmod +x /usr/local/bin/offgrid
    echo "  Installed to /usr/local/bin/offgrid"
else
    echo "  Need sudo to install to /usr/local/bin"
    sudo cp "$BINARY" /usr/local/bin/offgrid
    sudo chmod +x /usr/local/bin/offgrid
    echo "  Installed to /usr/local/bin/offgrid"
fi

# Create models directory
MODELS_DIR="$HOME/.offgrid-llm/models"
mkdir -p "$MODELS_DIR"

# Import models
if [ -d "$USB_ROOT/offgrid-models" ]; then
    echo "Importing models..."
    offgrid import "$USB_ROOT/offgrid-models"
else
    echo "No models found on USB."
fi

echo
echo "Installation complete!"
echo "Run 'offgrid serve' to start the server."
`
	case "windows":
		filename = "install.ps1"
		content = `# OffGrid LLM Windows Installer
$ErrorActionPreference = "Stop"

Write-Host "OffGrid LLM USB Installer" -ForegroundColor Cyan
Write-Host "=========================" -ForegroundColor Cyan
Write-Host

# Detect USB root
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$USBRoot = Split-Path -Parent $ScriptDir

# Check for binary
$Binary = Join-Path $ScriptDir "offgrid.exe"
if (-not (Test-Path $Binary)) {
    Write-Host "Error: offgrid.exe not found in installer/" -ForegroundColor Red
    Write-Host "Please download offgrid from https://github.com/takuphilchan/offgrid-llm"
    exit 1
}

# Install to Program Files
$InstallDir = "$env:ProgramFiles\OffGrid"
Write-Host "Installing to $InstallDir..."

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

Copy-Item $Binary "$InstallDir\offgrid.exe" -Force
Write-Host "  Installed offgrid.exe"

# Add to PATH if not already there
$CurrentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
if ($CurrentPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$CurrentPath;$InstallDir", "Machine")
    Write-Host "  Added to system PATH"
}

# Create models directory
$ModelsDir = "$env:USERPROFILE\.offgrid-llm\models"
if (-not (Test-Path $ModelsDir)) {
    New-Item -ItemType Directory -Path $ModelsDir -Force | Out-Null
}

# Import models
$USBModels = Join-Path $USBRoot "offgrid-models"
if (Test-Path $USBModels) {
    Write-Host "Importing models..."
    & "$InstallDir\offgrid.exe" import $USBModels
} else {
    Write-Host "No models found on USB."
}

Write-Host
Write-Host "Installation complete!" -ForegroundColor Green
Write-Host "Run 'offgrid serve' to start the server."
`
	default:
		return fmt.Errorf("unsupported platform: %s", platform)
	}

	scriptPath := filepath.Join(installerDir, filename)
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		return err
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
