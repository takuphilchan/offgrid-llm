package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func (s *Server) acceptDocumentImport(r *http.Request) (*rag.ImportRecord, error) {
	data, name, sourceKey, err := s.documentImportSource(r)
	if err != nil {
		return nil, err
	}
	store, err := s.ragEngine.ImportStore()
	if err != nil {
		return nil, err
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		key = sourceKey
	}
	if key == "" {
		var random [16]byte
		if _, err = rand.Read(random[:]); err != nil {
			return nil, err
		}
		key = hex.EncodeToString(random[:])
	}
	return store.AcceptImport(modelActor(r), key, name, data)
}

// Progress and pending originals are owner-scoped. The normal Knowledge manager
// permission is also checked at the router on every read/retry/cancel operation.
func (s *Server) handleDocumentImports(w http.ResponseWriter, r *http.Request) {
	if !s.requireKnowledgeStorage(w) {
		return
	}
	store, err := s.ragEngine.ImportStore()
	if err != nil {
		writeDocumentIngestError(w, err)
		return
	}
	owner, id := modelActor(r), r.URL.Query().Get("id")
	respond := func(status int, value any) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
	var value any
	switch r.Method {
	case http.MethodGet:
		if r.URL.Query().Get("view") == "extraction" && id != "" {
			value, err = store.ImportExtraction(owner, id)
		} else if r.URL.Query().Has("view") {
			writeError(w, "Choose an import to review its retained extraction", 400)
			return
		} else if id != "" {
			value, err = store.Import(owner, id)
		} else {
			var items []*rag.ImportRecord
			items, err = store.Imports(owner)
			value = map[string]any{"imports": items}
		}
	case http.MethodPost:
		if id == "" {
			value, err = s.acceptDocumentImport(r)
			if err == nil {
				respond(http.StatusAccepted, value)
				return
			}
		} else {
			switch r.URL.Query().Get("action") {
			case "retry":
				_, err = s.ragEngine.RunImport(r.Context(), owner, id)
				if err == nil {
					value, err = store.Import(owner, id)
				}
			case "cancel":
				err = s.ragEngine.StopImport(owner, id, false)
				if err == nil {
					value, err = store.Import(owner, id)
				}
			default:
				writeError(w, "Choose retry or cancel for this saved import", 400)
				return
			}
		}
	case http.MethodDelete:
		if id == "" {
			writeError(w, "Import identity is required", 400)
			return
		}
		err = s.ragEngine.StopImport(owner, id, true)
		value = map[string]bool{"success": err == nil}
	default:
		writeError(w, "Method not allowed", 405)
		return
	}
	if err != nil {
		if errors.Is(err, rag.ErrDocumentHeld) || errors.Is(err, rag.ErrRetentionUnavailable) {
			writeRetentionError(w, err)
			return
		}
		if errors.Is(err, rag.ErrImportNotFound) {
			writeErrorWithCode(w, "Import not found", 404, "document_import_not_found")
			return
		}
		if errors.Is(err, rag.ErrImportConflict) {
			writeErrorWithCode(w, "Import state changed. Refresh its status before continuing.", 409, "document_import_conflict")
			return
		}
		writeDocumentIngestError(w, err)
		return
	}
	respond(200, value)
}
