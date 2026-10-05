package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

const modelPackagePath = "/api/v2/models/packages"
const maxModelPackageRequestBytes = models.MaxPackageBytes + (8 << 20)

// Mutation permissions are applied before consuming multipart input. This route
// extends model management; it does not install or execute speech runtimes.
func (s *Server) handleModelPackages(w http.ResponseWriter, r *http.Request) {
	permission := users.PermissionModelsManage
	if r.Method == http.MethodGet {
		permission = users.PermissionModels
	}
	s.requirePermissionWhenAuthEnabled(permission, s.serveModelPackages)(w, r)
}

func (s *Server) serveModelPackages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if s.registry == nil {
		writeErrorWithCode(w, "Model storage is unavailable.", 503, "model_storage_unavailable")
		return
	}
	store := s.registry.Packages()
	if r.URL.Path == modelPackagePath {
		switch r.Method {
		case http.MethodGet:
			items, err := store.List()
			if err != nil {
				writeErrorWithCode(w, "Model package storage needs attention. Existing files have been preserved.", 503, "model_storage_unavailable")
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"packages": items})
		case http.MethodPost:
			s.importModelPackage(w, r)
		default:
			writeErrorWithCode(w, "Use GET to inspect packages or POST to import one.", 405, "method_not_allowed")
		}
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, modelPackagePath+"/"), "/")
	if len(parts) != 3 {
		writeErrorWithCode(w, "Specify a package ID, revision and operation.", 404, "package_not_found")
		return
	}
	id, revision, operation := parts[0], parts[1], parts[2]
	if operation == "verify" && r.Method == http.MethodPost {
		state, err := store.Verify(r.Context(), id, revision)
		if err != nil {
			if errors.Is(err, models.ErrPackageNotFound) {
				writeErrorWithCode(w, "This package is not installed.", 404, "package_not_found")
				return
			}
			writeErrorWithCode(w, "Package verification failed. Re-import a complete compatible revision; existing files are preserved.", 422, "package_integrity_failed")
			return
		}
		json.NewEncoder(w).Encode(state)
		return
	}
	if operation == "remove" && r.Method == http.MethodPost {
		// Queued/resumable work is part of the same authority as deletion. A
		// worker must not recreate a package immediately after confirmed removal.
		s.downloadMutex.Lock()
		defer s.downloadMutex.Unlock()
		for _, work := range s.modelOperations {
			if work.Target.Package == nil || work.Target.Package.ID != id || work.Target.Package.Revision != revision {
				continue
			}
			if s.downloadCancelFuncs["package:"+work.ID] != nil || work.active() || (!work.Discarded && work.State != "complete") {
				writeErrorWithCode(w, "Cancel and discard unfinished model operations before removing this package.", 409, "package_in_use")
				return
			}
			if err := store.SettleAcquisition(*work.Target.Package, work.ID, true); err != nil {
				modelHTTPError(w, err)
				return
			}
		}
		if err := store.Remove(id, revision); err != nil {
			switch {
			case errors.Is(err, models.ErrPackageInUse):
				writeErrorWithCode(w, "This package is being used. Stop its active work before removing it.", 409, "package_in_use")
			case errors.Is(err, models.ErrPackageNotFound):
				writeErrorWithCode(w, "This package is not installed.", 404, "package_not_found")
			default:
				writeErrorWithCode(w, "Package removal failed. No other package was selected for removal.", 400, "package_removal_failed")
			}
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"removed": true})
		return
	}
	writeErrorWithCode(w, "Use POST with verify or remove.", 405, "method_not_allowed")
}

// Multipart bodies stream directly to the shared package staging transaction.
// The first part is manifest; subsequent form names are exact declared paths in
// manifest order. Reject extra parts BEFORE activation with a completion check.
type packagePartReader struct{ part *multipart.Part }

func (p *packagePartReader) Read(b []byte) (int, error) { return p.part.Read(b) }
func (p *packagePartReader) Close() error {
	// The store has read at most declared-size+1 bytes. Do not drain arbitrarily
	// large malformed parts with multipart.Part.Close; abort the request instead.
	return nil
}

func (s *Server) importModelPackage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxModelPackageRequestBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeErrorWithCode(w, "Select a manifest and its declared model files.", 400, "invalid_package_upload")
		return
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "manifest" {
		writeErrorWithCode(w, "The first upload part must be the manifest.", 400, "invalid_package_upload")
		return
	}
	manifest, err := models.DecodePackageManifest(part)
	if err != nil {
		writeErrorWithCode(w, err.Error(), 422, "invalid_package_manifest")
		return
	}
	state, err := s.registry.Packages().Import(r.Context(), manifest, func(ctx context.Context, artifact models.PackageArtifact) (io.ReadCloser, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		part, err := reader.NextPart()
		if err != nil {
			return nil, err
		}
		if part.FormName() != artifact.Path {
			return nil, errors.New("missing or out-of-order artifact")
		}
		return &packagePartReader{part}, nil
	}, func() error {
		next, err := reader.NextPart()
		if err != io.EOF || next != nil {
			return errors.New("undeclared multipart artifact")
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, models.ErrPackageConflict) {
			writeErrorWithCode(w, "This revision already exists. Verify it or import a different revision.", 409, "package_conflict")
			return
		}
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			writeErrorWithCode(w, "The package upload exceeds the size limit.", 413, "package_too_large")
			return
		}
		writeErrorWithCode(w, "Import failed. Check all declared files, sizes, digests, available disk space and the connection. No incomplete package was activated.", 422, "package_import_failed")
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(state)
}
