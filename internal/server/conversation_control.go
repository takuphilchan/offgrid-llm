package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) controlScope(r *http.Request) agents.ControlScope {
	return agents.ControlScope{Workspace: s.workspaceID, Actor: s.agentActor(r)}
}

func writeControlError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agents.ErrSourceAccess):
		writeJobError(w, 403, "conversation_context_unavailable", "Current actor or source access does not permit continuing this input. Its saved state remains available for inspection or Stop.", false)
	case errors.Is(err, sessions.ErrKnowledgeUnavailable), errors.Is(err, sessions.ErrInvalidSessionName), errors.Is(err, sessions.ErrSessionNotFound), errors.Is(err, sessions.ErrTurnConflict), errors.Is(err, sessions.ErrAccessDenied):
		writeSessionError(w, err)
	case errors.Is(err, agents.ErrControlNotFound), errors.Is(err, agents.ErrProfileNotFound):
		writeJobError(w, 404, "conversation_resource_not_found", "This conversation resource is unavailable.", false)
	case errors.Is(err, agents.ErrControlCommand):
		writeJobError(w, 400, "invalid_conversation_command", "Use a supported command, request identity and current revision.", false)
	case errors.Is(err, agents.ErrRunConflict):
		writeJobError(w, 409, "conversation_conflict", "The revision changed or this request identity already names different input. Refresh the snapshot before a new command.", false)
	default:
		writeJobReadError(w, err)
	}
}

func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.agentManager == nil {
		writeJobReadError(w, agents.ErrRunStorage)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/conversations")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	id := parts[0]
	scope := s.controlScope(r)
	if r.Method == http.MethodPost && (id == "" || len(parts) == 2 && parts[1] == "commands") {
		s.handleConversationCommand(w, r, id)
		return
	}
	if r.Method == http.MethodPost && id != "" && len(parts) == 2 && parts[1] == "inputs" {
		s.handleSubmitConversationInput(w, r, id)
		return
	}
	if r.Method == http.MethodPost && id != "" && len(parts) == 4 && parts[1] == "inputs" && parts[3] == "commands" {
		s.handleConversationInputControl(w, r, id, parts[2])
		return
	}
	if r.Method != http.MethodGet {
		writeJobError(w, 405, "method_not_allowed", "Use GET to observe or POST to submit a command.", false)
		return
	}
	if id == "" {
		page, err := s.agentManager.ControlRecords(scope, "conversation", r.URL.Query().Get("after"), 100)
		if err != nil {
			writeControlError(w, err)
			return
		}
		writeJSON(w, 200, page)
		return
	}
	c, err := s.agentManager.Conversation(scope, id)
	if err != nil {
		writeControlError(w, err)
		return
	}
	if len(parts) == 1 {
		c.PendingQuestion = s.pendingConversationQuestion(r, c)
		writeJSON(w, 200, c)
		return
	}
	if parts[1] == "inputs" && (len(parts) == 2 || len(parts) == 3) {
		s.handleConversationInputs(w, r, id, parts[2:])
		return
	}
	if len(parts) != 2 {
		writeControlError(w, agents.ErrControlNotFound)
		return
	}
	switch parts[1] {
	case "events":
		s.handleControlEvents(w, r, id, "conversation")
	case "configuration":
		e, err := s.effectiveConversationPreferences(r, c, "")
		if err != nil {
			writeControlError(w, err)
			return
		}
		writeJSON(w, 200, e)
	default:
		writeControlError(w, agents.ErrControlNotFound)
	}
}

func (s *Server) handleConversationCommand(w http.ResponseWriter, r *http.Request, id string) {
	var command agents.ConversationCommand
	if decodeControlEnvelope(w, r, &command) != nil || command.TargetID != id {
		writeControlError(w, agents.ErrControlCommand)
		return
	}
	scope := s.controlScope(r)
	previous, err := s.agentManager.ConversationCommandReceipt(scope, command)
	if err != nil {
		writeControlError(w, err)
		return
	}
	if previous != nil {
		writeJSON(w, 200, previous)
		return
	}
	if !s.validateConversationSelection(w, r, command) {
		return
	}
	if (command.Kind == "create_work" || command.Kind == "create_conversation" || command.Kind == "import_conversation") && (s.config == nil || !s.config.EnableConversationPreview) {
		writeJobError(w, 503, "conversation_admission_disabled", "New conversation admission is disabled. Existing work remains available through its saved identity.", false)
		return
	}
	var receipt *agents.ConversationCommandReceipt
	if command.Kind == "use_project_folder" {
		receipt, err = s.applyProjectFolder(w, r, command)
		if receipt == nil && err == nil {
			return
		}
	} else if command.Kind == "import_conversation" {
		receipt, err = s.importConversationCommand(r, command)
	} else if command.Kind == "create_work" {
		if s.agentRunner == nil || s.registry == nil {
			writeJobReadError(w, agents.ErrRunStorage)
			return
		}
		conversation, e := s.agentManager.Conversation(scope, id)
		if e != nil {
			writeControlError(w, e)
			return
		}
		config, model, ok := s.resolveConversationRun(w, r, conversation, command)
		if !ok {
			return
		}
		receipt, err = s.agentRunner.AdmitConversationWork(scope, command, model, config)
	} else {
		receipt, err = s.agentManager.ApplyConversationCommand(scope, command)
	}
	if err != nil {
		writeControlError(w, err)
		return
	}
	w.Header().Set("Location", "/api/v2/conversations/"+receipt.TargetID)
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) handleControlEvents(w http.ResponseWriter, r *http.Request, id, kind string) {
	after := int64(0)
	limit := 100
	var err error
	if raw := r.URL.Query().Get("after"); raw != "" {
		after, err = strconv.ParseInt(raw, 10, 64)
	}
	if raw := r.URL.Query().Get("limit"); err == nil && raw != "" {
		limit, err = strconv.Atoi(raw)
	}
	if err != nil {
		writeControlError(w, agents.ErrControlCommand)
		return
	}
	page, err := s.agentManager.ControlEvents(s.controlScope(r), id, kind, after, limit)
	if err != nil {
		writeControlError(w, err)
		return
	}
	if kind == "conversation" {
		for i := range page.Events {
			if !strings.HasPrefix(page.Events[i].Kind, "input_") {
				continue
			}
			var entry api.ConversationInboxEntry
			if json.Unmarshal(page.Events[i].Payload, &entry) != nil || entry.ID == "" || entry.ConversationID != id {
				writeControlError(w, agents.ErrRunStorage)
				return
			}
			page.Events[i].Payload, err = json.Marshal(s.publicConversationInput(r, entry))
			if err != nil {
				writeControlError(w, err)
				return
			}
		}
	}
	writeJSON(w, 200, page)
}

func (s *Server) handleControlProjects(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.agentManager == nil {
		writeJobReadError(w, agents.ErrRunStorage)
		return
	}
	if r.Method == http.MethodPost {
		s.handleProjectCommand(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJobError(w, 405, "method_not_allowed", "Use GET to inspect projects.", false)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v2/projects"), "/")
	if id == "" {
		page, err := s.agentManager.ControlRecords(s.controlScope(r), "project", r.URL.Query().Get("after"), 100)
		if err != nil {
			writeControlError(w, err)
			return
		}
		writeJSON(w, 200, page)
		return
	}
	p, err := s.agentManager.Project(s.controlScope(r), id)
	if raw := r.URL.Query().Get("revision"); raw != "" {
		revision, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil {
			writeControlError(w, agents.ErrControlCommand)
			return
		}
		p, err = s.agentManager.ProjectRevision(s.controlScope(r), id, revision)
	}
	if err != nil {
		writeControlError(w, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) handleControlWork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.agentManager == nil {
		writeJobReadError(w, agents.ErrRunStorage)
		return
	}
	if r.Method == http.MethodPost {
		s.handleWorkControlCommand(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJobError(w, 405, "method_not_allowed", "Use the run's existing control endpoint until work control is available.", false)
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v2/work-items"), "/"), "/")
	if parts[0] == "" {
		page, err := s.agentManager.ControlRecords(s.controlScope(r), "work", r.URL.Query().Get("after"), 100)
		if err != nil {
			writeControlError(w, err)
			return
		}
		writeJSON(w, 200, page)
		return
	}
	work, err := s.agentManager.WorkItem(s.controlScope(r), parts[0])
	if err != nil {
		writeControlError(w, err)
		return
	}
	task, found := s.agentManager.GetTask(work.RunID)
	if !found || task.Actor != s.agentActor(r) || task.DeletedAt != nil {
		writeControlError(w, agents.ErrControlNotFound)
		return
	}
	if s.rejectUnavailableTask(w, r, task) {
		return
	}
	if len(parts) == 1 {
		writeJSON(w, 200, map[string]any{"work": work, "run": taskResponse(task)})
		return
	}
	if len(parts) == 2 && parts[1] == "events" {
		s.handleControlEvents(w, r, work.ID, "work")
		return
	}
	if len(parts) >= 2 && parts[1] == "changes" {
		s.handleWorkChanges(w, r, work.ID, task, parts[2:])
		return
	}
	writeControlError(w, agents.ErrControlNotFound)
}
