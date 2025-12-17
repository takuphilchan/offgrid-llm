package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestHandleHealth(t *testing.T) {
	server := New()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestHandleRoot(t *testing.T) {
	server := New()

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	server.handleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if name, ok := response["name"].(string); !ok || name != "OffGrid LLM" {
		t.Errorf("Expected name 'OffGrid LLM', got %v", response["name"])
	}
}

func TestHandleListModels(t *testing.T) {
	server := New()

	req := httptest.NewRequest("GET", "/v1/models", nil)
	w := httptest.NewRecorder()

	server.handleListModels(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response api.ModelListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Object != "list" {
		t.Errorf("Expected object 'list', got %s", response.Object)
	}
}

func TestHandleChatCompletions_InvalidMethod(t *testing.T) {
	server := New()

	req := httptest.NewRequest("GET", "/v1/chat/completions", nil)
	w := httptest.NewRecorder()

	server.handleChatCompletions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleChatCompletions_InvalidRequest(t *testing.T) {
	server := New()

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	server.handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleChatCompletions_MissingModel(t *testing.T) {
	server := New()

	reqBody := api.ChatCompletionRequest{
		Messages: []api.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleChatCompletions_MissingMessages(t *testing.T) {
	server := New()

	reqBody := api.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []api.ChatMessage{},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCompletions_InvalidMethod(t *testing.T) {
	server := New()

	req := httptest.NewRequest("GET", "/v1/completions", nil)
	w := httptest.NewRecorder()

	server.handleCompletions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	writeError(w, "Test error", http.StatusInternalServerError)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	var response api.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if response.Error.Message != "Test error" {
		t.Errorf("Expected error message 'Test error', got %s", response.Error.Message)
	}

	if response.Error.Type != "api_error" {
		t.Errorf("Expected error type 'api_error', got %s", response.Error.Type)
	}
}
