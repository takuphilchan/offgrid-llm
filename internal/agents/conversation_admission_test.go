package agents

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestHeldConversationAdmissionIsIdempotentAndCannotExecute(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	var calls atomic.Int32
	r := NewRunner(m, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls.Add(1)
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Hello"}), nil
	})
	origin := ConversationOrigin{Workspace: "workspace", Conversation: "chat", TurnID: "turn-1"}
	ids := make(chan string, 8)
	var wg sync.WaitGroup
	var fresh atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, created, err := r.CreateHeldRequest("hello", "model", "alice", DefaultAgentConfig(), "request-1", "digest", origin)
			if err != nil {
				t.Error(err)
				return
			}
			if created {
				fresh.Add(1)
			}
			ids <- task.ID
		}()
	}
	wg.Wait()
	close(ids)
	var id string
	for current := range ids {
		if id != "" && current != id {
			t.Fatal("duplicate canonical job")
		}
		id = current
	}
	if fresh.Load() != 1 || id == "" {
		t.Fatal("incorrect admission count", fresh.Load())
	}
	for _, action := range []string{"start", "resume", "approve"} {
		if _, err := r.Continue(context.Background(), id, "alice", action, "", false); !errors.Is(err, ErrRunConflict) {
			t.Fatal("held job executed", action, err)
		}
	}
	if err := m.StartTask(id); !errors.Is(err, ErrRunConflict) {
		t.Fatal("legacy start bypassed hold", err)
	}
	if calls.Load() != 0 {
		t.Fatal("inference before linkage")
	}
	if _, _, err := r.CreateHeldRequest("different", "model", "alice", DefaultAgentConfig(), "request-1", "changed", origin); !errors.Is(err, ErrRunConflict) {
		t.Fatal("conflicting identity accepted", err)
	}
	if _, _, err := r.CreateRequest("hello", "model", "alice", DefaultAgentConfig(), "request-1", "digest"); !errors.Is(err, ErrRunConflict) {
		t.Fatal("unlinked request borrowed conversation job", err)
	}
	other := origin
	other.Conversation = "another"
	if _, err := r.ReleaseConversationAdmission(id, "alice", "digest", other); !errors.Is(err, ErrRunConflict) {
		t.Fatal("wrong origin released", err)
	}
	if _, err := r.ReleaseConversationAdmission(id, "bob", "digest", origin); !errors.Is(err, ErrTaskNotFound) {
		t.Fatal("cross-actor release", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := r.ReleaseConversationAdmission(id, "alice", "digest", origin); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("release itself dispatched inference")
	}
	done, err := r.Continue(context.Background(), id, "alice", "start", "", false)
	if err != nil || done.Status != TaskCompleted || calls.Load() != 1 {
		t.Fatal(done, err)
	}
	if _, err := r.ReleaseConversationAdmission(id, "alice", "digest", origin); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("completed admission replayed")
	}
}

func TestHeldConversationSurvivesRestartAndFailedRelease(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, &runTestTools{}, nil)
	origin := ConversationOrigin{Workspace: "workspace", Conversation: "chat", TurnID: "turn-1"}
	task, _, err := r.CreateHeldRequest("hello", "model", "alice", DefaultAgentConfig(), "request-1", "digest", origin)
	if err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(broken, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	m.dataDir = broken
	if _, err := r.ReleaseConversationAdmission(task.ID, "alice", "digest", origin); err == nil {
		t.Fatal("failed persistence acknowledged")
	}
	current, _ := m.GetTask(task.ID)
	if !current.AdmissionHeld {
		t.Fatal("memory released before persistence")
	}
	m.dataDir = dir
	restarted := NewManagerWithPersistence(nil, nil, nil, dir)
	if err := restarted.StorageError(); err != nil {
		t.Fatal(err)
	}
	r = NewRunner(restarted, &runTestTools{}, nil)
	retry, fresh, err := r.CreateHeldRequest("hello", "model", "alice", DefaultAgentConfig(), "request-1", "digest", origin)
	if err != nil || fresh || retry.ID != task.ID || !retry.AdmissionHeld || retry.Status != TaskInterrupted {
		t.Fatal("restart changed admission", retry, fresh, err)
	}
	if _, err := r.Continue(context.Background(), task.ID, "alice", "resume", "", false); !errors.Is(err, ErrRunConflict) {
		t.Fatal("restart bypassed hold", err)
	}
	released, err := r.ReleaseConversationAdmission(task.ID, "alice", "digest", origin)
	if err != nil || released.AdmissionHeld || released.Status != TaskInterrupted {
		t.Fatal("bookkeeping resumed interrupted task", released, err)
	}
}
