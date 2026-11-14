package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationDecisionStopWinsLateForegroundCompletion(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	input := acceptTestConversationInput(t, r, scope, c, "correction")
	started := make(chan struct{})
	r = NewRunner(m, NewToolRegistry(), func(ctx context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if len(task.Steps) == 0 {
			return taskCall("conversation_decide", `{"kind":"correction","instruction_version":1}`), nil
		}
		close(started)
		<-ctx.Done()
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Late reply must not apply the correction"}), nil
	})
	defer r.Shutdown(context.Background())
	foreground, err := r.AdmitConversationTurn(scope, c.ID, input.ID, c.Revision, "model", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Continue(context.Background(), foreground.ID, scope.Actor, "start", "", true); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("foreground did not reach the recorded decision boundary")
	}
	if _, err = r.Stop(foreground.ID, scope.Actor, "cancel", ""); err != nil {
		t.Fatal(err)
	}
	settleTask(t, r, foreground.ID)
	entry, err := m.ConversationInput(scope, c.ID, input.ID)
	if err != nil || entry.Decision == nil || entry.RunStatus != TaskCancelled || entry.AppliedAt != nil {
		t.Fatal("late reply replaced stopped outcome", entry, err)
	}
	if _, err = r.ApplyConversationDecision(scope, c.ID, input.ID); !errors.Is(err, ErrRunConflict) {
		t.Fatal("stopped decision applied", err)
	}
	root, _ := m.GetTask(work.RunID)
	if root.InstructionVersion != 1 || !hasConversationHold(root) {
		t.Fatal("late completion changed root work", root)
	}
}
