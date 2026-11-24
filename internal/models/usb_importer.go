package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
			if info.Name() == PackageDirectory {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() {
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
var errUSBModelIdentical = errors.New("model already exists with identical bytes")

func (u *USBImporter) ImportModel(sourcePath string, expectedSHA256 string, onProgress func(ImportProgress)) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot access source file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("model source must be a regular file")
	}
	fileName := filepath.Base(sourcePath)
	expectedSHA256 = strings.ToLower(strings.TrimSpace(expectedSHA256))
	if expectedSHA256 != "" {
		if digest, err := hex.DecodeString(expectedSHA256); err != nil || len(digest) != sha256.Size {
			return fmt.Errorf("invalid manifest digest for %s", fileName)
		}
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	opened, err := source.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(info, opened) {
		return fmt.Errorf("model source changed during import")
	}
	progress := ImportProgress{FilePath: sourcePath, FileName: fileName, BytesTotal: info.Size(), Status: "copying"}
	notify := func() {
		if onProgress != nil {
			onProgress(progress)
		}
	}
	registry := u.registry
	if registry == nil {
		registry = NewRegistry(u.modelsDir)
	}
	reader := &usbProgressReader{Reader: source, progress: &progress, notify: notify}
	_, err = registry.importGGUF(context.Background(), fileName, info.Size(), reader, expectedSHA256)
	if errors.Is(err, ErrGGUFImportExists) {
		// An update never replaces an installed filename. Only identical bytes
		// count as an idempotent retry; conflicting models need a new identity.
		dest := filepath.Join(u.modelsDir, fileName)
		target, statErr := os.Lstat(dest)
		if statErr == nil && target.Mode().IsRegular() {
			sourceHash, hashErr := u.calculateFileHash(sourcePath)
			if hashErr != nil {
				return hashErr
			}
			if expectedSHA256 != "" && sourceHash != expectedSHA256 {
				return fmt.Errorf("manifest verification failed for %s", fileName)
			}
			destHash, hashErr := u.calculateFileHash(dest)
			if hashErr == nil && sourceHash == destHash {
				progress.Status, progress.Percent, progress.BytesDone = "skipped", 100, info.Size()
				progress.Message = "File already exists with matching checksum"
				notify()
				return errUSBModelIdentical
			}
		}
		return fmt.Errorf("model filename conflicts with installed data; use a new filename: %s", fileName)
	}
	if err != nil {
		return err
	}
	progress.Status, progress.Percent, progress.BytesDone = "complete", 100, info.Size()
	notify()
	return nil
}

type usbProgressReader struct {
	io.Reader
	progress *ImportProgress
	notify   func()
}

func (r *usbProgressReader) Read(b []byte) (int, error) {
	n, err := r.Reader.Read(b)
	r.progress.BytesDone += int64(n)
	if r.progress.BytesTotal > 0 {
		r.progress.Percent = float64(r.progress.BytesDone) / float64(r.progress.BytesTotal) * 100
	}
	if err == io.EOF {
		r.progress.Status = "verifying"
	}
	r.notify()
	return n, err
}

// ImportAll imports all GGUF files from a USB drive
func (u *USBImporter) ImportAll(usbPath string, onProgress func(ImportProgress)) (int, error) {
	return u.importAll(usbPath, onProgress, getDiskSpace)
}

func (u *USBImporter) importAll(usbPath string, onProgress func(ImportProgress), diskSpace func(string) (int64, error)) (int, error) {
	// Try to load manifest first
	manifest, manifestErr := u.LoadManifest(usbPath)
	if manifestErr != nil && !errors.Is(manifestErr, os.ErrNotExist) {
		return 0, manifestErr
	}
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
	if idx != nil {
		found := map[string]bool{}
		for _, file := range modelFiles {
			name := strings.ToLower(filepath.Base(file))
			if _, listed := idx.byFileName[name]; !listed {
				continue
			}
			if found[name] {
				return 0, fmt.Errorf("ambiguous model filename in package: %s", name)
			}
			found[name] = true
		}
		for name := range idx.byFileName {
			if !found[name] {
				return 0, fmt.Errorf("manifest model is missing: %s", name)
			}
		}
	}

	// Check available disk space
	var totalSize int64
	for _, file := range modelFiles {
		if idx != nil {
			if _, listed := idx.byFileName[strings.ToLower(filepath.Base(file))]; !listed {
				continue
			}
		}
		info, err := os.Stat(file)
		if err == nil {
			totalSize += info.Size()
		}
	}

	availableSpace, err := diskSpace(u.modelsDir)
	if err == nil && availableSpace < totalSize {
		return 0, fmt.Errorf("insufficient disk space: need %s, available %s",
			formatBytes(totalSize), formatBytes(availableSpace))
	}

	imported := 0
	skipped := 0
	manifestSkipped := 0
	var failures []error

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
			if errors.Is(err, errUSBModelIdentical) {
				skipped++
			} else {
				failures = append(failures, fmt.Errorf("import %s: %w", filepath.Base(modelFile), err))
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

	return imported, errors.Join(failures...)
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
		filepath.Join(usbPath, "models", "manifest.json"),
	}

	for _, manifestPath := range manifestPaths {
		info, err := os.Lstat(manifestPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > 1<<20 {
			return nil, fmt.Errorf("invalid USB manifest file")
		}
		{
			file, err := os.Open(manifestPath)
			if err != nil {
				return nil, err
			}
			defer file.Close()

			var manifest PackageManifest
			decoder := json.NewDecoder(io.LimitReader(file, (1<<20)+1))
			if err := decoder.Decode(&manifest); err != nil {
				return nil, fmt.Errorf("invalid USB manifest: %w", err)
			}
			if decoder.Decode(new(any)) != io.EOF {
				return nil, fmt.Errorf("invalid trailing USB manifest data")
			}
			if len(manifest.Models) == 0 {
				return nil, fmt.Errorf("USB manifest contains no models")
			}
			seen := map[string]bool{}
			for _, entry := range manifest.Models {
				name := strings.ToLower(entry.FileName)
				digest, err := hex.DecodeString(entry.SHA256)
				if !validArtifactPath(entry.FileName) || strings.Contains(entry.FileName, "/") || !strings.HasSuffix(name, ".gguf") || seen[name] || err != nil || len(digest) != sha256.Size {
					return nil, fmt.Errorf("invalid or duplicate USB manifest model entry")
				}
				seen[name] = true
			}
			return &manifest, nil
		}
	}

	return nil, fmt.Errorf("no manifest found: %w", os.ErrNotExist)
}
