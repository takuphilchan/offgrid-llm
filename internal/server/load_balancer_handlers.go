package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

// handleLoadBalancer returns load balancer status and configuration
func (s *Server) handleLoadBalancer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.loadBalancer == nil {
		json.NewEncoder(w).Encode(map[string]any{
			"enabled": false,
			"message": "Load balancer not configured",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		stats := s.loadBalancer.GetStats()
		stats["enabled"] = true
		json.NewEncoder(w).Encode(stats)

	case http.MethodPut:
		var req struct {
			Strategy string `json:"strategy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
			return
		}
		s.loadBalancer.SetStrategy(inference.BalancingStrategy(req.Strategy))
		json.NewEncoder(w).Encode(map[string]any{
			"success":  true,
			"strategy": req.Strategy,
		})

	default:
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleLoadBalancerBackends handles backend list operations
func (s *Server) handleLoadBalancerBackends(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.loadBalancer == nil {
		http.Error(w, `{"error": "load balancer not configured"}`, http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		backends := s.loadBalancer.ListBackends()
		json.NewEncoder(w).Encode(map[string]any{
			"backends": backends,
		})

	case http.MethodPost:
		var backend inference.Backend
		if err := json.NewDecoder(r.Body).Decode(&backend); err != nil {
			http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
			return
		}
		s.loadBalancer.AddBackend(backend)
		json.NewEncoder(w).Encode(map[string]any{
			"success":    true,
			"backend_id": backend.ID,
		})

	default:
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleLoadBalancerBackend handles individual backend operations
func (s *Server) handleLoadBalancerBackend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.loadBalancer == nil {
		http.Error(w, `{"error": "load balancer not configured"}`, http.StatusNotFound)
		return
	}

	// Extract backend ID from path
	path := strings.TrimPrefix(r.URL.Path, "/v1/loadbalancer/backends/")
	backendID := strings.TrimSuffix(path, "/")

	if backendID == "" {
		http.Error(w, `{"error": "backend_id required"}`, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if err := s.loadBalancer.RemoveBackend(backendID); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success":    true,
			"backend_id": backendID,
		})

	case http.MethodPut:
		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
			return
		}
		if err := s.loadBalancer.UpdateBackend(backendID, updates); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success":    true,
			"backend_id": backendID,
		})

	default:
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
	}
}
