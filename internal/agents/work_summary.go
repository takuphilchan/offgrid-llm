package agents

import "time"

// WorkSummary is detached metadata, not an executable task or an authority grant.
// It deliberately excludes results, instructions, tool arguments and transcripts.
type WorkSummary struct {
	ID                string              `json:"id"`
	Title             string              `json:"title"`
	Status            TaskStatus          `json:"status"`
	CreatedAt         time.Time           `json:"created_at"`
	Model             string              `json:"model,omitempty"`
	Origin            *ConversationOrigin `json:"origin,omitempty"`
	Outcome           string              `json:"outcome,omitempty"`
	Deletable         bool                `json:"deletable"`
	ArchivedAt        *time.Time          `json:"archived_at,omitempty"`
	Archivable        bool                `json:"archivable"`
	Protection        string              `json:"protection,omitempty"`
	Actor             string              `json:"-"`
	ProfilePresent    bool                `json:"-"`
	SourceIDs         []string            `json:"-"`
	UnscopedKnowledge bool                `json:"-"`
}

// ListWorkSummaries takes one lock for a consistent root-job projection. Unlike
// ListTasks it never JSON-copies a task's possibly very large execution history.
func (m *Manager) ListWorkSummaries() []WorkSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rows := make([]WorkSummary, 0, len(m.tasks))
	for _, task := range m.tasks {
		if task.DeletedAt != nil {
			continue
		}
		// A retained child remains subordinate even when its parent's history is
		// removed. Promoting it would make cleared work reappear as a new root.
		if parent := m.tasks[task.ParentID]; task.ParentID != "" && parent != nil {
			continue
		}
		title := []rune(task.Prompt)
		if len(title) > 180 {
			title = title[:180]
		}
		row := WorkSummary{ID: task.ID, Title: string(title), Status: task.Status, CreatedAt: task.CreatedAt, Model: task.Model,
			Actor: task.Actor, ProfilePresent: task.Config.Profile != nil, Outcome: TaskOutcome(task)}
		if task.Origin != nil {
			origin := *task.Origin
			row.Origin = &origin
		}
		row.SourceIDs = append([]string(nil), task.Config.KnowledgeDependencies...)
		for _, doc := range task.Config.KnowledgeDocuments {
			row.SourceIDs = append(row.SourceIDs, doc.ID)
		}
		for _, source := range task.Config.KnowledgeSources {
			row.SourceIDs = append(row.SourceIDs, source.DocumentID)
		}
		row.UnscopedKnowledge = task.Config.KnowledgeContext != "" && len(row.SourceIDs) == 0
		row.Deletable = CanDeleteTask(task)
		row.ArchivedAt, row.Archivable = task.ArchivedAt, CanArchiveTask(task)
		if !row.Deletable {
			switch {
			case task.Status == TaskUncertain:
				row.Protection = "unknown_effects"
			case task.PendingApproval != nil:
				row.Protection = "pending_approval"
			case task.Checkpoint != nil && task.Checkpoint.ExecutingCall != "":
				row.Protection = "unsettled_action"
			case task.Status == TaskInterrupted:
				row.Protection = "recovery_checkpoint"
			default:
				row.Protection = "active_work"
			}
		}
		for _, link := range task.ChildHistory {
			if child := m.tasks[link.ID]; child != nil && child.DeletedAt == nil {
				childRoot := *child
				childRoot.ParentID = ""
				if !CanArchiveTask(&childRoot) {
					row.Archivable = false
				}
				if !CanDeleteTask(child) {
					row.Deletable = false
					row.Protection = "child_work"
				}
			}
		}
		rows = append(rows, row)
	}
	return rows
}
