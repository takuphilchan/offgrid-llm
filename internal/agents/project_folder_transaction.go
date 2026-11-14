package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func parseProjectSnapshot(observed execution.ProjectContextSnapshot) (*ParsedProjectFile, error) {
	if len(observed.Identity) != 64 || !validProfileText(observed.Name, 256, true) {
		return nil, ErrControlCommand
	}
	if _, err := hex.DecodeString(observed.Identity); err != nil {
		return nil, ErrControlCommand
	}
	if !observed.Present {
		if len(observed.Configuration) != 0 || observed.SHA256 != "" {
			return nil, ErrControlCommand
		}
		return nil, nil
	}
	parsed, err := ParseProjectFile(observed.Configuration)
	if err != nil || parsed.Digest != observed.SHA256 {
		return nil, ErrControlCommand
	}
	return parsed, nil
}

// The caller owns the transaction and validates the conversation boundary.
// No host operation, task admission, or authorization inference occurs here.
func writeProjectFolder(tx *sql.Tx, scope ControlScope, conversation *Conversation, ref execution.Reference, observed execution.ProjectContextSnapshot, parsed *ParsedProjectFile, kind, requestID string) (*ConversationCommandReceipt, error) {
	key, _ := json.Marshal([]string{"folder-project-v1", scope.Workspace, scope.Actor, observed.Identity})
	hash := sha256.Sum256(key)
	id := "project-" + hex.EncodeToString(hash[:])
	project := Project{ID: id, Revision: 1, Name: observed.Name, FolderIdentity: observed.Identity, ConfigurationSource: "file"}
	old, err := readControlRecord(tx, scope, id, "project")
	fresh := errors.Is(err, ErrControlNotFound)
	if err != nil && !fresh {
		return nil, err
	}
	if !fresh {
		if err = json.Unmarshal(old.Snapshot, &project); err != nil {
			return nil, err
		}
		if project.FolderIdentity != observed.Identity {
			return nil, ErrRunConflict
		}
	}
	digest := ""
	if parsed != nil {
		digest = parsed.Digest
	}
	if fresh || project.ConfigurationSource == "file" && project.ConfigurationDigest != digest {
		project.ConfigurationDigest = digest
		project.Preferences, project.Resources = ConversationPreferences{}, nil
		if parsed != nil {
			project.Preferences, project.Resources = parsed.Preferences, parsed.Resources
		}
		if !fresh {
			project.Revision++
		}
		if err = putControlRecord(tx, scope, id, "project", project.Revision, &project); err != nil {
			return nil, err
		}
		if _, err = appendControlEvent(tx, scope, id, project.Revision, "project_folder_configuration", &project, ""); err != nil {
			return nil, err
		}
	}
	var previous []byte
	err = tx.QueryRow(`SELECT reference FROM agent_conversation_folders WHERE workspace=? AND actor=? AND conversation_id=?`, scope.Workspace, scope.Actor, conversation.ID).Scan(&previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var previousRef execution.Reference
	same := err == nil && json.Unmarshal(previous, &previousRef) == nil && previousRef == ref && conversation.ProjectID == id
	if !same {
		conversation.ContextRevision++
	}
	conversation.ProjectID, conversation.Revision = id, conversation.Revision+1
	if err = putControlRecord(tx, scope, conversation.ID, "conversation", conversation.Revision, conversation); err != nil {
		return nil, err
	}
	reference, err := json.Marshal(ref)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO agent_conversation_folders(workspace,actor,conversation_id,project_id,reference) VALUES(?,?,?,?,?) ON CONFLICT(workspace,actor,conversation_id) DO UPDATE SET project_id=excluded.project_id,reference=excluded.reference`, scope.Workspace, scope.Actor, conversation.ID, id, reference); err != nil {
		return nil, err
	}
	cursor, err := appendControlEvent(tx, scope, conversation.ID, conversation.Revision, kind, map[string]any{"project_id": id, "project_revision": project.Revision}, "")
	if err != nil {
		return nil, err
	}
	return &ConversationCommandReceipt{RequestID: requestID, TargetID: conversation.ID, Revision: conversation.Revision, EventCursor: cursor, Conversation: conversation, Project: &project}, nil
}
