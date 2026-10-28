package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

type MemoryConsent struct {
	Revision int  `json:"revision"`
	Enabled  bool `json:"enabled"`
}

type MemoryEntry struct {
	ID        string    `json:"id"`
	Revision  int       `json:"revision"`
	Label     string    `json:"label"`
	Content   string    `json:"content"`
	SourceJob string    `json:"source_job,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MemoryWrite struct {
	Operation        string `json:"operation"`
	ID               string `json:"id,omitempty"`
	ExpectedRevision int    `json:"expected_revision"`
	Label            string `json:"label"`
	Content          string `json:"content"`
	SourceJob        string `json:"source_job,omitempty"`
}

func ownedMemoryProfile(db *sql.DB, workspace, actor, profile string) error {
	var found int
	err := db.QueryRow(`SELECT 1 FROM agent_profiles WHERE id=? AND workspace=? AND actor=?`, profile, workspace, actor).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrContextNotFound
	}
	return err
}

func (m *Manager) GetMemoryConsent(workspace, actor, profile string) (MemoryConsent, error) {
	var consent MemoryConsent
	if !validProfileOwner(workspace, actor) {
		return consent, ErrContextNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.contextDatabase()
	if err != nil {
		return consent, err
	}
	defer db.Close()
	if err = ownedMemoryProfile(db, workspace, actor, profile); err != nil {
		return consent, err
	}
	err = db.QueryRow(`SELECT revision,enabled FROM agent_memory_consent WHERE workspace=? AND actor=? AND profile=?`, workspace, actor, profile).Scan(&consent.Revision, &consent.Enabled)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return consent, err
}

func (m *Manager) SetMemoryConsent(workspace, actor, profile string, expected int, enabled bool) (MemoryConsent, error) {
	var consent MemoryConsent
	if !validProfileOwner(workspace, actor) || expected < 0 {
		return consent, ErrContextConflict
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.contextDatabase()
	if err != nil {
		return consent, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return consent, err
	}
	defer tx.Rollback()
	// The owning profile and compare-and-swap are checked by the same writer.
	var result sql.Result
	if expected == 0 {
		result, err = tx.Exec(`INSERT INTO agent_memory_consent(workspace,actor,profile,revision,enabled) SELECT ?,?,?,1,? WHERE EXISTS(SELECT 1 FROM agent_profiles WHERE id=? AND workspace=? AND actor=? AND archived=0) ON CONFLICT DO NOTHING`, workspace, actor, profile, enabled, profile, workspace, actor)
	} else {
		result, err = tx.Exec(`UPDATE agent_memory_consent SET revision=revision+1,enabled=? WHERE workspace=? AND actor=? AND profile=? AND revision=? AND EXISTS(SELECT 1 FROM agent_profiles WHERE id=? AND workspace=? AND actor=? AND archived=0)`, enabled, workspace, actor, profile, expected, profile, workspace, actor)
	}
	if err != nil {
		return consent, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return consent, err
	}
	if count != 1 {
		return consent, ErrContextConflict
	}
	if err = tx.Commit(); err != nil {
		return consent, err
	}
	return MemoryConsent{Revision: expected + 1, Enabled: enabled}, nil
}

// consentRevision=0 is an explicit user inspection/edit, never model authority.
// Model callers must supply their admission-pinned positive revision.
func checkMemoryConsent(tx *sql.Tx, workspace, actor, profile string, revision int) error {
	if revision < 0 {
		return ErrMemoryConsent
	}
	var found int
	if revision == 0 {
		err := tx.QueryRow(`SELECT 1 FROM agent_profiles WHERE id=? AND workspace=? AND actor=?`, profile, workspace, actor).Scan(&found)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrContextNotFound
		}
		return err
	}
	err := tx.QueryRow(`SELECT 1 FROM agent_memory_consent c JOIN agent_profiles p ON c.profile=p.id AND c.actor=p.actor AND c.workspace=p.workspace WHERE c.workspace=? AND c.actor=? AND c.profile=? AND c.revision=? AND c.enabled=1 AND p.archived=0`, workspace, actor, profile, revision).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrMemoryConsent
	}
	return err
}

func (m *Manager) ListMemory(workspace, actor, profile string, consentRevision int) ([]MemoryEntry, error) {
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
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = checkMemoryConsent(tx, workspace, actor, profile, consentRevision); err != nil {
		return nil, err
	}
	rows, err := tx.Query(`SELECT id,revision,label,content,source_job,updated_at FROM agent_memory_entries WHERE workspace=? AND actor=? AND profile=? AND deleted=0 ORDER BY updated_at DESC,id LIMIT 128`, workspace, actor, profile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []MemoryEntry{}
	total := 0
	for rows.Next() {
		var entry MemoryEntry
		var updated string
		if err = rows.Scan(&entry.ID, &entry.Revision, &entry.Label, &entry.Content, &entry.SourceJob, &updated); err != nil {
			return nil, err
		}
		entry.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
		if err != nil {
			return nil, err
		}
		if !validProfileText(entry.Content, 8192, true) || !validProfileText(entry.Label, 128, true) {
			return nil, ErrRunStorage
		}
		total += len(entry.Content) + len(entry.Label)
		if consentRevision > 0 && total > 16<<10 {
			break
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (m *Manager) WriteMemory(workspace, actor, profile string, consentRevision int, write MemoryWrite) (*MemoryEntry, error) {
	if !validProfileOwner(workspace, actor) || !validProfileText(write.Operation, 256, true) || !validProfileText(write.Label, 128, true) || !validProfileText(write.Content, 8192, true) || !validProfileText(write.SourceJob, 128, false) || write.ExpectedRevision < 0 || (write.ID == "") != (write.ExpectedRevision == 0) {
		return nil, ErrContextConflict
	}
	encoded, err := json.Marshal(struct {
		Consent int
		Write   MemoryWrite
	}{consentRevision, write})
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(encoded)
	digest := hex.EncodeToString(sum[:])
	m.mu.Lock()
	defer m.mu.Unlock()
	if consentRevision > 0 {
		task := m.tasks[write.SourceJob]
		if task == nil || task.Actor != actor || task.Status != TaskRunning || task.ParentID != "" || task.DeletedAt != nil || task.Config.Profile == nil || task.Config.Profile.Workspace != workspace || task.Config.Profile.ID != profile || task.Config.MemoryConsentRevision != consentRevision {
			return nil, ErrMemoryConsent
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
	// Acquire the SQLite writer before checking consent to serialize revocation,
	// including other service managers/processes, with this entire write.
	if _, err = tx.Exec(`UPDATE agent_profiles SET revision=revision WHERE id=? AND workspace=? AND actor=?`, profile, workspace, actor); err != nil {
		return nil, err
	}
	if err = checkMemoryConsent(tx, workspace, actor, profile, consentRevision); err != nil {
		return nil, err
	}
	var previousDigest string
	var previous []byte
	err = tx.QueryRow(`SELECT digest,result FROM agent_memory_operations WHERE workspace=? AND actor=? AND profile=? AND operation=?`, workspace, actor, profile, write.Operation).Scan(&previousDigest, &previous)
	if err == nil {
		if digest != previousDigest || len(previous) == 0 {
			return nil, ErrContextConflict
		}
		var entry MemoryEntry
		if json.Unmarshal(previous, &entry) != nil {
			return nil, ErrRunStorage
		}
		return &entry, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var count, size int
	err = tx.QueryRow(`SELECT count(*),coalesce(sum(length(CAST(content AS BLOB))+length(CAST(label AS BLOB))),0) FROM agent_memory_entries WHERE workspace=? AND actor=? AND profile=? AND deleted=0 AND id<>?`, workspace, actor, profile, write.ID).Scan(&count, &size)
	if err != nil {
		return nil, err
	}
	if count >= 128 || size+len(write.Content)+len(write.Label) > 512<<10 {
		return nil, ErrContextConflict
	}
	entry := &MemoryEntry{ID: write.ID, Revision: write.ExpectedRevision + 1, Label: write.Label, Content: write.Content, SourceJob: write.SourceJob, UpdatedAt: time.Now().UTC()}
	if write.ID == "" {
		entry.ID = "memory-" + runID()
		_, err = tx.Exec(`INSERT INTO agent_memory_entries(id,workspace,actor,profile,revision,label,content,source_job,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, entry.ID, workspace, actor, profile, 1, entry.Label, entry.Content, entry.SourceJob, entry.UpdatedAt.Format(time.RFC3339Nano))
	} else {
		var result sql.Result
		result, err = tx.Exec(`UPDATE agent_memory_entries SET revision=revision+1,label=?,content=?,source_job=?,updated_at=? WHERE id=? AND workspace=? AND actor=? AND profile=? AND revision=? AND deleted=0`, entry.Label, entry.Content, entry.SourceJob, entry.UpdatedAt.Format(time.RFC3339Nano), write.ID, workspace, actor, profile, write.ExpectedRevision)
		if err == nil {
			n, e := result.RowsAffected()
			if e != nil {
				return nil, e
			}
			if n != 1 {
				return nil, ErrContextConflict
			}
		}
	}
	if err != nil {
		return nil, err
	}
	response, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(`INSERT INTO agent_memory_operations(workspace,actor,profile,operation,digest,entry_id,result) VALUES(?,?,?,?,?,?,?)`, workspace, actor, profile, write.Operation, digest, entry.ID, response)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return entry, nil
}

func (m *Manager) DeleteMemory(workspace, actor, profile, id string, expected int) error {
	if !validProfileOwner(workspace, actor) || expected < 1 {
		return ErrContextConflict
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.contextDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE agent_memory_entries SET revision=revision+1,label='',content='',source_job='',deleted=1 WHERE id=? AND workspace=? AND actor=? AND profile=? AND revision=? AND deleted=0`, id, workspace, actor, profile, expected)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrContextConflict
	}
	// Do not retain deleted private text in the idempotency response cache.
	if _, err = tx.Exec(`UPDATE agent_memory_operations SET result=x'' WHERE workspace=? AND actor=? AND profile=? AND entry_id=?`, workspace, actor, profile, id); err != nil {
		return err
	}
	return tx.Commit()
}
