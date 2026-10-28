package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func conversationRequestID(workspace, id string) string {
	sum := sha256.Sum256([]byte(workspace + "\x00" + id))
	return "chat-" + hex.EncodeToString(sum[:])
}

func (s *Server) admissionStage(stage string) error {
	if s.sessionHandlers.admissionFault != nil {
		return s.sessionHandlers.admissionFault(stage)
	}
	return nil
}

func (s *Server) submitConversationJob(w http.ResponseWriter, r *http.Request, name string, req sessionGenerateRequest) {
	h := s.sessionHandlers
	if s.agentManager == nil || s.agentRunner == nil || s.registry == nil {
		writeJobReadError(w, agents.ErrRunStorage)
		return
	}
	if req.Agent.ID == "" || req.Agent.Revision < 1 || len(req.Content) > 32000 || len(req.RequestID) < 8 || len(req.RequestID) > 128 || !isSafeModelID(req.RequestID) {
		writeJobError(w, 400, "invalid_agent_turn", "Provide an agent revision and a stable request ID.", false)
		return
	}
	if req.ComputerApprovalMode != "" && req.ComputerApprovalMode != "full_task" && req.ComputerApprovalMode != "scoped_changes" && req.ComputerApprovalMode != "ask_every_time" {
		writeJobError(w, 400, "invalid_computer_policy", "Choose a supported approval policy.", false)
		return
	}
	if req.ComputerScope != "" && (req.ComputerScope != "desktop" || req.ComputerApprovalMode == "") {
		writeJobError(w, 400, "invalid_computer_scope", "Desktop access requires an explicit policy and local consent.", false)
		return
	}
	if req.Profile != "" && req.Profile != "interactive" && req.Profile != "extended" || req.MaxTokens < 0 || req.MaxTokens > 4096 {
		writeJobError(w, 400, "invalid_turn_options", "Choose a supported inference profile and output budget.", false)
		return
	}
	// The existing per-conversation lock serializes admission with generation and
	// deletion, but is released before inference. It never serializes all chats.
	unlock, err := h.lockSessionContext(r.Context(), name)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	defer unlock()
	scope := h.scoped(r)
	conversation, err := scope.Load(name)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	actor := s.agentActor(r)
	if h.requireAuth && conversation.OwnerID != actor {
		writeJobError(w, 403, "conversation_owner_required", "Run agent work in your own conversation.", false)
		return
	}
	encoded, _ := json.Marshal(struct {
		Conversation string
		Request      sessionGenerateRequest
	}{name, req})
	sum := sha256.Sum256(encoded)
	digest, requestID := hex.EncodeToString(sum[:]), conversationRequestID(s.workspaceID, req.RequestID)
	intent, err := s.agentManager.ConversationIntent(s.workspaceID, actor, requestID)
	if err != nil {
		writeJobReadError(w, err)
		return
	}
	if intent != nil && (intent.Detached || intent.Digest != digest || intent.Origin.Conversation != name) {
		writeJobCommandError(w, agents.ErrRunConflict)
		return
	}
	if intent == nil {
		if conversation.Turn.Active() {
			writeSessionError(w, sessions.ErrTurnConflict)
			return
		}
		job := jobSubmitRequest{ExecutionRequestVersion: req.ExecutionRequestVersion, Prompt: req.Content, Model: req.ModelID, RequestID: requestID, ProfileID: req.Agent.ID, ProfileRevision: req.Agent.Revision, Restrictions: req.Restrictions, ComputerScope: req.ComputerScope, ComputerApprovalMode: req.ComputerApprovalMode}
		job.Execution = req.Execution
		config, valid := s.resolveJobConfig(w, r, &job)
		if !valid {
			return
		}
		config.ContextWindow = s.chatContextWindowForModel(req.Profile, job.Model)
		if !config.TextOnlyInference && s.agentToolPolicy != nil {
			pin, err := s.agentToolPolicy.Select(r.Context(), job.Model, config.ContextWindow)
			if err != nil {
				writeJobError(w, 422, "agent_adapter_unavailable", "Native tool qualification could not be verified for this conversation context. No turn was started.", false)
				return
			}
			config.ToolAdapter = pin
		}
		config.ConversationProfile = req.Profile
		if config.ConversationProfile == "" {
			config.ConversationProfile = "interactive"
		}
		if config.ContextWindow/4 < config.MaxTokens {
			config.MaxTokens = config.ContextWindow / 4
		}
		if req.MaxTokens > 0 && req.MaxTokens < config.MaxTokens {
			config.MaxTokens = req.MaxTokens
		}
		if req.UseKnowledgeBase && len(selectedProfileKnowledge(config)) == 0 {
			writeJobError(w, 422, "profile_knowledge_empty", "Select authorized Knowledge documents in this agent profile.", false)
			return
		}
		// Bound older context independently of the current request; record
		// omissions explicitly and let actual runtime token accounting enforce
		// the final model window, including tools and the private envelope.
		bytes := 0
		start := len(conversation.Messages)
		for start > 0 && len(conversation.Messages)-start < 32 {
			message := conversation.Messages[start-1]
			if bytes+len(message.Content) > 24000 {
				break
			}
			bytes += len(message.Content)
			start--
		}
		config.ConversationOmitted = start
		for _, message := range conversation.Messages[start:] {
			if message.Role == "user" || message.Role == "assistant" {
				config.ConversationContext = append(config.ConversationContext, api.ChatMessage{Role: message.Role, Content: message.Content})
			}
		}
		intent, _, err = s.agentManager.SaveConversationIntent(agents.ConversationIntent{Workspace: s.workspaceID, Actor: actor, RequestID: requestID, Digest: digest, Origin: agents.ConversationOrigin{Workspace: s.workspaceID, Conversation: name, TurnID: req.RequestID}, Prompt: req.Content, Model: job.Model, Config: config})
		if err != nil {
			writeJobCommandError(w, err)
			return
		}
		if err := s.admissionStage("intent_saved"); err != nil {
			writeJobReadError(w, err)
			return
		}
	}
	task, err := s.finishConversationAdmission(scope, intent, false)
	if err != nil {
		writeJobCommandError(w, err)
		return
	}
	// Retrying the SAME admitted request may dispatch only never-started pending
	// work. Interrupted/uncertain/terminal work is observation-only here.
	if task.Status == agents.TaskPending && task.StartedAt == nil {
		accepted := task
		task, err = s.agentRunner.Continue(s.agentContext(r, true), task.ID, actor, "start", "", true)
		if err != nil {
			if !errors.Is(err, agents.ErrRunConflict) {
				writeAgentRunError(w, accepted, err)
				return
			}
			task, _ = s.agentManager.GetTask(accepted.ID)
		}
	}
	if task == nil {
		writeJobReadError(w, agents.ErrTaskNotFound)
		return
	}
	w.Header().Set("Location", "/api/v2/jobs/"+task.ID)
	latest, loadErr := scope.Load(name)
	if loadErr != nil {
		writeSessionError(w, loadErr)
		return
	}
	responseTurn := latest.Turn
	if responseTurn == nil || responseTurn.ID != intent.Origin.TurnID {
		responseTurn = &sessions.Turn{ID: intent.Origin.TurnID, Prompt: task.Prompt, Model: task.Model, Profile: task.Config.ConversationProfile, MaxTokens: task.Config.MaxTokens, Status: string(task.Status), Output: task.Result, Error: task.Error, JobID: task.ID, JobAdmission: "linked", Agent: &sessions.AgentSelection{ID: task.Config.Profile.ID, Revision: task.Config.Profile.Revision}}
	}
	writeJSON(w, 202, map[string]any{"turn": responseTurn, "job": taskResponse(task)})
}

func (s *Server) finishConversationAdmission(scope *sessions.ScopedManager, intent *agents.ConversationIntent, recovery bool) (*agents.Task, error) {
	if intent == nil || intent.Detached {
		return nil, agents.ErrRunConflict
	}
	task, found, err := s.agentManager.FindTaskRequest(intent.Actor, intent.RequestID, intent.Digest)
	if err != nil {
		return nil, err
	}
	prompt, model, config := intent.Prompt, intent.Model, intent.Config
	if found {
		if task.Origin == nil || *task.Origin != intent.Origin {
			return nil, agents.ErrRunConflict
		}
		prompt, model, config = task.Prompt, task.Model, task.Config
	}
	if config.Profile == nil {
		return nil, agents.ErrRunStorage
	}
	conversation, err := scope.Load(intent.Origin.Conversation)
	if err != nil {
		return nil, err
	}
	// A late retry of a historical turn may inspect its canonical job, but
	// must not overwrite the current turn or reattach a deleted association.
	if found && conversation.Turn != nil && conversation.Turn.ID != intent.Origin.TurnID {
		for _, link := range conversation.Jobs {
			if link.JobID == task.ID && link.TurnID == intent.Origin.TurnID {
				return task, nil
			}
		}
		return nil, agents.ErrRunConflict
	}
	turn := &sessions.Turn{ID: intent.Origin.TurnID, RequestHash: intent.Digest, Prompt: prompt, Model: model, Profile: config.ConversationProfile, MaxTokens: config.MaxTokens, Agent: &sessions.AgentSelection{ID: config.Profile.ID, Revision: config.Profile.Revision}, Knowledge: len(selectedProfileKnowledge(config)) > 0}
	if _, _, err := scope.BeginJobTurn(intent.Origin.Conversation, turn); err != nil {
		return nil, err
	}
	if err := s.admissionStage("turn_saved"); err != nil {
		return nil, err
	}
	if !found {
		task, _, err = s.agentRunner.CreateHeldRequest(prompt, model, intent.Actor, config, intent.RequestID, intent.Digest, intent.Origin)
		if err != nil {
			return nil, err
		}
		if recovery {
			task, err = s.agentRunner.Pause(task.ID, task.Actor, false)
			if err != nil {
				return nil, err
			}
		}
	}
	if err := s.admissionStage("job_saved"); err != nil {
		return nil, err
	}
	if err := s.agentManager.BindConversationIntent(intent, task.ID); err != nil {
		return nil, err
	}
	if err := s.admissionStage("intent_bound"); err != nil {
		return nil, err
	}
	if _, err := scope.LinkJobTurn(intent.Origin.Conversation, intent.Origin.TurnID, intent.Digest, task.ID); err != nil {
		return nil, err
	}
	if err := s.admissionStage("turn_linked"); err != nil {
		return nil, err
	}
	task, err = s.agentRunner.ReleaseConversationAdmission(task.ID, task.Actor, intent.Digest, intent.Origin)
	if err != nil {
		return nil, err
	}
	if err := s.admissionStage("job_released"); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Server) conversationScope(actor string) *sessions.ScopedManager {
	return s.sessionHandlers.manager.WithAccess(sessions.Access{UserID: actor, All: actor == "local-admin" && !s.sessionHandlers.requireAuth})
}

func (s *Server) projectConversationTask(snapshot *agents.Task) {
	if snapshot == nil || snapshot.Origin == nil || s.sessionHandlers == nil {
		return
	}
	// Observer callbacks can overlap; project the latest canonical snapshot.
	task, exists := s.agentManager.GetTask(snapshot.ID)
	if !exists || task.AdmissionHeld {
		return
	}
	if err := s.attachConversationTask(task); err != nil && !errors.Is(err, sessions.ErrTurnConflict) && !errors.Is(err, sessions.ErrSessionNotFound) {
		log.Printf("Conversation projection unavailable; canonical job retained: %v", err)
	}
}

func (s *Server) attachConversationTask(task *agents.Task) error {
	i := task.Origin
	if i == nil {
		return agents.ErrRunConflict
	}
	scope := s.conversationScope(task.Actor)
	if task.Status == agents.TaskCompleted {
		if err := s.admissionStage("before_final_write"); err != nil {
			return err
		}
		_, err := scope.CommitJobTurn(i.Conversation, i.TurnID, task.RequestDigest, task.ID, task.Model, task.Result)
		if err != nil {
			return err
		}
		return s.admissionStage("after_final_write")
	}
	phase, output := string(task.Status), task.Result
	if task.Progress != nil {
		phase, output = task.Progress.Phase, task.Progress.Preview
	}
	return scope.ProjectJobTurn(i.Conversation, i.TurnID, task.RequestDigest, task.ID, string(task.Status), phase, output, task.Error)
}

func (s *Server) recoverConversationAdmissions() error {
	intents, err := s.agentManager.PendingConversationIntents(s.workspaceID)
	if err != nil {
		return err
	}
	for _, intent := range intents {
		unlock := s.sessionHandlers.lockSession(intent.Origin.Conversation)
		task, err := s.finishConversationAdmission(s.conversationScope(intent.Actor), &intent, true)
		if err == nil {
			err = s.attachConversationTask(task)
		}
		unlock()
		if err != nil && !errors.Is(err, sessions.ErrTurnConflict) && !errors.Is(err, sessions.ErrSessionNotFound) {
			log.Printf("Conversation bookkeeping needs explicit recovery: %v", err)
		}
	}
	return nil
}

func (s *Server) handleConversationJobTurn(w http.ResponseWriter, r *http.Request, name, action string, conversation *sessions.Session) {
	if err := authorizeChatAccess(r.Context(), s.sessionHandlers.requireAuth, false); err != nil {
		writeServiceError(w, err)
		return
	}
	turn := conversation.Turn
	actor := s.agentActor(r)
	if action == "turn/recover" && r.Method == http.MethodPost {
		var request struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || request.ID != turn.ID {
			writeSessionError(w, sessions.ErrTurnConflict)
			return
		}
		unlock := s.sessionHandlers.lockSession(name)
		defer unlock()
		intent, err := s.agentManager.ConversationIntent(s.workspaceID, actor, conversationRequestID(s.workspaceID, turn.ID))
		if err != nil || intent == nil {
			writeJobReadError(w, agents.ErrTaskNotFound)
			return
		}
		task, err := s.finishConversationAdmission(s.sessionHandlers.scoped(r), intent, true)
		if err == nil {
			err = s.attachConversationTask(task)
		}
		if err != nil {
			writeJobCommandError(w, err)
			return
		}
		current, err := s.sessionHandlers.scoped(r).Load(name)
		if err != nil {
			writeSessionError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"turn": current.Turn, "job": taskResponse(task)})
		return
	}
	if turn.JobID == "" {
		if action == "turn" && r.Method == http.MethodGet {
			writeJSON(w, 200, map[string]any{"turn": turn, "recovery_required": true})
			return
		}
		writeJobError(w, 409, "conversation_link_pending", "Recover this turn's saved admission before using job controls.", true)
		return
	}
	task, exists := s.agentManager.GetTask(turn.JobID)
	if !exists || task.Actor != actor {
		writeJobReadError(w, agents.ErrTaskNotFound)
		return
	}
	if action == "turn" && r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]any{"turn": turn, "job": taskResponse(task), "recovery_required": task.Status == agents.TaskCompleted && turn.Status != "completed"})
		return
	}
	if action == "turn/events" && r.Method == http.MethodGet {
		if r.URL.Query().Get("id") != turn.ID {
			writeSessionError(w, sessions.ErrTurnConflict)
			return
		}
		copy := r.Clone(r.Context())
		copy.URL.Path = "/api/v2/jobs/" + task.ID + "/events"
		s.handleJobRead(w, copy)
		return
	}
	if action == "turn/cancel" && r.Method == http.MethodPost {
		var request struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || request.ID != turn.ID {
			writeSessionError(w, sessions.ErrTurnConflict)
			return
		}
		copy := r.Clone(r.Context())
		copy.Body = io.NopCloser(strings.NewReader(`{}`))
		s.handleJobCommand(w, copy, task.ID, "cancel")
		return
	}
	writeError(w, "Method not allowed", 405)
}

func (s *Server) deleteAgentConversation(w http.ResponseWriter, r *http.Request, name string) bool {
	h := s.sessionHandlers
	unlock, available := h.tryLockSession(name)
	if !available {
		writeSessionError(w, sessions.ErrTurnConflict)
		return true
	}
	defer unlock()
	conversation, err := h.scoped(r).Load(name)
	if err != nil {
		writeSessionError(w, err)
		return true
	}
	if len(conversation.Jobs) == 0 && (conversation.Turn == nil || conversation.Turn.JobAdmission == "") {
		return false
	}
	actor, choice := s.agentActor(r), r.URL.Query().Get("work")
	if h.requireAuth && conversation.OwnerID != actor {
		writeJobError(w, 403, "conversation_owner_required", "Only the owner can detach agent work.", false)
		return true
	}
	if choice != "" && choice != "retain" && choice != "stop" {
		writeError(w, "Choose retain or stop for linked work", 400)
		return true
	}
	if conversation.Turn != nil && conversation.Turn.JobAdmission == "intent" {
		writeJobError(w, 409, "conversation_link_pending", "Recover this accepted turn before deleting the conversation.", true)
		return true
	}
	for _, link := range conversation.Jobs {
		task, exists := s.agentManager.GetTask(link.JobID)
		if !exists {
			continue
		}
		if task.Actor != actor {
			writeJobReadError(w, agents.ErrTaskNotFound)
			return true
		}
		active := task.Status != agents.TaskCompleted && task.Status != agents.TaskFailed && task.Status != agents.TaskCancelled
		if active && choice == "" {
			writeJobError(w, 409, "conversation_has_work", "Stop active work or explicitly retain it in the work inspector before deleting.", false)
			return true
		}
		if choice == "stop" && active {
			task, err = s.agentRunner.Stop(task.ID, actor, "cancel", "")
			if err != nil {
				writeJobCommandError(w, err)
				return true
			}
			if s.browserHub != nil {
				s.browserHub.StopTask(actor, task.ID)
			}
		}
		if choice == "stop" && (s.agentRunner.WorkerActive(task.ID) || task.Status == agents.TaskUncertain) {
			writeJobError(w, 409, "task_stop_pending", "Work has not safely settled. Wait, or retain it in the inspector before deleting.", true)
			return true
		}
	}
	if err := s.agentManager.DetachConversationIntents(s.workspaceID, actor, name); err != nil {
		writeJobReadError(w, err)
		return true
	}
	if err := h.scoped(r).DeleteRetainingJobs(name); err != nil {
		writeSessionError(w, err)
		return true
	}
	writeJSON(w, 200, map[string]any{"success": true, "work_retained": true})
	return true
}
