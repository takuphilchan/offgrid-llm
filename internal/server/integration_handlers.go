package server

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type integrationStatus struct {
	integrations.Profile
	Ready         bool     `json:"ready"`
	Status        string   `json:"status"`
	ModelID       string   `json:"model_id,omitempty"`
	ContextWindow int      `json:"context_window"`
	Warnings      []string `json:"warnings"`
}

type integrationsResponse struct {
	Provider     string              `json:"provider"`
	BaseURL      string              `json:"base_url"`
	Integrations []integrationStatus `json:"integrations"`
}

type integrationSetupResponse struct {
	Integration integrationStatus  `json:"integration"`
	Setup       integrations.Setup `json:"setup"`
}

func (s *Server) handleIntegrations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeIntegrationError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}
	models := s.integrationModels()
	contextWindow := s.effectiveContextWindow()
	requestedModel := strings.TrimSpace(r.URL.Query().Get("model"))
	items := make([]integrationStatus, 0, len(s.integrationRegistry.List()))
	for _, adapter := range s.integrationRegistry.List() {
		items = append(items, evaluateIntegration(adapter.Profile(), models, contextWindow, requestedModel))
	}
	writeJSON(w, http.StatusOK, integrationsResponse{
		Provider:     "offgrid",
		BaseURL:      fmt.Sprintf("http://127.0.0.1:%d/v1", s.config.ServerPort),
		Integrations: items,
	})
}

func (s *Server) handleIntegration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeIntegrationError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/integrations/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "setup" {
		writeIntegrationError(w, http.StatusNotFound, "not_found", "Integration endpoint not found")
		return
	}
	adapter, ok := s.integrationRegistry.Get(parts[0])
	if !ok {
		writeIntegrationError(w, http.StatusNotFound, "integration_not_found", "Unknown integration")
		return
	}

	models := s.integrationModels()
	contextWindow := s.effectiveContextWindow()
	status := evaluateIntegration(adapter.Profile(), models, contextWindow, strings.TrimSpace(r.URL.Query().Get("model")))
	if status.ModelID == "" {
		writeIntegrationError(w, http.StatusConflict, "model_required", "Install a chat model or provide ?model=<id>")
		return
	}
	baseURL := strings.TrimSpace(r.URL.Query().Get("base_url"))
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://127.0.0.1:%d", s.config.ServerPort)
	}
	if err := validateIntegrationBaseURL(baseURL); err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_base_url", err.Error())
		return
	}
	maxOutput := 4096
	if value := strings.TrimSpace(r.URL.Query().Get("max_output_tokens")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			writeIntegrationError(w, http.StatusBadRequest, "invalid_max_output_tokens", "max_output_tokens must be a positive integer")
			return
		}
		maxOutput = parsed
	}
	setup, err := adapter.Render(integrations.Options{
		BaseURL:         baseURL,
		ModelID:         status.ModelID,
		ContextWindow:   contextWindow,
		MaxOutputTokens: maxOutput,
	})
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_setup", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, integrationSetupResponse{Integration: status, Setup: setup})
}

func (s *Server) integrationModels() []api.Model {
	models := s.registry.ListModels()
	result := make([]api.Model, 0, len(models))
	for _, model := range models {
		if model.Type != "embedding" {
			result = append(result, model)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func evaluateIntegration(profile integrations.Profile, models []api.Model, contextWindow int, requestedModel string) integrationStatus {
	status := integrationStatus{Profile: profile, ContextWindow: contextWindow, Warnings: []string{}}
	blocked := false
	if requestedModel != "" {
		found := false
		for _, model := range models {
			if model.ID == requestedModel {
				found = true
				break
			}
		}
		if found {
			status.ModelID = requestedModel
		} else {
			status.Warnings = append(status.Warnings, fmt.Sprintf("model %q is not installed", requestedModel))
			blocked = true
		}
	} else if len(models) > 0 {
		status.ModelID = models[0].ID
	} else {
		status.Warnings = append(status.Warnings, "no chat model is installed")
		blocked = true
	}
	if contextWindow < profile.MinimumContext {
		status.Warnings = append(status.Warnings, fmt.Sprintf("context window is %d; OffGrid agent integrations require at least %d", contextWindow, profile.MinimumContext))
		blocked = true
	} else if contextWindow < profile.RecommendedContext {
		status.Warnings = append(status.Warnings, fmt.Sprintf("context window is %d; %d is recommended for long %s sessions", contextWindow, profile.RecommendedContext, profile.Name))
	}
	status.Ready = status.ModelID != "" && !blocked
	if status.Ready {
		status.Status = "ready"
	} else {
		status.Status = "needs_configuration"
	}
	return status
}

func validateIntegrationBaseURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("base_url must be an absolute HTTP or HTTPS URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("base_url must use HTTP or HTTPS")
	}
	if parsed.User != nil {
		return fmt.Errorf("base_url must not contain credentials")
	}
	return nil
}

func writeIntegrationError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
