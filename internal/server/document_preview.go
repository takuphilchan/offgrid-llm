package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

// Shared, read-only presentation of retained sources. No upload, index or model
// invocation occurs here. The caller rechecks current authority after parsing.
func serveDocumentPreview(w http.ResponseWriter, r *http.Request, parser *rag.DocumentParser, name string, data []byte, allowed func() bool, retained ...retainedImageText) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	digest := sha256.Sum256(data)
	sha := hex.EncodeToString(digest[:])
	if expected := r.URL.Query().Get("sha256"); expected != "" && expected != sha {
		writeErrorWithCode(w, "The source changed. Reopen its preview.", 409, "document_citation_stale")
		return
	}
	ext := strings.ToLower(filepath.Ext(name))
	thumbnail := r.URL.Query().Get("thumbnail") == "true"
	if values, present := r.URL.Query()["thumbnail"]; present && (len(values) != 1 || values[0] != "true" && values[0] != "false" || !r.URL.Query().Has("page")) {
		writeError(w, "Invalid document preview", 400)
		return
	}
	if r.URL.Query().Has("page") {
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		tiff := ext == ".tif" || ext == ".tiff"
		image := ext == ".png" || ext == ".jpg" || ext == ".jpeg" || tiff
		if err != nil || page < 1 || page > 1000 || ext != ".pdf" && !image || image && !tiff && page != 1 {
			writeError(w, "Invalid document page", 400)
			return
		}
		var pixels []byte
		if thumbnail {
			pixels, err = parser.RenderPreviewThumbnail(r.Context(), data, ext, page)
		} else if image {
			pixels, err = parser.RenderImagePage(r.Context(), data, ext, page)
		} else {
			pixels, err = parser.RenderPreviewPage(r.Context(), data, page)
		}
		if err != nil {
			writeDocumentIngestError(w, err)
			return
		}
		if !allowed() {
			writeError(w, "Source unavailable", 404)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("X-OffGrid-Source-SHA256", sha)
		if thumbnail {
			w.Header().Set("X-OffGrid-Preview", "thumbnail-v1")
		}
		_, _ = w.Write(pixels)
		return
	}
	result, err := parser.PreviewContext(r.Context(), data, name, ext)
	if applyRetainedImageText(result, sha, retained) && result.State == rag.ExtractionComplete {
		err = nil
	}
	code := ""
	if err != nil {
		var problem *rag.ParseError
		if !errors.As(err, &problem) || result == nil || problem.Code != "document_ocr_required" {
			writeDocumentIngestError(w, err)
			return
		}
		code = problem.Code
	}
	if result == nil {
		writeError(w, "Source unavailable", 422)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	type previewUnit struct {
		rag.SourceUnit
		ImageGeometry *rag.ImageGeometry `json:"image_geometry,omitempty"`
	}
	units := make([]previewUnit, len(result.SourceUnits))
	geometry := rag.PreviewImageGeometry(r.Context(), data, ext)
	for i, unit := range result.SourceUnits {
		units[i].SourceUnit = unit
		if unit.Kind == "page" && unit.Index > 0 && unit.Index <= len(geometry) {
			units[i].ImageGeometry = &geometry[unit.Index-1]
		}
	}
	if !allowed() {
		writeError(w, "Source unavailable", 404)
		return
	}
	thumbnails := ext == ".pdf" || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".tif" || ext == ".tiff"
	_ = json.NewEncoder(w).Encode(map[string]any{"name": name, "sha256": sha, "format": strings.TrimPrefix(ext, "."), "pages": result.PageCount, "units": units, "state": result.State, "error_code": code, "evidence": result.Coverage(), "thumbnails_available": thumbnails})
}
