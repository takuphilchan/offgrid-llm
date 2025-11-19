package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// USBExporter handles exporting models to USB drives or SD cards
type USBExporter struct {
	modelsDir string
	registry  *Registry
}

// NewUSBExporter creates a new USB exporter
func NewUSBExporter(modelsDir string, registry *Registry) *USBExporter {
	return &USBExporter{
		modelsDir: modelsDir,
		registry:  registry,
	}
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
