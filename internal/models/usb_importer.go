package models

import (
	"crypto/sha256"
	"encoding/hex"
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
}

// NewUSBImporter creates a new USB importer
func NewUSBImporter(modelsDir string, registry *Registry) *USBImporter {
	return &USBImporter{
		modelsDir: modelsDir,
		registry:  registry,
	}
}

// ImportProgress represents import progress
type ImportProgress struct {
	FilePath   string
	FileName   string
	BytesTotal int64
	BytesDone  int64
	Percent    float64
	Status     string // "copying", "verifying", "complete", "failed"
	Error      error
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
func (u *USBImporter) ImportModel(sourcePath string, onProgress func(ImportProgress)) error {
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

		destHash, err := u.calculateFileHash(destPath)
		if err == nil && sourceHash == destHash {
			progress.Status = "complete"
			progress.Percent = 100
			progress.BytesDone = sourceInfo.Size()
			if onProgress != nil {
				onProgress(progress)
			}
			return nil // File already exists and matches
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

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("cannot create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy with progress tracking
	buffer := make([]byte, 1024*1024) // 1MB buffer
	var bytesCopied int64

	for {
		n, err := sourceFile.Read(buffer)
		if n > 0 {
			if _, writeErr := destFile.Write(buffer[:n]); writeErr != nil {
				os.Remove(destPath) // Clean up partial file
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
			os.Remove(destPath) // Clean up partial file
			return fmt.Errorf("read error: %w", err)
		}
	}

	// Verify size
	if bytesCopied != sourceInfo.Size() {
		os.Remove(destPath)
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

	destHash, err := u.calculateFileHash(destPath)
	if err != nil {
		return fmt.Errorf("cannot verify destination: %w", err)
	}

	if sourceHash != destHash {
		os.Remove(destPath)
		return fmt.Errorf("verification failed: hash mismatch")
	}

	// Success
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
	modelFiles, err := u.ScanUSBDrive(usbPath)
	if err != nil {
		return 0, err
	}

	if len(modelFiles) == 0 {
		return 0, fmt.Errorf("no GGUF model files found in %s", usbPath)
	}

	imported := 0
	for _, modelFile := range modelFiles {
		if err := u.ImportModel(modelFile, onProgress); err != nil {
			fmt.Printf("Failed to import %s: %v\n", modelFile, err)
			continue
		}
		imported++
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
