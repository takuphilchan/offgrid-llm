package agents

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestUserWorkerAnswerReachesParentAfterSettlement(t *testing.T) {
	for _, route := range []string{"conversation", "worker"} {
		for _, failed := range []bool{false, true} {
			name := route + "/completed"
			if failed {
				name = route + "/failed"
			}
			t.Run(name, func(t *testing.T) {
				m, r, scope, root, child, c := conversationQuestionFixture(t)
				command := questionInputCommand(c, root)
				if route == "conversation" {
					if _, err := r.AcceptConversationInput(scope, command); err != nil {
						t.Fatal(err)
					}
				} else {
					worker, err := m.WorkerSession(scope, child.Worker.SessionID)
					if err != nil {
						t.Fatal(err)
					}
					if _, err = r.CommandWorker(scope, api.WorkerCommand{RequestID: command.RequestID, TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "answer", Payload: api.WorkerCommandPayload{QuestionID: command.Payload.QuestionID, Content: command.Payload.Content}}); err != nil {
						t.Fatal(err)
					}
				}
				var parentCalls atomic.Int32
				r.caller = func(_ context.Context, task *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
					if task.ParentID != "" {
						// The worker deliberately does not repeat the user's decision.
						return runAnswer(api.ChatMessage{Role: "assistant", Content: "Research evidence saved."}), nil
					}
					parentCalls.Add(1)
					assertRecordedUserAnswer(t, messages, command.Payload.Content)
					return runAnswer(api.ChatMessage{Role: "assistant", Content: "Comparison including offline runtimes."}), nil
				}
				if failed {
					if _, err := m.updateTask(child.ID, func(task *Task) error {
						finishTask(task, TaskFailed, "The requested evidence could not be obtained.")
						return nil
					}); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
						t.Fatal(err)
					}
				}
				r.advanceDelegation(context.Background(), root.ID)
				status := TaskCompleted
				if failed {
					status = TaskFailed
				}
				done := awaitTask(t, m, root.ID, status)
				settleTask(t, r, root.ID)
				assertRecordedUserAnswer(t, done.Checkpoint.Messages, command.Payload.Content)
				if done.Delegation != nil && len(done.Delegation.QuestionAnswers) != 0 {
					t.Fatal("settlement retained an answer for duplicate application")
				}
				if !failed && parentCalls.Load() != 1 {
					t.Fatal("parent synthesis did not execute exactly once")
				}
				reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
				if err := reloaded.StorageError(); err != nil {
					t.Fatal(err)
				}
				saved, _ := reloaded.GetTask(root.ID)
				assertRecordedUserAnswer(t, saved.Checkpoint.Messages, command.Payload.Content)
			})
		}
	}
}

func assertRecordedUserAnswer(t *testing.T, messages []api.ChatMessage, content string) {
	t.Helper()
	answers, receipt := 0, -1
	for index, message := range messages {
		if message.Role == "tool" && message.Name == "worker_answer" {
			receipt = index
		}
		if message.Role == "user" && strings.Contains(message.StringContent(), "Recorded user answer") && strings.Contains(message.StringContent(), content) {
			answers++
			if receipt < 0 || receipt >= index {
				t.Error("answer precedes the pending tool receipt")
			}
		}
	}
	if answers != 1 {
		t.Errorf("expected exactly one primary user answer, got %d", answers)
	}
}
