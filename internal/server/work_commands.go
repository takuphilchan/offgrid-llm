package server

import (
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) handleWorkControlCommand(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v2/work-items"), "/"), "/")
	var command api.WorkCommand
	if len(parts) != 2 || parts[1] != "commands" || decodeControlEnvelope(w, r, &command) != nil || command.TargetID != parts[0] {
		writeControlError(w, agents.ErrControlCommand)
		return
	}
	if s.agentRunner == nil {
		writeControlError(w, agents.ErrRunStorage)
		return
	}
	work, err := s.agentManager.WorkItem(s.controlScope(r), command.TargetID)
	if err != nil {
		writeControlError(w, err)
		return
	}
	task, exists := s.agentManager.GetTask(work.RunID)
	if !exists || task.Actor != s.agentActor(r) {
		writeControlError(w, agents.ErrControlNotFound)
		return
	}
	// Stop and Pause remain available after source access is revoked. Their
	// receipt is redacted below; a control decision never restores content access.
	canRead := s.taskKnowledgeAllowed(task.Actor, task)
	previous, err := s.agentManager.WorkCommandReceipt(s.controlScope(r), command)
	if err != nil {
		writeControlError(w, err)
		return
	}
	if previous != nil {
		if !canRead {
			previous.Work = nil
		}
		writeJSON(w, http.StatusOK, previous)
		return
	}
	if !canRead && command.Kind != "stop" && command.Kind != "pause" {
		writeJobError(w, 403, "knowledge_unavailable", "A source used by this work is no longer available.", false)
		return
	}
	receipt, err := s.agentRunner.ControlWork(s.controlScope(r), command)
	if err != nil {
		writeControlError(w, err)
		return
	}
	if !canRead {
		receipt.Work = nil
	}
	writeJSON(w, http.StatusAccepted, receipt)
}
