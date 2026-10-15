package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

type profileResourceIssue struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

type effectiveAgentProfile struct {
	Profile      *agents.AgentProfile               `json:"profile"`
	Ready        bool                               `json:"ready"`
	Issues       []profileResourceIssue             `json:"issues"`
	Capabilities map[string]capabilities.Descriptor `json:"capabilities"`
	Children     map[string]*effectiveAgentProfile  `json:"children,omitempty"`
}

func (s *Server) profileAdmin(r *http.Request) bool {
	if s.config == nil || !s.config.RequireAuth {
		return true
	}
	u := users.UserFromContext(r.Context())
	return u != nil && u.HasPermission(users.PermissionAdmin)
}

// The existing workspace-admin MCP credentials are not personal connections.
// This catalog never exposes their config, URLs, commands, or credentials.
func (s *Server) profileToolCatalog(r *http.Request) map[string]capabilities.Descriptor {
	out := map[string]capabilities.Descriptor{}
	if s.toolRegistry == nil {
		return out
	}
	for _, tool := range s.toolRegistry.ToolsForExecution(r.Context(), agents.ToolExecution{Actor: s.agentActor(r)}) {
		d, ok := s.toolRegistry.Capability(tool.Function.Name)
		if !ok || d.Name == "shell" || (d.Kind == capabilities.Execute && d.Namespace != "mcp") {
			continue
		}
		if !profileToolAllowed(s.profileAdmin(r), d) {
			continue
		}
		out[d.Name] = d
	}
	return out
}

func (s *Server) effectiveProfile(r *http.Request, profile *agents.AgentProfile, ancestors map[string]bool, depth int, count *int) *effectiveAgentProfile {
	effective := &effectiveAgentProfile{Profile: profile, Ready: true, Issues: []profileResourceIssue{}, Capabilities: map[string]capabilities.Descriptor{}}
	issue := func(kind, ref, code, message string) {
		effective.Ready = false
		effective.Issues = append(effective.Issues, profileResourceIssue{kind, ref, code, message})
	}
	if profile.Archived {
		issue("profile", profile.ID, "profile_archived", "Archived profiles cannot start new work.")
	}
	if s.registry == nil {
		issue("model", profile.Settings.Model, "model_unavailable", "Model storage is unavailable.")
	} else if _, err := s.registry.GetChatModel(profile.Settings.Model); err != nil {
		issue("model", profile.Settings.Model, "model_unavailable", "Select an installed chat model. No model was downloaded.")
	}
	if profile.Settings.Computer && !s.profileAdmin(r) {
		issue("computer", "desktop", "computer_admin_required", "Computer control currently requires workspace administrator permission.")
	}
	if len(profile.Settings.Knowledge) > 0 {
		if err := authorizeChatAccess(r.Context(), s.config != nil && s.config.RequireAuth, true); err != nil {
			issue("knowledge", "", "knowledge_denied", "Knowledge access is not permitted.")
		} else {
			for _, id := range profile.Settings.Knowledge {
				if s.ragEngine == nil || s.ragEngine.StorageError() != nil || s.ragEngine.GetDocument(id) == nil {
					issue("knowledge", id, "knowledge_unavailable", "This Knowledge document is unavailable. Select an existing document.")
				}
			}
		}
	}
	catalog := s.profileToolCatalog(r)
	for _, ref := range profile.Settings.Tools {
		d, ok := catalog[ref.Tool]
		if !ok || d.Source != ref.Provider || d.Revision != ref.SchemaRevision || d.Location != ref.Location {
			issue("tool", ref.Tool, "tool_reference_changed", "Tool is unavailable or its provider, schema or policy changed. Review the current tool before selecting it again.")
			continue
		}
		effective.Capabilities[d.Name] = d
	}
	if len(profile.Settings.Delegates) > 0 {
		effective.Children = map[string]*effectiveAgentProfile{}
		ancestors[profile.ID] = true
		defer delete(ancestors, profile.ID)
		for _, id := range profile.Settings.Delegates {
			*count++
			if ancestors[id] || depth >= 1 || *count > 4 {
				issue("delegate", id, "delegation_limit", "Select at most four direct child profiles. Children cannot delegate or form cycles.")
				continue
			}
			child, err := s.agentManager.GetProfile(s.workspaceID, s.agentActor(r), id, 0)
			if err != nil {
				issue("delegate", id, "profile_unavailable", "Select an available profile owned by your account.")
				continue
			}
			resolved := s.effectiveProfile(r, child, ancestors, depth+1, count)
			effective.Children[id] = resolved
			if !resolved.Ready {
				issue("delegate", id, "delegate_unavailable", "A delegated profile has unavailable resources.")
			}
			if child.Settings.Computer {
				issue("delegate", id, "delegate_computer_denied", "Delegated profiles cannot control the desktop.")
			}
			for _, document := range child.Settings.Knowledge {
				found := false
				for _, parentDocument := range profile.Settings.Knowledge {
					if document == parentDocument {
						found = true
						break
					}
				}
				if !found {
					issue("delegate", id, "delegate_knowledge_denied", "Child Knowledge must be a subset of the parent Knowledge.")
				}
			}
			for name, d := range resolved.Capabilities {
				parent, allowed := effective.Capabilities[name]
				if !allowed || parent != d || d.Kind != capabilities.Read || d.Risk != capabilities.RiskLow {
					issue("delegate", id, "delegate_tools_denied", "Child tools must be reviewed low-risk reads and a subset of the parent tools.")
				}
			}
		}
	}
	return effective
}

func (s *Server) resolveProfile(r *http.Request, profile *agents.AgentProfile) *effectiveAgentProfile {
	count := 0
	return s.effectiveProfile(r, profile, map[string]bool{}, 0, &count)
}

func (s *Server) applyJobProfile(w http.ResponseWriter, r *http.Request, req *jobSubmitRequest, config *agents.AgentConfig) bool {
	profile, err := s.agentManager.GetProfile(s.workspaceID, s.agentActor(r), req.ProfileID, req.ProfileRevision)
	if err != nil {
		writeProfileError(w, err)
		return false
	}
	effective := s.resolveProfile(r, profile)
	if !effective.Ready {
		writeJSON(w, 422, effective)
		return false
	}
	if req.Model != "" && req.Model != profile.Settings.Model {
		writeJobError(w, 409, "profile_model_mismatch", "Use this profile's installed model or save a new profile revision.", false)
		return false
	}
	if !profile.Settings.Computer && (req.ComputerScope != "" || len(req.Expectations) > 0) {
		writeJobError(w, 403, "profile_computer_disabled", "Computer access is disabled for this profile.", false)
		return false
	}
	if req.MaxIterations > profile.Settings.MaxIterations {
		writeJobError(w, 422, "profile_budget_exceeded", "Task budget cannot exceed the selected profile's budget.", false)
		return false
	}
	req.Model = profile.Settings.Model
	config.Profile, config.Instructions = profile, profile.Settings.Instructions
	config.AllowedCapabilities = effective.Capabilities
	if !restrictProfileConfig(w, req.Restrictions, config) {
		return false
	}
	// Profile-enabled computer work uses the current task-wide flow, including
	// API/CLI clients that omit composer options. This only proposes a scope
	// and policy; on-demand discovery and explicit host consent still apply.
	if profile.Settings.Computer && !config.RestrictComputer && config.RequestedComputerScope == "" {
		config.RequestedComputerScope = "desktop"
		if config.RequestedComputerApprovalMode == "" {
			config.RequestedComputerApprovalMode = "full_task"
		}
	}
	config.MaxIterations = profile.Settings.MaxIterations
	if req.MaxIterations != 0 {
		config.MaxIterations = req.MaxIterations
	}
	if config.Instructions != "" {
		config.SystemPrompt += "\n\nAgent instructions (do not grant permissions):\n" + config.Instructions
	}
	for id, child := range effective.Children {
		childConfig := agents.DefaultAgentConfig()
		childConfig.Profile, childConfig.Instructions = child.Profile, child.Profile.Settings.Instructions
		childConfig.AllowedCapabilities, childConfig.MaxIterations = child.Capabilities, child.Profile.Settings.MaxIterations
		for name, descriptor := range childConfig.AllowedCapabilities {
			if config.AllowedCapabilities[name] != descriptor {
				delete(childConfig.AllowedCapabilities, name)
			}
		}
		selected := []string{}
		for _, id := range child.Profile.Settings.Knowledge {
			for _, parentID := range selectedProfileKnowledge(*config) {
				if id == parentID {
					selected = append(selected, id)
					break
				}
			}
		}
		childConfig.KnowledgeSelection = &selected
		if !s.attachProfileKnowledge(w, r, req.Prompt, &childConfig) {
			return false
		}
		if config.DelegateProfiles == nil {
			config.DelegateProfiles = map[string]agents.AgentConfig{}
		}
		config.DelegateProfiles[id] = childConfig
	}
	return s.attachProfileKnowledge(w, r, req.Prompt, config)
}

func (s *Server) attachProfileKnowledge(w http.ResponseWriter, r *http.Request, prompt string, config *agents.AgentConfig) bool {
	documents := selectedProfileKnowledge(*config)
	if len(documents) > 0 {
		if s.ragEngine == nil || !s.ragEngine.IsEnabled() {
			writeJobError(w, 503, "knowledge_unavailable", "Enable Knowledge before running this profile.", true)
			return false
		}
		opts := rag.DefaultSearchOptions()
		opts.DocumentFilter = append([]string(nil), documents...)
		opts.TopK = 5
		material, err := s.ragEngine.Search(r.Context(), prompt, opts)
		if err != nil {
			writeJobError(w, 503, "knowledge_retrieval_failed", "Knowledge retrieval failed. No task was started.", true)
			return false
		}
		if material == nil || len(material.Results) == 0 {
			writeJobError(w, 422, "knowledge_empty", "No supporting material was found in this profile's selected Knowledge.", false)
			return false
		}
		material.TruncateContext(6000)
		config.KnowledgeContext = material.Context
	}
	return true
}

func writeProfileError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agents.ErrProfileNotFound):
		writeJobError(w, 404, "profile_not_found", "Agent profile not found.", false)
	case errors.Is(err, agents.ErrProfileConflict):
		writeJobError(w, 409, "profile_revision_changed", "This profile changed or was archived. Reload it before saving.", false)
	case errors.Is(err, agents.ErrInvalidProfile):
		writeJobError(w, 400, "invalid_profile", "Provide a name, installed model, valid references and a budget of 1 to 50 iterations.", false)
	default:
		writeJobError(w, 503, "profile_storage_unavailable", "Profile storage is unavailable. Check saved state before retrying.", true)
	}
}

func (s *Server) handleAgentProfiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.agentManager == nil || s.workspaceID == "" {
		writeProfileError(w, agents.ErrRunStorage)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/agent-profiles")
	if path == "/resources" && r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]any{"tools": s.profileToolCatalog(r), "computer_permitted": s.profileAdmin(r), "connection_scope": "workspace-admin"})
		return
	}
	id, action := "", ""
	if path != "" && path != "/" {
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) > 2 || parts[0] == "" {
			writeProfileError(w, agents.ErrProfileNotFound)
			return
		}
		id = parts[0]
		if len(parts) == 2 {
			action = parts[1]
		}
	}
	actor := s.agentActor(r)
	if r.Method == http.MethodGet {
		if id == "" {
			profiles, err := s.agentManager.ListProfiles(s.workspaceID, actor, r.URL.Query().Get("archived") == "true")
			if err != nil {
				writeProfileError(w, err)
				return
			}
			writeJSON(w, 200, map[string]any{"profiles": profiles})
			return
		}
		if action != "" && action != "effective" {
			writeProfileError(w, agents.ErrProfileNotFound)
			return
		}
		revision := 0
		if raw := r.URL.Query().Get("revision"); raw != "" {
			var err error
			revision, err = strconv.Atoi(raw)
			if err != nil || revision < 1 {
				writeProfileError(w, agents.ErrInvalidProfile)
				return
			}
		}
		profile, err := s.agentManager.GetProfile(s.workspaceID, actor, id, revision)
		if err != nil {
			writeProfileError(w, err)
			return
		}
		if action == "effective" {
			writeJSON(w, 200, s.resolveProfile(r, profile))
		} else {
			writeJSON(w, 200, profile)
		}
		return
	}
	if !((id == "" && r.Method == http.MethodPost) || (id != "" && action == "" && r.Method == http.MethodPut) || (id != "" && action == "archive" && r.Method == http.MethodPost)) {
		writeJobError(w, 405, "method_not_allowed", "Use GET, POST to create/archive, or PUT to save a revision.", false)
		return
	}
	var req struct {
		ExpectedRevision int                    `json:"expected_revision"`
		Settings         agents.ProfileSettings `json:"settings"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
		writeProfileError(w, agents.ErrInvalidProfile)
		return
	}
	if action == "archive" {
		if err := s.agentManager.ArchiveProfile(s.workspaceID, actor, id, req.ExpectedRevision); err != nil {
			writeProfileError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"id": id, "archived": true})
		return
	}
	if id != "" {
		current, err := s.agentManager.GetProfile(s.workspaceID, actor, id, 0)
		if err != nil {
			writeProfileError(w, err)
			return
		}
		if current.Revision != req.ExpectedRevision || current.Archived {
			writeProfileError(w, agents.ErrProfileConflict)
			return
		}
	}
	proposed := &agents.AgentProfile{ID: id, Workspace: s.workspaceID, Actor: actor, Settings: req.Settings}
	effective := s.resolveProfile(r, proposed)
	if !effective.Ready {
		// A rejected draft has no persisted identity or revision. Do not expose
		// it as an immutable profile (revision zero violates that contract).
		writeJSON(w, 422, map[string]any{"ready": false, "issues": effective.Issues})
		return
	}
	profile, err := s.agentManager.SaveProfile(s.workspaceID, actor, id, req.ExpectedRevision, req.Settings)
	if err != nil {
		writeProfileError(w, err)
		return
	}
	status := 200
	if id == "" {
		status = 201
	}
	writeJSON(w, status, profile)
}
