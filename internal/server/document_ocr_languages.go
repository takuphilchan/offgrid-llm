package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

// The caller's Knowledge-management permission is checked by the route before
// reading a pack. The body is data, not a URL, path or executable provider.
func (s *Server) handleOCRLanguages(w http.ResponseWriter, r *http.Request) {
	if !s.requireKnowledgeStorage(w) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		if r.Header.Get("Content-Type") != "application/octet-stream" {
			writeErrorWithCode(w, "Send the OCR language file as application/octet-stream.", 415, "document_ocr_language_invalid")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, rag.MaxOCRLanguageBytes)
		if err := s.ragEngine.ImportOCRLanguage(r.Context(), r.URL.Query().Get("language"), r.Body); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeErrorWithCode(w, "OCR language packs must be 32 MiB or smaller.", 413, "document_limit")
				return
			}
			writeDocumentIngestError(w, err)
			return
		}
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, "Method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"languages": s.ragEngine.OCRLanguages(), "document_formats": s.ragEngine.DocumentParser().FormatReadiness()})
}
