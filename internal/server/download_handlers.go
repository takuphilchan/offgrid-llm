package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/audit"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

// handleDownloadModel downloads a model from HuggingFace
func (s *Server) handleDownloadModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Repository      string `json:"repository"`
		FileName        string `json:"file_name"`
		Quantization    string `json:"quantization"` // Optional: just the quant like "Q4_K_M"
		ModelID         string `json:"model_id"`     // Optional stable catalog ID used as the local filename.
		EnableKnowledge bool   `json:"enable_knowledge"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Repository == "" && !req.EnableKnowledge {
		writeError(w, "repository is required", http.StatusBadRequest)
		return
	}
	if req.EnableKnowledge && s.config.RequireAuth {
		user := users.GetUser(r)
		if user == nil || !user.HasPermission(users.PermissionRAGManage) {
			writeError(w, "Knowledge setup requires permission to manage knowledge", http.StatusForbidden)
			return
		}
	}

	// If only quantization is provided, fetch the model to find the actual filename
	if req.FileName == "" && req.Quantization != "" {
		hf := models.NewHuggingFaceClient()
		modelInfo, err := hf.GetModelInfo(req.Repository)
		if err == nil {
			quantUpper := strings.ToUpper(req.Quantization)
			// Extract base quant type (e.g., "Q4" from "Q4_K_M")
			baseQuant := quantUpper
			if idx := strings.Index(quantUpper, "_"); idx > 0 {
				baseQuant = quantUpper[:idx]
			}

			// Find GGUF file with matching quantization
			for _, sibling := range modelInfo.Siblings {
				if !strings.HasSuffix(sibling.Filename, ".gguf") {
					continue
				}
				fnameUpper := strings.ToUpper(sibling.Filename)
				// Try exact match first, then base quant match
				if strings.Contains(fnameUpper, quantUpper) ||
					strings.Contains(fnameUpper, "-"+baseQuant+".") ||
					strings.Contains(fnameUpper, "_"+baseQuant+".") ||
					strings.Contains(fnameUpper, "-"+baseQuant+"_") {
					req.FileName = sibling.Filename
					break
				}
			}
		}
	}

	if req.FileName == "" {
		writeError(w, "file_name is required or could not be determined", http.StatusBadRequest)
		return
	}

	// Check if model already exists
	sourceFileName := strings.TrimSpace(req.FileName)
	destFileName := filepath.Base(sourceFileName)
	if req.ModelID != "" {
		modelID := strings.TrimSpace(req.ModelID)
		if !isSafeModelID(modelID) {
			writeError(w, "model_id may contain only letters, numbers, dot, underscore, and hyphen", http.StatusBadRequest)
			return
		}
		destFileName = modelID + ".gguf"
	}
	if destFileName == "" || destFileName == "." || destFileName == string(filepath.Separator) {
		writeError(w, "invalid file_name", http.StatusBadRequest)
		return
	}
	if !strings.HasSuffix(strings.ToLower(destFileName), ".gguf") {
		destFileName += ".gguf"
		sourceFileName += ".gguf"
	}
	destPath := filepath.Join(s.config.ModelsDir, destFileName)
	if !isSafeModelID(strings.TrimSuffix(destFileName, ".gguf")) {
		writeError(w, "Invalid local model filename; provide a safe model_id", 400)
		return
	}
	if _, err := os.Stat(destPath); err == nil {
		if !req.EnableKnowledge {
			if err := s.registry.ScanModels(); err != nil {
				writeError(w, "Model exists but could not be discovered. Check storage access.", 500)
				return
			}
			// Clear stale retry UI only after discovery and durable state agree.
			s.downloadMutex.Lock()
			if _, active := s.downloadCancelFuncs[destFileName]; !active {
				s.downloadProgress[destFileName] = &DownloadProgress{FileName: destFileName, Status: "complete", Percent: 100, StartedAt: time.Now().Unix(), Repository: req.Repository, SourceFile: sourceFileName, ModelID: strings.TrimSuffix(destFileName, ".gguf"), Quantization: req.Quantization}
				if err := s.saveDownloadsLocked(); err != nil {
					s.downloadMutex.Unlock()
					writeError(w, "Model exists but its saved state could not be updated. Check storage access.", 500)
					return
				}
			}
			s.downloadMutex.Unlock()
			// Model already exists.
			response := map[string]interface{}{
				"success":   true,
				"status":    "complete",
				"exists":    true,
				"message":   fmt.Sprintf("Model %s already exists", destFileName),
				"file_name": destFileName,
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				log.Printf("Error encoding response: %v", err)
			}
			return
		}
	}
	if req.Repository == "" {
		if _, err := os.Stat(destPath); err != nil {
			writeError(w, "Embedding model is not installed", 404)
			return
		}
	}

	// Create one authoritative progress/cancellation record before launching the
	// worker. A cancelled .tmp file is deliberately retained for a later resume.
	baseCtx := s.runtimeCtx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(baseCtx)
	s.downloadMutex.Lock()
	if _, active := s.downloadCancelFuncs[destFileName]; active {
		s.downloadMutex.Unlock()
		cancel()
		writeError(w, "download is already active for "+destFileName, http.StatusConflict)
		return
	}
	if previous := s.downloadProgress[destFileName]; previous != nil && req.Repository != "" {
		if info, statErr := os.Stat(destPath + ".tmp"); statErr == nil && info.Size() > 0 &&
			(previous.Repository != req.Repository || previous.SourceFile != sourceFileName) {
			s.downloadMutex.Unlock()
			cancel()
			writeError(w, "A retained partial download belongs to a different source. Remove or finish it before reusing this model ID.", http.StatusConflict)
			return
		}
	}
	s.downloadCancelFuncs[destFileName] = cancel
	s.downloadProgress[destFileName] = &DownloadProgress{
		FileName:   destFileName,
		Status:     "downloading",
		StartedAt:  time.Now().Unix(),
		Repository: req.Repository, SourceFile: sourceFileName, ModelID: strings.TrimSuffix(destFileName, ".gguf"), Quantization: req.Quantization, EnableKnowledge: req.EnableKnowledge,
	}
	if err := s.saveDownloadsLocked(); err != nil {
		delete(s.downloadCancelFuncs, destFileName)
		delete(s.downloadProgress, destFileName)
		s.downloadMutex.Unlock()
		cancel()
		writeError(w, "Cannot save download request. Check storage access and free space.", 500)
		return
	}
	s.downloadWorkers.Add(1)
	s.downloadMutex.Unlock()

	// Start download in background
	go func() {
		defer s.downloadWorkers.Done()
		s.downloadModelAsync(ctx, req.Repository, sourceFileName, destFileName)
	}()

	// Audit model download initiation
	user := users.GetUser(r)
	userName := "anonymous"
	if user != nil {
		userName = user.Username
	}
	s.auditLog(audit.EventTypeModel, "model_download_started", userName, r.RemoteAddr, map[string]string{
		"repository": req.Repository,
		"file_name":  req.FileName,
	}, true)

	response := map[string]interface{}{
		"success":   true,
		"message":   fmt.Sprintf("Download started for %s/%s", req.Repository, sourceFileName),
		"status":    "downloading",
		"file_name": destFileName,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func isSafeModelID(modelID string) bool {
	if modelID == "" || len(modelID) > 128 {
		return false
	}
	for _, char := range modelID {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func (s *Server) downloadModelAsync(ctx context.Context, repository, sourceFileName, progressKey string) {
	// Clean up cancel func when done
	defer func() {
		s.downloadMutex.Lock()
		if cancel := s.downloadCancelFuncs[progressKey]; cancel != nil {
			cancel()
		}
		delete(s.downloadCancelFuncs, progressKey)
		if err := s.saveDownloadsLocked(); err != nil {
			if progress := s.downloadProgress[progressKey]; progress != nil {
				progress.Status = "failed"
				progress.Error = "Could not save download state. Check disk space; refresh installed models before retrying."
			}
		}
		s.downloadMutex.Unlock()
	}()

	log.Printf("Starting resumable download: %s/%s", repository, sourceFileName)
	startTime := time.Now()
	destPath := filepath.Join(s.config.ModelsDir, progressKey)
	hf := models.NewHuggingFaceClient()
	var initialBytes int64 = -1
	progressCallback := func(done, total int64) {
		if initialBytes < 0 {
			initialBytes = done
		}
		s.downloadMutex.Lock()
		if progress := s.downloadProgress[progressKey]; progress != nil {
			progress.BytesDone = done
			progress.BytesTotal = total
			if total > 0 {
				progress.Percent = min(100, float64(done)/float64(total)*100)
			}
			if elapsed := time.Since(startTime).Seconds(); elapsed > 0 {
				progress.Speed = float64(done-initialBytes) / elapsed
			}
			if total > 0 && done >= total && progress.Status == "downloading" {
				progress.Status = "finalizing"
				progress.Speed = 0
			}
		}
		s.downloadMutex.Unlock()
	}
	var err error
	if info, statErr := os.Stat(destPath); statErr == nil {
		progressCallback(info.Size(), info.Size())
	} else {
		err = hf.DownloadGGUFContext(ctx, repository, sourceFileName, destPath, progressCallback)
	}
	if err == nil {
		err = s.registry.ScanModels()
	}
	if err == nil {
		_, err = s.registry.GetModel(strings.TrimSuffix(progressKey, ".gguf"))
	}
	s.downloadMutex.RLock()
	progress := s.downloadProgress[progressKey]
	enableKnowledge := progress != nil && progress.EnableKnowledge
	s.downloadMutex.RUnlock()
	if err == nil && enableKnowledge {
		if s.ragEngine == nil {
			err = fmt.Errorf("knowledge engine is unavailable")
		} else {
			err = s.ragEngine.Enable(ctx, strings.TrimSuffix(progressKey, ".gguf"))
		}
		if err != nil {
			err = fmt.Errorf("model is installed, but knowledge setup failed: %w; choose Resume to retry setup", err)
		}
	}
	fileInfo, statErr := os.Stat(destPath)
	if err == nil {
		err = statErr
	}
	if err != nil {
		status := "failed"
		message := err.Error()
		if errors.Is(err, models.ErrFinalizeDownload) {
			message = "Downloaded data was kept, but the model file is still in use or could not be moved. Close applications using it, then choose Resume."
		}
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			status = "cancelled"
			message = "Download cancelled; partial data was kept for resume"
		}
		log.Printf("Model download %s: %s: %v", status, progressKey, err)
		s.downloadMutex.Lock()
		if progress := s.downloadProgress[progressKey]; progress != nil {
			progress.Status = status
			progress.Error = message
			progress.Speed = 0
		}
		if saveErr := s.saveDownloadsLocked(); saveErr != nil {
			if progress := s.downloadProgress[progressKey]; progress != nil {
				progress.Status = "failed"
				progress.Error = "Download stopped, but its recovery state could not be saved. Check storage access and refresh installed models."
			}
		}
		s.downloadMutex.Unlock()
		return
	}

	// Publish completion only after discovery. Otherwise a client's one-time
	// completion refresh can race the scan and never show the installed model.
	s.downloadMutex.Lock()
	if progress := s.downloadProgress[progressKey]; progress != nil {
		if statErr == nil {
			progress.BytesDone = fileInfo.Size()
			progress.BytesTotal = fileInfo.Size()
		}
		progress.Status = "complete"
		progress.Percent = 100
		progress.Error = ""
		progress.Speed = 0
	}
	if saveErr := s.saveDownloadsLocked(); saveErr != nil {
		if progress := s.downloadProgress[progressKey]; progress != nil {
			progress.Status = "failed"
			progress.Error = "The model was installed, but completion could not be saved. Check storage access and refresh installed models."
		}
	}
	s.downloadMutex.Unlock()
	log.Printf("Download completed: %s", progressKey)

}

// handleDownloadProgress returns download progress for all active downloads
func (s *Server) handleDownloadProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	s.downloadMutex.RLock()
	progress := make(map[string]*DownloadProgress)
	for k, v := range s.downloadProgress {
		if v != nil {
			copy := *v
			progress[k] = &copy
		}
	}
	s.downloadMutex.RUnlock()

	if err := json.NewEncoder(w).Encode(progress); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleCancelDownload cancels an active download
func (s *Server) handleCancelDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		FileName string `json:"file_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.downloadMutex.Lock()
	// Find and cancel matching download
	canceled := false
	for fileName, cancelFunc := range s.downloadCancelFuncs {
		// Match by exact name or if request is empty (cancel all)
		if req.FileName == "" || fileName == req.FileName {
			cancelFunc()
			// The worker owns the terminal state. Keeping it active here prevents
			// Resume from racing the file handle that is still winding down.
			if progress, ok := s.downloadProgress[fileName]; ok {
				progress.Error = "Cancellation requested; waiting for the download worker to stop."
			}
			canceled = true
			log.Printf("Cancelled download: %s", fileName)
			if req.FileName != "" {
				break // Only cancel specific file
			}
		}
	}
	s.downloadMutex.Unlock()

	response := map[string]interface{}{
		"success":   canceled,
		"cancelled": canceled,
	}
	if !canceled && req.FileName != "" {
		response["message"] = "No active download found for: " + req.FileName
	} else if canceled {
		response["message"] = "Download cancelled"
	}

	json.NewEncoder(w).Encode(response)
}

// handleVerifyModel computes and returns the SHA256 hash of a model file
func (s *Server) handleVerifyModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	modelID := r.URL.Query().Get("model")
	if modelID == "" {
		writeError(w, "model parameter required", http.StatusBadRequest)
		return
	}

	// Find model in registry - use GetModel to get the full metadata with path
	modelMeta, err := s.registry.GetModel(modelID)
	if err != nil || modelMeta == nil {
		writeError(w, "Model not found", http.StatusNotFound)
		return
	}

	modelPath := modelMeta.Path
	if modelPath == "" {
		writeError(w, "Model path not available", http.StatusNotFound)
		return
	}

	// Check if file exists
	fileInfo, err := os.Stat(modelPath)
	if os.IsNotExist(err) {
		writeError(w, "Model file not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, "Failed to inspect model file", http.StatusInternalServerError)
		return
	}

	type VerifyResponse struct {
		ModelID  string `json:"model_id"`
		FileName string `json:"file_name"`
		Size     int64  `json:"size"`
		SizeGB   string `json:"size_gb"`
		SHA256   string `json:"sha256"`
		Verified bool   `json:"verified"`
		Message  string `json:"message,omitempty"`
	}

	// Compute SHA256
	file, err := os.Open(modelPath)
	if err != nil {
		writeError(w, "Failed to open model file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	hasher := sha256.New()
	buffer := make([]byte, 1024*1024)
	for {
		if err := r.Context().Err(); err != nil {
			return
		}
		n, readErr := file.Read(buffer)
		if n > 0 {
			if _, err := hasher.Write(buffer[:n]); err != nil {
				writeError(w, "Failed to compute hash", http.StatusInternalServerError)
				return
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			writeError(w, "Failed to compute hash", http.StatusInternalServerError)
			return
		}
	}

	hash := hex.EncodeToString(hasher.Sum(nil))

	resp := VerifyResponse{
		ModelID:  modelID,
		FileName: filepath.Base(modelPath),
		Size:     fileInfo.Size(),
		SizeGB:   fmt.Sprintf("%.2f GB", float64(fileInfo.Size())/(1024*1024*1024)),
		SHA256:   hash,
		Verified: true,
		Message:  "Integrity verified",
	}

	json.NewEncoder(w).Encode(resp)
}
