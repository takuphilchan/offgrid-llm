package server

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

const modelGGUFImportPath = "/api/v2/models/import/gguf"

func (s *Server) handleGGUFImport(w http.ResponseWriter, r *http.Request) {
	s.requirePermissionWhenAuthEnabled(users.PermissionModelsManage, s.importGGUF)(w, r)
}

func (s *Server) importGGUF(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		writeErrorWithCode(w, "Use POST to import a GGUF file.", 405, "method_not_allowed")
		return
	}
	if s.registry == nil {
		writeErrorWithCode(w, "Model storage is unavailable.", 503, "model_storage_unavailable")
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	size, sizeErr := strconv.ParseInt(r.URL.Query().Get("size"), 10, 64)
	if err != nil || media != "application/octet-stream" || sizeErr != nil || size < 1<<20 || size > models.MaxPackageBytes {
		writeErrorWithCode(w, "Choose one complete GGUF file between 1 MiB and 32 GiB.", 422, "gguf_import_invalid")
		return
	}
	// Large offline files need a bounded transfer window independent of the
	// ordinary request deadline. Cancellation remains request-scoped.
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(30 * time.Minute))
	_ = controller.SetWriteDeadline(time.Now().Add(30 * time.Minute))
	receipt, err := s.registry.ImportGGUF(r.Context(), r.URL.Query().Get("filename"), size, r.Body)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrGGUFImportBusy):
			writeErrorWithCode(w, "Another model import is active. Wait for it to finish; this file was not imported.", 503, "gguf_import_busy")
		case errors.Is(err, models.ErrGGUFImportExists):
			writeErrorWithCode(w, "A model with this filename already exists. Inspect installed models; no file was replaced.", 409, "gguf_import_exists")
		case errors.Is(err, models.ErrGGUFImportInvalid):
			writeErrorWithCode(w, "Choose a complete single-file GGUF model, not a projector or split shard. The upload was not installed.", 422, "gguf_import_invalid")
		default:
			writeErrorWithCode(w, "The model could not be stored. Check storage and retry; existing models were preserved.", 503, "gguf_import_storage")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(receipt)
}
