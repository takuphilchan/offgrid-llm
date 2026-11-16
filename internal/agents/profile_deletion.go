package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

var ErrProfileDeleteBlocked = errors.New("profile has active or independently retained work")

// ProfileDeletionReview is an owner-scoped, optimistic review of the entire
// operation. Linked canonical work has its own retention boundary and cannot be
// erased by deleting a configuration profile.
type ProfileDeletionReview struct {
	Revision     int    `json:"revision"`
	Jobs         int    `json:"jobs"`
	Memory       int    `json:"memory"`
	BlockedJobs  int    `json:"blocked_jobs"`
	Dependencies int    `json:"dependencies"`
	Token        string `json:"token"`
}

func tableExists(tx *sql.Tx, name string) (bool, error) {
	var n int
	err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n)
	return n != 0, err
}

func profileDeletionReview(tx *sql.Tx, workspace, actor, id string) (ProfileDeletionReview, []*Task, error) {
	r := ProfileDeletionReview{}
	var archived int
	err := tx.QueryRow(`SELECT revision,archived FROM agent_profiles WHERE id=? AND workspace=? AND actor=?`, id, workspace, actor).Scan(&r.Revision, &archived)
	if errors.Is(err, sql.ErrNoRows) {
		return r, nil, ErrProfileNotFound
	}
	if err != nil {
		return r, nil, err
	}
	h := sha256.New()
	enc := json.NewEncoder(h)
	_ = enc.Encode([]any{workspace, actor, id, r.Revision, archived})
	rows, err := tx.Query(`SELECT snapshot FROM agent_tasks WHERE actor=? AND json_extract(snapshot,'$.config.profile.id')=? AND json_extract(snapshot,'$.config.profile.workspace')=? ORDER BY id`, actor, id, workspace)
	if err != nil {
		return r, nil, err
	}
	var tasks []*Task
	selected := map[string]bool{}
	for rows.Next() {
		var data []byte
		var task Task
		if err = rows.Scan(&data); err != nil {
			break
		}
		if err = json.Unmarshal(data, &task); err != nil {
			break
		}
		if task.DeletedAt == nil {
			tasks = append(tasks, &task)
			selected[task.ID] = true
			_, _ = h.Write(data)
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return r, nil, err
	}
	r.Jobs = len(tasks)
	for _, task := range tasks {
		blocked := !CanDeleteTask(task) || task.Config.Automation != nil
		if task.ParentID != "" && !selected[task.ParentID] {
			blocked = true
		}
		for _, child := range task.ChildHistory {
			if !selected[child.ID] {
				blocked = true
			}
		}
		if blocked {
			r.BlockedJobs++
		}
	}
	// Current configurations referencing this profile are dependencies even when
	// archived/disabled. Their owner must explicitly remove the reference first.
	rows, err = tx.Query(`SELECT r.snapshot FROM agent_profiles p JOIN agent_profile_revisions r ON r.profile_id=p.id AND r.revision=p.revision WHERE p.workspace=? AND p.actor=? AND p.id<>? ORDER BY p.id`, workspace, actor, id)
	if err != nil {
		return r, nil, err
	}
	for rows.Next() {
		var data []byte
		var p AgentProfile
		if err = rows.Scan(&data); err != nil {
			break
		}
		if err = json.Unmarshal(data, &p); err != nil {
			break
		}
		for _, delegate := range p.Settings.Delegates {
			if delegate == id {
				r.Dependencies++
				_, _ = h.Write(data)
				break
			}
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return r, nil, err
	}
	// A pinned collaborator may still be used by a different job even after the
	// current parent profile was edited. Never orphan that saved execution graph.
	rows, err = tx.Query(`SELECT id,snapshot FROM agent_tasks WHERE actor=? ORDER BY id`, actor)
	if err != nil {
		return r, nil, err
	}
	for rows.Next() {
		var taskID string
		var data []byte
		var task Task
		if err = rows.Scan(&taskID, &data); err != nil {
			break
		}
		if selected[taskID] {
			continue
		}
		if err = json.Unmarshal(data, &task); err != nil {
			break
		}
		if task.DeletedAt == nil && configUsesProfile(task.Config, workspace, id) {
			r.Dependencies++
			_, _ = h.Write(data)
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return r, nil, err
	}
	if exists, e := tableExists(tx, "agent_schedules"); e != nil {
		return r, nil, e
	} else if exists {
		var scheduleRefs int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM agent_schedules s JOIN agent_schedule_revisions r ON r.schedule_id=s.id AND r.revision=s.revision WHERE s.workspace=? AND s.actor=? AND (json_extract(r.snapshot,'$.config.profile.id')=? OR json_type(r.snapshot,'$.config.delegate_profiles.'||?) IS NOT NULL)`, workspace, actor, id, id).Scan(&scheduleRefs); err != nil {
			return r, nil, err
		}
		r.Dependencies += scheduleRefs
	}
	if exists, e := tableExists(tx, "agent_memory_entries"); e != nil {
		return r, nil, e
	} else if exists {
		memoryRows, err := tx.Query(`SELECT id,revision FROM agent_memory_entries WHERE workspace=? AND actor=? AND profile=? ORDER BY id`, workspace, actor, id)
		if err != nil {
			return r, nil, err
		}
		for memoryRows.Next() {
			var entryID string
			var revision int64
			if err = memoryRows.Scan(&entryID, &revision); err != nil {
				break
			}
			r.Memory++
			_ = enc.Encode([]any{entryID, revision})
		}
		if err == nil {
			err = memoryRows.Err()
		}
		memoryRows.Close()
		if err != nil {
			return r, nil, err
		}
	}
	_ = enc.Encode(r)
	r.Token = hex.EncodeToString(h.Sum(nil))
	return r, tasks, nil
}

func (m *Manager) ReviewProfileDeletion(workspace, actor, id string) (ProfileDeletionReview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.profileDatabase()
	if err != nil {
		return ProfileDeletionReview{}, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return ProfileDeletionReview{}, err
	}
	defer tx.Rollback()
	r, _, err := profileDeletionReview(tx, workspace, actor, id)
	return r, err
}

func (m *Manager) DeleteProfile(workspace, actor, id string, revision int, token string) (ProfileDeletionReview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.profileDatabase()
	if err != nil {
		return ProfileDeletionReview{}, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return ProfileDeletionReview{}, err
	}
	defer tx.Rollback()
	// Reserve the writer before reviewing, also across independent managers.
	if _, err = tx.Exec(`UPDATE agent_profiles SET id=id WHERE id=? AND workspace=? AND actor=?`, id, workspace, actor); err != nil {
		return ProfileDeletionReview{}, err
	}
	r, tasks, err := profileDeletionReview(tx, workspace, actor, id)
	if err != nil {
		return r, err
	}
	if revision != r.Revision || token == "" || token != r.Token {
		return r, ErrProfileConflict
	}
	if r.BlockedJobs != 0 || r.Dependencies != 0 {
		return r, ErrProfileDeleteBlocked
	}
	now := time.Now().UTC()
	tombstones := make([]*Task, 0, len(tasks))
	for _, task := range tasks {
		tombstone := &Task{ID: task.ID, Actor: task.Actor, Status: task.Status, CreatedAt: task.CreatedAt, DeletedAt: &now, RequestID: task.RequestID, RequestDigest: task.RequestDigest}
		if err = putTaskTransaction(tx, tombstone); err != nil {
			return r, err
		}
		if _, err = tx.Exec(`UPDATE agent_tasks SET event_floor=COALESCE((SELECT MAX(sequence) FROM agent_task_events WHERE task_id=?),event_floor) WHERE id=?`, task.ID, task.ID); err != nil {
			return r, err
		}
		if _, err = tx.Exec(`DELETE FROM agent_task_events WHERE task_id=?`, task.ID); err != nil {
			return r, err
		}
		tombstones = append(tombstones, tombstone)
	}
	for _, table := range []string{"agent_memory_operations", "agent_memory_entries", "agent_memory_consent"} {
		if exists, e := tableExists(tx, table); e != nil {
			return r, e
		} else if exists {
			if _, err = tx.Exec(`DELETE FROM `+table+` WHERE workspace=? AND actor=? AND profile=?`, workspace, actor, id); err != nil {
				return r, err
			}
		}
	}
	if _, err = tx.Exec(`INSERT INTO agent_profile_tombstones(id,workspace,actor,deleted_at) VALUES(?,?,?,?)`, id, workspace, actor, now.Format(time.RFC3339Nano)); err != nil {
		return r, err
	}
	if _, err = tx.Exec(`DELETE FROM agent_profile_revisions WHERE profile_id=?`, id); err != nil {
		return r, err
	}
	if _, err = tx.Exec(`DELETE FROM agent_profiles WHERE id=? AND workspace=? AND actor=?`, id, workspace, actor); err != nil {
		return r, err
	}
	if err = tx.Commit(); err != nil {
		return r, err
	}
	for _, task := range tombstones {
		m.tasks[task.ID] = task
	}
	return r, nil
}

// A request prepared before deletion cannot recreate profile work afterwards.
func rejectDeletedProfile(tx *sql.Tx, config AgentConfig) error {
	exists, err := tableExists(tx, "agent_profile_tombstones")
	if err != nil || !exists {
		return err
	}
	if config.Profile != nil {
		var n int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM agent_profile_tombstones WHERE id=?`, config.Profile.ID).Scan(&n); err != nil {
			return err
		}
		if n != 0 {
			return ErrProfileNotFound
		}
	}
	for _, child := range config.DelegateProfiles {
		if err = rejectDeletedProfile(tx, child); err != nil {
			return err
		}
	}
	return nil
}

func configUsesProfile(config AgentConfig, workspace, id string) bool {
	if config.Profile != nil && config.Profile.Workspace == workspace && config.Profile.ID == id {
		return true
	}
	for _, child := range config.DelegateProfiles {
		if configUsesProfile(child, workspace, id) {
			return true
		}
	}
	return false
}
