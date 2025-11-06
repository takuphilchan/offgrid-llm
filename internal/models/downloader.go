package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Downloader handles model downloads from various sources
type Downloader struct {
	catalog    *ModelCatalog
	modelsDir  string
	client     *http.Client
	onProgress func(DownloadProgress)
}

// DownloadProgress represents download progress
type DownloadProgress struct {
	ModelID       string
	Variant       string
	BytesTotal    int64
	BytesDone     int64
	Percent       float64
	Speed         int64 // Bytes per second
	TimeRemaining time.Duration
	Status        string // "downloading", "verifying", "complete", "failed"
	Error         error
}

// NewDownloader creates a new model downloader
func NewDownloader(modelsDir string, catalog *ModelCatalog) *Downloader {
	return &Downloader{
		catalog:   catalog,
		modelsDir: modelsDir,
		client: &http.Client{
			Timeout: 0, // No timeout for large downloads
		},
	}
}

// SetProgressCallback sets a callback for download progress
func (d *Downloader) SetProgressCallback(callback func(DownloadProgress)) {
	d.onProgress = callback
}

// Download downloads a model by ID and quantization
func (d *Downloader) Download(modelID, quantization string) error {
	// Find model in catalog
	entry := d.catalog.FindModel(modelID)
	if entry == nil {
		return fmt.Errorf("model not found in catalog: %s", modelID)
	}

	// Find variant
	variant := entry.FindVariant(quantization)
	if variant == nil {
		return fmt.Errorf("quantization not found: %s", quantization)
	}

	// Try sources in priority order
	for _, source := range variant.Sources {
		err := d.downloadFromSource(modelID, quantization, variant, source)
		if err == nil {
			return nil // Success
		}

		// If not a mirror, fail immediately
		if !source.Mirror {
			return err
		}

		// Try next mirror
		fmt.Printf("Source failed, trying mirror: %v\n", err)
	}

	return fmt.Errorf("all sources failed for %s", modelID)
}

// downloadFromSource downloads from a specific source
func (d *Downloader) downloadFromSource(modelID, quantization string, variant *ModelVariant, source ModelSource) error {
	progress := DownloadProgress{
		ModelID:    modelID,
		Variant:    quantization,
		BytesTotal: variant.Size,
		Status:     "downloading",
	}

	// Create temporary file
	tmpPath := filepath.Join(d.modelsDir, fmt.Sprintf(".%s-%s.tmp", modelID, quantization))
	destPath := filepath.Join(d.modelsDir, fmt.Sprintf("%s.%s.gguf", modelID, quantization))

	// Check if partially downloaded
	var bytesWritten int64
	if stat, err := os.Stat(tmpPath); err == nil {
		bytesWritten = stat.Size()
		progress.BytesDone = bytesWritten
		d.notifyProgress(progress)
	}

	// Create HTTP request with Range support for resume
	req, err := http.NewRequest("GET", source.URL, nil)
	if err != nil {
		return err
	}

	if bytesWritten > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", bytesWritten))
	}

	// Execute request
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Open file for writing
	flag := os.O_CREATE | os.O_WRONLY
	if bytesWritten > 0 {
		flag |= os.O_APPEND
	}

	file, err := os.OpenFile(tmpPath, flag, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Download with progress tracking
	startTime := time.Now()
	lastUpdate := time.Now()
	updateInterval := 500 * time.Millisecond

	buffer := make([]byte, 32*1024) // 32KB buffer

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}

			bytesWritten += int64(n)

			// Update progress periodically
			if time.Since(lastUpdate) >= updateInterval {
				elapsed := time.Since(startTime).Seconds()
				speed := int64(float64(bytesWritten) / elapsed)

				progress.BytesDone = bytesWritten
				progress.Percent = float64(bytesWritten) / float64(variant.Size) * 100
				progress.Speed = speed

				if speed > 0 {
					remaining := variant.Size - bytesWritten
					progress.TimeRemaining = time.Duration(float64(remaining)/float64(speed)) * time.Second
				}

				d.notifyProgress(progress)
				lastUpdate = time.Now()
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// Verify file size
	if bytesWritten != variant.Size {
		return fmt.Errorf("incomplete download: got %d bytes, expected %d", bytesWritten, variant.Size)
	}

	// Verify SHA256 if provided
	if variant.SHA256 != "" {
		progress.Status = "verifying"
		d.notifyProgress(progress)

		if err := d.verifyChecksum(tmpPath, variant.SHA256); err != nil {
			return err
		}
	}

	// Move to final location
	if err := os.Rename(tmpPath, destPath); err != nil {
		return err
	}

	progress.Status = "complete"
	progress.Percent = 100
	d.notifyProgress(progress)

	return nil
}

// verifyChecksum verifies file SHA256
func (d *Downloader) verifyChecksum(path, expectedHash string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actualHash := hex.EncodeToString(hash.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// notifyProgress calls the progress callback if set
func (d *Downloader) notifyProgress(progress DownloadProgress) {
	if d.onProgress != nil {
		d.onProgress(progress)
	}
}

// ListAvailableModels returns models from the catalog
func (d *Downloader) ListAvailableModels() []CatalogEntry {
	return d.catalog.Models
}

// GetModelInfo returns detailed info about a model
func (d *Downloader) GetModelInfo(modelID string) (*CatalogEntry, error) {
	entry := d.catalog.FindModel(modelID)
	if entry == nil {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}
	return entry, nil
}
