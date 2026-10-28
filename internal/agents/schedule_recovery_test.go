package agents

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestScheduleCrashBoundariesObserveWithoutCreatingOrResuming(t *testing.T) {
	for _, phase := range []string{"claimed", "created", "linked", "completed"} {
		t.Run(phase, func(t *testing.T) {
			m, settings, config, now := scheduleFixture(t)
			schedule, err := m.SaveSchedule("work", "alice", "", 0, settings, config, now)
			if err != nil {
				t.Fatal(err)
			}
			o, err := m.ClaimScheduleOccurrence(schedule, schedule.NextAt.Add(time.Second), now)
			if err != nil {
				t.Fatal(err)
			}
			rootID := ""
			if phase != "claimed" {
				rootConfig, err := ScheduleOccurrenceConfig(schedule, *o)
				if err != nil {
					t.Fatal(err)
				}
				runner := NewRunner(m, NewToolRegistry(), nil)
				root, _, err := runner.CreateRequest(settings.Prompt, config.Profile.Settings.Model, "alice", rootConfig, o.RequestID, o.RequestDigest)
				if err != nil {
					t.Fatal(err)
				}
				rootID = root.ID
				if phase == "linked" || phase == "completed" {
					if err = m.LinkScheduleOccurrence(schedule, *o, root.ID); err != nil {
						t.Fatal(err)
					}
				}
				if phase == "completed" {
					if err = m.CompleteTask(root.ID, "persisted answer", nil); err != nil {
						t.Fatal(err)
					}
				}
			}
			restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
			resumed := 0
			runner := NewRunner(restored, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				resumed++
				return nil, nil
			})
			if err = runner.ReconcileScheduleLedger(context.Background(), "work", true); err != nil {
				t.Fatal(err)
			}
			if err = runner.ReconcileScheduleLedger(context.Background(), "work", true); err != nil {
				t.Fatal("duplicate recovery", err)
			}
			history, err := restored.ListScheduleOccurrences("work", "alice", schedule.ID)
			if err != nil || len(history) != 1 {
				t.Fatal(history, err)
			}
			want := "interrupted"
			if phase == "claimed" {
				want = "missed"
			}
			if phase == "completed" {
				want = "completed"
			}
			if history[0].State != want || history[0].JobID != rootID || resumed != 0 {
				t.Fatal(history, rootID, resumed)
			}
			count := 0
			if rootID != "" {
				count = 1
			}
			if len(restored.ListTasks()) != count {
				t.Fatal("recovery created a root")
			}
		})
	}
}

func TestScheduleUnsettledWorkerAndUnknownEffectBlockOverlap(t *testing.T) {
	m, settings, config, now := scheduleFixture(t)
	schedule, err := m.SaveSchedule("work", "alice", "", 0, settings, config, now)
	if err != nil {
		t.Fatal(err)
	}
	o, err := m.ClaimScheduleOccurrence(schedule, schedule.NextAt.Add(time.Second), now)
	if err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(m, NewToolRegistry(), nil)
	rootConfig, err := ScheduleOccurrenceConfig(schedule, *o)
	if err != nil {
		t.Fatal(err)
	}
	root, _, err := runner.CreateRequest(settings.Prompt, config.Profile.Settings.Model, "alice", rootConfig, o.RequestID, o.RequestDigest)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.LinkScheduleOccurrence(schedule, *o, root.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.CompleteTask(root.ID, "done", nil); err != nil {
		t.Fatal(err)
	}
	runner.active[root.ID] = func() {} // A terminal row is not proof a worker drained.
	if err = runner.ReconcileScheduleLedger(context.Background(), "work", false); err != nil {
		t.Fatal(err)
	}
	history, _ := m.ListScheduleOccurrences("work", "alice", schedule.ID)
	if history[0].State != "admitted" {
		t.Fatal("settling worker certified", history)
	}
	delete(runner.active, root.ID)
	_, err = m.updateTask(root.ID, func(task *Task) error {
		task.Status = TaskUncertain
		task.Checkpoint.ExecutingCall = "unsettled"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = runner.ReconcileScheduleLedger(context.Background(), "work", false); err != nil {
		t.Fatal(err)
	}
	history, _ = m.ListScheduleOccurrences("work", "alice", schedule.ID)
	if history[0].State != "interrupted" {
		t.Fatal(history)
	}
	next, err := m.GetSchedule("work", "alice", schedule.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := m.ClaimScheduleOccurrence(next, next.NextAt.Add(time.Second), now)
	if err != nil || blocked.Reason != "previous_work_unsettled" {
		t.Fatal(blocked, err)
	}
}

func TestOccurrenceConcurrentClaimsOnlyOneWins(t *testing.T) {
	m, settings, config, now := scheduleFixture(t)
	schedule, err := m.SaveSchedule("work", "alice", "", 0, settings, config, now)
	if err != nil {
		t.Fatal(err)
	}
	other := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, manager := range []*Manager{m, other} {
		wg.Add(1)
		go func(manager *Manager) {
			defer wg.Done()
			_, err := manager.ClaimScheduleOccurrence(schedule, schedule.NextAt.Add(time.Second), now)
			results <- err
		}(manager)
	}
	wg.Wait()
	close(results)
	success, fail := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else {
			fail++
		}
	}
	if success != 1 || fail != 1 {
		t.Fatal(success, fail)
	}
	history, err := m.ListScheduleOccurrences("work", "alice", schedule.ID)
	if err != nil || len(history) != 1 {
		t.Fatal(history, err)
	}
}
