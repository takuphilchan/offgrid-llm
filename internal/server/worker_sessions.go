package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) handleWorkerSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.agentManager == nil {
		writeControlError(w, agents.ErrRunStorage)
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v2/workers"), "/"), "/")
	scope := s.controlScope(r)
	if parts[0] == "" && r.Method == http.MethodGet {
		page, err := s.agentManager.ControlRecords(scope, "worker", r.URL.Query().Get("after"), 100)
		if err != nil {
			writeControlError(w, err)
			return
		}
		visible := []json.RawMessage{}
		for _, data := range page.Items {
			var worker api.WorkerSession
			if json.Unmarshal(data, &worker) != nil {
				writeControlError(w, agents.ErrRunStorage)
				return
			}
			task, exists := s.agentManager.GetTask(worker.CurrentRunID)
			if exists && task.Actor == scope.Actor && task.DeletedAt == nil && s.taskKnowledgeAllowed(scope.Actor, task) {
				visible = append(visible, data)
			}
		}
		page.Items = visible
		writeJSON(w, 200, page)
		return
	}
	worker, err := s.agentManager.WorkerSession(scope, parts[0])
	if err != nil {
		writeControlError(w, err)
		return
	}
	task, exists := s.agentManager.GetTask(worker.CurrentRunID)
	if !exists || task.Actor != scope.Actor || task.DeletedAt != nil {
		writeControlError(w, agents.ErrControlNotFound)
		return
	}
	canRead := s.taskKnowledgeAllowed(scope.Actor, task)
	if r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "commands" {
		s.handleWorkerCommand(w, r, worker, canRead)
		return
	}
	if r.Method != http.MethodGet {
		writeJobError(w, 405, "method_not_allowed", "Use GET to inspect or POST for a worker command.", false)
		return
	}
	if !canRead {
		writeJobError(w, 403, "knowledge_unavailable", "A source used by this worker is no longer available.", false)
		return
	}
	if len(parts) == 1 {
		writeJSON(w, 200, worker)
		return
	}
	if len(parts) != 2 {
		writeControlError(w, agents.ErrControlNotFound)
		return
	}
	switch parts[1] {
	case "events":
		s.handleControlEvents(w, r, worker.ID, "worker")
	case "messages":
		after, limit := int64(0), 100
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
		page, err := s.agentManager.WorkerMessages(scope, worker.ID, after, limit)
		if err != nil {
			writeControlError(w, err)
			return
		}
		writeJSON(w, 200, page)
	default:
		writeControlError(w, agents.ErrControlNotFound)
	}
}

func (s *Server) handleWorkerCommand(w http.ResponseWriter, r *http.Request, worker *api.WorkerSession, canRead bool) {
	var command api.WorkerCommand
	if decodeControlEnvelope(w, r, &command) != nil || command.TargetID != worker.ID {
		writeControlError(w, agents.ErrControlCommand)
		return
	}
	if s.agentRunner == nil {
		writeControlError(w, agents.ErrRunStorage)
		return
	}
	previous, err := s.agentManager.WorkerCommandReceipt(s.controlScope(r), command)
	if err != nil {
		writeControlError(w, err)
		return
	}
	if previous != nil {
		if !canRead {
			previous.Worker, previous.Message = nil, nil
		}
		writeJSON(w, 200, previous)
		return
	}
	if !canRead && command.Kind != "stop" && command.Kind != "close" {
		writeJobError(w, 403, "knowledge_unavailable", "A source used by this worker is no longer available.", false)
		return
	}
	receipt, err := s.agentRunner.CommandWorker(s.controlScope(r), command)
	if err != nil {
		writeControlError(w, err)
		return
	}
	if !canRead {
		receipt.Worker, receipt.Message = nil, nil
	}
	writeJSON(w, 202, receipt)
}
