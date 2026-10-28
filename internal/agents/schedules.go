package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/automation"
)

var ErrSchedule = errors.New("invalid schedule: review an owned profile, explicit calendar, current resources and bounded unattended task")

type ScheduleSettings struct {
	Name     string              `json:"name"`
	Prompt   string              `json:"prompt"`
	Calendar automation.Calendar `json:"calendar"`
	Enabled  bool                `json:"enabled"`
}

type AgentSchedule struct {
	Version     int              `json:"version"`
	ID          string           `json:"id"`
	Workspace   string           `json:"workspace"`
	Actor       string           `json:"actor"`
	Revision    int              `json:"revision"`
	Digest      string           `json:"digest"`
	Settings    ScheduleSettings `json:"settings"`
	Config      AgentConfig      `json:"config"`
	NextAt      time.Time        `json:"next_at"`
	CreatedAt   time.Time        `json:"created_at"`
	ReviewEpoch string           `json:"review_epoch,omitempty"`
}

type ScheduleOrigin struct {
	Version    int       `json:"version"`
	Schedule   string    `json:"schedule"`
	Revision   int       `json:"revision"`
	Occurrence string    `json:"occurrence"`
	DueAt      time.Time `json:"due_at"`
}

func (m *Manager) scheduleDatabase() (*sql.DB, error) {
	db, err := m.profileDatabase()
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS agent_schedules (
id TEXT PRIMARY KEY,workspace TEXT NOT NULL,actor TEXT NOT NULL,revision INTEGER NOT NULL,enabled INTEGER NOT NULL,next_at INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS agent_schedules_due ON agent_schedules(enabled,next_at);
CREATE TABLE IF NOT EXISTS agent_schedule_revisions (
schedule_id TEXT NOT NULL REFERENCES agent_schedules(id),revision INTEGER NOT NULL,snapshot BLOB NOT NULL,PRIMARY KEY(schedule_id,revision));
CREATE TABLE IF NOT EXISTS agent_schedule_occurrences (
id TEXT PRIMARY KEY,schedule_id TEXT NOT NULL REFERENCES agent_schedules(id),revision INTEGER NOT NULL,due_at INTEGER NOT NULL,state TEXT NOT NULL,reason TEXT NOT NULL,job_id TEXT NOT NULL,request_id TEXT NOT NULL,request_digest TEXT NOT NULL,covered_until INTEGER NOT NULL,created_at INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS agent_schedule_occurrences_history ON agent_schedule_occurrences(schedule_id,due_at);
CREATE INDEX IF NOT EXISTS agent_schedule_occurrences_state ON agent_schedule_occurrences(schedule_id,state);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func validateSchedule(settings ScheduleSettings, config AgentConfig, workspace, actor string) (ScheduleSettings, error) {
	calendar, err := settings.Calendar.Canonical()
	if err != nil || !validProfileOwner(workspace, actor) || !validProfileText(settings.Name, 128, true) || !validProfileText(settings.Prompt, 32000, true) || !config.TaskFirst || config.Profile == nil || config.Profile.Workspace != workspace || config.Profile.Actor != actor || config.Profile.Archived || config.Profile.Revision < 1 || validateProfileSettings(config.Profile.Settings) != nil || config.MaxIterations < 1 || config.MaxIterations > config.Profile.Settings.MaxIterations || config.MaxTokens < 1 || config.MaxTokens > 32768 || config.MemoryConsentRevision < 0 {
		return settings, ErrSchedule
	}
	if config.ComputerSession != "" || config.RequestedComputerScope != "" || config.ComputerScope != "" || config.ComputerAccessOnDemand || !config.RestrictComputer || len(config.NativeExpectations) != 0 || len(config.ConversationContext) != 0 || config.Automation != nil || config.validateExecution() != nil || config.validateTextOnly() != nil {
		return settings, ErrSchedule
	}
	settings.Calendar = calendar
	return settings, nil
}

func scheduleDigest(schedule *AgentSchedule) string {
	copy := *schedule
	copy.Digest = ""
	copy.NextAt = time.Time{}
	encoded, _ := json.Marshal(copy)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func decodeSchedule(data []byte, id, workspace, actor string, revision int, enabled bool, next int64) (*AgentSchedule, error) {
	var schedule AgentSchedule
	if json.Unmarshal(data, &schedule) != nil || schedule.Version != 1 || schedule.ID != id || schedule.Workspace != workspace || schedule.Actor != actor || schedule.Revision != revision || schedule.Settings.Enabled != enabled || schedule.Digest != scheduleDigest(&schedule) {
		return nil, fmt.Errorf("invalid schedule snapshot or digest")
	}
	if _, err := validateSchedule(schedule.Settings, schedule.Config, workspace, actor); err != nil {
		return nil, err
	}
	schedule.NextAt = time.Unix(next, 0).UTC()
	actual, err := schedule.Settings.Calendar.Next(schedule.NextAt.Add(-time.Nanosecond))
	if err != nil || !actual.Equal(schedule.NextAt) {
		return nil, fmt.Errorf("schedule cursor is not a valid first calendar instant")
	}
	return &schedule, nil
}

// SaveSchedule creates no job and grants no authority. The service supplies a
// reviewed immutable config; provider/actor readiness must be checked again at
// the actual occurrence. Connections are owned by this manager, not HTTP clients.
func (m *Manager) SaveSchedule(workspace, actor, id string, expected int, settings ScheduleSettings, config AgentConfig, now time.Time) (*AgentSchedule, error) {
	return m.SaveScheduleWithEpoch(workspace, actor, id, expected, settings, config, now, "")
}

func (m *Manager) SaveScheduleWithEpoch(workspace, actor, id string, expected int, settings ScheduleSettings, config AgentConfig, now time.Time, epoch string) (*AgentSchedule, error) {
	settings, err := validateSchedule(settings, config, workspace, actor)
	if err != nil || expected < 0 || (id == "") != (expected == 0) || now.IsZero() || !validProfileText(epoch, 128, false) {
		return nil, ErrSchedule
	}
	next, err := settings.Calendar.Next(now)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	var detached AgentConfig
	if err = json.Unmarshal(encoded, &detached); err != nil {
		return nil, err
	}
	config = detached
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.scheduleDatabase()
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
		var count int
		if err = tx.QueryRow(`SELECT count(*) FROM agent_schedules WHERE workspace=? AND actor=?`, workspace, actor).Scan(&count); err != nil {
			return nil, err
		}
		if count >= 128 {
			return nil, ErrSchedule
		}
		id = "schedule-" + runID()
		_, err = tx.Exec(`INSERT INTO agent_schedules(id,workspace,actor,revision,enabled,next_at) VALUES(?,?,?,1,?,?)`, id, workspace, actor, settings.Enabled, next.Unix())
	} else {
		var result sql.Result
		result, err = tx.Exec(`UPDATE agent_schedules SET revision=revision+1,enabled=?,next_at=? WHERE id=? AND workspace=? AND actor=? AND revision=?`, settings.Enabled, next.Unix(), id, workspace, actor, expected)
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
	// Check the pinned immutable profile inside the same writer transaction.
	var profileData []byte
	err = tx.QueryRow(`SELECT r.snapshot FROM agent_profiles p JOIN agent_profile_revisions r ON p.id=r.profile_id WHERE p.id=? AND p.workspace=? AND p.actor=? AND p.archived=0 AND r.revision=?`, config.Profile.ID, workspace, actor, config.Profile.Revision).Scan(&profileData)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrContextNotFound
	}
	if err != nil {
		return nil, err
	}
	profile, err := decodeProfile(profileData, config.Profile.ID, workspace, actor, config.Profile.Revision, false)
	if err != nil || profile.Digest != config.Profile.Digest {
		return nil, ErrSchedule
	}
	schedule := &AgentSchedule{Version: 1, ID: id, Workspace: workspace, Actor: actor, Revision: expected + 1, Settings: settings, Config: config, NextAt: next, CreatedAt: now.UTC()}
	schedule.ReviewEpoch = epoch
	schedule.Digest = scheduleDigest(schedule)
	snapshot, err := json.Marshal(schedule)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO agent_schedule_revisions(schedule_id,revision,snapshot) VALUES(?,?,?)`, id, schedule.Revision, snapshot); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return schedule, nil
}

func (m *Manager) GetSchedule(workspace, actor, id string, revision int) (*AgentSchedule, error) {
	if !validProfileOwner(workspace, actor) || revision < 0 {
		return nil, ErrContextNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.scheduleDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var current int
	var enabled bool
	var next int64
	err = db.QueryRow(`SELECT revision,enabled,next_at FROM agent_schedules WHERE id=? AND workspace=? AND actor=?`, id, workspace, actor).Scan(&current, &enabled, &next)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrContextNotFound
	}
	if err != nil {
		return nil, err
	}
	if revision == 0 {
		revision = current
	}
	var data []byte
	err = db.QueryRow(`SELECT snapshot FROM agent_schedule_revisions WHERE schedule_id=? AND revision=?`, id, revision).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrContextNotFound
	}
	if err != nil {
		return nil, err
	}
	if revision != current {
		var old AgentSchedule
		if json.Unmarshal(data, &old) != nil {
			return nil, ErrSchedule
		}
		enabled = old.Settings.Enabled
		next = old.NextAt.Unix()
	}
	return decodeSchedule(data, id, workspace, actor, revision, enabled, next)
}

func (m *Manager) ListSchedules(workspace, actor string) ([]*AgentSchedule, error) {
	if !validProfileOwner(workspace, actor) {
		return nil, ErrContextNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.scheduleDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT s.id,s.revision,s.enabled,s.next_at,r.snapshot FROM agent_schedules s JOIN agent_schedule_revisions r ON r.schedule_id=s.id AND r.revision=s.revision WHERE s.workspace=? AND s.actor=? ORDER BY s.id LIMIT 128`, workspace, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*AgentSchedule{}
	for rows.Next() {
		var id string
		var revision int
		var enabled bool
		var next int64
		var data []byte
		if err = rows.Scan(&id, &revision, &enabled, &next, &data); err != nil {
			return nil, err
		}
		item, err := decodeSchedule(data, id, workspace, actor, revision, enabled, next)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
