package agents

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOccurrenceClaimLinkNoDuplicateOrOverlap(t *testing.T) {
	m, settings, config, now := scheduleFixture(t)
	schedule, err := m.SaveSchedule("work", "alice", "", 0, settings, config, now)
	if err != nil {
		t.Fatal(err)
	}
	due := schedule.NextAt
	occurrence, err := m.ClaimScheduleOccurrence(schedule, due.Add(time.Second), now)
	if err != nil || occurrence.State != "claimed" {
		t.Fatal(occurrence, err)
	}
	if _, err = m.ClaimScheduleOccurrence(schedule, due.Add(2*time.Second), now); !errors.Is(err, ErrContextConflict) {
		t.Fatal("duplicate claimed", err)
	}
	rootConfig, digest := occurrenceRequest(schedule, *occurrence)
	runner := NewRunner(m, NewToolRegistry(), nil)
	job, created, err := runner.CreateRequest(settings.Prompt, config.Profile.Settings.Model, "alice", rootConfig, occurrence.RequestID, digest)
	if err != nil || !created {
		t.Fatal(job, created, err)
	}
	if err = m.ScheduleAllowsTask(job); err == nil {
		t.Fatal("unlinked root authorized")
	}
	if err = m.LinkScheduleOccurrence(schedule, *occurrence, job.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.ScheduleAllowsTask(job); err != nil {
		t.Fatal(err)
	}
	again, created, err := runner.CreateRequest(settings.Prompt, config.Profile.Settings.Model, "alice", rootConfig, occurrence.RequestID, digest)
	if err != nil || created || again.ID != job.ID {
		t.Fatal(again, created, err)
	}
	next, err := m.GetSchedule("work", "alice", schedule.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := m.ClaimScheduleOccurrence(next, next.NextAt.Add(time.Second), now)
	if err != nil || blocked.State != "blocked" || blocked.Reason != "previous_work_unsettled" {
		t.Fatal(blocked, err)
	}
	if _, err = m.DisableSchedule("work", "alice", schedule.ID, 1, due); err != nil {
		t.Fatal(err)
	}
	if err = m.ScheduleAllowsTask(job); err == nil {
		t.Fatal("disabled policy authorized old job")
	}
	if _, err = m.SaveSchedule("work", "alice", schedule.ID, 2, settings, config, due.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err = m.ScheduleAllowsTask(job); err == nil {
		t.Fatal("re-enabled policy restored old job")
	}
	if len(m.ListTasks()) != 1 {
		t.Fatal("duplicate root")
	}
	_ = runner.Shutdown(context.Background())
}

func TestOccurrenceDowntimeClockJumpsAndBlockedHostDoNotReplay(t *testing.T) {
	m, settings, config, now := scheduleFixture(t)
	schedule, err := m.SaveSchedule("work", "alice", "", 0, settings, config, now)
	if err != nil {
		t.Fatal(err)
	}
	later := schedule.NextAt.Add(72 * time.Hour)
	missed, err := m.ClaimScheduleOccurrence(schedule, later, later.Add(-time.Second))
	if err != nil || missed.State != "missed" || !missed.CoveredUntil.Equal(later) {
		t.Fatal(missed, err)
	}
	next, err := m.GetSchedule("work", "alice", schedule.ID, 0)
	if err != nil || !next.NextAt.After(later) {
		t.Fatal(next, err)
	}
	if due, err := m.DueSchedules("work", later.Add(-time.Hour)); err != nil || len(due) != 0 {
		t.Fatal("backward clock eligible", due, err)
	}
	o, err := m.ClaimScheduleOccurrence(next, next.NextAt.Add(time.Second), later)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.BlockScheduleOccurrence(next, *o, "execution_host_unavailable"); err != nil {
		t.Fatal(err)
	}
	if _, err = m.ClaimScheduleOccurrence(next, next.NextAt.Add(2*time.Second), later); !errors.Is(err, ErrContextConflict) {
		t.Fatal("blocked host occurrence replayed", err)
	}
	history, err := m.ListScheduleOccurrences("work", "alice", schedule.ID)
	if err != nil || len(history) != 2 || history[0].Reason != "execution_host_unavailable" {
		t.Fatal(history, err)
	}
	if len(m.ListTasks()) != 0 {
		t.Fatal("history created work")
	}
	if _, err = m.ListScheduleOccurrences("work", "bob", schedule.ID); !errors.Is(err, ErrContextNotFound) {
		t.Fatal("cross-owner history", err)
	}
}

func TestOccurrencePersistenceFailureDoesNotAdvanceOrDispatch(t *testing.T) {
	m, settings, config, now := scheduleFixture(t)
	schedule, err := m.SaveSchedule("work", "alice", "", 0, settings, config, now)
	if err != nil {
		t.Fatal(err)
	}
	db, err := m.scheduleDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER occurrence_fixture_failure BEFORE INSERT ON agent_schedule_occurrences BEGIN SELECT RAISE(ABORT,'fixture failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err = m.ClaimScheduleOccurrence(schedule, schedule.NextAt.Add(time.Second), now); err == nil {
		t.Fatal("failed claim acknowledged")
	}
	current, err := m.GetSchedule("work", "alice", schedule.ID, 0)
	if err != nil || !current.NextAt.Equal(schedule.NextAt) {
		t.Fatal("cursor advanced without claim", current, err)
	}
	if history, e := m.ListScheduleOccurrences("work", "alice", schedule.ID); e != nil || len(history) != 0 {
		t.Fatal(history, e)
	}
	if len(m.ListTasks()) != 0 {
		t.Fatal("claim created work")
	}
}

func TestClockRollbackAndPolicyRevisionCannotRepeatRecordedSlot(t *testing.T) {
	m, settings, config, now := scheduleFixture(t)
	schedule, err := m.SaveSchedule("work", "alice", "", 0, settings, config, now)
	if err != nil {
		t.Fatal(err)
	}
	o, err := m.ClaimScheduleOccurrence(schedule, schedule.NextAt.Add(time.Second), now)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.BlockScheduleOccurrence(schedule, *o, "host_unavailable"); err != nil {
		t.Fatal(err)
	}
	// A clock rollback plus reviewed policy edit must not revive that same slot.
	revised, err := m.SaveSchedule("work", "alice", schedule.ID, 1, settings, config, now)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := m.ClaimScheduleOccurrence(revised, revised.NextAt.Add(time.Second), now)
	if err != nil || repeated.State != "blocked" || repeated.Reason != "slot_already_recorded" {
		t.Fatal(repeated, err)
	}
	if len(m.ListTasks()) != 0 {
		t.Fatal("recorded slot replayed")
	}
}
