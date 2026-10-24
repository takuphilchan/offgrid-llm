package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// OpenClaw's config patch merges objects but replaces arrays. Read the narrow
// OffGrid model array first so a newly selected model cannot remove a model
// that was already configured. A concurrent addition is still protected by
// OpenClaw's own config-patch clobber check.
func readOpenClawModels(ctx context.Context, executable string) ([]json.RawMessage, error) {
	command, err := openClawCommand(executable, []string{"config", "get", "models.providers.offgrid.models", "--json"})
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := runIntegrationProcess(bindIntegrationContext(ctx, command)); err != nil {
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
	if contexts[modelID] < int(model["contextWindow"].(float64)) {
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
		if saved.ID == modelID {
			// Only update the explicitly selected model. Other models' authored
			// context limits and custom fields belong to the user.
			var configured map[string]json.RawMessage
			if err := json.Unmarshal(entry, &configured); err != nil {
				return fmt.Errorf("existing OffGrid model %q is invalid: %w", saved.ID, err)
			}
			contextValue, err := json.Marshal(model["contextWindow"])
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
			}
			updated, err := json.Marshal(configured)
			if err != nil {
				return err
			}
			existing[index] = updated
		}
	}
	// The first provider entry is OffGrid CLI's preference when OpenClaw has no
	// authored OffGrid primary. Keep every entry and custom field, but put the
	// explicitly installed model first without changing OpenClaw's global default.
	ordered := make([]json.RawMessage, 0, len(existing)+1)
	var selected json.RawMessage
	for _, entry := range existing {
		var saved struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(entry, &saved); err != nil {
			return err
		}
		if saved.ID == modelID {
			selected = entry
		} else {
			ordered = append(ordered, entry)
		}
	}
	if !seen[modelID] {
		var err error
		selected, err = json.Marshal(model)
		if err != nil {
			return err
		}
	}
	provider["models"] = append([]json.RawMessage{selected}, ordered...)
	return nil
}
