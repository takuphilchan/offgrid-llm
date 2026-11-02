package serviceclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocumentErrorsKeepActionableCodesWithoutPrivateDiagnostics(t *testing.T) {
	for _, code := range []string{"document_storage_limit", "document_original_unavailable", "document_encrypted", "document_empty", "document_limit", "document_timeout", "document_parser_unavailable", "document_parser_protocol", "document_ocr_required", "document_ocr_language_missing", "document_ocr_language_invalid", "document_ocr_language_unavailable", "document_ocr_language_storage", "document_ocr_unavailable", "document_ocr_failed", "document_invalid", "document_unsupported_format", "document_cancelled"} {
		for _, flat := range []bool{false, true} {
			t.Run(code, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(422)
					if flat {
						_ = json.NewEncoder(w).Encode(map[string]any{"error_code": code, "error": "PRIVATE source bytes \u001b[2J"})
					} else {
						_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "PRIVATE source bytes \u001b[2J"}})
					}
				}))
				defer server.Close()
				client, _ := New(server.URL, "", server.Client())
				_, err := client.Do(context.Background(), "GET", "/v1/rag/status", "", nil)
				var safe *Error
				if !errors.As(err, &safe) || safe.Code != code || strings.Contains(err.Error(), "PRIVATE") || strings.Contains(err.Error(), "HTTP 422") || safe.Retryable {
					t.Fatalf("lost safe typed recovery: %#v", err)
				}
			})
		}
	}
}
