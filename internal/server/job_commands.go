package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

func (s *Server) handleJobCommand(w http.ResponseWriter, r *http.Request, id, action string) {
	if action == "artifact" {
		s.downloadTaskArtifact(w, r, id)
		return
	}
	if s.agentRunner == nil || s.agentManager == nil {
		writeJobReadError(w, agents.ErrRunStorage)
		return
	}
	task, ok := s.agentManager.GetTask(id)
	legacyDelete := ok && action == "delete" && r.Method == http.MethodDelete && task.Actor == "" && s.agentActor(r) == "local-admin"
	if !ok || (task.Actor != s.agentActor(r) && !legacyDelete) {
		writeJobReadError(w, agents.ErrTaskNotFound)
		return
	}
	if action == "delete" && r.Method == http.MethodDelete {
		if err := s.agentRunner.Delete(id, s.agentActor(r)); err != nil {
			writeJobCommandError(w, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"success": true})
		return
	}
	if action == "export" && r.Method == http.MethodGet {
		w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.json"`)
		// Owner-scoped public evidence, not the private model checkpoint.
		value := taskResponse(task)
		value["prompt"] = task.Prompt
		value["format"] = "offgrid-task-evidence-v1"
		writeJSON(w, 200, value)
		return
	}
	if r.Method != http.MethodPost {
		writeJobError(w, 405, "method_not_allowed", "Use POST for task commands.", false)
		return
	}
	var req struct {
		ApprovalID  string `json:"approval_id"`
		CallID      string `json:"call_id"`
		Result      string `json:"result"`
		RequestID   string `json:"request_id"`
		Instruction string `json:"instruction"`
		Mode        string `json:"mode"`
		Target      string `json:"target"`
		URL         string `json:"url"`
		Async       bool   `json:"async"` // compatibility: execution is always detached here
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
		writeJobError(w, 400, "invalid_task_command", "Provide a valid task command.", false)
		return
	}
	var err error
	switch action {
	case "approve", "resume":
		task, err = s.agentRunner.Continue(s.agentContext(r, true), id, task.Actor, action, req.ApprovalID, true)
	case "pause", "takeover":
		if action == "takeover" && s.browserHub != nil {
			s.browserHub.StopSession(task.Actor, task.Config.ComputerSession)
		}
		task, err = s.agentRunner.Pause(id, task.Actor, action == "takeover")
	case "cancel", "deny":
		// Validate denial first; an incorrect approval ID must not revoke a session.
		task, err = s.agentRunner.Stop(id, task.Actor, action, req.ApprovalID)
		if err == nil && s.browserHub != nil {
			s.browserHub.StopSession(task.Actor, task.Config.ComputerSession)
		}
	case "steer":
		task, err = s.agentRunner.Steer(id, task.Actor, req.RequestID, req.Instruction)
	case "reconnect":
		mode, target, url := req.Mode, req.Target, req.URL
		if mode == "" && task.LastAccess != nil {
			mode, target, url = task.LastAccess.Mode, task.LastAccess.Target, task.LastAccess.URL
		}
		args, _ := json.Marshal(map[string]string{"mode": mode, "target": target, "url": url})
		var input *agents.InputRequired
		if !errors.As(computerAccessRequest(args), &input) {
			writeJobError(w, 400, "invalid_computer_access", "Choose an application or public HTTPS page.", false)
			return
		}
		task, err = s.agentRunner.RequestAccess(id, task.Actor, mode, target, url)
		if err == nil && task.PendingInput != nil && s.browserHub != nil {
			s.browserHub.StopSession(task.Actor, task.PendingInput.PreviousSession)
		}
	case "reconcile":
		if strings.TrimSpace(req.Result) == "" || len(req.Result) > 16000 {
			writeJobError(w, 400, "invalid_reconciliation", "Describe the observed outcome.", false)
			return
		}
		task, err = s.agentRunner.Reconcile(id, task.Actor, req.CallID, req.Result)
	default:
		writeJobError(w, 404, "unknown_task_command", "This task command is not available.", false)
		return
	}
	if err != nil {
		writeJobCommandError(w, err)
		return
	}
	writeJSON(w, 202, taskResponse(task))
}

func writeJobCommandError(w http.ResponseWriter, err error) {
	if errors.Is(err, agents.ErrRunConflict) {
		writeJobError(w, 409, "task_state_changed", "The task is still settling, its approval expired, or its state changed. Refresh this saved task before continuing.", true)
		return
	}
	writeJobReadError(w, err)
}
