package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/automation"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Deterministic integration, not a local-model qualification: real owned
// profiles, rooted file operations, dependent children and normalized decisions.
func TestNativeProgramResearchWriterParentSaveUsesOneBudget(t *testing.T) {
	for _, scheduled := range []bool{false, true} {
		t.Run(map[bool]string{false: "interactive", true: "scheduled"}[scheduled], func(t *testing.T) {
			nativeProgramResearchWriterParentSave(t, scheduled)
		})
	}
}

func nativeProgramResearchWriterParentSave(t *testing.T, scheduled bool) {
	s := profileFixture(t)
	ref, folder := approvedExecutionFixture(t, s)
	const source = "Approved test report — café 日本語\nAttendance was 42, measured on 2026-10-25."
	const draft = "Briefing — café 日本語: attendance was 42. Source: source.txt (2026-10-25). Quality outside this mechanical fixture is unverified."
	if err := os.WriteFile(filepath.Join(folder, "source.txt"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	profiles := map[string]*agents.AgentProfile{}
	for _, name := range []string{"Researcher", "Writer"} {
		p, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: name, Model: "model", Instructions: "Return supported findings or a draft; do not modify files.", MaxIterations: 6})
		if err != nil {
			t.Fatal(err)
		}
		profiles[name] = p
	}
	parent, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "Coordinator", Model: "model", MaxIterations: 24, Delegates: []string{profiles["Researcher"].ID, profiles["Writer"].ID}})
	if err != nil {
		t.Fatal(err)
	}
	request := jobSubmitRequest{Prompt: "Read source.txt. Have Researcher inspect the facts and Writer produce a briefing. Save the exact final draft as briefing.txt and verify its bytes.", ProfileID: parent.ID, ProfileRevision: 1, Execution: &ref}
	w := httptest.NewRecorder()
	config, ok := s.resolveJobConfig(w, httptest.NewRequest("POST", "/api/v2/jobs", nil), &request)
	if !ok {
		t.Fatal(w.Code, w.Body.String())
	}
	// This deterministic orchestration fixture returns short fixed decisions.
	// Reserve 1K output explicitly within the same 8K context; reserving the
	// unrelated 2K default makes catalog growth/path length dominate this test.
	// Production allocation and the independent context-overflow tests stay intact.
	config.MaxTokens = 1024
	var mu sync.Mutex
	calls := map[string]int{}
	s.agentRunner = agents.NewRunner(s.agentManager, &browserRunTools{RunTools: s.toolRegistry, server: s}, func(_ context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		if err := s.scheduledTaskAccess(context.Background(), task, ""); err != nil {
			return nil, err
		}
		mu.Lock()
		calls[task.ID]++
		step := calls[task.ID]
		mu.Unlock()
		name, args, text := "", "{}", ""
		if task.ParentID != "" {
			if task.Config.Execution != nil || task.Config.ComputerSession != "" || task.Config.Automation != nil || task.Config.MemoryConsentRevision != 0 {
				t.Error("child acquired independent authority")
			}
			for _, tool := range tools {
				if strings.HasPrefix(tool.Function.Name, "workspace_") || tool.Function.Name == "delegate_tasks" {
					t.Error("child received mutating/recursive tools", tool.Function.Name)
				}
			}
			switch task.Config.Profile.ID {
			case profiles["Researcher"].ID:
				if !strings.Contains(messages[len(messages)-1].StringContent(), source) {
					t.Error("research brief lost exact source")
				}
				text = "Supported finding: attendance was 42. Source: source.txt, dated 2026-10-25."
			case profiles["Writer"].ID:
				if !strings.Contains(messages[len(messages)-1].StringContent(), "Supported finding: attendance was 42") {
					t.Error("writer missing prerequisite")
				}
				text = draft
			default:
				t.Error("unpinned specialist")
			}
		} else {
			switch step {
			case 1:
				name = "workspace_read"
				args = `{"path":"source.txt"}`
			case 2:
				last := messages[len(messages)-1].StringContent()
				var read struct {
					Text string `json:"text"`
				}
				if json.Unmarshal([]byte(last), &read) != nil || read.Text != source {
					t.Error("source read differs", last)
				}
				name = "delegate_tasks"
				data, _ := json.Marshal(map[string]any{"tasks": []map[string]any{{"key": "research", "goal": "Inspect this explicitly selected source, report supported facts and limitations: " + read.Text, "depends_on": []string{}, "tools": []string{}, "profile_id": profiles["Researcher"].ID}, {"key": "write", "goal": "Draft a concise sourced briefing from the research prerequisite. Preserve Unicode.", "depends_on": []string{"research"}, "tools": []string{}, "profile_id": profiles["Writer"].ID}}})
				args = string(data)
			case 3:
				if !strings.Contains(messages[len(messages)-1].StringContent(), draft) {
					t.Error("parent lost writer result")
				}
				name = "workspace_write"
				data, _ := json.Marshal(map[string]string{"path": "briefing.txt", "content": draft})
				args = string(data)
			case 4:
				name = "workspace_read"
				args = `{"path":"briefing.txt"}`
			case 5:
				var read struct {
					Text string `json:"text"`
				}
				if json.Unmarshal([]byte(messages[len(messages)-1].StringContent()), &read) != nil || read.Text != draft {
					t.Error("final read differs")
				}
				text = "Saved and read back briefing.txt; subjective quality is not independently qualified."
			default:
				t.Error("unexpected model call", step)
			}
		}
		message := api.ChatMessage{Role: "assistant", Content: text}
		reason := "stop"
		if name != "" {
			message.ToolCalls = []api.ToolCall{{ID: task.ID + "-" + name, Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}}
			reason = "tool_calls"
		}
		response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: message, FinishReason: reason}}}
		if err := inference.ValidateToolDecision(&api.ChatCompletionRequest{Tools: tools}, response); err != nil {
			return nil, err
		}
		return response, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer s.agentRunner.Shutdown(context.Background())
	s.agentRunner.StartCoordinator(ctx)
	var root *agents.Task
	if scheduled {
		now := time.Date(2026, 10, 26, 8, 59, 57, 0, time.UTC)
		s.schedules = newScheduleController(s, func() time.Time { return now })
		scheduledConfiguration(&config)
		policy, e := s.agentManager.SaveScheduleWithEpoch(s.workspaceID, "local-admin", "", 0, agents.ScheduleSettings{Name: "Collaborative report", Prompt: request.Prompt, Enabled: true, Calendar: automation.Calendar{Zone: "UTC", Hour: 9, Weekdays: []int{1}}}, config, now, s.schedules.epoch)
		if e != nil {
			t.Fatal(e)
		}
		o, e := s.agentManager.ClaimPreparedScheduleOccurrence(policy, config, policy.NextAt.Add(time.Second), now, s.schedules.epoch)
		if e != nil {
			t.Fatal(e)
		}
		pinned, e := agents.ScheduleOccurrenceConfig(policy, *o)
		if e != nil {
			t.Fatal(e)
		}
		root, _, err = s.agentRunner.CreateRequest(request.Prompt, "model", "local-admin", pinned, o.RequestID, o.RequestDigest)
		if err == nil {
			err = s.agentManager.LinkScheduleOccurrence(policy, *o, root.ID)
		}
	} else {
		root, err = s.agentRunner.Create(request.Prompt, "model", "local-admin", config)
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.agentRunner.Continue(ctx, root.ID, root.Actor, "start", "", true); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		root, _ = s.agentManager.GetTask(root.ID)
		if root.Status == agents.TaskCompleted || root.Status == agents.TaskFailed || root.Status == agents.TaskInterrupted {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if root.Status != agents.TaskCompleted || len(root.ChildHistory) != 2 || len(s.agentManager.ListTasks()) != 3 || root.Checkpoint.Iteration+root.Checkpoint.ReservedChildIterations > 24 {
		t.Fatal(root.Status, root.Error, root.Context, root.Steps)
	}
	actual, err := os.ReadFile(filepath.Join(folder, "briefing.txt"))
	if err != nil || string(actual) != draft {
		t.Fatal(string(actual), err)
	}
	unchanged, err := os.ReadFile(filepath.Join(folder, "source.txt"))
	if err != nil || string(unchanged) != source {
		t.Fatal("source mutated", err)
	}
}
