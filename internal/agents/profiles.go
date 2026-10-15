package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

var (
	ErrProfileNotFound = errors.New("agent profile not found")
	ErrProfileConflict = errors.New("agent profile revision changed or profile archived")
	ErrInvalidProfile  = errors.New("invalid agent profile")
)

// References identify resources in the workspace. Credentials and executable
// commands are deliberately absent. The service resolves current authority.
type ProfileToolReference struct {
	Provider       string `json:"provider"`
	Tool           string `json:"tool"`
	SchemaRevision string `json:"schema_revision"`
	Location       string `json:"location"`
}

type ProfileSettings struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description,omitempty"`
	Instructions  string                 `json:"instructions,omitempty"`
	Model         string                 `json:"model"`
	Knowledge     []string               `json:"knowledge,omitempty"`
	Tools         []ProfileToolReference `json:"tools,omitempty"`
	Computer      bool                   `json:"computer,omitempty"`
	MaxIterations int                    `json:"max_iterations"`
	Delegates     []string               `json:"delegates,omitempty"`
}

type AgentProfile struct {
	ID        string          `json:"id"`
	Workspace string          `json:"workspace"`
	Actor     string          `json:"actor"`
	Revision  int             `json:"revision"`
	Digest    string          `json:"digest"`
	Settings  ProfileSettings `json:"settings"`
	Archived  bool            `json:"archived"`
	CreatedAt time.Time       `json:"created_at"`
}

func validProfileText(s string, max int, required bool) bool {
	return utf8.ValidString(s) && len(s) <= max && !strings.ContainsRune(s, 0) && (!required || strings.TrimSpace(s) != "")
}

func validateProfileSettings(s ProfileSettings) error {
	if !validProfileText(s.Name, 128, true) || !validProfileText(s.Description, 512, false) || !validProfileText(s.Instructions, 8192, false) || !validProfileText(s.Model, 256, true) || s.MaxIterations < 1 || s.MaxIterations > 50 || len(s.Knowledge) > 32 || len(s.Tools) > 128 || len(s.Delegates) > 4 {
		return ErrInvalidProfile
	}
	for _, refs := range [][]string{s.Knowledge, s.Delegates} {
		seen := map[string]bool{}
		for _, id := range refs {
			if !validProfileText(id, 256, true) || seen[id] {
				return ErrInvalidProfile
			}
			seen[id] = true
		}
	}
	seen := map[string]bool{}
	for _, ref := range s.Tools {
		if !validProfileText(ref.Provider, 256, true) || !validProfileText(ref.Tool, 256, true) || !validProfileText(ref.SchemaRevision, 256, true) || (ref.Location != "host" && ref.Location != "service" && ref.Location != "remote") {
			return ErrInvalidProfile
		}
		key := ref.Provider + "\x00" + ref.Tool
		if seen[key] {
			return ErrInvalidProfile
		}
		seen[key] = true
	}
	return nil
}

func (m *Manager) profileDatabase() (*sql.DB, error) {
	if m.dataDir == "" || m.storageErr != nil {
		return nil, ErrRunStorage
	}
	db, err := openTaskDatabase(m.dataDir)
	if err != nil {
		return nil, err
	}
	// Additive tables leave existing tasks and their schema untouched. A prior
	// runtime can still read old jobs without rewriting profile records.
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS agent_profiles (
id TEXT PRIMARY KEY, workspace TEXT NOT NULL, actor TEXT NOT NULL,
revision INTEGER NOT NULL, archived INTEGER NOT NULL DEFAULT 0);
CREATE INDEX IF NOT EXISTS agent_profiles_owner ON agent_profiles(workspace,actor);
CREATE TABLE IF NOT EXISTS agent_profile_revisions (
profile_id TEXT NOT NULL REFERENCES agent_profiles(id), revision INTEGER NOT NULL,
snapshot BLOB NOT NULL, PRIMARY KEY(profile_id,revision));`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func validProfileOwner(workspace, actor string) bool {
	return validProfileText(workspace, 256, true) && validProfileText(actor, 256, true)
}

// ProfileAllowsCapability is an additional restriction, never a grant. Current
// actor policy and host consent are independently checked before execution.
func ProfileAllowsCapability(config AgentConfig, name string, descriptor capabilities.Descriptor) bool {
	if config.Profile == nil {
		return true
	}
	if companionCapability(descriptor) || (name == "request_computer_access" && descriptor.Namespace == "computer" && descriptor.Source == "workspace") {
		return config.Profile.Settings.Computer && !config.RestrictComputer
	}
	if name == "task_save_artifact" && descriptor.Source == "workspace" && descriptor.Namespace == "workspace" {
		return true
	}
	pinned, exists := config.AllowedCapabilities[name]
	return exists && pinned == descriptor
}

// FindTaskRequest is observation-only; it cannot resume or dispatch saved work.
// Keep the digest/tombstone check identical to CreateRequest's admission check.
func (m *Manager) FindTaskRequest(actor, requestID, digest string) (*Task, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, previous := range m.tasks {
		if previous.Actor == actor && previous.RequestID == requestID {
			if previous.RequestDigest != digest || previous.DeletedAt != nil {
				return nil, true, ErrRunConflict
			}
			copy, err := copyTask(previous)
			return copy, true, err
		}
	}
	return nil, false, nil
}

// SaveProfile uses expectedRevision=0 only for creation. Every edit inserts an
// immutable snapshot and advances the current pointer in the same transaction.
func (m *Manager) SaveProfile(workspace, actor, id string, expectedRevision int, settings ProfileSettings) (*AgentProfile, error) {
	if !validProfileOwner(workspace, actor) || validateProfileSettings(settings) != nil || expectedRevision < 0 || (id == "") != (expectedRevision == 0) {
		return nil, ErrInvalidProfile
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	// Copy slices before storing/returning the snapshot; caller changes cannot
	// change either the digest or the persisted revision.
	if err = json.Unmarshal(encoded, &settings); err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.profileDatabase()
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
		id = "profile-" + runID()
		_, err = tx.Exec(`INSERT INTO agent_profiles(id,workspace,actor,revision) VALUES(?,?,?,1)`, id, workspace, actor)
	} else {
		// Acquire the write lock with a compare-and-swap before reading. Two
		// managers must not both observe the old revision and then upgrade a
		// stale SQLite read transaction to a writer.
		var result sql.Result
		result, err = tx.Exec(`UPDATE agent_profiles SET revision=revision+1 WHERE id=? AND workspace=? AND actor=? AND revision=? AND archived=0`, id, workspace, actor, expectedRevision)
		if err != nil {
			return nil, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if count != 1 {
			var found int
			if err = tx.QueryRow(`SELECT 1 FROM agent_profiles WHERE id=? AND workspace=? AND actor=?`, id, workspace, actor).Scan(&found); errors.Is(err, sql.ErrNoRows) {
				return nil, ErrProfileNotFound
			} else if err != nil {
				return nil, err
			}
			return nil, ErrProfileConflict
		}
	}
	if err != nil {
		return nil, err
	}
	profile := &AgentProfile{ID: id, Workspace: workspace, Actor: actor, Revision: expectedRevision + 1, Digest: hex.EncodeToString(digest[:]), Settings: settings, CreatedAt: time.Now().UTC()}
	for _, child := range settings.Delegates {
		if child == id {
			return nil, ErrInvalidProfile
		}
	}
	snapshot, err := json.Marshal(profile)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO agent_profile_revisions(profile_id,revision,snapshot) VALUES(?,?,?)`, id, profile.Revision, snapshot); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return profile, nil
}

func decodeProfile(data []byte, id, workspace, actor string, revision int, archived bool) (*AgentProfile, error) {
	var profile AgentProfile
	if json.Unmarshal(data, &profile) != nil || profile.ID != id || profile.Workspace != workspace || profile.Actor != actor || profile.Revision != revision || validateProfileSettings(profile.Settings) != nil {
		return nil, fmt.Errorf("invalid profile snapshot")
	}
	encoded, _ := json.Marshal(profile.Settings)
	digest := sha256.Sum256(encoded)
	if profile.Digest != hex.EncodeToString(digest[:]) {
		return nil, fmt.Errorf("profile digest mismatch")
	}
	profile.Archived = archived
	return &profile, nil
}

// GetProfile revision=0 reads the current revision. Archived snapshots remain
// inspectable for the owning actor, but admission must reject archived profiles.
func (m *Manager) GetProfile(workspace, actor, id string, revision int) (*AgentProfile, error) {
	if !validProfileOwner(workspace, actor) || id == "" || revision < 0 {
		return nil, ErrProfileNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.profileDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var data []byte
	var selected, archived int
	err = db.QueryRow(`SELECT r.snapshot,r.revision,p.archived FROM agent_profiles p JOIN agent_profile_revisions r ON r.profile_id=p.id AND r.revision=CASE WHEN ?=0 THEN p.revision ELSE ? END WHERE p.id=? AND p.workspace=? AND p.actor=?`, revision, revision, id, workspace, actor).Scan(&data, &selected, &archived)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeProfile(data, id, workspace, actor, selected, archived != 0)
}

func (m *Manager) ListProfiles(workspace, actor string, includeArchived bool) ([]AgentProfile, error) {
	if !validProfileOwner(workspace, actor) {
		return nil, ErrProfileNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.profileDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT p.id,r.snapshot,p.revision,p.archived FROM agent_profiles p JOIN agent_profile_revisions r ON r.profile_id=p.id AND r.revision=p.revision WHERE p.workspace=? AND p.actor=? AND (? OR p.archived=0) ORDER BY p.id`, workspace, actor, includeArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := []AgentProfile{}
	for rows.Next() {
		var id string
		var data []byte
		var revision, archived int
		if err = rows.Scan(&id, &data, &revision, &archived); err != nil {
			return nil, err
		}
		profile, err := decodeProfile(data, id, workspace, actor, revision, archived != 0)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, *profile)
	}
	return profiles, rows.Err()
}

func (m *Manager) ArchiveProfile(workspace, actor, id string, expectedRevision int) error {
	if !validProfileOwner(workspace, actor) || id == "" || expectedRevision < 1 {
		return ErrProfileNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.profileDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := db.Exec(`UPDATE agent_profiles SET archived=1 WHERE id=? AND workspace=? AND actor=? AND revision=?`, id, workspace, actor, expectedRevision)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count == 1 {
		return err
	}
	var found int
	err = db.QueryRow(`SELECT 1 FROM agent_profiles WHERE id=? AND workspace=? AND actor=?`, id, workspace, actor).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrProfileNotFound
	}
	if err != nil {
		return err
	}
	return ErrProfileConflict
}
