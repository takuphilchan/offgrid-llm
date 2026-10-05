package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

type modelResolutionRecord struct {
	ID         string                   `json:"id"`
	Actor      string                   `json:"-"`
	ExpiresAt  time.Time                `json:"expires_at"`
	Resolution models.PackageResolution `json:"resolution"`
	Preflight  models.PackagePreflight  `json:"preflight"`
}

func hasSpeechPackage(packages []models.PackageState) bool {
	for _, p := range packages {
		if p.Manifest != nil {
			return true
		}
	}
	return false
}

type modelOperationRequest struct {
	RequestID         string `json:"request_id"`
	Action            string `json:"action"`
	ResolutionID      string `json:"resolution_id,omitempty"`
	SourceOperationID string `json:"source_operation_id,omitempty"`
}

func modelActor(r *http.Request) string {
	if u := users.GetUser(r); u != nil {
		return u.ID
	}
	return "local"
}
func (s *Server) canReadModelOperation(r *http.Request, o *ModelOperation) bool {
	if o == nil {
		return false
	}
	if o.ActorID == modelActor(r) {
		return true
	}
	u := users.GetUser(r)
	return u != nil && u.HasPermission(users.PermissionAdmin)
}
func (s *Server) hubForModels() *models.HuggingFaceClient {
	s.downloadMutex.Lock()
	defer s.downloadMutex.Unlock()
	if s.modelHub == nil {
		s.modelHub = newModelHub()
	}
	return s.modelHub
}
func decodeModelRequest(w http.ResponseWriter, r *http.Request, out any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil || d.Decode(new(any)) != io.EOF {
		writeErrorWithCode(w, "Provide a valid model request.", 400, "invalid_model_request")
		return false
	}
	return true
}
func modelHTTPError(w http.ResponseWriter, err error) {
	var coded *models.ModelAcquisitionError
	if errors.As(err, &coded) {
		status := 422
		if coded.Code == "insufficient_space" {
			status = 507
		}
		if coded.Code == "source_unavailable" {
			status = 502
		}
		writeErrorWithCode(w, coded.Message, status, coded.Code)
		return
	}
	switch {
	case errors.Is(err, models.ErrPackageInUse):
		writeErrorWithCode(w, "This package has active work. Wait for it to settle before changing it.", 409, "package_in_use")
	case errors.Is(err, models.ErrPackageConflict):
		writeErrorWithCode(w, "This exact package is already installed. Verify or repair it instead.", 409, "package_conflict")
	case errors.Is(err, models.ErrPackageNotFound):
		writeErrorWithCode(w, "This package is not installed.", 404, "package_not_found")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeErrorWithCode(w, "The model request was interrupted. No transfer was started.", 408, "model_request_interrupted")
	default:
		writeErrorWithCode(w, "Model storage could not complete this request. Existing data is preserved; check disk space and permissions.", 503, "model_storage_unavailable")
	}
}

func (s *Server) handleModelsV2(w http.ResponseWriter, r *http.Request) {
	permission := users.PermissionModels
	if r.Method != http.MethodGet {
		permission = users.PermissionModelsManage
	}
	s.requirePermissionWhenAuthEnabled(permission, s.serveModelsV2)(w, r)
}

func (s *Server) serveModelsV2(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Request-ID", uuid.NewString())
	if s.registry == nil {
		writeErrorWithCode(w, "Model storage is unavailable.", 503, "model_storage_unavailable")
		return
	}
	category := models.ModelCategory(r.URL.Query().Get("category"))
	if category != "" && !category.Valid() {
		writeErrorWithCode(w, "Choose a supported model category.", 400, "invalid_category")
		return
	}
	switch r.URL.Path {
	case "/api/v2/models":
		if r.Method != http.MethodGet {
			break
		}
		packages, err := s.registry.Packages().List()
		if err != nil {
			modelHTTPError(w, err)
			return
		}
		if hasSpeechPackage(packages) {
			speech, err := s.getSpeechRuntime()
			if err != nil {
				writeErrorWithCode(w, "Speech inventory is unavailable.", 503, "speech_inventory_error")
				return
			}
			profiles, err := speech.Profiles(r.Context())
			if err != nil {
				modelHTTPError(w, err)
				return
			}
			for i := range packages {
				for _, profile := range profiles {
					if packages[i].ID == profile.ID && packages[i].Revision == profile.Revision {
						packages[i].RuntimeCompatible = profile.Available
						packages[i].SmokeTested = profile.SmokeTested
						packages[i].Issue = profile.Issue
					}
				}
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"models": models.Inventory(s.registry.ListModels(), packages, category)})
		return
	case "/api/v2/models/catalog":
		if r.Method != http.MethodGet {
			break
		}
		if r.URL.Query().Get("source") == "huggingface" {
			query := strings.TrimSpace(r.URL.Query().Get("q"))
			if query == "" {
				writeErrorWithCode(w, "Enter a Hugging Face search query.", 400, "query_required")
				return
			}
			// An explicit owner/repository is deterministic and does not need the
			// Hub search index. This also keeps model setup usable behind slow or
			// filtered VPNs; discovery performs the reviewed-layout check next.
			if strings.Count(query, "/") == 1 && !strings.ContainsAny(query, "?&#\\") {
				json.NewEncoder(w).Encode(map[string]any{"models": []models.TypedCatalogEntry{}, "repositories": []map[string]any{{"id": query, "size_bytes": 0}}})
				return
			}
			searchContext, cancelSearch := context.WithTimeout(r.Context(), 5*time.Second)
			items, err := s.hubForModels().SearchModelsContext(searchContext, models.SearchFilter{Query: query, Category: category, OnlyGGUF: category == models.CategoryLanguage || category == models.CategoryEmbeddings, MetadataOnly: true, ExcludeGated: true, ExcludePrivate: true, Limit: 20})
			cancelSearch()
			if err != nil {
				writeErrorWithCode(w, "Hugging Face search is unavailable. Check the service's network or proxy settings.", 502, "source_unavailable")
				return
			}
			repos := []map[string]any{}
			for _, item := range items {
				repos = append(repos, map[string]any{"id": item.Model.ID, "downloads": item.Model.Downloads, "likes": item.Model.Likes, "size_bytes": item.TotalSize})
			}
			json.NewEncoder(w).Encode(map[string]any{"models": []models.TypedCatalogEntry{}, "repositories": repos})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"models": models.DefaultCatalog().TypedEntries(category), "repositories": []any{}})
		return
	case "/api/v2/models/discover":
		if r.Method != http.MethodGet {
			break
		}
		result, err := s.hubForModels().DiscoverPackages(r.Context(), r.URL.Query().Get("repository"))
		if err != nil {
			modelHTTPError(w, err)
			return
		}
		json.NewEncoder(w).Encode(result)
		return
	case "/api/v2/models/resolve":
		if r.Method != http.MethodPost {
			break
		}
		s.resolveModelPackage(w, r)
		return
	case "/api/v2/models/operations":
		switch r.Method {
		case http.MethodGet:
			s.downloadMutex.RLock()
			operations := []*ModelOperation{}
			for _, o := range s.modelOperations {
				if s.canReadModelOperation(r, o) {
					operations = append(operations, cloneModelOperation(o))
				}
			}
			s.downloadMutex.RUnlock()
			sort.Slice(operations, func(i, j int) bool { return operations[i].CreatedAt.After(operations[j].CreatedAt) })
			json.NewEncoder(w).Encode(map[string]any{"operations": operations})
			return
		case http.MethodPost:
			s.submitModelOperation(w, r)
			return
		}
	default:
		prefix := "/api/v2/models/operations/"
		if strings.HasPrefix(r.URL.Path, prefix) {
			s.manageModelOperation(w, r, strings.Split(strings.TrimPrefix(r.URL.Path, prefix), "/"))
			return
		}
		writeErrorWithCode(w, "Model resource not found.", 404, "model_resource_not_found")
		return
	}
	writeErrorWithCode(w, "This method is not available for this model resource.", 405, "method_not_allowed")
}

func (s *Server) resolveModelPackage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CatalogID    string `json:"catalog_id,omitempty"`
		Repository   string `json:"repository,omitempty"`
		Revision     string `json:"revision,omitempty"`
		Variant      string `json:"variant,omitempty"`
		Architecture string `json:"architecture,omitempty"`
	}
	if !decodeModelRequest(w, r, &req) {
		return
	}
	var result models.PackageResolution
	if req.CatalogID != "" {
		if req.Repository != "" || req.Revision != "" || req.Variant != "" || req.Architecture != "" {
			writeErrorWithCode(w, "Select a catalog entry or an explicit repository variant, not both.", 400, "invalid_model_request")
			return
		}
		items, err := models.CuratedPackageResolutions()
		if err != nil {
			modelHTTPError(w, err)
			return
		}
		for _, item := range items {
			if item.Manifest.ID == req.CatalogID {
				result = item
				break
			}
		}
		if result.Manifest.ID == "" {
			writeErrorWithCode(w, "This catalog entry is unavailable.", 404, "model_not_found")
			return
		}
	} else {
		var err error
		result, err = s.hubForModels().ResolvePackage(r.Context(), models.PackageResolutionRequest{Repository: req.Repository, Revision: req.Revision, Variant: req.Variant, Architecture: req.Architecture})
		if err != nil {
			modelHTTPError(w, err)
			return
		}
	}
	preflight, err := s.registry.Packages().Preflight(result.Manifest, 0)
	if err != nil {
		modelHTTPError(w, err)
		return
	}
	if speech, err := s.getSpeechRuntime(); err == nil {
		preflight.RuntimeAvailable = speech.AdapterAvailable(r.Context(), result.Manifest.Runtime.Adapter)
		if preflight.RuntimeAvailable {
			preflight.Readiness.Issue = "model_not_validated"
			preflight.Warnings[0] = "The matching runtime is installed. This model variant still needs compatibility and inference checks; installation does not establish speech quality or interactive latency."
		} else {
			preflight.Readiness.Issue = "runtime_unavailable"
			preflight.Warnings[0] = "This service has no available " + result.Manifest.Runtime.Adapter + " adapter. You can store this package, but it cannot run here yet."
		}
	}
	record := modelResolutionRecord{ID: uuid.NewString(), Actor: modelActor(r), ExpiresAt: time.Now().UTC().Add(10 * time.Minute), Resolution: result, Preflight: preflight}
	s.downloadMutex.Lock()
	if s.modelResolutions == nil {
		s.modelResolutions = map[string]modelResolutionRecord{}
	}
	for id, item := range s.modelResolutions {
		if time.Now().After(item.ExpiresAt) {
			delete(s.modelResolutions, id)
		}
	}
	if len(s.modelResolutions) >= 64 {
		s.downloadMutex.Unlock()
		writeErrorWithCode(w, "Too many pending previews. Wait for older previews to expire.", 429, "model_preview_limit")
		return
	}
	s.modelResolutions[record.ID] = record
	s.downloadMutex.Unlock()
	json.NewEncoder(w).Encode(record)
}
