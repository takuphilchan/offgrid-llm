package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/mcp"
)

// handleAgentMCP manages MCP server connections
func (s *Server) handleAgentMCP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// List connected MCP servers
		servers := s.toolRegistry.GetMCPServers()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"servers": servers,
		})

	case http.MethodPost:
		// Add new MCP server
		var req struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
			return
		}

		if req.Name == "" || req.URL == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "name and url are required"})
			return
		}

		// Try to connect to the MCP server and discover tools
		count, err := s.toolRegistry.ConnectAndPersistMCP(req.Name, req.URL)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("failed to connect to MCP server: %v", err),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "connected",
			"server":      req.Name,
			"tools_added": count,
		})

	case http.MethodDelete:
		name := r.URL.Query().Get("name")
		if strings.TrimSpace(name) == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Connection name is required.", "code": "mcp_name_required"})
			return
		}
		count, err := s.toolRegistry.RemoveMCPServer(name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Could not save the connection removal. The connection has not been removed. Please retry.", "code": "mcp_remove_failed"})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "removed", "server": name, "tools_removed": count})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
	}
}

// handleAgentMCPTest tests connectivity to an MCP server without adding it
func (s *Server) handleAgentMCPTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	if req.URL == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "url is required"})
		return
	}

	// Test connection to MCP server
	count, err := s.toolRegistry.TestMCPConnection(req.URL)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "ok",
		"tools_count": count,
	})
}

// handleMCPMarketplace handles the MCP server marketplace
func (s *Server) handleMCPMarketplace(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		category := r.URL.Query().Get("category")
		search := r.URL.Query().Get("search")

		var servers []*mcp.MarketplaceServer
		if search != "" {
			servers = s.mcpMarketplace.SearchServers(search)
		} else {
			servers = s.mcpMarketplace.ListAvailable(category)
		}

		// Get installed servers to mark which are installed
		installed := s.mcpMarketplace.ListInstalled()
		installedMap := make(map[string]*mcp.InstalledServer)
		for _, i := range installed {
			installedMap[i.ID] = i
		}

		// Build response with installation status
		type ServerWithStatus struct {
			*mcp.MarketplaceServer
			Installed bool `json:"installed"`
			Enabled   bool `json:"enabled,omitempty"`
		}
		result := make([]ServerWithStatus, len(servers))
		for i, s := range servers {
			result[i] = ServerWithStatus{
				MarketplaceServer: s,
				Installed:         installedMap[s.ID] != nil,
			}
			if inst, ok := installedMap[s.ID]; ok {
				result[i].Enabled = inst.Enabled
			}
		}

		json.NewEncoder(w).Encode(map[string]any{
			"servers":         result,
			"categories":      s.mcpMarketplace.GetCategories(),
			"installed_count": len(installed),
		})

	default:
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleMCPMarketplaceAction handles install/uninstall/configure actions
func (s *Server) handleMCPMarketplaceAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract server ID from path
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/agents/mcp/marketplace/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, `{"error": "server ID required"}`, http.StatusBadRequest)
		return
	}
	serverID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch r.Method {
	case http.MethodGet:
		// Get server details
		server, err := s.mcpMarketplace.GetServer(serverID)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusNotFound)
			return
		}

		installed, isInstalled := s.mcpMarketplace.GetInstalled(serverID)

		json.NewEncoder(w).Encode(map[string]any{
			"server":    server,
			"installed": isInstalled,
			"config":    installed,
		})

	case http.MethodPost:
		switch action {
		case "install":
			var req struct {
				Env map[string]string `json:"env"`
			}
			json.NewDecoder(r.Body).Decode(&req)

			if err := s.mcpMarketplace.Install(serverID, req.Env); err != nil {
				json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"error":   err.Error(),
				})
				return
			}

			json.NewEncoder(w).Encode(map[string]any{
				"success":   true,
				"server_id": serverID,
			})

		case "uninstall":
			if err := s.mcpMarketplace.Uninstall(serverID); err != nil {
				json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"error":   err.Error(),
				})
				return
			}

			json.NewEncoder(w).Encode(map[string]any{
				"success":   true,
				"server_id": serverID,
			})

		case "enable":
			if err := s.mcpMarketplace.Enable(serverID); err != nil {
				json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"error":   err.Error(),
				})
				return
			}

			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"enabled": true,
			})

		case "disable":
			if err := s.mcpMarketplace.Disable(serverID); err != nil {
				json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"error":   err.Error(),
				})
				return
			}

			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"enabled": false,
			})

		case "configure":
			var req struct {
				Env map[string]string `json:"env"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
				return
			}

			if err := s.mcpMarketplace.UpdateEnv(serverID, req.Env); err != nil {
				json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"error":   err.Error(),
				})
				return
			}

			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
			})

		default:
			http.Error(w, `{"error": "unknown action"}`, http.StatusBadRequest)
		}

	case http.MethodDelete:
		// Uninstall server
		if err := s.mcpMarketplace.Uninstall(serverID); err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"success":   true,
			"server_id": serverID,
		})

	default:
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
	}
}
