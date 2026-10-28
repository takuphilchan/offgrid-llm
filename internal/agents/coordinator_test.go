package agents

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestScheduleCoordinatorSinglePreparationAndShutdownDrain(t *testing.T) {
	m, _, _, _ := scheduleFixture(t)
	r := NewRunner(m, NewToolRegistry(), nil)
	entered := make(chan struct{})
	exited := make(chan struct{})
	var calls atomic.Int32
	r.Coordinate = func(ctx context.Context, _ time.Time) error {
		calls.Add(1)
		close(entered)
		<-ctx.Done()
		close(exited)
		return ctx.Err()
	}
	r.coordinateOccurrences(context.Background(), time.Now())
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("callback not admitted")
	}
	for i := 0; i < 20; i++ {
		r.coordinateOccurrences(context.Background(), time.Now())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	default:
		t.Fatal("shutdown returned before callback drained")
	}
	r.coordinateOccurrences(context.Background(), time.Now())
	if calls.Load() != 1 {
		t.Fatal("overlap or dispatch after shutdown", calls.Load())
	}
}

func TestScheduleSlowPreparationDoesNotStarveQueuedCanonicalRoot(t *testing.T) {
	m, settings, config, now := scheduleFixture(t)
	policy, err := m.SaveSchedule("work", "alice", "", 0, settings, config, now)
	if err != nil {
		t.Fatal(err)
	}
	o, err := m.ClaimScheduleOccurrence(policy, policy.NextAt.Add(time.Second), now)
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := ScheduleOccurrenceConfig(policy, *o)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRunner(m, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Existing queue progressed."}), nil
	})
	root, _, err := r.CreateRequest(settings.Prompt, config.Profile.Settings.Model, "alice", pinned, o.RequestID, o.RequestDigest)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.LinkScheduleOccurrence(policy, *o, root.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.QueueScheduledRoot(root.ID, root.Actor); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	exited := make(chan struct{})
	r.Coordinate = func(ctx context.Context, _ time.Time) error {
		close(entered)
		<-ctx.Done()
		close(exited)
		return ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer r.Shutdown(context.Background())
	r.StartCoordinator(ctx)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("callback not entered")
	}
	result := awaitTask(t, m, root.ID, TaskCompleted)
	if result.Result != "Existing queue progressed." || len(m.ListTasks()) != 1 {
		t.Fatal(result)
	}
	select {
	case <-exited:
		t.Fatal("callback unexpectedly completed before root")
	default:
	}
}
