package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func (s *Server) handleDiscardDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		FileName string `json:"file_name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || !strings.HasSuffix(req.FileName, ".gguf") || !isSafeModelID(strings.TrimSuffix(req.FileName, ".gguf")) {
		writeError(w, "An exact download filename is required", http.StatusBadRequest)
		return
	}
	if decoder.Decode(new(any)) != io.EOF {
		writeError(w, "One download request is required", http.StatusBadRequest)
		return
	}
	s.downloadMutex.Lock()
	defer s.downloadMutex.Unlock()
	item := s.downloadProgress[req.FileName]
	if _, active := s.downloadCancelFuncs[req.FileName]; active || item != nil && item.Status != "failed" && item.Status != "cancelled" && item.Status != "interrupted" {
		writeError(w, "Cancel and wait for the download to stop before discarding partial data", http.StatusConflict)
		return
	}
	if item != nil {
		if err := models.DiscardGGUFPartial(s.config.ModelsDir, req.FileName, item.Bundle); err != nil {
			writeError(w, "Could not remove partial download data. Check storage access and retry.", http.StatusInternalServerError)
			return
		}
		delete(s.downloadProgress, req.FileName)
		if err := s.saveDownloadsLocked(); err != nil {
			// A retry is safe after deletion: missing partials are accepted. Keep
			// the entry visible until its removal can be durably acknowledged.
			s.downloadProgress[req.FileName] = item
			writeError(w, "Partial data was removed, but download history could not be saved. Retry removal.", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
