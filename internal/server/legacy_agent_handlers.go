package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) handleAgentTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.agentManager.StorageError(); err != nil {
		writeAgentError(w, err)
		return
	}
	tasks := []any{}
	for _, task := range s.agentManager.ListTasks() {
		if !s.profileAdmin(r) && (task.Config.Profile == nil || task.Actor != s.agentActor(r)) {
			continue
		}
		if task.Actor != "" && task.Actor != s.agentActor(r) {
			continue
		}
		deletable := s.removableTask(task, s.agentActor(r))
		task.Checkpoint = nil
		task.ContextArchive = nil
		tasks = append(tasks, struct {
			*agents.Task
			Deletable bool   `json:"deletable"`
			Outcome   string `json:"outcome,omitempty"`
		}{task, deletable, agents.TaskOutcome(task)})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (s *Server) handleAgentWorkflows(w http.ResponseWriter, r *http.Request) {
	handleLegacyCoordination(w, r)
}

// handleAgentOrchestrate handles multi-agent orchestration requests
func (s *Server) handleAgentOrchestrate(w http.ResponseWriter, r *http.Request) {
	handleLegacyCoordination(w, r)
}

// handleAgentTools lists and manages agent tools
func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	capabilityList := []capabilities.Descriptor{}
	if s.capabilityBroker != nil {
		capabilityList = s.capabilityBroker.List()
	}
	json.NewEncoder(w).Encode(map[string]any{"capabilities": capabilityList, "count": len(capabilityList)})
}

func (s *Server) handleAgentTools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Check for ?all=true to include disabled tools with status
		if r.URL.Query().Get("all") == "true" {
			toolsWithStatus := s.toolRegistry.GetAllToolsWithStatus()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"tools":         toolsWithStatus,
				"total":         len(toolsWithStatus),
				"enabled_count": s.toolRegistry.GetEnabledCount(),
			})
			return
		}

		// List only enabled tools
		tools := s.toolRegistry.GetTools()

		type ToolInfo struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			Parameters  map[string]interface{} `json:"parameters,omitempty"`
			Type        string                 `json:"type"`
		}

		var toolList []ToolInfo
		for _, tool := range tools {
			toolList = append(toolList, ToolInfo{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
				Type:        tool.Type,
			})
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"tools": toolList,
			"count": len(toolList),
		})

	case http.MethodPatch:
		// Enable or disable a tool
		var req struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
			return
		}

		if req.Name == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "tool name is required"})
			return
		}

		if err := s.toolRegistry.SetToolEnabled(req.Name, req.Enabled); err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		action := "disabled"
		if req.Enabled {
			action = "enabled"
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        action,
			"tool":          req.Name,
			"enabled_count": s.toolRegistry.GetEnabledCount(),
		})

	case http.MethodPost:
		// Register a new tool dynamically
		var req struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			Parameters  map[string]interface{} `json:"parameters"`
			Type        string                 `json:"type"` // "shell", "http", "script"
			Command     string                 `json:"command,omitempty"`
			URL         string                 `json:"url,omitempty"`
			Script      string                 `json:"script,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
			return
		}

		if req.Name == "" || req.Description == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "name and description are required"})
			return
		}

		// Create tool definition
		tool := api.Tool{
			Type: "function",
			Function: api.FunctionDef{
				Name:        req.Name,
				Description: req.Description,
				Parameters:  req.Parameters,
			},
		}

		// Create executor based on type
		var executor agents.SimpleExecutor
		switch req.Type {
		case "shell":
			command := req.Command
			executor = func(ctx context.Context, args json.RawMessage) (string, error) {
				return agents.ExecuteTool(ctx, "shell", json.RawMessage(fmt.Sprintf(`{"command": "%s"}`, command)))
			}
		case "http":
			url := req.URL
			executor = func(ctx context.Context, args json.RawMessage) (string, error) {
				return agents.ExecuteTool(ctx, "http_get", json.RawMessage(fmt.Sprintf(`{"url": "%s"}`, url)))
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "unsupported tool type, use 'shell' or 'http'"})
			return
		}

		s.toolRegistry.RegisterTool(tool, executor)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "registered",
			"tool":   req.Name,
		})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
	}
}
