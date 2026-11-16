package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func (s *Server) acceptDocumentImport(r *http.Request) (*rag.ImportRecord, error) {
	language, err := documentOCRLanguage(r)
	if err != nil {
		return nil, err
	}
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media == "application/json" && (!r.URL.Query().Has("collection_id") || !r.URL.Query().Has("collection_revision")) {
		return nil, &rag.ParseError{Code: "collection_review_required", Message: "Upgrade the client and review a destination collection and its current audience before publishing a conversation attachment."}
	}
	data, name, sourceKey, sourceLanguage, history, err := s.documentImportSnapshot(r)
	if err != nil {
		return nil, err
	}
	if sourceLanguage != "" {
		if r.URL.Query().Has("ocr_language") && language != sourceLanguage {
			return nil, rag.ErrImportConflict
		}
		language = sourceLanguage
	}
	store, err := s.ragEngine.ImportStore()
	if err != nil {
		return nil, err
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		key = sourceKey
		if key != "" && r.URL.Query().Has("collection_id") {
			digest := sha256.Sum256([]byte(key + "\x00" + r.URL.Query().Get("collection_id") + "\x00" + r.URL.Query().Get("collection_revision")))
			key = "collection-" + hex.EncodeToString(digest[:])
		}
	}
	if key == "" {
		var random [16]byte
		if _, err = rand.Read(random[:]); err != nil {
			return nil, err
		}
		key = hex.EncodeToString(random[:])
	}
	q := r.URL.Query()
	if q.Has("collection_id") || q.Has("collection_revision") {
		revision, err := strconv.Atoi(q.Get("collection_revision"))
		if err != nil || revision < 1 || q.Get("collection_id") == "" {
			return nil, rag.ErrCollectionInvalid
		}
		return store.AcceptImportWithRevisions(modelActor(r), key, name, data, q.Get("collection_id"), language, history, revision)
	}
	return store.AcceptImportWithRevisions(modelActor(r), key, name, data, rag.WorkspaceCollection, language, history)
}

// Progress and pending originals are owner-scoped. The normal Knowledge manager
// permission is also checked at the router on every read/retry/cancel operation.
func (s *Server) handleDocumentImports(w http.ResponseWriter, r *http.Request) {
	if !s.requireKnowledgeStorage(w) {
		return
	}
	store, err := s.ragEngine.ImportStore()
	if err != nil {
		if errors.Is(err, rag.ErrCollectionInvalid) || errors.Is(err, rag.ErrCollectionConflict) || errors.Is(err, rag.ErrCollectionUnavailable) {
			writeCollectionError(w, err)
			return
		}
		writeDocumentIngestError(w, err)
		return
	}
	owner, id := modelActor(r), r.URL.Query().Get("id")
	if r.Method == http.MethodPost && id != "" && r.URL.Query().Has("ocr_language") {
		language, selectionErr := documentOCRLanguage(r)
		current, readErr := store.Import(owner, id)
		if selectionErr != nil || readErr != nil || current.OCRLanguage != language {
			writeErrorWithCode(w, "The OCR selection cannot change on a saved import. Submit a new import to choose another language.", 409, "document_ocr_language_invalid")
			return
		}
	}
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
