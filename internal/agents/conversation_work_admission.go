package agents

import "database/sql"

// Both explicit client commands and foreground decisions use this reducer.
// The caller owns the transaction and publishes the task only after commit.
func createConversationWork(tx *sql.Tx, scope ControlScope, conversation *Conversation, input *ConversationInput, task *Task) (*WorkItem, error) {
	if task == nil || task.Actor != scope.Actor || task.Origin == nil || task.Origin.Workspace != scope.Workspace || task.Origin.Conversation != conversation.ID || !task.AdmissionHeld {
		return nil, ErrControlCommand
	}
	if task.Status != TaskPending && (task.Status != TaskInput || task.PendingInput == nil) {
		return nil, ErrControlCommand
	}
	project, err := readControlRecord(tx, scope, conversation.ProjectID, "project")
	if err != nil {
		return nil, err
	}
	if task.Config.ProjectID != "" && (task.Config.ProjectID != project.ID || task.Config.ProjectRevision != project.Revision) {
		return nil, ErrRunConflict
	}
	task.Config.ProjectID, task.Config.ProjectRevision = project.ID, project.Revision
	policy := resolvedWorkerPolicy(task)
	if !validWorkerPolicy(&policy) {
		return nil, ErrControlCommand
	}
	task.Config.WorkerPolicy = &policy
	task.Config.ResultIdentityVersion = 1
	task.Config.ConversationContextRevision = conversation.ContextRevision
	if prepared := task.Config.EffectivePreferences; prepared != nil {
		user, err := readUserPreferences(tx, scope)
		if err != nil {
			return nil, err
		}
		if prepared.UserPreferencesRevision != user.Revision || prepared.Model != task.Model {
			return nil, ErrRunConflict
		}
	}
	work := &WorkItem{ID: "work-" + runID(), Revision: 1, ProjectID: conversation.ProjectID, Objective: input.Content, Acceptance: input.Acceptance, RunID: task.ID, Status: task.Status, InstructionVersion: 1, ProjectRevision: project.Revision}
	if work.Acceptance == nil {
		work.Acceptance = []string{}
	}
	work.RunIDs = []string{task.ID}
	work.WorkerPolicy = &policy
	task.WorkID, task.InstructionVersion = work.ID, 1
	task.AdmissionHeld, task.SchedulerReady = false, task.Status == TaskPending
	if err = putControlRecord(tx, scope, work.ID, "work", 1, work); err != nil {
		return nil, err
	}
	if err = putTaskTransaction(tx, task); err != nil {
		return nil, err
	}
	if _, err = appendControlEvent(tx, scope, work.ID, 1, "work_accepted", work, task.ID); err != nil {
		return nil, err
	}
	conversation.FocusedWorkID = work.ID
	return work, nil
}
