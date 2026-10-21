package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handlePlugins handles plugin list and scan operations
func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.pluginManager == nil {
		http.Error(w, `{"error": "plugin system not initialized"}`, http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		plugins := s.pluginManager.ListPlugins()
		json.NewEncoder(w).Encode(map[string]any{
			"plugins": plugins,
			"count":   len(plugins),
		})

	case http.MethodPost:
		// Scan for new plugins
		if err := s.pluginManager.ScanPlugins(); err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		plugins := s.pluginManager.ListPlugins()
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"plugins": plugins,
			"count":   len(plugins),
		})

	default:
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handlePluginAction handles individual plugin operations
func (s *Server) handlePluginAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.pluginManager == nil {
		http.Error(w, `{"error": "plugin system not initialized"}`, http.StatusNotFound)
		return
	}

	// Extract plugin ID from path
	path := strings.TrimPrefix(r.URL.Path, "/v1/plugins/")
	parts := strings.SplitN(path, "/", 2)
	pluginID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	if pluginID == "" {
		http.Error(w, `{"error": "plugin_id required"}`, http.StatusBadRequest)
		return
	}

	plugin, exists := s.pluginManager.GetPlugin(pluginID)
	if !exists && action != "install" {
		http.Error(w, `{"error": "plugin not found"}`, http.StatusNotFound)
		return
	}

	switch action {
	case "enable":
		if err := s.pluginManager.EnablePlugin(pluginID); err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success":   true,
			"plugin_id": pluginID,
		})

	case "disable":
		if err := s.pluginManager.DisablePlugin(pluginID); err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success":   true,
			"plugin_id": pluginID,
		})

	case "execute":
		if r.Method != http.MethodPost {
			http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		var input map[string]any
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, `{"error": "invalid input"}`, http.StatusBadRequest)
			return
		}
		output, err := s.pluginManager.ExecutePlugin(pluginID, input)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"output":  output,
		})

	case "uninstall":
		if err := s.pluginManager.UninstallPlugin(pluginID); err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success":   true,
			"plugin_id": pluginID,
		})

	case "config":
		if r.Method == http.MethodPut {
			var config map[string]string
			if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
				http.Error(w, `{"error": "invalid config"}`, http.StatusBadRequest)
				return
			}
			if err := s.pluginManager.UpdatePluginConfig(pluginID, config); err != nil {
				json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"error":   err.Error(),
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"success":   true,
				"plugin_id": pluginID,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"plugin_id": pluginID,
				"config":    plugin.Config,
			})
		}

	default:
		// Return plugin details
		json.NewEncoder(w).Encode(plugin)
	}
}
