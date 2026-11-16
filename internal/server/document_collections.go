package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func writeCollectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, rag.ErrCollectionUnavailable):
		writeErrorWithCode(w, "Collection or document not found", 404, "collection_not_found")
	case errors.Is(err, rag.ErrCollectionConflict):
		writeErrorWithCode(w, "The collection or its audience changed. Review the current policies before trying again.", 409, "collection_conflict")
	case errors.Is(err, rag.ErrCollectionInvalid):
		writeErrorWithCode(w, "Provide a valid name, explicit membership and current policy revisions.", 400, "collection_invalid")
	default:
		writeErrorWithCode(w, "Collection operation was not confirmed. Inspect current state before retrying.", 503, "collection_unavailable")
	}
}

func decodeCollectionRequest(w http.ResponseWriter, r *http.Request, value any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 96<<10))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil || d.Decode(new(any)) != io.EOF {
		writeCollectionError(w, rag.ErrCollectionInvalid)
		return false
	}
	return true
}

// Route guards apply service-level read/manage rights. Store policies are an
// additional authority boundary; global administration never supplies an owner.
func (s *Server) handleCollections(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.requireKnowledgeStorage(w) {
		return
	}
	store, err := s.ragEngine.ImportStore()
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	actor, id := modelActor(r), r.URL.Query().Get("id")
	var value any
	status := 200
	switch r.Method {
	case http.MethodGet:
		if id == "" {
			var policies []*rag.CollectionPolicy
			policies, err = store.Collections(actor)
			value = map[string]any{"collections": policies}
		} else if r.URL.Query().Get("view") == "audit" {
			after := int64(0)
			if r.URL.Query().Has("after") {
				after, err = strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
			}
			if err != nil || after < 0 {
				writeCollectionError(w, rag.ErrCollectionInvalid)
				return
			}
			var events []rag.CollectionAudit
			events, err = store.CollectionAudit(actor, id, after)
			value = map[string]any{"events": events}
		} else if r.URL.Query().Has("view") {
			writeCollectionError(w, rag.ErrCollectionInvalid)
			return
		} else {
			value, err = store.Collection(actor, id)
		}
	case http.MethodPost:
		if id != "" {
			writeCollectionError(w, rag.ErrCollectionInvalid)
			return
		}
		var request struct {
			Name string `json:"name"`
		}
		if !decodeCollectionRequest(w, r, &request) {
			return
		}
		// Creation is private. Sharing is a separate owner/revision-checked update.
		value, err = store.CreateCollection(actor, request.Name, nil, nil)
		status = 201
	case http.MethodPut:
		var request rag.CollectionUpdate
		if !decodeCollectionRequest(w, r, &request) {
			return
		}
		if id == "" || request.Readers == nil || request.Editors == nil {
			writeCollectionError(w, rag.ErrCollectionInvalid)
			return
		}
		value, err = store.UpdateCollection(actor, id, request.ExpectedRevision, request.Name, request.Readers, request.Editors)
	default:
		w.Header().Set("Allow", "GET, POST, PUT")
		writeError(w, "Method not allowed", 405)
		return
	}
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) handleDocumentCollection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.requireKnowledgeStorage(w) {
		return
	}
	store, err := s.ragEngine.ImportStore()
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	var value *rag.DocumentCollection
	switch r.Method {
	case http.MethodGet:
		value, err = store.DocumentCollection(modelActor(r), r.URL.Query().Get("id"))
	case http.MethodPut:
		var request rag.CollectionMove
		if !decodeCollectionRequest(w, r, &request) {
			return
		}
		value, err = s.ragEngine.ForActor(modelActor(r)).MoveDocument(r.Context(), request)
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeError(w, "Method not allowed", 405)
		return
	}
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

// A distinct route prevents an older service from ignoring destination query
// fields and importing private content into its legacy shared workspace.
func (s *Server) handleCollectionImport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, "Method not allowed", 405)
		return
	}
	q := r.URL.Query()
	revision, err := strconv.Atoi(q.Get("collection_revision"))
	if err != nil || revision < 1 || q.Get("collection_id") == "" || q.Has("id") || q.Has("action") {
		writeCollectionError(w, rag.ErrCollectionInvalid)
		return
	}
	s.handleDocumentImports(w, r)
}
