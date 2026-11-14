package agents

import (
	"database/sql"
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// ExecutionProjectContext is prepared by the service from the approved host.
// It is never decoded from a renderer, model tool, or public request envelope.
type ExecutionProjectContext struct {
	Snapshot execution.ProjectContextSnapshot
}

type ExecutionProjectBinding struct {
	ConversationID  string `json:"conversation_id"`
	ProjectID       string `json:"project_id"`
	ContextRevision uint64 `json:"context_revision"`
}

func ExecutionProjectSelectionCurrent(task *Task, c *Conversation) bool {
	if task.Origin == nil || task.Origin.Conversation != c.ID {
		return false
	}
	revision, project := task.Config.ConversationContextRevision, task.Config.ProjectID
	if prior := task.Config.ExecutionProject; prior != nil {
		if prior.ConversationID != c.ID {
			return false
		}
		revision, project = prior.ContextRevision, prior.ProjectID
	}
	return c.ContextRevision == revision && c.ProjectID == project
}

func bindExecutionProject(tx *sql.Tx, task *Task, resource execution.Resource, project *ExecutionProjectContext) error {
	if task.Origin == nil || task.WorkID == "" || task.ConversationTurn != nil || task.ParentID != "" {
		return ErrControlCommand
	}
	scope := ControlScope{Workspace: task.Origin.Workspace, Actor: task.Actor}
	parsed, err := parseProjectSnapshot(project.Snapshot)
	if err != nil {
		return err
	}
	record, err := readControlRecord(tx, scope, task.Origin.Conversation, "conversation")
	if err != nil {
		return err
	}
	var c Conversation
	if err = json.Unmarshal(record.Snapshot, &c); err != nil {
		return err
	}
	workRecord, err := readControlRecord(tx, scope, task.WorkID, "work")
	if err != nil {
		return err
	}
	var work WorkItem
	if err = json.Unmarshal(workRecord.Snapshot, &work); err != nil {
		return err
	}
	if work.RunID != task.ID {
		return ErrRunConflict
	}
	// New messages can advance the conversation revision without changing its
	// folder choice. A newer explicit selection must never be overwritten by
	// an approval that was prepared for an earlier work context.
	if !ExecutionProjectSelectionCurrent(task, &c) {
		return nil
	}
	_, err = writeProjectFolder(tx, scope, &c, resource.Reference, project.Snapshot, parsed, "execution_project_selected", task.ResolvedInputID)
	if err == nil {
		task.Config.ExecutionProject = &ExecutionProjectBinding{ConversationID: c.ID, ProjectID: c.ProjectID, ContextRevision: c.ContextRevision}
	}
	return err
}
