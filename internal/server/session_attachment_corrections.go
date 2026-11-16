package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func (h *SessionHandlers) sessionAttachmentCorrection(w http.ResponseWriter, r *http.Request, name, id string) {
	var body struct {
		ExpectedRevision   string                `json:"expected_revision"`
		ExtractionRevision string                `json:"extraction_revision"`
		Edits              []document.Correction `json:"edits"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF || len(body.ExpectedRevision) != 64 || len(body.ExtractionRevision) > 128 || body.ExtractionRevision == "" || len(body.Edits) < 1 || len(body.Edits) > 256 {
		writeErrorWithCode(w, "Provide the current source and complete text blocks to correct.", 400, "document_correction_invalid")
		return
	}
	result, err := h.scoped(r).CorrectAttachment(r.Context(), name, id, body.ExpectedRevision, body.ExtractionRevision, body.Edits)
	if errors.Is(err, sessions.ErrAttachmentCorrectionInvalid) {
		writeErrorWithCode(w, "Provide the current source and complete text blocks to correct.", 400, "document_correction_invalid")
		return
	}
	if err != nil {
		writeSessionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"attachment": result})
}
