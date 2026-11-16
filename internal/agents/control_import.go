package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// ConversationImport is prepared from an owned session under the session
// mutation lock. It is not a public payload and carries no live host grants.
type ConversationImport struct {
	LegacyName         string
	Source             []byte
	Preferences        ConversationPreferences
	ConfigurationKnown bool
	Messages           []json.RawMessage
	LegacyRunIDs       []string
	CreatedAt          time.Time
}

type ConversationImportManifest = api.ConversationImportManifest

func (m *Manager) ImportedConversation(scope ControlScope, name string) (*ConversationImportManifest, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return nil, ErrControlNotFound
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return readConversationImport(db, scope, name)
}

func readConversationImport(q controlReader, scope ControlScope, name string) (*ConversationImportManifest, error) {
	var data []byte
	err := q.QueryRow(`SELECT manifest FROM agent_conversation_imports WHERE workspace=? AND actor=? AND legacy_name=?`, scope.Workspace, scope.Actor, name).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var manifest ConversationImportManifest
	err = json.Unmarshal(data, &manifest)
	return &manifest, err
}

func (m *Manager) ImportConversation(scope ControlScope, input ConversationImport) (*ConversationImportManifest, error) {
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE agent_schema SET version=version WHERE singleton=1`); err != nil {
		return nil, err
	}
	manifest, err := importConversationTransaction(tx, scope, input)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return manifest, nil
}

func importConversationTransaction(tx *sql.Tx, scope ControlScope, input ConversationImport) (*ConversationImportManifest, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) || !validProfileText(input.LegacyName, 256, true) || len(input.Source) > 64<<20 || !json.Valid(input.Source) || validateConversationPreferences(input.Preferences) != nil {
		return nil, ErrControlCommand
	}
	hash := sha256.Sum256(input.Source)
	digest := hex.EncodeToString(hash[:])
	previous, err := readConversationImport(tx, scope, input.LegacyName)
	if err != nil {
		return nil, err
	}
	if previous != nil {
		if _, err := readControlRecord(tx, scope, previous.ConversationID, "conversation"); err != nil {
			return nil, err
		}
		if previous.SourceSHA256 != digest {
			return nil, ErrRunConflict
		}
		return previous, nil
	}
	project, err := defaultProject(tx, scope)
	if err != nil {
		return nil, err
	}
	c := &Conversation{ID: "conversation-" + runID(), Revision: 1, Title: input.LegacyName, ProjectID: project.ID, CreatedAt: input.CreatedAt, Preferences: input.Preferences, ConfigurationKnown: input.ConfigurationKnown}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	if err = validatePinnedPreference(tx, scope, c.Preferences); err != nil {
		return nil, err
	}
	if err = putControlRecord(tx, scope, c.ID, "conversation", 1, c); err != nil {
		return nil, err
	}
	for i, message := range input.Messages {
		if !json.Valid(message) {
			return nil, ErrControlCommand
		}
		if _, err = tx.Exec(`INSERT INTO agent_conversation_messages(workspace,actor,conversation_id,message_id,payload) VALUES(?,?,?,?,?)`, scope.Workspace, scope.Actor, c.ID, fmt.Sprintf("import-%d", i), []byte(message)); err != nil {
			return nil, err
		}
	}
	// Existing jobs retain their origin, receipts, grants and recovery contracts.
	// Do not rewrite active legacy runs into the new control model.
	for _, id := range input.LegacyRunIDs {
		var actor string
		if err = tx.QueryRow(`SELECT actor FROM agent_tasks WHERE id=?`, id).Scan(&actor); err != nil {
			return nil, err
		}
		if actor != scope.Actor {
			return nil, ErrControlNotFound
		}
	}
	manifest := &ConversationImportManifest{Version: 1, LegacyName: input.LegacyName, ConversationID: c.ID, SourceSHA256: digest, Authority: "agent_control_store", ConfigurationKnown: input.ConfigurationKnown, LegacyRunIDs: input.LegacyRunIDs, ImportedAt: time.Now().UTC()}
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO agent_conversation_imports(workspace,actor,legacy_name,conversation_id,source_digest,manifest,source) VALUES(?,?,?,?,?,?,?)`, scope.Workspace, scope.Actor, input.LegacyName, c.ID, digest, data, input.Source); err != nil {
		return nil, err
	}
	if _, err = appendControlEvent(tx, scope, c.ID, 1, "conversation_imported", manifest, ""); err != nil {
		return nil, err
	}
	return manifest, nil
}

func (m *Manager) ImportedConversationSource(scope ControlScope, id string) ([]byte, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return nil, ErrControlNotFound
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var data []byte
	var digest string
	err = db.QueryRow(`SELECT source,source_digest FROM agent_conversation_imports WHERE workspace=? AND actor=? AND conversation_id=? AND NOT EXISTS (SELECT 1 FROM agent_conversation_deletions d WHERE d.workspace=agent_conversation_imports.workspace AND d.actor=agent_conversation_imports.actor AND d.conversation_id=agent_conversation_imports.conversation_id)`, scope.Workspace, scope.Actor, id).Scan(&data, &digest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != digest {
		return nil, ErrRunStorage
	}
	return data, nil
}
