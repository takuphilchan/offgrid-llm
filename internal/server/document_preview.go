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
func serveDocumentPreview(w http.ResponseWriter, r *http.Request, parser *rag.DocumentParser, name string, data []byte, allowed func() bool) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	digest := sha256.Sum256(data)
	sha := hex.EncodeToString(digest[:])
	if expected := r.URL.Query().Get("sha256"); expected != "" && expected != sha {
		writeErrorWithCode(w, "The source changed. Reopen its preview.", 409, "document_citation_stale")
		return
	}
	ext := strings.ToLower(filepath.Ext(name))
	if r.URL.Query().Has("page") {
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil || page < 1 || page > 1000 || ext != ".pdf" {
			writeError(w, "Invalid PDF page", 400)
			return
		}
		pixels, err := parser.RenderPreviewPage(r.Context(), data, page)
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
		_, _ = w.Write(pixels)
		return
	}
	result, err := parser.PreviewContext(r.Context(), data, name, ext)
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
	if !allowed() {
		writeError(w, "Source unavailable", 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"name": name, "sha256": sha, "format": strings.TrimPrefix(ext, "."), "pages": result.PageCount, "units": result.SourceUnits, "state": result.State, "error_code": code})
}
