package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) agentActor(r *http.Request) string {
	if s.config != nil && s.config.RequireAuth {
		return users.GetUserID(r)
	}
	return "local-admin"
}

func (s *Server) callAgentModel(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
	release, err := s.acquireInference(ctx, task.Model)
	if err != nil {
		return nil, err
	}
	defer release()
	temperature := float32(task.Config.Temperature)
	maxTokens := task.Config.MaxTokens
	request := &api.ChatCompletionRequest{Model: task.Model, Messages: messages, Tools: tools, ToolChoice: "auto", Temperature: &temperature, MaxTokens: &maxTokens}
	configureComputerRequest(request, task)
	return s.engine.ChatCompletion(ctx, request)
}

func writeAgentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agents.ErrTaskNotFound):
		writeError(w, "Agent run not found", http.StatusNotFound)
	case errors.Is(err, agents.ErrRunConflict):
		writeError(w, "Run state changed or approval expired. Refresh the run before continuing.", http.StatusConflict)
	default:
		log.Printf("Agent execution: %v", err)
		writeError(w, "Agent execution or storage is unavailable. Inspect the run status and service logs before retrying.", http.StatusServiceUnavailable)
	}
}

// A persisted run may exist even when admission or execution failed. Include
// its identity so callers inspect it rather than creating duplicate work.
func writeAgentRunError(w http.ResponseWriter, task *agents.Task, err error) {
	if task == nil {
		writeAgentError(w, err)
		return
	}
	status := http.StatusServiceUnavailable
	if errors.Is(err, agents.ErrRunConflict) {
		status = http.StatusConflict
	}
	log.Printf("Agent run %s: %v", task.ID, err)
	response := taskResponse(task)
	if task.Error == "" {
		response["error"] = "Run could not continue. Inspect its saved state before retrying."
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

func taskResponse(task *agents.Task) map[string]any {
	steps := task.Steps
	if steps == nil {
		steps = []agents.Step{}
	}
	response := map[string]any{"task_id": task.ID, "run_id": task.ID, "status": task.Status, "output": task.Result, "steps": steps, "pending_approval": task.PendingApproval}
	response["progress"] = task.Progress
	if task.Config.ComputerSession != "" {
		response["computer_session"] = task.Config.ComputerSession
		response["computer_expected_text"] = task.Config.ComputerExpectedText
	}
	response["started_at"] = task.StartedAt
	response["resumable"] = task.Checkpoint != nil && task.Checkpoint.ExecutingCall == "" && (task.Status == agents.TaskInterrupted || task.Status == agents.TaskPending || task.Status == agents.TaskWaiting)
	if task.Error != "" {
		response["error"] = task.Error
	}
	if task.Checkpoint != nil && task.Checkpoint.ExecutingCall != "" {
		response["uncertain_call_id"] = task.Checkpoint.ExecutingCall
		if len(task.Checkpoint.Calls) > 0 {
			call := task.Checkpoint.Calls[0]
			response["uncertain_call"] = map[string]any{"tool": call.Function.Name, "arguments": json.RawMessage(call.Function.Arguments)}
		}
	}
	return response
}

func (s *Server) agentContext(r *http.Request, async bool) context.Context {
	if !async {
		return r.Context()
	}
	if s.runtimeCtx != nil {
		return s.runtimeCtx
	}
	return context.Background()
}

func (s *Server) handleAgentRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Prompt               string          `json:"prompt"`
		ComputerSession      string          `json:"computer_session"`
		ComputerExpectedText string          `json:"computer_expected_text"`
		Task                 string          `json:"task"`
		Model                string          `json:"model"`
		Style                string          `json:"style"`
		Stream               bool            `json:"stream"`
		Async                bool            `json:"async"`
		MaxIterations        int             `json:"max_iterations"`
		MaxSteps             int             `json:"max_steps"`
		SystemPrompt         string          `json:"system_prompt"`
		ApprovedTools        json.RawMessage `json:"approved_tools"`
		ApprovedToolCalls    json.RawMessage `json:"approved_tool_calls"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request", 400)
		return
	}
	if len(req.ApprovedTools) > 0 || len(req.ApprovedToolCalls) > 0 {
		writeError(w, "Preapproved tools are not accepted. Approve a pending call on its existing run ID.", 400)
		return
	}
	if req.Prompt == "" {
		req.Prompt = req.Task
	}
	if strings.TrimSpace(req.Prompt) == "" || req.Model == "" {
		writeError(w, "Prompt and model are required", 400)
		return
	}
	if s.agentRunner == nil {
		writeError(w, "Agent runtime unavailable", 503)
		return
	}
	if _, err := s.registry.GetModel(req.Model); err != nil {
		writeError(w, "Model not found", 404)
		return
	}
	config := agents.DefaultAgentConfig()
	if req.ComputerSession != "" {
		if len(req.ComputerExpectedText) > 1000 || (req.ComputerExpectedText != "" && strings.TrimSpace(req.ComputerExpectedText) == "") {
			writeError(w, "Optional expected page text must contain 1–1000 UTF-8 bytes, or be omitted for automatic page verification.", 400)
			return
		}
		if s.browserHub == nil || s.browserHub.Check(s.agentActor(r), req.ComputerSession, "") != nil {
			writeError(w, "Select an active, unused local browser session", 409)
			return
		}
		config.ComputerSession = req.ComputerSession
		config.ComputerExpectedText = req.ComputerExpectedText
		config.ComputerVerification = "page-evidence-v1"
	}
	config.SystemPrompt, config.ReasoningStyle = req.SystemPrompt, req.Style
	if config.ComputerSession != "" {
		configureComputerTask(&config)
	}
	if req.Style != "" && req.Style != "react" && req.Style != "plan-execute" && req.Style != "cot" {
		writeError(w, "Unsupported agent style", http.StatusBadRequest)
		return
	}
	if req.MaxIterations == 0 {
		req.MaxIterations = req.MaxSteps
	}
	if req.MaxIterations != 0 {
		config.MaxIterations = req.MaxIterations
	}
	if config.MaxIterations < 1 || config.MaxIterations > 50 {
		writeError(w, "max_iterations must be between 1 and 50", 400)
		return
	}
	if config.ComputerSession != "" {
		check := s.checkComputerModel(r.Context(), req.Model)
		if !check.Passed {
			status := http.StatusUnprocessableEntity
			if check.Retryable {
				status = http.StatusServiceUnavailable
			}
			writeJSON(w, status, computerCheckError(check))
			return
		}
		if s.browserHub.Check(s.agentActor(r), config.ComputerSession, "") != nil {
			writeError(w, "Browser session expired during the model check; pair again", 409)
			return
		}
	}
	task, err := s.agentRunner.Create(strings.TrimSpace(req.Prompt), req.Model, s.agentActor(r), config)
	if err != nil {
		writeAgentError(w, err)
		return
	}
	if config.ComputerSession != "" {
		if err := s.browserHub.Reserve(s.agentActor(r), config.ComputerSession, task.ID); err != nil {
			stopped, stopErr := s.agentRunner.Stop(task.ID, s.agentActor(r), "cancel", "")
			if stopErr != nil {
				writeAgentRunError(w, task, stopErr)
				return
			}
			writeAgentRunError(w, stopped, agents.ErrRunConflict)
			return
		}
	}
	async := req.Async || req.Stream
	createdID := task.ID
	task, err = s.agentRunner.Continue(s.agentContext(r, async), createdID, s.agentActor(r), "start", "", async)
	if err != nil {
		if task == nil {
			task, _ = s.agentManager.GetTask(createdID)
		}
		writeAgentRunError(w, task, err)
		return
	}
	if req.Stream {
		s.streamAgentTask(w, r, task.ID)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if async {
		w.WriteHeader(http.StatusAccepted)
	} else if task.Status == agents.TaskWaiting {
		w.WriteHeader(http.StatusConflict)
	}
	json.NewEncoder(w).Encode(taskResponse(task))
}

// All state changes refer to an existing run; no action accepts a new prompt or
// replacement tool arguments. Reloads and double clicks cannot replay old work.
func (s *Server) handleAgentTaskAction(w http.ResponseWriter, r *http.Request) {
	if s.agentRunner == nil {
		writeError(w, "Agent runtime unavailable", 503)
		return
	}
	if err := s.agentManager.StorageError(); err != nil {
		writeAgentError(w, err)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/agents/tasks/"), "/")
	if len(parts) < 1 || len(parts) > 2 || parts[0] == "" {
		writeError(w, "Not found", 404)
		return
	}
	id, actor := parts[0], s.agentActor(r)
	if len(parts) == 1 && r.Method == http.MethodDelete {
		if err := s.agentRunner.Delete(id, actor); err != nil {
			writeAgentError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	}
	if len(parts) == 2 && parts[1] == "events" && r.Method == http.MethodGet {
		task, ok := s.agentManager.GetTask(id)
		if !ok || task.Actor != actor {
			writeAgentError(w, agents.ErrTaskNotFound)
			return
		}
		s.streamAgentTask(w, r, id)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		task, ok := s.agentManager.GetTask(id)
		if !ok || (task.Actor != actor && task.Actor != "") {
			writeAgentError(w, agents.ErrTaskNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(taskResponse(task))
		return
	}
	if r.Method != http.MethodPost || len(parts) != 2 {
		writeError(w, "Method not allowed", 405)
		return
	}
	var req struct {
		ApprovalID string `json:"approval_id"`
		CallID     string `json:"call_id"`
		Result     string `json:"result"`
		Async      bool   `json:"async"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, "Invalid run action", 400)
		return
	}
	var task *agents.Task
	var err error
	switch parts[1] {
	case "approve", "resume":
		task, err = s.agentRunner.Continue(s.agentContext(r, req.Async), id, actor, parts[1], req.ApprovalID, req.Async)
	case "deny", "cancel":
		task, err = s.agentRunner.Stop(id, actor, parts[1], req.ApprovalID)
	case "reconcile":
		task, err = s.agentRunner.Reconcile(id, actor, req.CallID, req.Result)
	default:
		writeError(w, "Unknown run action", 404)
		return
	}
	if err != nil {
		writeAgentRunError(w, task, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if task.Status == agents.TaskRunning {
		w.WriteHeader(http.StatusAccepted)
	}
	json.NewEncoder(w).Encode(taskResponse(task))
}

func (s *Server) streamAgentTask(w http.ResponseWriter, r *http.Request, id string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, "Streaming unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	steps := 0
	lastPayload := ""
	lastSent := time.Now()
	controller := http.NewResponseController(w)
	defer controller.SetWriteDeadline(time.Time{})
	for {
		// Slow/disconnected viewers must not retain a blocked handler indefinitely.
		_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := s.agentManager.StorageError(); err != nil {
			fmt.Fprint(w, "data: {\"type\":\"error\",\"error\":\"Agent task storage unavailable; inspect service logs.\"}\n\n")
			flusher.Flush()
			return
		}
		task, ok := s.agentManager.GetTask(id)
		if !ok || task.Actor != s.agentActor(r) {
			return
		}
		data := taskResponse(task)
		data["type"] = "status"
		for ; steps < len(task.Steps); steps++ {
			step := task.Steps[steps]
			payload, _ := json.Marshal(map[string]any{"type": "step", "step_type": step.Type, "content": step.Content, "tool_name": step.ToolName, "tool_result": step.ToolResult})
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return
			}
		}
		switch task.Status {
		case agents.TaskCompleted:
			data["type"] = "done"
		case agents.TaskWaiting:
			data["type"] = "approval_required"
		case agents.TaskInterrupted, agents.TaskUncertain, agents.TaskFailed, agents.TaskCancelled:
			data["type"] = "error"
		}
		payload, _ := json.Marshal(data)
		if string(payload) != lastPayload {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return
			}
			lastPayload, lastSent = string(payload), time.Now()
			flusher.Flush()
		} else if time.Since(lastSent) >= 5*time.Second {
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			lastSent = time.Now()
			flusher.Flush()
		}
		if task.Status != agents.TaskRunning && task.Status != agents.TaskPending {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
