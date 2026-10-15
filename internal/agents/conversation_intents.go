package agents

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// ConversationIntent is the private admission outbox. It pins resolved inputs
// before either store is changed further; it is never returned to a client or
// supplied as tool metadata. Linked jobs become the execution/result authority.
type ConversationIntent struct {
	Workspace string             `json:"workspace"`
	Actor     string             `json:"actor"`
	RequestID string             `json:"request_id"`
	Digest    string             `json:"digest"`
	Origin    ConversationOrigin `json:"origin"`
	Prompt    string             `json:"prompt,omitempty"`
	Model     string             `json:"model,omitempty"`
	Config    AgentConfig        `json:"config,omitempty"`
	JobID     string             `json:"job_id,omitempty"`
	Detached  bool               `json:"detached,omitempty"`
}

func (m *Manager) conversationDatabase() (*sql.DB, error) {
	if m.dataDir == "" {
		return nil, ErrRunStorage
	}
	if err := m.StorageError(); err != nil {
		return nil, err
	}
	db, err := openTaskDatabase(m.dataDir)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS agent_conversation_intents (
workspace TEXT NOT NULL, actor TEXT NOT NULL, request_id TEXT NOT NULL,
digest TEXT NOT NULL, conversation TEXT NOT NULL, turn_id TEXT NOT NULL,
payload BLOB, job_id TEXT NOT NULL DEFAULT '', detached INTEGER NOT NULL DEFAULT 0,
PRIMARY KEY(workspace,actor,request_id));`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func scanConversationIntent(row interface{ Scan(...any) error }) (*ConversationIntent, error) {
	var i ConversationIntent
	var payload []byte
	if err := row.Scan(&i.Workspace, &i.Actor, &i.RequestID, &i.Digest, &i.Origin.Conversation, &i.Origin.TurnID, &payload, &i.JobID, &i.Detached); err != nil {
		return nil, err
	}
	i.Origin.Workspace = i.Workspace
	if len(payload) > 0 {
		var saved struct {
			Prompt string
			Model  string
			Config AgentConfig
		}
		if err := json.Unmarshal(payload, &saved); err != nil {
			return nil, err
		}
		i.Prompt, i.Model, i.Config = saved.Prompt, saved.Model, saved.Config
	}
	return &i, nil
}

const intentColumns = `workspace,actor,request_id,digest,conversation,turn_id,payload,job_id,detached`

func (m *Manager) ConversationIntent(workspace, actor, requestID string) (*ConversationIntent, error) {
	db, err := m.conversationDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	i, err := scanConversationIntent(db.QueryRow(`SELECT `+intentColumns+` FROM agent_conversation_intents WHERE workspace=? AND actor=? AND request_id=?`, workspace, actor, requestID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return i, err
}

func (m *Manager) SaveConversationIntent(i ConversationIntent) (*ConversationIntent, bool, error) {
	if !validProfileOwner(i.Workspace, i.Actor) || !i.Origin.valid() || i.Origin.Workspace != i.Workspace || i.RequestID == "" || i.Digest == "" || i.Prompt == "" || i.Model == "" || i.JobID != "" || i.Detached {
		return nil, false, ErrRunConflict
	}
	payload, err := json.Marshal(struct {
		Prompt string
		Model  string
		Config AgentConfig
	}{i.Prompt, i.Model, i.Config})
	if err != nil || len(payload) > 2<<20 {
		return nil, false, ErrRunConflict
	}
	db, err := m.conversationDatabase()
	if err != nil {
		return nil, false, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`INSERT INTO agent_conversation_intents(workspace,actor,request_id,digest,conversation,turn_id,payload) VALUES(?,?,?,?,?,?,?) ON CONFLICT(workspace,actor,request_id) DO NOTHING`, i.Workspace, i.Actor, i.RequestID, i.Digest, i.Origin.Conversation, i.Origin.TurnID, payload)
	if err != nil {
		return nil, false, err
	}
	saved, err := scanConversationIntent(tx.QueryRow(`SELECT `+intentColumns+` FROM agent_conversation_intents WHERE workspace=? AND actor=? AND request_id=?`, i.Workspace, i.Actor, i.RequestID))
	if err != nil {
		return nil, false, err
	}
	if saved.Digest != i.Digest || saved.Origin != i.Origin || saved.Detached {
		return nil, false, ErrRunConflict
	}
	count, _ := result.RowsAffected()
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return saved, count == 1, nil
}

// Bind clears the redundant private prompt/config only after the canonical job
// has durably saved them. A minimal ledger remains after conversation deletion.
func (m *Manager) BindConversationIntent(i *ConversationIntent, jobID string) error {
	if i == nil || jobID == "" {
		return ErrRunConflict
	}
	task, exists := m.GetTask(jobID)
	if !exists || task.Actor != i.Actor || task.RequestID != i.RequestID || task.RequestDigest != i.Digest || !sameConversationOrigin(task.Origin, &i.Origin) {
		return ErrRunConflict
	}
	db, err := m.conversationDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := db.Exec(`UPDATE agent_conversation_intents SET job_id=?,payload=NULL WHERE workspace=? AND actor=? AND request_id=? AND digest=? AND detached=0 AND (job_id='' OR job_id=?)`, jobID, i.Workspace, i.Actor, i.RequestID, i.Digest, jobID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return ErrRunConflict
	}
	return nil
}

func (m *Manager) DetachConversationIntents(workspace, actor, conversation string) error {
	db, err := m.conversationDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`UPDATE agent_conversation_intents SET detached=1,payload=NULL,conversation='' WHERE workspace=? AND actor=? AND conversation=?`, workspace, actor, conversation)
	return err
}

// Unbound intents are recovery bookkeeping, not permission to resume a job.
func (m *Manager) PendingConversationIntents(workspace string) ([]ConversationIntent, error) {
	db, err := m.conversationDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT `+intentColumns+` FROM agent_conversation_intents WHERE workspace=? AND detached=0`, workspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ConversationIntent
	for rows.Next() {
		i, err := scanConversationIntent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *i)
	}
	return result, rows.Err()
}
