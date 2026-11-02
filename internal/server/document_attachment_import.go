package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

// An attachment reference is not authority. The import route checks Knowledge
// management; the scoped session reader independently checks source access.
func (s *Server) documentImportSource(r *http.Request) ([]byte, string, string, error) {
	if err := r.Context().Err(); err != nil {
		return nil, "", "", err
	}
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media != "application/json" {
		data, name, _, err := readDocumentUpload(r)
		return data, name, "", err
	}
	invalid := &rag.ParseError{Code: "document_attachment_invalid", Message: "Choose a saved conversation file with its current source identity."}
	data, err := io.ReadAll(io.LimitReader(r.Body, 8193))
	if err != nil || len(data) > 8192 {
		return nil, "", "", invalid
	}
	var request struct {
		Attachment struct {
			Session string `json:"session"`
			ID      string `json:"id"`
			SHA256  string `json:"sha256"`
		} `json:"attachment"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, "", "", invalid
	}
	ref := request.Attachment
	digest, err := hex.DecodeString(ref.SHA256)
	if err != nil || len(digest) != 32 || len(ref.Session) == 0 || len(ref.Session) > 256 || len(ref.ID) != 37 || !strings.HasPrefix(ref.ID, "file-") {
		return nil, "", "", invalid
	}
	if s.sessionHandlers == nil {
		return nil, "", "", &rag.ParseError{Code: "document_attachment_unavailable", Message: "The saved conversation file is unavailable."}
	}
	attachment, original, err := s.sessionHandlers.scoped(r).AttachmentOriginal(ref.Session, ref.ID)
	if err != nil {
		return nil, "", "", &rag.ParseError{Code: "document_attachment_unavailable", Message: "The saved conversation file is unavailable."}
	}
	if attachment.SHA256 != ref.SHA256 {
		return nil, "", "", rag.ErrImportConflict
	}
	if err := r.Context().Err(); err != nil {
		return nil, "", "", err
	}
	// A lost acceptance acknowledgment can be checked/repeated explicitly without
	// creating another copy. This does not retry extraction or model inference.
	key := sha256.Sum256([]byte(ref.Session + "\x00" + ref.ID + "\x00" + ref.SHA256))
	return original, attachment.Name, "attachment-" + hex.EncodeToString(key[:]), nil
}
