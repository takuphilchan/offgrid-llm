package agents

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// DisableSchedule revokes this live policy revision without requiring a now-
// unavailable model/host to be reapproved. Re-enable requires full SaveSchedule.
func (m *Manager) DisableSchedule(workspace, actor, id string, expected int, now time.Time) (*AgentSchedule, error) {
	if !validProfileOwner(workspace, actor) || expected < 1 || now.IsZero() {
		return nil, ErrSchedule
	}
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
	result, err := tx.Exec(`UPDATE agent_schedules SET revision=revision+1,enabled=0 WHERE id=? AND workspace=? AND actor=? AND revision=?`, id, workspace, actor, expected)
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, ErrContextConflict
	}
	var data []byte
	if err = tx.QueryRow(`SELECT snapshot FROM agent_schedule_revisions WHERE schedule_id=? AND revision=?`, id, expected).Scan(&data); err != nil {
		return nil, err
	}
	var previous AgentSchedule
	if json.Unmarshal(data, &previous) != nil {
		return nil, ErrSchedule
	}
	item, err := decodeSchedule(data, id, workspace, actor, expected, previous.Settings.Enabled, previous.NextAt.Unix())
	if err != nil {
		return nil, err
	}
	item.Revision++
	item.Settings.Enabled = false
	var next int64
	if err = tx.QueryRow(`SELECT next_at FROM agent_schedules WHERE id=?`, id).Scan(&next); err != nil {
		return nil, err
	}
	item.NextAt = time.Unix(next, 0).UTC()
	item.CreatedAt = now.UTC()
	item.Digest = scheduleDigest(item)
	snapshot, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO agent_schedule_revisions(schedule_id,revision,snapshot) VALUES(?,?,?)`, id, item.Revision, snapshot); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (m *Manager) ScheduleAllowsTask(task *Task) error {
	if task == nil {
		return ErrSchedule
	}
	if task.Config.Automation == nil {
		return nil
	}
	pin := task.Config.Automation
	if pin.Version != 1 || pin.Revision < 1 || task.ParentID != "" || task.Config.Profile == nil || pin.Schedule == "" || pin.Occurrence == "" || pin.DueAt.IsZero() {
		return ErrSchedule
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.scheduleDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	var data []byte
	var next int64
	err = db.QueryRow(`SELECT r.snapshot,s.next_at FROM agent_schedules s JOIN agent_schedule_revisions r ON r.schedule_id=s.id AND r.revision=s.revision JOIN agent_schedule_occurrences o ON o.schedule_id=s.id WHERE s.id=? AND s.workspace=? AND s.actor=? AND s.enabled=1 AND s.revision=? AND o.id=? AND o.revision=? AND o.due_at=? AND o.job_id=? AND o.request_id=? AND o.request_digest=? AND o.state IN ('admitted','interrupted')`, pin.Schedule, task.Config.Profile.Workspace, task.Actor, pin.Revision, pin.Occurrence, pin.Revision, pin.DueAt.Unix(), task.ID, task.RequestID, task.RequestDigest).Scan(&data, &next)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrContextConflict
	}
	if err != nil {
		return err
	}
	policy, err := decodeSchedule(data, pin.Schedule, task.Config.Profile.Workspace, task.Actor, pin.Revision, true, next)
	if err != nil {
		return err
	}
	if task.Config.Profile.Actor != task.Actor || task.Config.Profile.ID != policy.Config.Profile.ID || task.Config.Profile.Revision != policy.Config.Profile.Revision || task.Config.Profile.Digest != policy.Config.Profile.Digest {
		return ErrSchedule
	}
	if task.Model != "" {
		expected := policy.Config
		expected.Automation = pin
		expected.SystemPrompt, err = expected.initialSystemPrompt()
		if err != nil || task.Model != policy.Config.Profile.Settings.Model || !sameScheduleAuthority(expected, task.Config) {
			return ErrSchedule
		}
	}
	return nil
}

// Only the application coordinator uses global due discovery. The public list
// remains actor scoped and cannot submit or claim a slot.
func (m *Manager) DueSchedules(workspace string, now time.Time) ([]*AgentSchedule, error) {
	if !validProfileText(workspace, 256, true) || now.IsZero() {
		return nil, ErrSchedule
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.scheduleDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT s.id,s.actor,s.revision,s.next_at,r.snapshot FROM agent_schedules s JOIN agent_schedule_revisions r ON r.schedule_id=s.id AND r.revision=s.revision WHERE s.workspace=? AND s.enabled=1 AND s.next_at<=? ORDER BY s.next_at,s.id LIMIT 32`, workspace, now.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*AgentSchedule{}
	for rows.Next() {
		var id, actor string
		var revision int
		var next int64
		var data []byte
		if err = rows.Scan(&id, &actor, &revision, &next, &data); err != nil {
			return nil, err
		}
		item, err := decodeSchedule(data, id, workspace, actor, revision, true, next)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
