package agents

import "testing"

func TestTaskSchedulingSharesWorkIdentityAndPrioritizesConversation(t *testing.T) {
	for _, tc := range []struct {
		task       Task
		work       string
		background bool
	}{
		{Task{ID: "root", WorkID: "work"}, "work", true},
		{Task{ID: "child", ParentID: "root", Worker: &WorkerBinding{WorkID: "work"}}, "work", true},
		{Task{ID: "legacy-child", ParentID: "legacy-root"}, "legacy-root", true},
		{Task{ID: "legacy-root"}, "legacy-root", true},
		{Task{ID: "input", ConversationTurn: &ConversationTurnBinding{}, Origin: &ConversationOrigin{Conversation: "conversation"}}, "conversation", false},
	} {
		tc.task.Actor = "accepted-owner"
		got := taskSchedulingScope(&tc.task)
		if got.Actor != tc.task.Actor || got.Work != tc.work || got.Background != tc.background {
			t.Fatal("incorrect task scheduling identity", got)
		}
	}
}
