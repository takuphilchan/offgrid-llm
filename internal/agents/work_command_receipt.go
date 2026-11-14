package agents

import "github.com/takuphilchan/offgrid-llm/pkg/api"

func (m *Manager) WorkCommandReceipt(scope ControlScope, c api.WorkCommand) (*ConversationCommandReceipt, error) {
	if err := validateWorkCommand(c); err != nil {
		return nil, err
	}
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return nil, ErrControlNotFound
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return recordedControlReceipt(db, scope, c.RequestID, workCommandDigest(c))
}
