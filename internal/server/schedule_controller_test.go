package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/automation"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func scheduleHTTP(s *Server, method, path string, body any) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	s.handleAgentSchedules(w, httptest.NewRequest(method, path, strings.NewReader(string(data))))
	return w
}

func scheduledFixture(t *testing.T, ref *execution.Reference) (*Server, *agents.AgentSchedule, *time.Time) {
	t.Helper()
	s := profileFixture(t)
	return createScheduleFixture(t, s, ref)
}

func createScheduleFixture(t *testing.T, s *Server, ref *execution.Reference) (*Server, *agents.AgentSchedule, *time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 26, 8, 59, 57, 0, time.UTC)
	s.schedules = newScheduleController(s, func() time.Time { return now })
	p, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "Scheduled reporter", Model: "model", MaxIterations: 6})
	if err != nil {
		t.Fatal(err)
	}
	body := scheduleWriteRequest{ProfileID: p.ID, ProfileRevision: p.Revision, Execution: ref, Settings: agents.ScheduleSettings{Name: "Morning report", Prompt: "Create a disposable exact Unicode report in the selected folder and read it back.", Enabled: true, Calendar: automation.Calendar{Zone: "UTC", Hour: 9, Weekdays: []int{0, 1, 2, 3, 4, 5, 6}}}}
	w := scheduleHTTP(s, "POST", "/api/v2/agent-schedules", body)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var view struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(w.Body.Bytes(), &view) != nil || view.ID == "" {
		t.Fatal(w.Body.String())
	}
	schedule, err := s.agentManager.GetSchedule(s.workspaceID, "local-admin", view.ID, 0)
	if err != nil || len(s.agentManager.ListTasks()) != 0 {
		t.Fatal(schedule, err, "creation executed work")
	}
	if err = s.schedules.coordinate(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	return s, schedule, &now
}

func TestScheduleControllerCanonicalWorkspaceFlowAndNoReplay(t *testing.T) {
	s := profileFixture(t)
	ref, folder := approvedExecutionFixture(t, s)
	s, schedule, now := createScheduleFixture(t, s, &ref)
	const wanted = "Scheduled report — café 日本語\n1| genuine content"
	var calls atomic.Int32
	s.agentRunner = agents.NewRunner(s.agentManager, &browserRunTools{RunTools: s.toolRegistry, server: s}, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if err := s.scheduledTaskAccess(ctx, task, ""); err != nil {
			return nil, err
		}
		if task.Config.MaxIterations != 6 || task.Config.Automation == nil || task.Config.ComputerAccessOnDemand {
			t.Error("authority or budget changed")
		}
		name, args := "", "{}"
		switch calls.Add(1) {
		case 1:
			name = "workspace_write"
			data, _ := json.Marshal(map[string]string{"path": "report.txt", "content": wanted})
			args = string(data)
		case 2:
			name = "workspace_read"
			args = `{"path":"report.txt"}`
		case 3:
			last := messages[len(messages)-1].StringContent()
			var read struct {
				Text string `json:"text"`
			}
			if json.Unmarshal([]byte(last), &read) != nil || read.Text != wanted {
				t.Error("read-back differs", last)
			}
		default:
			t.Error("unexpected extra inference")
		}
		message := api.ChatMessage{Role: "assistant", Content: "Exact report read back."}
		reason := "stop"
		if name != "" {
			message.Content = ""
			message.ToolCalls = []api.ToolCall{{ID: name, Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}}
			reason = "tool_calls"
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: message, FinishReason: reason}}}, nil
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.agentRunner.Shutdown(ctx)
	})
	*now = schedule.NextAt.Add(time.Second)
	prep, cancel := context.WithCancel(context.Background())
	if err := s.schedules.coordinate(prep, *now); err != nil {
		t.Fatal(err)
	}
	cancel() // Preparation ending must not cancel the independently owned root.
	deadline := time.Now().Add(5 * time.Second)
	var root *agents.Task
	for time.Now().Before(deadline) {
		items := s.agentManager.ListTasks()
		if len(items) == 1 {
			root = items[0]
			if root.Status == agents.TaskCompleted || root.Status == agents.TaskFailed || root.Status == agents.TaskInterrupted {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if root == nil || root.Status != agents.TaskCompleted || calls.Load() != 3 {
		t.Fatal(root, calls.Load())
	}
	actual, err := os.ReadFile(filepath.Join(folder, "report.txt"))
	if err != nil || string(actual) != wanted {
		t.Fatal(string(actual), err)
	}
	for i := 0; i < 3; i++ {
		*now = now.Add(time.Second)
		if err = s.schedules.coordinate(context.Background(), *now); err != nil {
			t.Fatal(err)
		}
	}
	history, err := s.agentManager.ListScheduleOccurrences(s.workspaceID, "local-admin", schedule.ID)
	if err != nil || len(history) != 1 || history[0].JobID != root.ID || history[0].State != "completed" || len(s.agentManager.ListTasks()) != 1 {
		t.Fatal(history, err)
	}
	for _, path := range []string{"/api/v2/agent-schedules", "/api/v2/agent-schedules/" + schedule.ID, "/api/v2/agent-schedules/" + schedule.ID + "/occurrences"} {
		if w := scheduleHTTP(s, "GET", path, nil); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if calls.Load() != 3 {
		t.Fatal("inspection replayed work")
	}
}

func TestScheduleControllerMissedAndUnavailableHostNeverCatchUp(t *testing.T) {
	for _, missed := range []bool{false, true} {
		t.Run(map[bool]string{false: "host revoked", true: "clock gap"}[missed], func(t *testing.T) {
			s := profileFixture(t)
			ref, _ := approvedExecutionFixture(t, s)
			s, schedule, now := createScheduleFixture(t, s, &ref)
			if !missed {
				if err := s.executionHosts.Revoke("local-admin", s.workspaceID, ref); err != nil {
					t.Fatal(err)
				}
			}
			*now = schedule.NextAt.Add(time.Second)
			if missed {
				*now = now.Add(2 * time.Hour)
			}
			if err := s.schedules.coordinate(context.Background(), *now); err != nil {
				t.Fatal(err)
			}
			history, err := s.agentManager.ListScheduleOccurrences(s.workspaceID, "local-admin", schedule.ID)
			want := "blocked"
			if missed {
				want = "missed"
			}
			if err != nil || len(history) != 1 || history[0].State != want || len(s.agentManager.ListTasks()) != 0 {
				t.Fatal(history, err)
			}
			*now = now.Add(time.Second)
			if err = s.schedules.coordinate(context.Background(), *now); err != nil {
				t.Fatal(err)
			}
			if len(s.agentManager.ListTasks()) != 0 {
				t.Fatal("replayed blocked occurrence")
			}
		})
	}
}

func TestScheduleDispatchRevocationAndNoComputerAuthority(t *testing.T) {
	s, schedule, now := scheduledFixture(t, nil)
	*now = schedule.NextAt.Add(time.Second)
	o, err := s.agentManager.ClaimPreparedScheduleOccurrence(schedule, schedule.Config, *now, s.schedules.boot, s.schedules.epoch)
	if err != nil {
		t.Fatal(err)
	}
	config, err := agents.ScheduleOccurrenceConfig(schedule, *o)
	if err != nil {
		t.Fatal(err)
	}
	root, _, err := s.agentRunner.CreateRequest(schedule.Settings.Prompt, "model", "local-admin", config, o.RequestID, o.RequestDigest)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.agentManager.LinkScheduleOccurrence(schedule, *o, root.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.scheduledTaskAccess(context.Background(), root, ""); err != nil {
		t.Fatal(err)
	}
	if err = s.scheduledTaskAccess(context.Background(), root, computerAccessToolName); err == nil {
		t.Fatal("scheduled desktop access")
	}
	if _, err = s.agentManager.DisableSchedule(s.workspaceID, root.Actor, schedule.ID, 1, *now); err != nil {
		t.Fatal(err)
	}
	if err = s.scheduledTaskAccess(context.Background(), root, "calculator"); err == nil {
		t.Fatal("disabled policy dispatched")
	}
	if _, err = s.agentManager.SaveSchedule(s.workspaceID, root.Actor, schedule.ID, 2, schedule.Settings, schedule.Config, *now); err != nil {
		t.Fatal(err)
	}
	if err = s.scheduledTaskAccess(context.Background(), root, "calculator"); err == nil {
		t.Fatal("re-enable revived original authority")
	}
}
