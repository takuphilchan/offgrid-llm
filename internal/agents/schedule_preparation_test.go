package agents

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSchedulePreparationRefreshesDataWithoutRepinningAuthority(t *testing.T) {
	m, settings, config, now := scheduleFixture(t)
	config.KnowledgeDocuments = []KnowledgeDocument{{ID: "reviewed-document", Name: "Original name"}}
	schedule, err := m.SaveSchedule("work", "alice", "", 0, settings, config, now)
	if err != nil {
		t.Fatal(err)
	}
	clone := func() AgentConfig {
		t.Helper()
		data, _ := json.Marshal(schedule.Config)
		var value AgentConfig
		if json.Unmarshal(data, &value) != nil {
			t.Fatal("clone")
		}
		return value
	}
	for _, field := range []string{"document", "model", "budget", "profile", "computer", "oversized"} {
		t.Run(field, func(t *testing.T) {
			changed := clone()
			switch field {
			case "document":
				changed.KnowledgeDocuments[0].ID = "foreign-document"
			case "model":
				changed.Profile.Settings.Model = "replacement"
			case "budget":
				changed.MaxIterations++
			case "profile":
				changed.Profile.Revision++
			case "computer":
				changed.RestrictComputer = false
			case "oversized":
				changed.KnowledgeContext = strings.Repeat("x", 65537)
			}
			if _, err := m.ClaimPreparedScheduleOccurrence(schedule, changed, schedule.NextAt.Add(time.Second), now); err == nil {
				t.Fatal("scope/budget changed")
			}
		})
	}
	fresh := clone()
	fresh.KnowledgeContext = "Fresh bounded evidence — café 日本語"
	fresh.KnowledgeDocuments[0].Name = "Current label"
	o, err := m.ClaimPreparedScheduleOccurrence(schedule, fresh, schedule.NextAt.Add(time.Second), now)
	if err != nil || o.State != "claimed" {
		t.Fatal(o, err)
	}
	pinned, err := PreparedScheduleOccurrenceConfig(schedule, *o, fresh)
	if err != nil || pinned.KnowledgeContext != fresh.KnowledgeContext || pinned.MaxIterations != config.MaxIterations {
		t.Fatal(pinned, err)
	}
	fresh.KnowledgeContext = "A different result cannot replace a persisted request"
	if _, err = PreparedScheduleOccurrenceConfig(schedule, *o, fresh); err == nil {
		t.Fatal("changed persisted request")
	}
	if len(m.ListTasks()) != 0 {
		t.Fatal("preparation dispatched work")
	}
}
