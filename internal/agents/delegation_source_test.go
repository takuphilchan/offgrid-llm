package agents

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestCompletedDelegationRetainsChildSourceRevocation(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	defer r.Shutdown(context.Background())
	if _, err := m.updateTask(child.ID, func(task *Task) error {
		task.Config.KnowledgeDependencies = []string{"child-only-source"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls.Add(1)
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Unauthorized synthesis"}), nil
	}
	r.SourceAccess = func(task *Task) bool { return !containsTool(task.Config.KnowledgeDependencies, "child-only-source") }
	r.advanceDelegation(context.Background(), root.ID)
	done := awaitTask(t, m, root.ID, TaskFailed)
	settleTask(t, r, root.ID)
	if calls.Load() != 0 || done.ErrorCode != "knowledge_unavailable" || !containsTool(done.Config.KnowledgeDependencies, "child-only-source") {
		t.Fatal("completed child output bypassed source revocation", calls.Load(), done.ErrorCode)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	saved, _ := reloaded.GetTask(root.ID)
	if !containsTool(saved.Config.KnowledgeDependencies, "child-only-source") {
		t.Fatal("restart lost derived source provenance")
	}
}
