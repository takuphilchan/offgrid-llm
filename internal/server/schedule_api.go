package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

func scheduleView(item *agents.AgentSchedule) map[string]any {
	view := map[string]any{"version": item.Version, "id": item.ID, "revision": item.Revision, "digest": item.Digest, "settings": item.Settings, "next_at": item.NextAt, "created_at": item.CreatedAt, "model": item.Config.Profile.Settings.Model, "max_iterations": item.Config.MaxIterations, "profile": map[string]any{"id": item.Config.Profile.ID, "revision": item.Config.Profile.Revision, "digest": item.Config.Profile.Digest, "name": item.Config.Profile.Settings.Name}, "execution": item.Config.Execution, "execution_workspace": item.Config.ExecutionWorkspace}
	return view
}

func writeScheduleError(w http.ResponseWriter, err error) {
	if errors.Is(err, agents.ErrSchedule) {
		writeJobError(w, 400, "invalid_schedule", err.Error(), false)
		return
	}
	writeContextError(w, err)
}

func (s *Server) handleAgentSchedules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.agentManager == nil || s.agentRunner == nil || s.schedules == nil || s.workspaceID == "" {
		writeScheduleError(w, agents.ErrRunStorage)
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v2/agent-schedules"), "/"), "/")
	if len(parts) > 2 || parts[0] == "" && len(parts) != 1 {
		writeContextError(w, agents.ErrContextNotFound)
		return
	}
	id, action, actor := parts[0], "", s.agentActor(r)
	if len(parts) == 2 {
		action = parts[1]
	}
	if r.Method == http.MethodGet {
		if id == "" {
			items, err := s.agentManager.ListSchedules(s.workspaceID, actor)
			if err != nil {
				writeScheduleError(w, err)
				return
			}
			views := []map[string]any{}
			for _, item := range items {
				views = append(views, scheduleView(item))
			}
			writeJSON(w, 200, map[string]any{"schedules": views})
			return
		}
		if action == "occurrences" {
			items, err := s.agentManager.ListScheduleOccurrences(s.workspaceID, actor, id)
			if err != nil {
				writeScheduleError(w, err)
				return
			}
			writeJSON(w, 200, map[string]any{"occurrences": items})
			return
		}
		if action != "" {
			writeContextError(w, agents.ErrContextNotFound)
			return
		}
		revision := 0
		if raw := r.URL.Query().Get("revision"); raw != "" {
			var err error
			revision, err = strconv.Atoi(raw)
			if err != nil || revision < 1 {
				writeScheduleError(w, agents.ErrSchedule)
				return
			}
		}
		item, err := s.agentManager.GetSchedule(s.workspaceID, actor, id, revision)
		if err != nil {
			writeScheduleError(w, err)
			return
		}
		writeJSON(w, 200, scheduleView(item))
		return
	}
	if action == "disable" && id != "" && r.Method == http.MethodPost {
		var request struct {
			ExpectedRevision int `json:"expected_revision"`
		}
		if !contextBody(w, r, &request) {
			return
		}
		item, err := s.agentManager.DisableSchedule(s.workspaceID, actor, id, request.ExpectedRevision, s.schedules.clock())
		if err != nil {
			writeScheduleError(w, err)
			return
		}
		// The persisted revision already denies all new dispatch. Stop only roots
		// owned by this schedule, preserving the runner's uncertainty semantics.
		for _, task := range s.agentManager.ScheduledRootCandidates(s.workspaceID) {
			if task.Actor == actor && task.Config.Automation.Schedule == id {
				_, _ = s.agentRunner.Stop(task.ID, actor, "cancel", "")
			}
		}
		writeJSON(w, 200, scheduleView(item))
		return
	}
	if !(action == "" && (id == "" && r.Method == http.MethodPost || id != "" && r.Method == http.MethodPut)) {
		writeJobError(w, 405, "method_not_allowed", "Use GET, POST creation/disable or PUT a reviewed revision. Inspection never runs work.", false)
		return
	}
	var request scheduleWriteRequest
	if !contextBody(w, r, &request) {
		return
	}
	if (id == "") != (request.ExpectedRevision == 0) {
		writeScheduleError(w, agents.ErrSchedule)
		return
	}
	if id != "" {
		current, err := s.agentManager.GetSchedule(s.workspaceID, actor, id, 0)
		if err != nil {
			writeScheduleError(w, err)
			return
		}
		if current.Revision != request.ExpectedRevision {
			writeScheduleError(w, agents.ErrContextConflict)
			return
		}
	}
	config, valid := s.resolveScheduleConfiguration(w, r, request)
	if !valid {
		return
	}
	item, err := s.agentManager.SaveScheduleWithEpoch(s.workspaceID, actor, id, request.ExpectedRevision, request.Settings, config, s.schedules.clock(), s.schedules.epoch)
	if err != nil {
		writeScheduleError(w, err)
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	writeJSON(w, status, scheduleView(item))
}
