package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

// A retained revision is the source of extracted text. Original pixels remain
// original pixels; neither reopening a citation nor changing runtimes reparses it.
func serveRetainedAttachmentPreview(w http.ResponseWriter, r *http.Request, a sessions.Attachment, data []byte, graph *document.Extraction, allowed func() bool) {
	if sha := r.URL.Query().Get("sha256"); sha != "" && sha != a.SHA256 {
		writeErrorWithCode(w, "The source changed. Reopen its preview.", 409, "document_citation_stale")
		return
	}
	ext := strings.ToLower(filepath.Ext(a.Name))
	geometry := rag.PreviewImageGeometry(r.Context(), data, ext)
	units := make([]map[string]any, 0, len(graph.Units))
	pages := 0
	for _, unit := range graph.Units {
		v := map[string]any{"kind": unit.Kind, "index": unit.Index, "name": unit.Name, "text": graph.Text[unit.Span.Start:unit.Span.End], "state": unit.State, "source_sha256": a.SHA256, "extractor_revision": graph.ID}
		if unit.Kind == "page" {
			pages++
			if unit.Index > 0 && unit.Index <= len(geometry) {
				v["image_geometry"] = geometry[unit.Index-1]
			}
		}
		units = append(units, v)
	}
	if !allowed() {
		writeError(w, "Source unavailable", 404)
		return
	}
	state := "complete"
	if graph.Coverage != nil && graph.Coverage.Status == "partial" {
		state = "partial"
	}
	thumbnails := ext == ".pdf" || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".tif" || ext == ".tiff"
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(map[string]any{"name": a.Name, "sha256": a.SHA256, "format": strings.TrimPrefix(ext, "."), "pages": pages, "units": units, "state": state, "error_code": "", "evidence": graph.Coverage, "thumbnails_available": thumbnails, "revision": a.Revision, "extraction_revision": graph.ID})
}
