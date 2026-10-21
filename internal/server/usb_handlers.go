package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

// handleUSBScan scans a USB drive for GGUF models
func (s *Server) handleUSBScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		USBPath string `json:"usb_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.USBPath == "" {
		writeError(w, "usb_path is required", http.StatusBadRequest)
		return
	}

	importer := models.NewUSBImporter(s.config.ModelsDir, s.registry)
	modelFiles, err := importer.ScanUSBDrive(req.USBPath)
	if err != nil {
		writeError(w, fmt.Sprintf("Failed to scan USB drive: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract model info
	type ModelInfo struct {
		FilePath     string `json:"file_path"`
		FileName     string `json:"file_name"`
		Size         int64  `json:"size"`
		ModelID      string `json:"model_id"`
		Quantization string `json:"quantization"`
	}

	var models []ModelInfo
	for _, path := range modelFiles {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		fileName := filepath.Base(path)
		modelID, quant := importer.GetModelInfo(fileName)

		models = append(models, ModelInfo{
			FilePath:     path,
			FileName:     fileName,
			Size:         info.Size(),
			ModelID:      modelID,
			Quantization: quant,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"usb_path": req.USBPath,
		"models":   models,
		"count":    len(models),
	})
}

// handleUSBImport imports models from USB
func (s *Server) handleUSBImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		writeError(w, "path is required", http.StatusBadRequest)
		return
	}

	// Verify path exists
	if _, err := os.Stat(req.Path); os.IsNotExist(err) {
		writeError(w, fmt.Sprintf("Path does not exist: %s", req.Path), http.StatusBadRequest)
		return
	}

	importer := models.NewUSBImporter(s.config.ModelsDir, s.registry)

	// Import all models from the USB path
	imported, err := importer.ImportAll(req.Path, nil)
	if err != nil {
		writeError(w, fmt.Sprintf("Import failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Rescan models to update registry
	if err := s.registry.ScanModels(); err != nil {
		fmt.Printf("Warning: failed to rescan models: %v\n", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"imported_count": imported,
		"status":         "success",
	})
}

// handleUSBExport exports models to USB
func (s *Server) handleUSBExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path             string `json:"path"`
		IncludeInstaller bool   `json:"include_installer"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		writeError(w, "path is required", http.StatusBadRequest)
		return
	}

	// Verify USB path exists
	if _, err := os.Stat(req.Path); os.IsNotExist(err) {
		writeError(w, fmt.Sprintf("Path does not exist: %s", req.Path), http.StatusBadRequest)
		return
	}

	var exporter *models.USBExporter
	if req.IncludeInstaller {
		// Find the offgrid binary to include
		binaryPath := ""
		if exe, err := os.Executable(); err == nil {
			binaryPath = exe
		}
		exporter = models.NewUSBExporterWithInstaller(s.config.ModelsDir, s.registry, binaryPath)
	} else {
		exporter = models.NewUSBExporter(s.config.ModelsDir, s.registry)
	}

	// Export all models
	exported, err := exporter.ExportAll(req.Path, nil)
	if err != nil {
		writeError(w, fmt.Sprintf("Export failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Calculate total size
	var totalSize int64
	allModels := s.registry.ListModels()
	for _, model := range allModels {
		meta, err := s.registry.GetModel(model.ID)
		if err == nil && meta != nil {
			totalSize += meta.Size
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"exported_count":     exported,
		"total_size_gb":      float64(totalSize) / (1024 * 1024 * 1024),
		"installer_included": req.IncludeInstaller,
		"status":             "success",
	})
}

func (s *Server) exportModelsAsync(modelIDs []string, usbPath string) {
	for _, modelID := range modelIDs {
		// Get model metadata from registry to find actual filename
		metadata, err := s.registry.GetModel(modelID)
		if err != nil {
			s.exportMutex.Lock()
			s.exportProgress[modelID] = &ExportProgress{
				FileName: modelID,
				Status:   "failed",
				Error:    "Model not found in registry",
			}
			s.exportMutex.Unlock()
			continue
		}

		sourcePath := metadata.Path

		// Check if model file exists
		info, err := os.Stat(sourcePath)
		if err != nil {
			s.exportMutex.Lock()
			s.exportProgress[modelID] = &ExportProgress{
				FileName: modelID,
				Status:   "failed",
				Error:    "Model file not found",
			}
			s.exportMutex.Unlock()
			continue
		}

		fileName := filepath.Base(sourcePath)

		// Initialize progress
		s.exportMutex.Lock()
		s.exportProgress[modelID] = &ExportProgress{
			FileName:   fileName,
			BytesTotal: info.Size(),
			BytesDone:  0,
			Percent:    0,
			Status:     "exporting",
		}
		s.exportMutex.Unlock()

		// Copy to USB with original filename
		destPath := filepath.Join(usbPath, fileName)

		sourceFile, err := os.Open(sourcePath)
		if err != nil {
			s.exportMutex.Lock()
			s.exportProgress[modelID].Status = "failed"
			s.exportProgress[modelID].Error = fmt.Sprintf("Cannot open source: %v", err)
			s.exportMutex.Unlock()
			continue
		}

		destFile, err := os.Create(destPath)
		if err != nil {
			sourceFile.Close()
			s.exportMutex.Lock()
			s.exportProgress[modelID].Status = "failed"
			s.exportProgress[modelID].Error = fmt.Sprintf("Cannot create destination: %v", err)
			s.exportMutex.Unlock()
			continue
		}

		// Copy with progress tracking
		buffer := make([]byte, 1024*1024) // 1MB buffer
		var bytesCopied int64
		lastUpdate := time.Now()

		for {
			n, err := sourceFile.Read(buffer)
			if n > 0 {
				if _, writeErr := destFile.Write(buffer[:n]); writeErr != nil {
					sourceFile.Close()
					destFile.Close()
					os.Remove(destPath)
					s.exportMutex.Lock()
					s.exportProgress[modelID].Status = "failed"
					s.exportProgress[modelID].Error = fmt.Sprintf("Write error: %v", writeErr)
					s.exportMutex.Unlock()
					break
				}

				bytesCopied += int64(n)

				// Update progress every 5 seconds or 100MB
				if time.Since(lastUpdate) > 5*time.Second || bytesCopied-s.exportProgress[modelID].BytesDone > 100*1024*1024 {
					s.exportMutex.Lock()
					s.exportProgress[modelID].BytesDone = bytesCopied
					s.exportProgress[modelID].Percent = float64(bytesCopied) / float64(info.Size()) * 100
					s.exportMutex.Unlock()
					lastUpdate = time.Now()
				}
			}

			if err == io.EOF {
				break
			}

			if err != nil {
				sourceFile.Close()
				destFile.Close()
				os.Remove(destPath)
				s.exportMutex.Lock()
				s.exportProgress[modelID].Status = "failed"
				s.exportProgress[modelID].Error = fmt.Sprintf("Read error: %v", err)
				s.exportMutex.Unlock()
				break
			}
		}

		sourceFile.Close()
		destFile.Close()

		// Final update
		if s.exportProgress[modelID].Status != "failed" {
			s.exportMutex.Lock()
			s.exportProgress[modelID].BytesDone = bytesCopied
			s.exportProgress[modelID].Percent = 100
			s.exportProgress[modelID].Status = "complete"
			s.exportMutex.Unlock()
		}
	}
}

// handleExportProgress returns export progress for all active exports
func (s *Server) handleExportProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	s.exportMutex.RLock()
	progress := make(map[string]*ExportProgress)
	for k, v := range s.exportProgress {
		progress[k] = v
	}
	s.exportMutex.RUnlock()

	if err := json.NewEncoder(w).Encode(progress); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}
