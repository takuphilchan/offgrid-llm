package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func commandDigest(c ConversationCommand) string {
	data, _ := json.Marshal(c)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validateConversationCommand(c ConversationCommand) error {
	if !validProfileText(c.RequestID, 128, true) || strings.ContainsAny(c.RequestID, "\r\n") || c.ExpectedRevision < 0 {
		return ErrControlCommand
	}
	p := c.Payload
	if c.Kind != "use_project_folder" && p.Folder != nil {
		return ErrControlCommand
	}
	if c.Kind != "link_work" && c.Kind != "focus_work" && p.WorkID != "" {
		return ErrControlCommand
	}
	if c.Kind != "import_conversation" && p.LegacyName != "" {
		return ErrControlCommand
	}
	switch c.Kind {
	case "clear_project_folder":
		if p.ProjectID != "" || p.Title != "" || p.Preferences != nil || p.Input != nil {
			return ErrControlCommand
		}
	case "use_project_folder":
		if p.Folder == nil || !validProfileText(p.Folder.Host, 256, true) || !validProfileText(p.Folder.Session, 256, true) || !validProfileText(p.Folder.Grant, 256, true) || p.Folder.Revision < 1 || p.ProjectID != "" || p.Title != "" || p.Preferences != nil || p.Input != nil {
			return ErrControlCommand
		}
	case "select_project":
		if !validProfileText(p.ProjectID, 128, true) || p.Title != "" || p.Preferences != nil || p.Input != nil {
			return ErrControlCommand
		}
	case "link_work", "focus_work":
		if !validProfileText(p.WorkID, 128, c.Kind == "link_work") || p.ProjectID != "" || p.Title != "" || p.Preferences != nil || p.Input != nil {
			return ErrControlCommand
		}
	case "import_conversation":
		if c.TargetID != "" || c.ExpectedRevision != 0 || !validProfileText(p.LegacyName, 256, true) || p.Title != "" || p.ProjectID != "" || p.Preferences != nil || p.Input != nil {
			return ErrControlCommand
		}
	case "create_conversation":
		if c.TargetID != "" || c.ExpectedRevision != 0 || p.Input != nil || !validProfileText(p.Title, 256, false) {
			return ErrControlCommand
		}
	case "configure_conversation":
		if p.Preferences == nil || p.Title != "" || p.ProjectID != "" || p.Input != nil {
			return ErrControlCommand
		}
	case "rename_conversation":
		if !validProfileText(p.Title, 256, true) || p.Preferences != nil || p.ProjectID != "" || p.Input != nil {
			return ErrControlCommand
		}
	case "create_work":
		if p.Input == nil || !validProfileText(p.Input.Content, 32000, true) || !validProfileText(p.Input.Model, 256, false) || len(p.Input.Acceptance) > 32 || p.Title != "" || p.ProjectID != "" || p.Preferences != nil {
			return ErrControlCommand
		}
		for _, condition := range p.Input.Acceptance {
			if !validProfileText(condition, 4096, true) {
				return ErrControlCommand
			}
		}
	default:
		return ErrControlCommand
	}
	if c.Kind != "create_conversation" && c.Kind != "import_conversation" && (!validProfileText(c.TargetID, 128, true) || c.ExpectedRevision < 1) {
		return ErrControlCommand
	}
	if p.Preferences != nil {
		return validateConversationPreferences(*p.Preferences)
	}
	return nil
}

func validateConversationPreferences(p ConversationPreferences) error {
	if !validProfileText(p.Model, 256, false) || (p.ProfileID == "") != (p.ProfileRevision == 0) || p.ProfileRevision < 0 {
		return ErrControlCommand
	}
	r := p.Restrictions
	if r.Tools != nil {
		if len(*r.Tools) > 128 {
			return ErrControlCommand
		}
		for _, ref := range *r.Tools {
			if !validProfileText(ref.Provider, 256, true) || !validProfileText(ref.Tool, 256, true) || !validProfileText(ref.SchemaRevision, 256, true) || !validProfileText(ref.Location, 256, true) {
				return ErrControlCommand
			}
		}
	}
	if r.Knowledge != nil {
		if len(*r.Knowledge) > 32 {
			return ErrControlCommand
		}
		for _, id := range *r.Knowledge {
			if !validProfileText(id, 256, true) {
				return ErrControlCommand
			}
		}
	}
	return nil
}

func recordedControlCommand(q controlReader, scope ControlScope, c ConversationCommand) (*ConversationCommandReceipt, error) {
	return recordedControlReceipt(q, scope, c.RequestID, commandDigest(c))
}

func recordedControlReceipt(q controlReader, scope ControlScope, requestID, expectedDigest string) (*ConversationCommandReceipt, error) {
	var digest string
	var data []byte
	err := q.QueryRow(`SELECT digest,receipt FROM agent_control_commands WHERE workspace=? AND actor=? AND request_id=?`, scope.Workspace, scope.Actor, requestID).Scan(&digest, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if digest != expectedDigest {
		return nil, ErrRunConflict
	}
	var receipt ConversationCommandReceipt
	err = json.Unmarshal(data, &receipt)
	if err == nil && receipt.Conversation != nil {
		_, err = readControlRecord(q, scope, receipt.Conversation.ID, "conversation")
	}
	return &receipt, err
}

// ConversationCommandReceipt resolves retries before consulting mutable resource
// availability. It is observation only and never drains the dispatch outbox.
func (m *Manager) ConversationCommandReceipt(scope ControlScope, c ConversationCommand) (*ConversationCommandReceipt, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return nil, ErrControlNotFound
	}
	if err := validateConversationCommand(c); err != nil {
		return nil, err
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return recordedControlCommand(db, scope, c)
}

func defaultProject(tx *sql.Tx, scope ControlScope) (*Project, error) {
	// The identifier is stable within this actor/workspace, not an access grant.
	const id = "project-default"
	r, err := readControlRecord(tx, scope, id, "project")
	if err == nil {
		var p Project
		err = json.Unmarshal(r.Snapshot, &p)
		return &p, err
	}
	if !errors.Is(err, ErrControlNotFound) {
		return nil, err
	}
	p := &Project{ID: id, Revision: 1, Name: "Private project", Default: true}
	if err = putControlRecord(tx, scope, id, "project", 1, p); err != nil {
		return nil, err
	}
	_, err = appendControlEvent(tx, scope, id, 1, "project_created", p, "")
	return p, err
}

// ApplyConversationCommand commits configuration commands. Executing input is
// admitted by Runner.AdmitConversationWork so the held run is in the SAME commit.
func (m *Manager) ApplyConversationCommand(scope ControlScope, c ConversationCommand) (*ConversationCommandReceipt, error) {
	if c.Kind == "create_work" || c.Kind == "import_conversation" || c.Kind == "use_project_folder" {
		return nil, ErrControlCommand
	}
	return m.commitConversationCommand(scope, c, nil)
}

func (m *Manager) commitConversationCommand(scope ControlScope, c ConversationCommand, task *Task) (*ConversationCommandReceipt, error) {
	if err := validateConversationCommand(c); err != nil {
		return nil, err
	}
	return m.withControlCommand(scope, c.RequestID, commandDigest(c), func(tx *sql.Tx) (*ConversationCommandReceipt, error) {
		var err error
		conversation := &Conversation{}
		if c.Kind == "create_conversation" {
			projectID := c.Payload.ProjectID
			if projectID == "" {
				p, e := defaultProject(tx, scope)
				if e != nil {
					return nil, e
				}
				projectID = p.ID
			}
			if _, err = readControlRecord(tx, scope, projectID, "project"); err != nil {
				return nil, err
			}
			conversation = &Conversation{ConfigurationKnown: true, ID: "conversation-" + runID(), Revision: 1, ProjectID: projectID, Title: c.Payload.Title, CreatedAt: time.Now().UTC()}
		} else {
			r, err := readControlRecord(tx, scope, c.TargetID, "conversation")
			if err != nil {
				return nil, err
			}
			if r.Revision != c.ExpectedRevision {
				return nil, ErrRunConflict
			}
			if err = json.Unmarshal(r.Snapshot, conversation); err != nil {
				return nil, err
			}
			conversation.Revision++
		}
		if c.Payload.Preferences != nil {
			previous := conversation.Preferences.Restrictions
			conversation.Preferences = *c.Payload.Preferences
			conversation.Preferences.Restrictions = preserveConversationRestrictions(previous, conversation.Preferences.Restrictions)
		}
		if c.Kind == "rename_conversation" {
			conversation.Title = c.Payload.Title
		}
		if err = applyConversationSelection(tx, scope, c, conversation); err != nil {
			return nil, err
		}
		if err = validatePinnedPreference(tx, scope, conversation.Preferences); err != nil {
			return nil, err
		}
		receipt := &ConversationCommandReceipt{RequestID: c.RequestID, TargetID: conversation.ID, Revision: conversation.Revision, Conversation: conversation}
		if c.Kind == "create_work" {
			receipt.Work, err = createConversationWork(tx, scope, conversation, c.Payload.Input, task)
			if err != nil {
				return nil, err
			}
		}
		if err = putControlRecord(tx, scope, conversation.ID, "conversation", conversation.Revision, conversation); err != nil {
			return nil, err
		}
		if receipt.Work != nil {
			if _, err = tx.Exec(`INSERT INTO agent_control_links(workspace,actor,conversation_id,work_id) VALUES(?,?,?,?)`, scope.Workspace, scope.Actor, conversation.ID, receipt.Work.ID); err != nil {
				return nil, err
			}
			input, err := json.Marshal(map[string]any{"id": c.RequestID, "role": "user", "content": c.Payload.Input.Content, "work_id": receipt.Work.ID, "instruction_version": 1})
			if err != nil {
				return nil, err
			}
			if _, err = tx.Exec(`INSERT INTO agent_conversation_messages(workspace,actor,conversation_id,message_id,payload) VALUES(?,?,?,?,?)`, scope.Workspace, scope.Actor, conversation.ID, c.RequestID, input); err != nil {
				return nil, err
			}
		}
		// The accepted input is retained in the event, alongside the pinned run.
		receipt.EventCursor, err = appendControlEvent(tx, scope, conversation.ID, conversation.Revision, c.Kind, c.Payload, "")
		if err != nil {
			return nil, err
		}
		return receipt, nil
	})
}

func validatePinnedPreference(tx *sql.Tx, scope ControlScope, p ConversationPreferences) error {
	if p.ProfileID == "" {
		return nil
	}
	var data []byte
	err := tx.QueryRow(`SELECT r.snapshot FROM agent_profile_revisions r JOIN agent_profiles p ON p.id=r.profile_id WHERE p.workspace=? AND p.actor=? AND p.id=? AND r.revision=?`, scope.Workspace, scope.Actor, p.ProfileID, p.ProfileRevision).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrProfileNotFound
	}
	if err != nil {
		return err
	}
	var profile AgentProfile
	if err = json.Unmarshal(data, &profile); err != nil {
		return err
	}
	// Preferences are allowed to name a pinned, unavailable revision. Live
	// readiness and authority are checked again on admission and dispatch.
	return nil
}
