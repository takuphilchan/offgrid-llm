package agents

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationInboxOrdersAgainstEffectDispatch(t *testing.T) {
	for _, alreadyDispatched := range []bool{false, true} {
		t.Run(map[bool]string{false: "before_dispatch", true: "after_dispatch"}[alreadyDispatched], func(t *testing.T) {
			m, r, scope, c := controlFixture(t)
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			defer releaseOnce.Do(func() { close(release) })
			tools := &runTestTools{}
			tools.authorize = func(string, json.RawMessage, ToolExecution) error {
				if !alreadyDispatched {
					close(entered)
					<-release
				}
				return nil
			}
			tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) {
				if alreadyDispatched {
					close(entered)
					<-release
				}
				return "Actual operation receipt", nil
			}
			r.tools = tools
			r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return taskCall("write", `{}`), nil
			}
			work := admitControlTestWork(t, r, scope, c)
			done := make(chan *Task, 1)
			failure := make(chan error, 1)
			go func() {
				task, err := r.Continue(context.Background(), work.RunID, scope.Actor, "start", "", false)
				done <- task
				failure <- err
			}()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("operation boundary not reached")
			}
			c, _ = m.Conversation(scope, c.ID)
			if _, err := r.AcceptConversationInput(scope, inboxCommand(c, "input", "Wait for the corrected instructions")); err != nil {
				t.Fatal(err)
			}
			releaseOnce.Do(func() { close(release) })
			var held *Task
			select {
			case held = <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("run did not yield")
			}
			if err := <-failure; err != nil {
				t.Fatal(err)
			}
			if held == nil || held.Status != TaskConversationInput || held.Checkpoint.ExecutingCall != "" || !hasConversationHold(held) {
				t.Fatal("wrong held checkpoint", held)
			}
			if tools.calls.Load() != int32(boolInt(alreadyDispatched)) {
				t.Fatal("dispatch ordering violated", tools.calls.Load())
			}
			receiptFound := false
			for _, step := range held.Steps {
				receiptFound = receiptFound || step.ToolResult == "Actual operation receipt"
			}
			if receiptFound != alreadyDispatched {
				t.Fatal("operation receipt lost or fabricated", held.Steps)
			}
		})
	}
}
