package agents

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSearchRequestsVisibilityAndProjection(t *testing.T) {
	manager := NewManager(nil, nil, nil)
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("task-%02d", i)
		manager.tasks[id] = &Task{ID: id, Actor: "alice", Prompt: "Report " + strings.Repeat("日", 240), CreatedAt: time.Unix(int64(i), 0), Config: AgentConfig{Profile: &AgentProfile{}}}
	}
	manager.tasks["foreign"] = &Task{ID: "foreign", Actor: "bob", Prompt: "Report", Config: AgentConfig{Profile: &AgentProfile{}}}
	manager.tasks["child"] = &Task{ID: "child", Actor: "alice", ParentID: "task-00", Prompt: "Report", Config: AgentConfig{Profile: &AgentProfile{}}}
	manager.tasks["legacy"] = &Task{ID: "legacy", Prompt: "Report"}
	deleted := time.Now()
	manager.tasks["deleted"] = &Task{ID: "deleted", Actor: "alice", Prompt: "Report", DeletedAt: &deleted}
	found, partial, err := manager.SearchRequests(context.Background(), "alice", false, "report", 20)
	if err != nil || !partial || len(found) != 20 || found[0].ID != "task-49" {
		t.Fatalf("%v %v %v", found, partial, err)
	}
	for _, result := range found {
		if !strings.HasPrefix(result.ID, "task-") || len([]rune(result.Title)) != 200 {
			t.Fatal(result)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := manager.SearchRequests(ctx, "alice", false, "report", 20); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestSearchRequestsScanLimitNeverClaimsExhaustiveAbsence(t *testing.T) {
	manager := NewManager(nil, nil, nil)
	for i := 0; i < 4097; i++ {
		id := fmt.Sprint(i)
		manager.tasks[id] = &Task{ID: id, Actor: "alice", Prompt: "ordinary request", Config: AgentConfig{Profile: &AgentProfile{}}}
	}
	found, partial, err := manager.SearchRequests(context.Background(), "alice", false, "missing phrase", 20)
	if err != nil || !partial || len(found) != 0 {
		t.Fatalf("%d %v %v", len(found), partial, err)
	}
}
