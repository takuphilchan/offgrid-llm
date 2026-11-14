package agents

import (
	"database/sql"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Caller holds runner and manager locks and the control transaction. Snapshots
// remain detached until the whole incorporation transaction commits.
func (r *Runner) conversationInputGraph(tx *sql.Tx, scope ControlScope, entry *api.ConversationInboxEntry) ([]*Task, error) {
	return r.conversationInputGraphOrdered(tx, scope, entry, true)
}

func (r *Runner) conversationInputGraphOrdered(tx *sql.Tx, scope ControlScope, entry *api.ConversationInboxEntry, ordered bool) ([]*Task, error) {
	var batch []*Task
	if entry.WorkID != "" {
		work, err := linkedInputWork(tx, scope, entry.ConversationID, entry.WorkID)
		if err != nil {
			return nil, err
		}
		root := r.manager.tasks[work.RunID]
		if root == nil || root.Actor != scope.Actor || root.DeletedAt != nil || root.WorkID != entry.WorkID || root.Origin == nil || root.Origin.Workspace != scope.Workspace {
			return nil, ErrControlNotFound
		}
		if ordered && hasConversationHold(root) && root.ConversationHold.InputIDs[0] != entry.ID {
			return nil, ErrRunConflict
		}
		ids := []string{root.ID}
		for _, child := range root.ChildHistory {
			ids = append(ids, child.ID)
		}
		for _, id := range ids {
			current := r.manager.tasks[id]
			if current == nil || current.DeletedAt != nil || current.Actor != scope.Actor || id != root.ID && current.ParentID != root.ID {
				return nil, ErrRunStorage
			}
			if r.active[id] != nil || current.ControlPending || current.Checkpoint == nil || current.Checkpoint.ExecutingCall != "" {
				return nil, ErrRunConflict
			}
			next, err := copyTask(current)
			if err != nil {
				return nil, err
			}
			batch = append(batch, next)
		}
	}
	return batch, nil
}
