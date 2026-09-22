package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const maxJSONRequestBytes = 8 << 20

// decodeJSON applies the common request-size and single-document rules used by
// OffGrid's JSON endpoints.
func decodeJSON(r *http.Request, target interface{}) error {
	return decodeJSONWithPolicy(r, target, false)
}

func decodeStrictJSON(r *http.Request, target interface{}) error {
	return decodeJSONWithPolicy(r, target, true)
}

func decodeJSONWithPolicy(r *http.Request, target interface{}, strict bool) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxJSONRequestBytes+1))
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("invalid request body: multiple JSON values")
		}
		return fmt.Errorf("invalid request body: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// captureResponseWriter lets one protocol adapter reuse a core HTTP handler
// without exposing transport-specific response types across packages.
type captureResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newCaptureResponseWriter() *captureResponseWriter {
	return &captureResponseWriter{header: make(http.Header)}
}

func (w *captureResponseWriter) Header() http.Header    { return w.header }
func (w *captureResponseWriter) WriteHeader(status int) { w.status = status }
func (w *captureResponseWriter) Write(content []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(content)
}

func (w *captureResponseWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func invokeJSONHandler(r *http.Request, payload interface{}, handler http.HandlerFunc) *captureResponseWriter {
	encoded, _ := json.Marshal(payload)
	request := r.Clone(r.Context())
	request.Method = http.MethodPost
	request.Body = io.NopCloser(bytes.NewReader(encoded))
	request.ContentLength = int64(len(encoded))
	writer := newCaptureResponseWriter()
	handler(writer, request)
	return writer
}
