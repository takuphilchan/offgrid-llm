package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/users"
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

func TestServerListenAddress(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{name: "default host", host: "", port: 11611, want: "localhost:11611"},
		{name: "localhost", host: "localhost", port: 11611, want: "localhost:11611"},
		{name: "all IPv4 interfaces", host: "0.0.0.0", port: 8080, want: "0.0.0.0:8080"},
		{name: "IPv6 loopback", host: "::1", port: 11611, want: "[::1]:11611"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serverListenAddress(tt.host, tt.port); got != tt.want {
				t.Fatalf("serverListenAddress(%q, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
			}
		})
	}
}

func TestIsLoopbackHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{host: "localhost", want: true},
		{host: "LOCALHOST", want: true},
		{host: "127.0.0.1", want: true},
		{host: "::1", want: true},
		{host: "[::1]", want: true},
		{host: "0.0.0.0", want: false},
		{host: "192.168.1.10", want: false},
		{host: "offgrid.local", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			if got := isLoopbackHost(tt.host); got != tt.want {
				t.Fatalf("isLoopbackHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestValidateServerExposure(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{name: "local unauthenticated", config: &config.Config{ServerHost: "localhost"}},
		{name: "remote authenticated", config: &config.Config{ServerHost: "0.0.0.0", RequireAuth: true}},
		{name: "remote explicit override", config: &config.Config{ServerHost: "0.0.0.0", AllowUnauthenticatedRemote: true}},
		{name: "remote unauthenticated rejected", config: &config.Config{ServerHost: "0.0.0.0"}, wantErr: true},
		{name: "missing config", config: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServerExposure(tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateServerExposure() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequirePermissionWhenAuthEnabled(t *testing.T) {
	store := users.NewUserStore("")
	_, userKey, err := store.CreateUser("regular-user", "password", users.RoleUser)
	if err != nil {
		t.Fatalf("CreateUser(regular-user): %v", err)
	}
	_, adminKey, err := store.CreateUser("admin-user", "password", users.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser(admin-user): %v", err)
	}

	server := &Server{config: &config.Config{RequireAuth: true}}
	authMiddleware := users.NewMiddleware(store)
	authMiddleware.SetRequireAuth(true)

	protected := server.requirePermissionWhenAuthEnabled(users.PermissionAdmin, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := authMiddleware.Wrap(http.HandlerFunc(protected))

	tests := []struct {
		name       string
		apiKey     string
		wantStatus int
	}{
		{name: "missing credentials", wantStatus: http.StatusUnauthorized},
		{name: "regular user", apiKey: userKey, wantStatus: http.StatusForbidden},
		{name: "administrator", apiKey: adminKey, wantStatus: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
			if tt.apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+tt.apiKey)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestRequirePermissionWhenAuthDisabled(t *testing.T) {
	server := &Server{config: &config.Config{RequireAuth: false}}
	handler := server.requirePermissionWhenAuthEnabled(users.PermissionAdmin, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodGet, "/v1/users", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestHandleQuotaRejectsOtherUser(t *testing.T) {
	store := users.NewUserStore("")
	_, userKey, err := store.CreateUser("regular-user", "password", users.RoleUser)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	server := &Server{config: &config.Config{RequireAuth: true}}
	authMiddleware := users.NewMiddleware(store)
	authMiddleware.SetRequireAuth(true)
	handler := authMiddleware.Wrap(http.HandlerFunc(server.handleQuota))

	req := httptest.NewRequest(http.MethodGet, "/v1/quota?user_id=another-user", nil)
	req.Header.Set("Authorization", "Bearer "+userKey)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}
