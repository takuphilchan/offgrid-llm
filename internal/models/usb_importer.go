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
)

// USBImporter handles importing models from USB drives or SD cards
type USBImporter struct {
	modelsDir string
	registry  *Registry
	validator *Validator
}

// NewUSBImporter creates a new USB importer
func NewUSBImporter(modelsDir string, registry *Registry) *USBImporter {
	return &USBImporter{
		modelsDir: modelsDir,
		registry:  registry,
		validator: NewValidator(modelsDir),
	}
}

// ImportProgress represents import progress
type ImportProgress struct {
	FilePath   string
	FileName   string
	BytesTotal int64
	BytesDone  int64
	Percent    float64
	Status     string // "copying", "verifying", "complete", "failed", "skipped"
	Error      error
	Message    string // Additional status message
}

type manifestIndex struct {
	byFileName map[string]string
}

// ScanUSBDrive scans a USB drive for GGUF model files
func (u *USBImporter) ScanUSBDrive(usbPath string) ([]string, error) {
	if _, err := os.Stat(usbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("USB path does not exist: %s", usbPath)
	}

	var modelFiles []string

	// Walk the USB directory looking for .gguf files
	err := filepath.Walk(usbPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if it's a GGUF file
		if strings.HasSuffix(strings.ToLower(info.Name()), ".gguf") {
			modelFiles = append(modelFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error scanning USB drive: %w", err)
	}

	return modelFiles, nil
}

// ImportModel imports a single model file from USB to models directory
func (u *USBImporter) ImportModel(sourcePath string, expectedSHA256 string, onProgress func(ImportProgress)) error {
	// Get file info
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot access source file: %w", err)
	}

	fileName := filepath.Base(sourcePath)
	destPath := filepath.Join(u.modelsDir, fileName)

	progress := ImportProgress{
		FilePath:   sourcePath,
		FileName:   fileName,
		BytesTotal: sourceInfo.Size(),
		Status:     "copying",
	}

	// Check if file already exists
	if _, err := os.Stat(destPath); err == nil {
		// File exists, check if it's the same
		sourceHash, err := u.calculateFileHash(sourcePath)
		if err != nil {
			return fmt.Errorf("cannot calculate source hash: %w", err)
		}

		if expectedSHA256 != "" && sourceHash != expectedSHA256 {
			return fmt.Errorf("manifest verification failed for %s: expected %s, got %s", fileName, expectedSHA256, sourceHash)
		}

		destHash, err := u.calculateFileHash(destPath)
		if err == nil && sourceHash == destHash {
			progress.Status = "skipped"
			progress.Percent = 100
			progress.BytesDone = sourceInfo.Size()
			progress.Message = "File already exists with matching checksum"
			if onProgress != nil {
				onProgress(progress)
			}
			return fmt.Errorf("model already exists: %s", fileName)
		}

		// Different file, remove old one
		if err := os.Remove(destPath); err != nil {
			return fmt.Errorf("cannot remove existing file: %w", err)
		}
	}

	// Create models directory if it doesn't exist
	if err := os.MkdirAll(u.modelsDir, 0755); err != nil {
		return fmt.Errorf("cannot create models directory: %w", err)
	}

	// Open source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create temporary file first (atomic import)
	tempPath := destPath + ".tmp"
	destFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("cannot create temporary file: %w", err)
	}
	defer func() {
		destFile.Close()
		// Clean up temp file if still exists (error case)
		os.Remove(tempPath)
	}()

	// Copy with progress tracking
	buffer := make([]byte, 1024*1024) // 1MB buffer
	var bytesCopied int64

	for {
		n, err := sourceFile.Read(buffer)
		if n > 0 {
			if _, writeErr := destFile.Write(buffer[:n]); writeErr != nil {
				return fmt.Errorf("write error: %w", writeErr)
			}

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
			return fmt.Errorf("read error: %w", err)
		}
	}

	// Verify size
	if bytesCopied != sourceInfo.Size() {
		return fmt.Errorf("incomplete copy: expected %d bytes, got %d", sourceInfo.Size(), bytesCopied)
	}

	// Verify integrity by comparing hashes
	progress.Status = "verifying"
	if onProgress != nil {
		onProgress(progress)
	}

	sourceHash, err := u.calculateFileHash(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot verify source: %w", err)
	}

	if expectedSHA256 != "" && sourceHash != expectedSHA256 {
		return fmt.Errorf("manifest verification failed for %s: expected %s, got %s", fileName, expectedSHA256, sourceHash)
	}

	destHash, err := u.calculateFileHash(tempPath)
	if err != nil {
		return fmt.Errorf("cannot verify destination: %w", err)
	}

	if sourceHash != destHash {
		return fmt.Errorf("verification failed: hash mismatch")
	}

	// Additional validation using the validator
	validationResult, err := u.validator.ValidateModel(tempPath)
	if err != nil {
		return fmt.Errorf("validation check failed: %w", err)
	}

	if !validationResult.Valid {
		return fmt.Errorf("model validation failed: %v", validationResult.Errors)
	}

	if validationResult.IsCorrupted {
		return fmt.Errorf("model file is corrupted")
	}

	if !validationResult.IsGGUF {
		return fmt.Errorf("not a valid GGUF file")
	}

	// Atomic move: rename temp file to final destination
	if err := os.Rename(tempPath, destPath); err != nil {
		return fmt.Errorf("failed to move file to destination: %w", err)
	}

	// Success - clear the cleanup flag since rename succeeded
	tempPath = ""

	progress.Status = "complete"
	progress.Percent = 100
	if onProgress != nil {
		onProgress(progress)
	}

	// Rescan registry to include new model
	if u.registry != nil {
		if err := u.registry.ScanModels(); err != nil {
			// Log but don't fail - the file was copied successfully
			fmt.Printf("Warning: failed to rescan models: %v\n", err)
		}
	}

	return nil
}

// ImportAll imports all GGUF files from a USB drive
func (u *USBImporter) ImportAll(usbPath string, onProgress func(ImportProgress)) (int, error) {
	// Try to load manifest first
	manifest, manifestErr := u.LoadManifest(usbPath)
	var idx *manifestIndex
	if manifestErr == nil && manifest != nil {
		fmt.Printf("Found package manifest: %d models, %.2f GB total\n",
			manifest.TotalModels, manifest.TotalSizeGB)
		idx = &manifestIndex{byFileName: make(map[string]string, len(manifest.Models))}
		for _, entry := range manifest.Models {
			name := strings.ToLower(strings.TrimSpace(entry.FileName))
			sha := strings.ToLower(strings.TrimSpace(entry.SHA256))
			if name != "" && sha != "" {
				idx.byFileName[name] = sha
			}
		}
	}

	modelFiles, err := u.ScanUSBDrive(usbPath)
	if err != nil {
		return 0, err
	}

	if len(modelFiles) == 0 {
		return 0, fmt.Errorf("no GGUF model files found in %s", usbPath)
	}

	// Check available disk space
	var totalSize int64
	for _, file := range modelFiles {
		info, err := os.Stat(file)
		if err == nil {
			totalSize += info.Size()
		}
	}

	availableSpace, err := getDiskSpace(u.modelsDir)
	if err == nil && availableSpace < totalSize {
		return 0, fmt.Errorf("insufficient disk space: need %s, available %s",
			formatBytes(totalSize), formatBytes(availableSpace))
	}

	imported := 0
	skipped := 0
	manifestSkipped := 0

	for _, modelFile := range modelFiles {
		fileName := strings.ToLower(filepath.Base(modelFile))
		expected := ""
		if idx != nil {
			var ok bool
			expected, ok = idx.byFileName[fileName]
			if !ok {
				manifestSkipped++
				if onProgress != nil {
					onProgress(ImportProgress{
						FilePath: modelFile,
						FileName: filepath.Base(modelFile),
						Status:   "skipped",
						Percent:  100,
						Message:  "Skipped (not listed in manifest)",
					})
				}
				continue
			}
		}

		err := u.ImportModel(modelFile, expected, onProgress)
		if err != nil {
			// Check if it was skipped (already exists)
			if strings.Contains(err.Error(), "already exists") {
				skipped++
			} else {
				fmt.Printf("Failed to import %s: %v\n", modelFile, err)
			}
			continue
		}
		imported++
	}

	if skipped > 0 {
		fmt.Printf("Skipped %d model(s) (already imported)\n", skipped)
	}
	if manifestSkipped > 0 {
		fmt.Printf("Skipped %d model(s) (not listed in manifest)\n", manifestSkipped)
	}

	return imported, nil
}

// calculateFileHash computes SHA256 hash of a file
func (u *USBImporter) calculateFileHash(filePath string) (string, error) {
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

// GetModelInfo extracts basic info from a GGUF filename
// Example: tinyllama-1.1b-chat.Q4_K_M.gguf -> model: tinyllama-1.1b-chat, quant: Q4_K_M
func (u *USBImporter) GetModelInfo(filename string) (modelID string, quantization string) {
	// Remove .gguf extension
	name := strings.TrimSuffix(filename, ".gguf")

	// Common quantization patterns
	quants := []string{"Q2_K", "Q3_K_S", "Q3_K_M", "Q3_K_L", "Q4_0", "Q4_K_S", "Q4_K_M",
		"Q5_0", "Q5_K_S", "Q5_K_M", "Q6_K", "Q8_0"}

	for _, q := range quants {
		if strings.HasSuffix(name, "."+q) {
			modelID = strings.TrimSuffix(name, "."+q)
			quantization = q
			return
		}
	}

	// No recognized quantization, use whole name as model ID
	modelID = name
	quantization = "unknown"
	return
}

// LoadManifest loads a package manifest from USB if available
func (u *USBImporter) LoadManifest(usbPath string) (*PackageManifest, error) {
	// Try common manifest locations
	manifestPaths := []string{
		filepath.Join(usbPath, "manifest.json"),
		filepath.Join(usbPath, "offgrid-models", "manifest.json"),
	}

	for _, manifestPath := range manifestPaths {
		if _, err := os.Stat(manifestPath); err == nil {
			file, err := os.Open(manifestPath)
			if err != nil {
				continue
			}
			defer file.Close()

			var manifest PackageManifest
			if err := json.NewDecoder(file).Decode(&manifest); err != nil {
				continue
			}
			return &manifest, nil
		}
	}

	return nil, fmt.Errorf("no manifest found")
}
