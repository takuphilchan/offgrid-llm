package agents

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/automation"
)

type ScheduleOccurrence struct {
	ID            string    `json:"id"`
	Schedule      string    `json:"schedule"`
	Revision      int       `json:"revision"`
	DueAt         time.Time `json:"due_at"`
	State         string    `json:"state"`
	Reason        string    `json:"reason,omitempty"`
	JobID         string    `json:"job_id,omitempty"`
	RequestID     string    `json:"request_id"`
	RequestDigest string    `json:"request_digest"`
	CoveredUntil  time.Time `json:"covered_until"`
	CreatedAt     time.Time `json:"created_at"`
}

func (o ScheduleOccurrence) Origin() *ScheduleOrigin {
	return &ScheduleOrigin{Version: 1, Schedule: o.Schedule, Revision: o.Revision, Occurrence: o.ID, DueAt: o.DueAt}
}

func occurrenceRequest(schedule *AgentSchedule, o ScheduleOccurrence) (AgentConfig, string) {
	return preparedOccurrenceRequest(schedule, o, schedule.Config)
}

func preparedOccurrenceRequest(schedule *AgentSchedule, o ScheduleOccurrence, config AgentConfig) (AgentConfig, string) {
	config.Automation = o.Origin()
	data, _ := json.Marshal(struct {
		Prompt, Model, Actor string
		Config               AgentConfig
	}{schedule.Settings.Prompt, config.Profile.Settings.Model, schedule.Actor, config})
	digest := sha256.Sum256(data)
	return config, hex.EncodeToString(digest[:])
}

// ClaimScheduleOccurrence atomically advances the cursor and records why this
// slot may or may not run. It never creates, resumes or dispatches a task.
func (m *Manager) ClaimScheduleOccurrence(schedule *AgentSchedule, now, boot time.Time) (*ScheduleOccurrence, error) {
	if schedule == nil {
		return nil, ErrSchedule
	}
	return m.ClaimPreparedScheduleOccurrence(schedule, schedule.Config, now, boot)
}

// Preparation may refresh bounded Knowledge text/labels under the same pinned
// document IDs. It cannot change capabilities, model, profile or budgets.
func (m *Manager) ClaimPreparedScheduleOccurrence(schedule *AgentSchedule, prepared AgentConfig, now, boot time.Time, epoch ...string) (*ScheduleOccurrence, error) {
	if schedule == nil || now.IsZero() || boot.IsZero() || schedule.Version != 1 || !schedule.Settings.Enabled || schedule.Digest != scheduleDigest(schedule) || schedule.NextAt.After(now) {
		return nil, ErrSchedule
	}
	if _, err := validateSchedule(schedule.Settings, schedule.Config, schedule.Workspace, schedule.Actor); err != nil || len(epoch) > 1 {
		return nil, ErrSchedule
	}
	if !sameScheduleAuthority(schedule.Config, prepared) {
		return nil, ErrSchedule
	}
	if !boundedPreparedKnowledge(prepared, 0) {
		return nil, ErrSchedule
	}
	next, err := schedule.Settings.Calendar.Next(now)
	if err != nil {
		return nil, err
	}
	o := &ScheduleOccurrence{ID: automation.OccurrenceID(schedule.ID, schedule.Revision, schedule.NextAt), Schedule: schedule.ID, Revision: schedule.Revision, DueAt: schedule.NextAt.UTC(), State: "claimed", RequestID: "schedule-" + hexDigest(automation.OccurrenceID(schedule.ID, schedule.Revision, schedule.NextAt)), CoveredUntil: now.UTC(), CreatedAt: now.UTC()}
	_, o.RequestDigest = preparedOccurrenceRequest(schedule, *o, prepared)
	freshReview := len(epoch) == 1 && epoch[0] != "" && epoch[0] == schedule.ReviewEpoch
	if o.DueAt.Before(boot) && !freshReview || now.Sub(o.DueAt) >= time.Minute {
		o.State = "missed"
		o.Reason = "service_unavailable_or_clock_jump"
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
	result, err := tx.Exec(`UPDATE agent_schedules SET next_at=? WHERE id=? AND workspace=? AND actor=? AND revision=? AND enabled=1 AND next_at=?`, next.Unix(), schedule.ID, schedule.Workspace, schedule.Actor, schedule.Revision, schedule.NextAt.Unix())
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
	var snapshot []byte
	if err = tx.QueryRow(`SELECT snapshot FROM agent_schedule_revisions WHERE schedule_id=? AND revision=?`, schedule.ID, schedule.Revision).Scan(&snapshot); err != nil {
		return nil, err
	}
	pinned, err := decodeSchedule(snapshot, schedule.ID, schedule.Workspace, schedule.Actor, schedule.Revision, true, schedule.NextAt.Unix())
	if err != nil || pinned.Digest != schedule.Digest {
		return nil, ErrSchedule
	}
	if o.State == "claimed" {
		var seen int
		if err = tx.QueryRow(`SELECT count(*) FROM agent_schedule_occurrences WHERE schedule_id=? AND due_at=?`, schedule.ID, o.DueAt.Unix()).Scan(&seen); err != nil {
			return nil, err
		}
		if seen > 0 {
			o.State = "blocked"
			o.Reason = "slot_already_recorded"
		}
	}
	if o.State == "claimed" {
		var active int
		if err = tx.QueryRow(`SELECT count(*) FROM agent_schedule_occurrences WHERE schedule_id=? AND state IN ('claimed','admitted','interrupted')`, schedule.ID).Scan(&active); err != nil {
			return nil, err
		}
		if active > 0 {
			o.State = "blocked"
			o.Reason = "previous_work_unsettled"
		}
	}
	_, err = tx.Exec(`INSERT INTO agent_schedule_occurrences(id,schedule_id,revision,due_at,state,reason,job_id,request_id,request_digest,covered_until,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, o.ID, o.Schedule, o.Revision, o.DueAt.Unix(), o.State, o.Reason, "", o.RequestID, o.RequestDigest, o.CoveredUntil.Unix(), o.CreatedAt.Unix())
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return o, nil
}

func hexDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func withoutKnowledgeText(config AgentConfig) AgentConfig {
	config.KnowledgeContext = ""
	config.KnowledgeDocuments = append([]KnowledgeDocument(nil), config.KnowledgeDocuments...)
	for i := range config.KnowledgeDocuments {
		config.KnowledgeDocuments[i].Name = ""
	}
	children := make(map[string]AgentConfig, len(config.DelegateProfiles))
	for id, child := range config.DelegateProfiles {
		children[id] = withoutKnowledgeText(child)
	}
	config.DelegateProfiles = children
	return config
}

func sameScheduleAuthority(a, b AgentConfig) bool {
	left, err := json.Marshal(withoutKnowledgeText(a))
	if err != nil {
		return false
	}
	right, err := json.Marshal(withoutKnowledgeText(b))
	return err == nil && bytes.Equal(left, right)
}

func boundedPreparedKnowledge(config AgentConfig, depth int) bool {
	if depth > 1 || !validProfileText(config.KnowledgeContext, 64<<10, false) || len(config.KnowledgeDocuments) > 32 || len(config.DelegateProfiles) > 4 {
		return false
	}
	for _, doc := range config.KnowledgeDocuments {
		if !validProfileText(doc.ID, 256, true) || !validProfileText(doc.Name, 4096, false) {
			return false
		}
	}
	for _, child := range config.DelegateProfiles {
		if !boundedPreparedKnowledge(child, depth+1) {
			return false
		}
	}
	return true
}

func validOccurrence(o *ScheduleOccurrence) bool {
	if o.ID != automation.OccurrenceID(o.Schedule, o.Revision, o.DueAt) || o.Revision < 1 || o.DueAt.IsZero() || o.RequestID != "schedule-"+hexDigest(o.ID) || len(o.RequestDigest) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(o.RequestDigest)
	if err != nil || len(decoded) != 32 || o.RequestDigest != hex.EncodeToString(decoded) {
		return false
	}
	switch o.State {
	case "claimed", "admitted", "interrupted", "missed", "blocked", "completed", "failed", "cancelled":
		return true
	}
	return false
}

// ListScheduleOccurrences is owner-scoped inspection only. It cannot repair a
// missing root by executing another task.
func (m *Manager) ListScheduleOccurrences(workspace, actor, id string) ([]ScheduleOccurrence, error) {
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
	var found int
	err = db.QueryRow(`SELECT 1 FROM agent_schedules WHERE id=? AND workspace=? AND actor=?`, id, workspace, actor).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrContextNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id,schedule_id,revision,due_at,state,reason,job_id,request_id,request_digest,covered_until,created_at FROM agent_schedule_occurrences WHERE schedule_id=? ORDER BY due_at DESC,id LIMIT 128`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ScheduleOccurrence{}
	for rows.Next() {
		var o ScheduleOccurrence
		var due, covered, created int64
		if err = rows.Scan(&o.ID, &o.Schedule, &o.Revision, &due, &o.State, &o.Reason, &o.JobID, &o.RequestID, &o.RequestDigest, &covered, &created); err != nil {
			return nil, err
		}
		o.DueAt = time.Unix(due, 0).UTC()
		o.CoveredUntil = time.Unix(covered, 0).UTC()
		o.CreatedAt = time.Unix(created, 0).UTC()
		if !validOccurrence(&o) {
			return nil, ErrSchedule
		}
		items = append(items, o)
	}
	return items, rows.Err()
}

func (m *Manager) LinkScheduleOccurrence(schedule *AgentSchedule, o ScheduleOccurrence, jobID string) error {
	if schedule == nil || !validOccurrence(&o) || o.Schedule != schedule.ID || o.Revision != schedule.Revision {
		return ErrSchedule
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	task := m.tasks[jobID]
	if task == nil || task.Actor != schedule.Actor || task.ParentID != "" || !sameScheduleOrigin(task.Config.Automation, o.Origin()) || task.RequestID != o.RequestID || task.RequestDigest != o.RequestDigest {
		return ErrContextConflict
	}
	db, err := m.scheduleDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := db.Exec(`UPDATE agent_schedule_occurrences SET job_id=?,state='admitted' WHERE id=? AND schedule_id=? AND revision=? AND request_id=? AND request_digest=? AND state='claimed' AND job_id='' AND EXISTS(SELECT 1 FROM agent_schedules WHERE id=? AND workspace=? AND actor=? AND revision=? AND enabled=1)`, jobID, o.ID, schedule.ID, schedule.Revision, o.RequestID, o.RequestDigest, schedule.ID, schedule.Workspace, schedule.Actor, schedule.Revision)
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
	return nil
}

// BlockScheduleOccurrence is valid only before root linkage. It cannot turn an
// active/uncertain action into a harmless failed slot.
func (m *Manager) BlockScheduleOccurrence(schedule *AgentSchedule, o ScheduleOccurrence, reason string) error {
	if schedule == nil || o.Schedule != schedule.ID || !validProfileText(reason, 128, true) {
		return ErrSchedule
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.scheduleDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := db.Exec(`UPDATE agent_schedule_occurrences SET state='blocked',reason=? WHERE id=? AND schedule_id=? AND state='claimed' AND job_id='' AND EXISTS(SELECT 1 FROM agent_schedules WHERE id=? AND workspace=? AND actor=?)`, reason, o.ID, schedule.ID, schedule.ID, schedule.Workspace, schedule.Actor)
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
	return nil
}
