package agents

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/automation"
)

func scheduleFixture(t *testing.T) (*Manager, ScheduleSettings, AgentConfig, time.Time) {
	t.Helper()
	m, p := memoryFixture(t)
	config := DefaultAgentConfig()
	config.TaskFirst = true
	config.RestrictComputer = true
	config.Profile = p
	config.Instructions = p.Settings.Instructions
	settings := ScheduleSettings{Name: "Morning report", Prompt: "Read the selected workspace and report actual findings.", Calendar: automation.Calendar{Zone: "Africa/Harare", Hour: 9, Weekdays: []int{0, 1, 2, 3, 4, 5, 6}}, Enabled: true}
	return m, settings, config, time.Date(2026, 10, 25, 6, 0, 0, 0, time.UTC)
}

func TestScheduleImmutablePolicyOwnershipAndDisable(t *testing.T) {
	m, settings, config, now := scheduleFixture(t)
	first, err := m.SaveSchedule("work", "alice", "", 0, settings, config, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.ListTasks()) != 0 {
		t.Fatal("saving scheduled work executed it")
	}
	settings.Name = "Updated report"
	second, err := m.SaveSchedule("work", "alice", first.ID, 1, settings, config, now)
	if err != nil || second.Revision != 2 || second.Digest == first.Digest {
		t.Fatal(second, err)
	}
	old, err := m.GetSchedule("work", "alice", first.ID, 1)
	if err != nil || old.Settings.Name == second.Settings.Name {
		t.Fatal(old, err)
	}
	for _, scope := range [][2]string{{"work", "bob"}, {"another", "alice"}} {
		if _, err = m.GetSchedule(scope[0], scope[1], first.ID, 0); !errors.Is(err, ErrContextNotFound) {
			t.Fatal("foreign read", err)
		}
		if items, e := m.ListSchedules(scope[0], scope[1]); e != nil || len(items) != 0 {
			t.Fatal(items, e)
		}
		if _, err = m.DisableSchedule(scope[0], scope[1], first.ID, 2, now); !errors.Is(err, ErrContextConflict) {
			t.Fatal("foreign disable", err)
		}
	}
	if _, err = m.DisableSchedule("work", "alice", first.ID, 1, now); !errors.Is(err, ErrContextConflict) {
		t.Fatal("stale disable", err)
	}
	disabled, err := m.DisableSchedule("work", "alice", first.ID, 2, now)
	if err != nil || disabled.Settings.Enabled || disabled.Revision != 3 {
		t.Fatal(disabled, err)
	}
	loaded, err := m.GetSchedule("work", "alice", first.ID, 0)
	if err != nil || loaded.Settings.Enabled {
		t.Fatal(loaded, err)
	}
	if due, e := m.DueSchedules("work", now.Add(48*time.Hour)); e != nil || len(due) != 0 {
		t.Fatal(due, e)
	}
	if _, err = m.SaveSchedule("work", "alice", first.ID, 2, settings, config, now); !errors.Is(err, ErrContextConflict) {
		t.Fatal(err)
	}
	if config.Automation != nil {
		t.Fatal("caller config changed")
	}
	first.Config.Profile.Settings.Name = "returned copy changed"
	if config.Profile.Settings.Name == first.Config.Profile.Settings.Name {
		t.Fatal("schedule aliased caller profile")
	}
}

func TestScheduleRejectsForgedProfileAndComputerAuthority(t *testing.T) {
	m, settings, config, now := scheduleFixture(t)
	for _, mutate := range []func(*AgentConfig){func(c *AgentConfig) { c.ComputerAccessOnDemand = true }, func(c *AgentConfig) { c.ComputerSession = "session" }, func(c *AgentConfig) { c.RestrictComputer = false }, func(c *AgentConfig) { c.MaxIterations = 51 }, func(c *AgentConfig) { c.Automation = &ScheduleOrigin{Version: 1} }, func(c *AgentConfig) { p := *c.Profile; p.Actor = "bob"; c.Profile = &p }, func(c *AgentConfig) { p := *c.Profile; p.Digest = "forged"; c.Profile = &p }} {
		changed := config
		mutate(&changed)
		if _, err := m.SaveSchedule("work", "alice", "", 0, settings, changed, now); err == nil {
			t.Fatal("forged policy saved", changed)
		}
	}
	if items, err := m.ListSchedules("work", "alice"); err != nil || len(items) != 0 {
		t.Fatal("partial publication", items, err)
	}
}

func TestScheduleConcurrentRevisionAndCorruptData(t *testing.T) {
	m, settings, config, now := scheduleFixture(t)
	first, err := m.SaveSchedule("work", "alice", "", 0, settings, config, now)
	if err != nil {
		t.Fatal(err)
	}
	other := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, manager := range []*Manager{m, other} {
		wg.Add(1)
		go func(manager *Manager) {
			defer wg.Done()
			_, err := manager.SaveSchedule("work", "alice", first.ID, 1, settings, config, now)
			results <- err
		}(manager)
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrContextConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	db, err := m.scheduleDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`UPDATE agent_schedule_revisions SET snapshot='{}' WHERE schedule_id=? AND revision=2`, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = other.GetSchedule("work", "alice", first.ID, 0); err == nil {
		t.Fatal("corrupt policy accepted")
	}
	if _, err = other.DueSchedules("work", now.Add(24*time.Hour)); err == nil {
		t.Fatal("corrupt policy eligible")
	}
}

func TestSchedulePersistenceFailureAndCursorCorruption(t *testing.T) {
	m, settings, config, now := scheduleFixture(t)
	db, err := m.scheduleDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER schedule_fixture_failure BEFORE INSERT ON agent_schedule_revisions BEGIN SELECT RAISE(ABORT,'fixture failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err = m.SaveSchedule("work", "alice", "", 0, settings, config, now); err == nil {
		t.Fatal("failed persistence reported success")
	}
	if items, e := m.ListSchedules("work", "alice"); e != nil || len(items) != 0 {
		t.Fatal("partial policy", items, e)
	}
	if _, err = db.Exec(`DROP TRIGGER schedule_fixture_failure`); err != nil {
		t.Fatal(err)
	}
	schedule, err := m.SaveSchedule("work", "alice", "", 0, settings, config, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_schedules SET next_at=next_at+120 WHERE id=?`, schedule.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = m.GetSchedule("work", "alice", schedule.ID, 0); err == nil {
		t.Fatal("off-calendar cursor accepted")
	}
}
