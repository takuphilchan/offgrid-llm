package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func projectCommandDigest(c api.ProjectCommand) string {
	data, _ := json.Marshal(c)
	hash := sha256.Sum256(append([]byte("project-command-v1\x00"), data...))
	return hex.EncodeToString(hash[:])
}

func (m *Manager) ProjectRevision(scope ControlScope, id string, revision int64) (*Project, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return nil, ErrControlNotFound
	}
	if revision < 1 {
		return nil, ErrControlCommand
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var data []byte
	err = db.QueryRow(`SELECT snapshot FROM agent_project_revisions WHERE workspace=? AND actor=? AND project_id=? AND revision=?`, scope.Workspace, scope.Actor, id, revision).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, ErrControlNotFound
	}
	if err != nil {
		return nil, err
	}
	var project Project
	err = json.Unmarshal(data, &project)
	return &project, err
}

func (m *Manager) ApplyProjectCommand(scope ControlScope, c api.ProjectCommand) (*ConversationCommandReceipt, error) {
	digest := projectCommandDigest(c)
	configurationDigest := ""
	if c.Payload.ConfigurationTOML != "" {
		p := c.Payload.Preferences
		if p.Model != "" || p.Delegation != nil || len(c.Payload.Resources) != 0 || validateProjectPreferences(p, nil) != nil {
			return nil, ErrControlCommand
		}
		parsed, err := ParseProjectFile([]byte(c.Payload.ConfigurationTOML))
		if err != nil {
			return nil, ErrControlCommand
		}
		c.Payload.Preferences = parsed.Preferences
		c.Payload.Resources = parsed.Resources
		configurationDigest = parsed.Digest
	}
	if !validProfileText(c.Payload.Name, 256, true) || validateProjectPreferences(c.Payload.Preferences, c.Payload.Resources) != nil {
		return nil, ErrControlCommand
	}
	switch c.Kind {
	case "create_project":
		if c.TargetID != "" || c.ExpectedRevision != 0 {
			return nil, ErrControlCommand
		}
	case "configure_project":
		if c.TargetID == "" || c.ExpectedRevision < 1 {
			return nil, ErrControlCommand
		}
	default:
		return nil, ErrControlCommand
	}
	return m.withControlCommand(scope, c.RequestID, digest, func(tx *sql.Tx) (*ConversationCommandReceipt, error) {
		project := &Project{ID: "project-" + runID(), Revision: 1}
		if c.Kind == "configure_project" {
			r, err := readControlRecord(tx, scope, c.TargetID, "project")
			if err != nil {
				return nil, err
			}
			if r.Revision != c.ExpectedRevision {
				return nil, ErrRunConflict
			}
			if err = json.Unmarshal(r.Snapshot, project); err != nil {
				return nil, err
			}
			project.Revision++
		}
		project.Name = c.Payload.Name
		project.Preferences = c.Payload.Preferences
		project.Resources = c.Payload.Resources
		// Explicit service edits supersede an earlier repository-file digest.
		project.ConfigurationDigest = configurationDigest
		project.ConfigurationSource = "service"
		if configurationDigest != "" {
			project.ConfigurationSource = "file"
		}
		if err := putControlRecord(tx, scope, project.ID, "project", project.Revision, project); err != nil {
			return nil, err
		}
		cursor, err := appendControlEvent(tx, scope, project.ID, project.Revision, c.Kind, project, "")
		if err != nil {
			return nil, err
		}
		return &ConversationCommandReceipt{RequestID: c.RequestID, TargetID: project.ID, Revision: project.Revision, EventCursor: cursor, Project: project}, nil
	})
}
