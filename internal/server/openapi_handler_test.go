package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAPISpecIsServed(t *testing.T) {
	recorder := httptest.NewRecorder()
	handleOpenAPISpec(recorder, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "openapi: 3.1.0") {
		t.Fatal("response does not contain the OpenAPI contract")
	}
}
