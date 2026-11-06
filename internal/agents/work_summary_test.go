package agents

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWorkSummaryDetachedBoundedAndProtected(t *testing.T) {
	now := time.Now()
	m := NewManager(nil, nil, nil)
	m.tasks = map[string]*Task{
		"root": {ID: "root", Prompt: strings.Repeat("界", 1000), Status: TaskCompleted, CreatedAt: now, Result: strings.Repeat("secret-result", 10000), Actor: "private-actor",
			Origin: &ConversationOrigin{Workspace: "w", Conversation: "c", TurnID: "t"}, Config: AgentConfig{KnowledgeDependencies: []string{"source"}}, ChildHistory: []ChildLink{{ID: "child"}}},
		"child":   {ID: "child", ParentID: "root", Status: TaskUncertain},
		"deleted": {ID: "deleted", DeletedAt: &now},
		"unknown": {ID: "unknown", Status: TaskUncertain, Checkpoint: &Checkpoint{ExecutingCall: "call"}},
	}
	rows := m.ListWorkSummaries()
	if len(rows) != 2 {
		t.Fatalf("roots=%d", len(rows))
	}
	for _, row := range rows {
		if row.Deletable {
			t.Fatal("protected work advertised as removable")
		}
		if row.ID == "root" {
			if len([]rune(row.Title)) != 180 || row.Protection != "child_work" {
				t.Fatalf("invalid root: %+v", row)
			}
			row.SourceIDs[0] = "changed"
			row.Origin.Conversation = "changed"
		}
		if row.ID == "unknown" && row.Protection != "unknown_effects" {
			t.Fatal(row.Protection)
		}
	}
	if m.tasks["root"].Config.KnowledgeDependencies[0] != "source" || m.tasks["root"].Origin.Conversation != "c" {
		t.Fatal("summary aliased authoritative task")
	}
	data, _ := json.Marshal(rows)
	for _, secret := range []string{"secret-result", "private-actor", "source_ids", "executing_call"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("summary leaked %s", secret)
		}
	}
	if len(data) > 2000 {
		t.Fatalf("unbounded summary: %d", len(data))
	}
	m.tasks["root"].DeletedAt = &now
	for _, row := range m.ListWorkSummaries() {
		if row.ID == "root" || row.ID == "child" {
			t.Fatal("removed parent history reappeared through a subordinate job")
		}
	}
}

func TestWorkSummarySurvivesRestartWithoutRevivingDeletedWork(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	for _, id := range []string{"kept", "removed"} {
		if _, err := m.CreateTask(id, id, nil); err != nil {
			t.Fatal(err)
		}
		if err := m.CompleteTask(id, "answer", nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.DeleteTask("removed"); err != nil {
		t.Fatal(err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, dir)
	if err := reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	rows := reloaded.ListWorkSummaries()
	if len(rows) != 1 || rows[0].ID != "kept" || !rows[0].Deletable {
		t.Fatalf("incorrect recovered summary: %+v", rows)
	}
}
