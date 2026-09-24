package server

import "github.com/takuphilchan/offgrid-llm/internal/agents"

// Both history views advertise the same deletion boundary. Runner.Delete
// rechecks it under its admission lock; list metadata is never authority.
func (s *Server) removableTask(task *agents.Task, actor string) bool {
	if !agents.CanDeleteTask(task) || (task.Actor != actor && !(task.Actor == "" && actor == "local-admin")) {
		return false
	}
	if task.ParentID != "" {
		if parent, ok := s.agentManager.GetTask(task.ParentID); ok && parent.DeletedAt == nil {
			return false
		}
	}
	for _, link := range task.ChildHistory {
		if child, ok := s.agentManager.GetTask(link.ID); ok && child.DeletedAt == nil && !agents.CanDeleteTask(child) {
			return false
		}
	}
	return true
}
