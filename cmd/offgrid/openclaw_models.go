package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// OpenClaw's config patch merges objects but replaces arrays. Read the narrow
// OffGrid model array first so a newly selected model cannot remove a model
// that was already configured. A concurrent addition is still protected by
// OpenClaw's own config-patch clobber check.
func readOpenClawModels(executable string) ([]json.RawMessage, error) {
	command, err := openClawCommand(executable, []string{"config", "get", "models.providers.offgrid.models", "--json"})
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := runIntegrationProcess(command); err != nil {
		// A fresh installation has no OffGrid provider. Only this exact unset
		// path is an empty array; auth, parse, and CLI failures must stop.
		if strings.Contains(stdout.String()+stderr.String(), "Config path is valid but unset: models.providers.offgrid.models") {
			return nil, nil
		}
		return nil, fmt.Errorf("read OpenClaw model config: %w", err)
	}
	if bytes.Equal(bytes.TrimSpace(stdout.Bytes()), []byte("null")) {
		return nil, nil
	}
	var models []json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &models); err != nil {
		return nil, fmt.Errorf("OpenClaw returned invalid model config: %w", err)
	}
	if models == nil {
		return nil, fmt.Errorf("OpenClaw returned no model array")
	}
	return models, nil
}

func preferredOpenClawModel(executable string) (string, error) {
	models, err := readOpenClawModels(executable)
	if err != nil {
		return "", err
	}
	for _, entry := range models {
		var model struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(entry, &model); err != nil || strings.TrimSpace(model.ID) == "" {
			return "", fmt.Errorf("OpenClaw has an invalid OffGrid model entry")
		}
		return model.ID, nil
	}
	return "", nil
}

func mergeOpenClawModels(provider map[string]any, existing []json.RawMessage, catalog []api.Model) error {
	proposed, ok := provider["models"].([]any)
	if !ok || len(proposed) != 1 {
		return fmt.Errorf("generated provider must contain exactly one model")
	}
	model, ok := proposed[0].(map[string]any)
	if !ok {
		return fmt.Errorf("generated model is invalid")
	}
	modelID, ok := model["id"].(string)
	if !ok || strings.TrimSpace(modelID) == "" {
		return fmt.Errorf("generated model has no ID")
	}
	for _, key := range []string{"contextWindow", "maxTokens"} {
		value, ok := model[key].(float64)
		if !ok || value < 1 {
			return fmt.Errorf("generated model has no valid %s", key)
		}
	}
	contexts := make(map[string]int, len(catalog))
	for _, available := range catalog {
		if available.SupportsChat() && available.ContextWindow > 1 {
			contexts[available.ID] = available.ContextWindow
		}
	}
	if contexts[modelID] != int(model["contextWindow"].(float64)) {
		return fmt.Errorf("selected model context changed during setup; retry without replacing existing config")
	}
	seen := make(map[string]bool, len(existing))
	for index, entry := range existing {
		var saved struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(entry, &saved); err != nil || strings.TrimSpace(saved.ID) == "" {
			return fmt.Errorf("existing OffGrid model entry is invalid")
		}
		if seen[saved.ID] {
			return fmt.Errorf("existing OffGrid model %q is duplicated", saved.ID)
		}
		seen[saved.ID] = true
		if currentContext, installed := contexts[saved.ID]; installed {
			// Refresh saved OffGrid contexts from live discovery, retaining
			// unknown fields and unavailable historical model entries.
			var configured map[string]json.RawMessage
			if err := json.Unmarshal(entry, &configured); err != nil {
				return fmt.Errorf("existing OffGrid model %q is invalid: %w", saved.ID, err)
			}
			contextValue, err := json.Marshal(currentContext)
			if err != nil {
				return err
			}
			configured["contextWindow"] = contextValue
			if saved.ID == modelID {
				maxOutput, err := json.Marshal(model["maxTokens"])
				if err != nil {
					return err
				}
				configured["maxTokens"] = maxOutput
			} else {
				var maxOutput int
				if err := json.Unmarshal(configured["maxTokens"], &maxOutput); err == nil && maxOutput >= currentContext {
					bounded, err := json.Marshal(currentContext - 1)
					if err != nil {
						return err
					}
					configured["maxTokens"] = bounded
				}
			}
			updated, err := json.Marshal(configured)
			if err != nil {
				return err
			}
			existing[index] = updated
		}
	}
	if !seen[modelID] {
		encoded, err := json.Marshal(model)
		if err != nil {
			return err
		}
		existing = append(existing, encoded)
	}
	provider["models"] = existing
	return nil
}

func inspectOpenClawProvider(executable, expectedBaseURL, expectedModelID string, expectedContext int) (bool, string) {
	command, err := openClawCommand(executable, []string{"config", "get", "models.providers.offgrid", "--json"})
	if err != nil {
		return false, err.Error()
	}
	var stdout bytes.Buffer
	command.Stdout, command.Stderr = &stdout, io.Discard
	if err := runIntegrationProcess(command); err != nil {
		return false, "OffGrid provider is not configured in OpenClaw"
	}
	var provider struct {
		BaseURL string `json:"baseUrl"`
		Models  []struct {
			ID            string `json:"id"`
			ContextWindow int    `json:"contextWindow"`
		} `json:"models"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &provider); err != nil {
		return false, "OpenClaw returned an invalid OffGrid provider configuration"
	}
	if strings.TrimRight(provider.BaseURL, "/") != strings.TrimRight(expectedBaseURL, "/") {
		return false, fmt.Sprintf("OpenClaw points at %s, expected %s", provider.BaseURL, expectedBaseURL)
	}
	for _, model := range provider.Models {
		if model.ID == expectedModelID {
			if model.ContextWindow != expectedContext {
				return false, fmt.Sprintf("model %s records %d context tokens in OpenClaw, but OffGrid currently allocates %d; rerun offgrid openclaw install --model %s", expectedModelID, model.ContextWindow, expectedContext, expectedModelID)
			}
			return true, ""
		}
	}
	return false, fmt.Sprintf("model %s is not configured in OpenClaw", expectedModelID)
}

func expectedOpenClawBaseURL(options openClawOptions) string {
	baseURL := options.BaseURL
	if baseURL == "" {
		baseURL = options.ServerURL
	}
	return strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1") + "/v1"
}
