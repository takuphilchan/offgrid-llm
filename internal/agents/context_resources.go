package agents

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
)

var (
	ErrContextNotFound = errors.New("agent context resource not found")
	ErrContextConflict = errors.New("agent context revision or operation changed")
	ErrMemoryConsent   = errors.New("automatic memory consent is disabled, stale or outside this profile")
)

type AgentSkill struct {
	agentcontext.SkillReference
	ResponseContract *agentcontext.ResponseContract `json:"response_contract,omitempty"`
	Name             string                         `json:"name"`
	Description      string                         `json:"description"`
	Draft            bool                           `json:"draft"`
	SourceJob        string                         `json:"source_job,omitempty"`
	CreatedAt        time.Time                      `json:"created_at"`
}

func (m *Manager) contextDatabase() (*sql.DB, error) {
	db, err := m.profileDatabase()
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS agent_context_skills (
id TEXT PRIMARY KEY,workspace TEXT NOT NULL,actor TEXT NOT NULL,name TEXT NOT NULL,revision INTEGER NOT NULL,draft INTEGER NOT NULL,source_job TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS agent_context_skills_owner ON agent_context_skills(workspace,actor);
CREATE TABLE IF NOT EXISTS agent_context_skill_revisions (
skill_id TEXT NOT NULL REFERENCES agent_context_skills(id),revision INTEGER NOT NULL,digest TEXT NOT NULL,bundle BLOB NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(skill_id,revision));
CREATE TABLE IF NOT EXISTS agent_memory_consent (
workspace TEXT NOT NULL,actor TEXT NOT NULL,profile TEXT NOT NULL REFERENCES agent_profiles(id),revision INTEGER NOT NULL,enabled INTEGER NOT NULL,PRIMARY KEY(workspace,actor,profile));
CREATE TABLE IF NOT EXISTS agent_memory_entries (
id TEXT PRIMARY KEY,workspace TEXT NOT NULL,actor TEXT NOT NULL,profile TEXT NOT NULL REFERENCES agent_profiles(id),revision INTEGER NOT NULL,label TEXT NOT NULL,content TEXT NOT NULL,source_job TEXT NOT NULL,deleted INTEGER NOT NULL DEFAULT 0,updated_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS agent_memory_entries_scope ON agent_memory_entries(workspace,actor,profile);
CREATE TABLE IF NOT EXISTS agent_memory_operations (
workspace TEXT NOT NULL,actor TEXT NOT NULL,profile TEXT NOT NULL,operation TEXT NOT NULL,digest TEXT NOT NULL,entry_id TEXT NOT NULL,result BLOB NOT NULL,PRIMARY KEY(workspace,actor,profile,operation));`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// ImportSkill accepts bytes, never a service filesystem path or executable code.
// expectedRevision=0 creates; edits retain the old immutable bytes.
func (m *Manager) ImportSkill(workspace, actor, id string, expectedRevision int, files []agentcontext.SkillFile, sourceJob string) (*AgentSkill, error) {
	if !validProfileOwner(workspace, actor) || expectedRevision < 0 || (id == "") != (expectedRevision == 0) || !validProfileText(sourceJob, 128, false) {
		return nil, agentcontext.ErrSkill
	}
	bundle, err := agentcontext.Parse(files)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(bundle)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if sourceJob != "" {
		task := m.tasks[sourceJob]
		if task == nil || task.Actor != actor || task.ParentID != "" || task.DeletedAt != nil || task.Config.Profile == nil || task.Config.Profile.Workspace != workspace {
			return nil, ErrContextNotFound
		}
	}
	db, err := m.contextDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if id == "" {
		id = "skill-" + runID()
		_, err = tx.Exec(`INSERT INTO agent_context_skills(id,workspace,actor,name,revision,draft,source_job) VALUES(?,?,?,?,1,?,?)`, id, workspace, actor, bundle.Name, sourceJob != "", sourceJob)
	} else {
		var result sql.Result
		result, err = tx.Exec(`UPDATE agent_context_skills SET revision=revision+1 WHERE id=? AND workspace=? AND actor=? AND revision=? AND name=? AND source_job=?`, id, workspace, actor, expectedRevision, bundle.Name, sourceJob)
		if err == nil {
			count, e := result.RowsAffected()
			if e != nil {
				return nil, e
			}
			if count != 1 {
				return nil, ErrContextConflict
			}
		}
	}
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_, err = tx.Exec(`INSERT INTO agent_context_skill_revisions(skill_id,revision,digest,bundle,created_at) VALUES(?,?,?,?,?)`, id, expectedRevision+1, bundle.Digest, encoded, now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &AgentSkill{SkillReference: agentcontext.SkillReference{ID: id, Revision: expectedRevision + 1, Digest: bundle.Digest}, ResponseContract: bundle.ResponseContract, Name: bundle.Name, Description: bundle.Description, Draft: sourceJob != "", SourceJob: sourceJob, CreatedAt: now}, nil
}

func (m *Manager) ListSkills(workspace, actor string) ([]AgentSkill, error) {
	if !validProfileOwner(workspace, actor) {
		return nil, ErrContextNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.contextDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT s.id,s.revision,r.digest,r.bundle,s.draft,s.source_job,r.created_at FROM agent_context_skills s JOIN agent_context_skill_revisions r ON s.id=r.skill_id AND s.revision=r.revision WHERE s.workspace=? AND s.actor=? ORDER BY s.name,s.id LIMIT 256`, workspace, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AgentSkill{}
	for rows.Next() {
		item, _, e := scanSkill(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

type skillScanner interface{ Scan(...interface{}) error }

func scanSkill(row skillScanner) (*AgentSkill, *agentcontext.SkillBundle, error) {
	var item AgentSkill
	var encoded []byte
	var created string
	if err := row.Scan(&item.ID, &item.Revision, &item.Digest, &encoded, &item.Draft, &item.SourceJob, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrContextNotFound
		}
		return nil, nil, err
	}
	var stored agentcontext.SkillBundle
	if json.Unmarshal(encoded, &stored) != nil {
		return nil, nil, fmt.Errorf("invalid stored skill")
	}
	bundle, err := agentcontext.ParseStored(stored.Files, stored.ResponseContract)
	if err != nil || bundle.Digest != item.Digest || bundle.Name != stored.Name || bundle.Description != stored.Description || stored.Digest != item.Digest || !item.SkillReference.Valid() {
		return nil, nil, fmt.Errorf("stored skill digest or metadata mismatch")
	}
	item.Name, item.Description = bundle.Name, bundle.Description
	item.ResponseContract = bundle.ResponseContract
	item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return nil, nil, err
	}
	return &item, &bundle, nil
}

func (m *Manager) GetSkill(workspace, actor string, ref agentcontext.SkillReference) (*AgentSkill, *agentcontext.SkillBundle, error) {
	if !validProfileOwner(workspace, actor) || !ref.Valid() {
		return nil, nil, ErrContextNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.contextDatabase()
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()
	item, bundle, err := scanSkill(db.QueryRow(`SELECT s.id,r.revision,r.digest,r.bundle,s.draft,s.source_job,r.created_at FROM agent_context_skills s JOIN agent_context_skill_revisions r ON s.id=r.skill_id WHERE s.id=? AND s.workspace=? AND s.actor=? AND r.revision=? AND r.digest=?`, ref.ID, workspace, actor, ref.Revision, ref.Digest))
	return item, bundle, err
}
