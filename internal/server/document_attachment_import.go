package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

// An attachment reference is not authority. The import route checks Knowledge
// management; the scoped session reader independently checks source access.
func (s *Server) documentImportSource(r *http.Request) ([]byte, string, string, error) {
	data, name, key, _, err := s.documentImportSourceWithOCR(r)
	return data, name, key, err
}

func (s *Server) documentImportSourceWithOCR(r *http.Request) ([]byte, string, string, string, error) {
	data, name, key, language, _, err := s.documentImportSnapshot(r)
	return data, name, key, language, err
}

func (s *Server) documentImportSnapshot(r *http.Request) ([]byte, string, string, string, []*document.Extraction, error) {
	if err := r.Context().Err(); err != nil {
		return nil, "", "", "", nil, err
	}
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media != "application/json" {
		data, name, _, err := readDocumentUpload(r)
		return data, name, "", "", nil, err
	}
	invalid := &rag.ParseError{Code: "document_attachment_invalid", Message: "Choose a saved conversation file with its current source identity."}
	data, err := io.ReadAll(io.LimitReader(r.Body, 8193))
	if err != nil || len(data) > 8192 {
		return nil, "", "", "", nil, invalid
	}
	var request struct {
		Attachment struct {
			Session  string `json:"session"`
			ID       string `json:"id"`
			SHA256   string `json:"sha256"`
			Revision string `json:"revision,omitempty"`
		} `json:"attachment"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, "", "", "", nil, invalid
	}
	ref := request.Attachment
	digest, err := hex.DecodeString(ref.SHA256)
	if err != nil || len(digest) != 32 || len(ref.Session) == 0 || len(ref.Session) > 256 || len(ref.ID) != 37 || !strings.HasPrefix(ref.ID, "file-") {
		return nil, "", "", "", nil, invalid
	}
	if s.sessionHandlers == nil {
		return nil, "", "", "", nil, &rag.ParseError{Code: "document_attachment_unavailable", Message: "The saved conversation file is unavailable."}
	}
	attachment, original, history, err := s.sessionHandlers.scoped(r).AttachmentPublication(r.Context(), ref.Session, ref.ID, ref.Revision)
	if errors.Is(err, sessions.ErrTurnConflict) {
		return nil, "", "", "", nil, &rag.ParseError{Code: "document_revision_required", Message: "Update the client and select the attachment revision before publishing corrected evidence."}
	}
	if err != nil {
		return nil, "", "", "", nil, &rag.ParseError{Code: "document_attachment_unavailable", Message: "The saved conversation file is unavailable."}
	}
	if attachment.SHA256 != ref.SHA256 {
		return nil, "", "", "", nil, rag.ErrImportConflict
	}
	if err := r.Context().Err(); err != nil {
		return nil, "", "", "", nil, err
	}
	// A lost acceptance acknowledgment can be checked/repeated explicitly without
	// creating another copy. This does not retry extraction or model inference.
	identity := ref.Session + "\x00" + ref.ID + "\x00" + ref.SHA256
	if ref.Revision != "" {
		identity += "\x00" + ref.Revision
	}
	key := sha256.Sum256([]byte(identity))
	return original, attachment.Name, "attachment-" + hex.EncodeToString(key[:]), attachment.OCRLanguage, history, nil
}
