package agents

// ScheduleRootForTask avoids copying a large parent transcript on every child
// decision. Returned configuration is an immutable authority snapshot, not a
// mutable task handle; provider callers must not modify it.
func (m *Manager) ScheduleRootForTask(task *Task) (*Task, error) {
	if task == nil {
		return nil, ErrTaskNotFound
	}
	if task.Config.Automation != nil {
		return task, nil
	}
	if task.ParentID == "" {
		return nil, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	parent := m.tasks[task.ParentID]
	if parent == nil || parent.Actor != task.Actor || parent.DeletedAt != nil {
		return nil, ErrTaskNotFound
	}
	if parent.Config.Automation == nil {
		return nil, nil
	}
	return &Task{ID: parent.ID, Actor: parent.Actor, Model: parent.Model, Status: parent.Status, RequestID: parent.RequestID, RequestDigest: parent.RequestDigest, Config: parent.Config}, nil
}

// ScheduledRootCandidates copies only authority identifiers for live roots.
// Idle clock polls must not serialize transcripts, tool output or archives.
func (m *Manager) ScheduledRootCandidates(workspace string) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := []*Task{}
	for _, task := range m.tasks {
		if task.DeletedAt != nil || task.ParentID != "" || task.Config.Automation == nil || task.Config.Profile == nil || task.Config.Profile.Workspace != workspace || task.Status == TaskCompleted || task.Status == TaskFailed || task.Status == TaskCancelled {
			continue
		}
		pin := *task.Config.Automation
		profile := task.Config.Profile
		items = append(items, &Task{ID: task.ID, Actor: task.Actor, Status: task.Status, RequestID: task.RequestID, RequestDigest: task.RequestDigest, Config: AgentConfig{Automation: &pin, Profile: &AgentProfile{ID: profile.ID, Workspace: profile.Workspace, Actor: profile.Actor, Revision: profile.Revision, Digest: profile.Digest}}})
	}
	return items
}

// QueueScheduledRoot marks only an admitted, never-started canonical root. The
// normal coordinator retries admission under its existing concurrency ceiling;
// restarted tasks are interrupted by storage recovery, never auto-replayed.
func (m *Manager) QueueScheduledRoot(id, actor string) error {
	_, err := m.updateTask(id, func(task *Task) error {
		if task.Actor != actor || task.Config.Automation == nil || task.Status != TaskPending || task.StartedAt != nil || task.Checkpoint == nil || task.Checkpoint.Iteration != 0 || task.Checkpoint.ExecutingCall != "" {
			return ErrRunConflict
		}
		task.SchedulerReady = true
		return nil
	})
	return err
}
