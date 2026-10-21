package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) handleLoRAList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.loraManager.GetStatus())
}

func (s *Server) handleLoRA(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/v1/lora/")
	parts := strings.Split(path, "/")

	switch r.Method {
	case http.MethodGet:
		if len(parts) == 0 || parts[0] == "" || parts[0] == "adapters" {
			// List all adapters - GetStatus already returns the right format
			json.NewEncoder(w).Encode(s.loraManager.GetStatus())
			return
		}
		adapter, ok := s.loraManager.GetAdapter(parts[0])
		if !ok {
			http.Error(w, `{"error": "adapter not found"}`, http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(adapter)

	case http.MethodPost:
		// Register or load adapter
		var req struct {
			Action      string  `json:"action"` // "register" or "load"
			Name        string  `json:"name"`
			Path        string  `json:"path"`
			Scale       float32 `json:"scale"`
			Description string  `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
			return
		}

		switch req.Action {
		case "register":
			if req.Scale == 0 {
				req.Scale = 1.0
			}
			adapter, err := s.loraManager.RegisterAdapter("", req.Name, req.Path, req.Scale, req.Description)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(adapter)
		case "load":
			adapterID := parts[0]
			writeError(w, fmt.Sprintf("LoRA runtime loading is experimental and not implemented for this backend (adapter: %s)", adapterID), http.StatusNotImplemented)
			return
		case "unload":
			adapterID := parts[0]
			if err := s.loraManager.UnloadAdapter(r.Context(), adapterID); err != nil {
				http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "unloaded", "id": adapterID})
		default:
			http.Error(w, `{"error": "invalid action"}`, http.StatusBadRequest)
		}

	case http.MethodDelete:
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, `{"error": "adapter id required"}`, http.StatusBadRequest)
			return
		}
		if err := s.loraManager.DeleteAdapter(parts[0]); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
