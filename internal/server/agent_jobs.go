package server

// Product job commands. HTTP only validates envelopes and maps errors; the
// existing runner owns task admission, interruption, persistence and execution.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

func (s *Server) handleJobSubmit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Request-ID", uuid.NewString())
	if r.Method == http.MethodGet {
		s.handleAgentTasks(w, r)
		return
	}
	if r.Method != http.MethodPost {
		writeJobError(w, 405, "method_not_allowed", "Submit a task with POST.", false)
		return
	}
	var req struct {
		Prompt        string `json:"prompt"`
		Model         string `json:"model"`
		RequestID     string `json:"request_id"`
		Style         string `json:"style,omitempty"`
		MaxIterations int    `json:"max_iterations,omitempty"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF || strings.TrimSpace(req.Prompt) == "" || len(req.Prompt) > 32000 || req.Model == "" || len(req.RequestID) < 8 || len(req.RequestID) > 128 || strings.ContainsAny(req.RequestID, "\x00\r\n") {
		writeJobError(w, 400, "invalid_task", "Provide a task, an installed model and a request ID.", false)
		return
	}
	if s.agentRunner == nil || s.registry == nil {
		writeJobReadError(w, agents.ErrRunStorage)
		return
	}
	if _, err := s.registry.GetModel(req.Model); err != nil {
		writeJobError(w, 422, "model_unavailable", "Choose an installed model in task settings.", false)
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	encoded, _ := json.Marshal(req)
	sum := sha256.Sum256(encoded)
	config := agents.DefaultAgentConfig()
	config.TaskFirst = true
	config.ContextWindow = s.effectiveContextWindow()
	if config.ContextWindow/4 < config.MaxTokens {
		config.MaxTokens = config.ContextWindow / 4
	}
	config.Temperature = 0
	config.MaxIterations = 30
	if req.MaxIterations != 0 {
		if req.MaxIterations < 1 || req.MaxIterations > 50 {
			writeJobError(w, 400, "invalid_task_budget", "Use 1 to 50 iterations.", false)
			return
		}
		config.MaxIterations = req.MaxIterations
	}
	if req.Style != "" {
		if req.Style != "react" && req.Style != "plan-execute" && req.Style != "cot" {
			writeJobError(w, 400, "invalid_task_style", "Unsupported task style.", false)
			return
		}
		config.ReasoningStyle = req.Style
	}
	config.SystemPrompt = taskFirstPrompt + serviceFilePrompt
	task, created, err := s.agentRunner.CreateRequest(req.Prompt, req.Model, s.agentActor(r), config, req.RequestID, hex.EncodeToString(sum[:]))
	if errors.Is(err, agents.ErrRunConflict) {
		writeJobError(w, 409, "request_conflict", "This request ID already identifies different work, or task admission is unavailable.", false)
		return
	}
	if err != nil {
		writeJobReadError(w, err)
		return
	}
	if created {
		accepted := task
		task, err = s.agentRunner.Continue(s.agentContext(r, true), task.ID, s.agentActor(r), "start", "", true)
		if err != nil {
			writeAgentRunError(w, accepted, err)
			return
		}
	}
	w.Header().Set("Location", "/api/v2/jobs/"+task.ID)
	writeJSON(w, http.StatusAccepted, taskResponse(task))
}

func (s *Server) handleJobInput(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJobError(w, 405, "method_not_allowed", "Resolve task access with POST.", false)
		return
	}
	var req struct {
		InputID string `json:"input_id"`
		Session string `json:"computer_session"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF || req.InputID == "" || req.Session == "" {
		writeJobError(w, 400, "invalid_task_input", "Select locally approved access for this task.", false)
		return
	}
	if s.agentRunner == nil || s.agentManager == nil || s.browserHub == nil {
		writeJobReadError(w, agents.ErrRunStorage)
		return
	}
	actor := s.agentActor(r)
	task, ok := s.agentManager.GetTask(id)
	if !ok || task.Actor != actor {
		writeJobReadError(w, agents.ErrTaskNotFound)
		return
	}
	if task.ResolvedInputID == req.InputID && task.Config.ComputerSession == req.Session {
		if task.Status == agents.TaskPending && !task.ComputerSessionExpired {
			var err error
			task, err = s.agentRunner.Continue(s.agentContext(r, true), id, actor, "start", "", true)
			if err != nil {
				writeAgentRunError(w, task, err)
				return
			}
		}
		writeJSON(w, 202, taskResponse(task))
		return
	}
	if task.Status != agents.TaskInput || task.PendingInput == nil || task.PendingInput.ID != req.InputID {
		writeJobError(w, 409, "task_input_changed", "This task no longer needs that access. Refresh its saved state.", false)
		return
	}
	driver, err := s.browserHub.Driver(actor, req.Session)
	if err != nil || ((driver == "browser") != (task.PendingInput.Mode == "browser")) {
		writeJobError(w, 409, "computer_scope_mismatch", "Connect the requested application or browser for this task.", false)
		return
	}
	mode, err := s.browserHub.ApprovalMode(actor, req.Session)
	if err != nil {
		writeJobError(w, 409, "computer_session_unavailable", "Local computer access has expired. Connect again to continue this saved task.", true)
		return
	}
	check := s.checkComputerModelDriver(r.Context(), task.Model, driver)
	if !check.Passed {
		writeJSON(w, 422, computerCheckError(check))
		return
	}
	if driver == "browser" && !s.computerVisionReady(task.Model) {
		// Same preflight as the compatibility route. A missing/failing vision
		// profile leaves structured controls available, never enables capture.
		_ = s.checkComputerVision(r.Context(), task.Model)
	}
	config := task.Config
	config.SystemPrompt = taskFirstPrompt + serviceFilePrompt
	config.ComputerSession = req.Session
	config.ComputerDriver = driver
	config.ComputerApprovalMode = string(mode)
	config.ComputerVerification = "page-evidence-v1"
	if nativeComputerDriver(driver) {
		config.ComputerVerification = "native-evidence-v2"
	}
	configureComputerTask(&config)
	// Reservation is idempotent only for this actor and saved task. A conflict
	// never steals a companion from another run or retries a dispatched action.
	if err = s.browserHub.ReserveTask(actor, req.Session, id); err != nil {
		writeJobError(w, 409, "computer_session_unavailable", "This computer access is assigned to another task or has expired.", false)
		return
	}
	task, err = s.agentRunner.ResolveComputerInput(id, actor, req.InputID, config)
	if err != nil {
		writeAgentRunError(w, task, err)
		return
	}
	task, err = s.agentRunner.Continue(s.agentContext(r, true), id, actor, "start", "", true)
	if err != nil {
		writeAgentRunError(w, task, err)
		return
	}
	writeJSON(w, 202, taskResponse(task))
}
