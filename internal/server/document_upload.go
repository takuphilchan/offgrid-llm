package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

const maxDocumentUploadBytes int64 = 50 << 20

func readDocumentUpload(r *http.Request) ([]byte, string, string, error) {
	if err := r.Context().Err(); err != nil {
		return nil, "", "", err
	}
	// ParseMultipartForm's argument only bounds RAM, not upload size. Bound the
	// entire body separately, allowing a small fixed multipart envelope overhead.
	r.Body = http.MaxBytesReader(nil, r.Body, maxDocumentUploadBytes+(1<<20))
	err := r.ParseMultipartForm(1 << 20)
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if err != nil {
		if r.Context().Err() != nil {
			return nil, "", "", r.Context().Err()
		}
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, "", "", err
		}
		return nil, "", "", &rag.ParseError{Code: "document_invalid", Message: "The document upload is incomplete or malformed. Choose the file again."}
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, "", "", &rag.ParseError{Code: "document_invalid", Message: "Choose a document to upload."}
	}
	defer file.Close()
	if header.Size > maxDocumentUploadBytes {
		return nil, "", "", &rag.ParseError{Code: "document_limit", Message: "Documents must be 50 MiB or smaller."}
	}
	content, err := io.ReadAll(io.LimitReader(file, maxDocumentUploadBytes+1))
	if err != nil {
		return nil, "", "", err
	}
	if int64(len(content)) > maxDocumentUploadBytes {
		return nil, "", "", &rag.ParseError{Code: "document_limit", Message: "Documents must be 50 MiB or smaller."}
	}
	if err := r.Context().Err(); err != nil {
		return nil, "", "", err
	}
	return content, header.Filename, header.Header.Get("Content-Type"), nil
}

func writeDocumentIngestError(w http.ResponseWriter, err error) {
	status, code, message := http.StatusInternalServerError, "document_index_failed", "The document could not be indexed. Check Knowledge readiness and try again."
	var problem *rag.ParseError
	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, rag.ErrCollectionUnavailable):
		status, code, message = http.StatusNotFound, "document_unavailable", "The document is unavailable in this Knowledge workspace."
	case errors.Is(err, rag.ErrImportConflict):
		status, code, message = http.StatusConflict, "document_import_conflict", "Import state changed. Refresh its status before continuing."
	case errors.Is(err, context.Canceled):
		status, code, message = http.StatusRequestTimeout, "document_cancelled", "Document import was cancelled."
	case errors.Is(err, context.DeadlineExceeded):
		status, code, message = http.StatusRequestTimeout, "document_timeout", "Document processing timed out. Try a smaller document."
	case errors.As(err, &tooLarge):
		status, code, message = http.StatusRequestEntityTooLarge, "document_limit", "Documents must be 50 MiB or smaller."
	case errors.As(err, &problem):
		status, code, message = http.StatusUnprocessableEntity, problem.Code, problem.Message
		if problem.Code == "document_limit" {
			status = http.StatusRequestEntityTooLarge
		}
		if problem.Code == "document_parser_unavailable" || problem.Code == "document_busy" {
			status = http.StatusServiceUnavailable
		}
	}
	log.Printf("Document ingestion failed: %s", code)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(IngestResponse{Success: false, Error: message, ErrorCode: code})
}
