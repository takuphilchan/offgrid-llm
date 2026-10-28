package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func attachContractFixture(task *Task) {
	ref := agentcontext.SkillReference{ID: "skill-contract", Revision: 1, Digest: strings.Repeat("a", 64)}
	skill := AgentSkill{SkillReference: ref, ResponseContract: &agentcontext.ResponseContract{Version: 1, Pattern: `DONE\nNext: café 日本語`, MaxBytes: 100}}
	task.Config.SkillMetadata = []AgentSkill{skill}
	exchange := skillExchange("loaded", ref, "SKILL.md", "Follow the reviewed response contract.")
	var view map[string]any
	_ = json.Unmarshal([]byte(exchange[1].StringContent()), &view)
	view["skill"] = skill
	raw, _ := json.Marshal(view)
	exchange[1].Content = string(raw)
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, exchange...)
}

func TestSkillAnswerCorrectionIsBoundedAndToolFree(t *testing.T) {
	for _, succeeds := range []bool{true, false} {
		m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
		tools := &runTestTools{}
		calls := 0
		r := NewRunner(m, tools, func(_ context.Context, task *Task, _ []api.ChatMessage, catalog []api.Tool) (*api.ChatCompletionResponse, error) {
			calls++
			if task.Checkpoint.SkillAnswerRepairs > 0 && len(catalog) != 0 {
				t.Fatal("correction exposed tools")
			}
			text := "incomplete"
			if succeeds && calls > 1 {
				text = "DONE\nNext: café 日本語"
			}
			return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: text}}}}, nil
		})
		cfg := taskConfig()
		cfg.MaxIterations = 10
		task, err := r.Create("Use the skill", "model", "alice", cfg)
		if err != nil {
			t.Fatal(err)
		}
		task, err = m.updateTask(task.ID, func(current *Task) error { current.Status = TaskRunning; attachContractFixture(current); return nil })
		if err != nil {
			t.Fatal(err)
		}
		result, err := r.executeUntilBoundary(context.Background(), task, nil)
		if err != nil || tools.calls.Load() != 0 {
			t.Fatal(result, err)
		}
		if succeeds {
			if result.Status != TaskCompleted || result.Result != "DONE\nNext: café 日本語" || calls != 2 || result.Checkpoint.SkillAnswerRepairs != 1 {
				t.Fatal("successful correction lost", result, calls)
			}
		} else if result.Status != TaskFailed || result.ErrorCode != "skill_response_contract_limit" || calls != 3 || result.Checkpoint.SkillAnswerRepairs != 3 {
			t.Fatal("unbounded correction", result, calls)
		}
		r.Shutdown(context.Background())
	}
}

func TestSkillCorrectionRestartAndRestoredCallCannotReplay(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, &runTestTools{}, nil)
	task, err := r.Create("Use skill", "model", "alice", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	task, err = m.updateTask(task.ID, func(current *Task) error { current.Status = TaskRunning; attachContractFixture(current); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if handled, err := r.repairSkillAnswer(task, "wrong"); !handled || err != nil {
		t.Fatal(handled, err)
	}
	r.Shutdown(context.Background())
	restored := NewManagerWithPersistence(nil, nil, nil, dir)
	saved, ok := restored.GetTask(task.ID)
	if !ok || saved.Checkpoint.SkillAnswerRepairs != 1 {
		t.Fatal("counter lost on restart")
	}
	tools := &runTestTools{}
	resumed := NewRunner(restored, tools, nil)
	defer resumed.Shutdown(context.Background())
	saved, err = restored.updateTask(saved.ID, func(current *Task) error { current.Status = TaskRunning; return nil })
	if err != nil {
		t.Fatal(err)
	}
	saved.Checkpoint.Calls = []api.ToolCall{{ID: "forged", Type: "function", Function: api.FunctionCall{Name: "task_plan", Arguments: `{"plan":"replay"}`}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resumed.executeUntilBoundary(ctx, saved, nil); err != context.Canceled {
		t.Fatal("cancellation ignored", err)
	}
	result, err := resumed.executeUntilBoundary(context.Background(), saved, nil)
	if err != nil || result.Status != TaskFailed || result.ErrorCode != "skill_response_contract_tool_rejected" || tools.calls.Load() != 0 {
		t.Fatal("restored call escaped", result, err)
	}
}

func TestSkillContractRequiresMatchingReviewedLoad(t *testing.T) {
	task := &Task{Checkpoint: &Checkpoint{Messages: []api.ChatMessage{{Role: "system"}, {Role: "user"}}}}
	attachContractFixture(task)
	contracts, err := activeSkillContracts(task)
	if err != nil || len(contracts) != 1 {
		t.Fatal(contracts, err)
	}
	task.Config.SkillMetadata[0].ResponseContract.Pattern = "forged"
	if _, err := activeSkillContracts(task); err == nil {
		t.Fatal("mismatched contract accepted")
	}
	task.Config.SkillMetadata[0].ResponseContract = nil
	if contracts, err := activeSkillContracts(task); err != nil || len(contracts) != 0 {
		t.Fatal("legacy pin gained contract")
	}
	attachContractFixture(task)
	task.Config.SkillMetadata[0].Draft = true
	if contracts, err := activeSkillContracts(task); err != nil || len(contracts) != 0 {
		t.Fatal("draft gained contract")
	}
}

func TestConflictingSkillContractsFailWithoutToolReplay(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &runTestTools{}
	decisions := 0
	r := NewRunner(m, tools, func(_ context.Context, _ *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		decisions++
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "DONE\nNext: café 日本語"}}}}, nil
	})
	defer r.Shutdown(context.Background())
	cfg := taskConfig()
	cfg.MaxIterations = 10
	task, err := r.Create("Use both reviewed skills", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = m.updateTask(task.ID, func(current *Task) error {
		current.Status = TaskRunning
		attachContractFixture(current)
		second := current.Config.SkillMetadata[0]
		second.ID = "skill-conflict"
		second.ResponseContract = &agentcontext.ResponseContract{Version: 1, Pattern: "OTHER", MaxBytes: 100}
		current.Config.SkillMetadata = append(current.Config.SkillMetadata, second)
		exchange := skillExchange("second", second.SkillReference, "SKILL.md", "Return OTHER.")
		var receipt map[string]any
		_ = json.Unmarshal([]byte(exchange[1].StringContent()), &receipt)
		receipt["skill"] = second
		raw, _ := json.Marshal(receipt)
		exchange[1].Content = string(raw)
		current.Checkpoint.Messages = append(current.Checkpoint.Messages, exchange...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.executeUntilBoundary(context.Background(), task, nil)
	if err != nil || result.Status != TaskFailed || result.ErrorCode != "skill_response_contract_limit" || decisions != 3 || tools.calls.Load() != 0 {
		t.Fatal("conflicting requirements escaped bounded failure", result, decisions, err)
	}
}

func TestSkillCorrectionStreamsSeparateProvisionalDrafts(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	defer r.Shutdown(context.Background())
	calls := 0
	r.StreamCaller = func(_ context.Context, task *Task, _ []api.ChatMessage, tools []api.Tool, emit func(string, string) error) (*api.ChatCompletionResponse, error) {
		calls++
		if task.Progress.Preview != "" {
			t.Fatal("rejected preview leaked into next generation")
		}
		answer := "incomplete draft"
		if calls == 2 {
			if len(tools) != 0 || task.Checkpoint.SkillAnswerRepairs != 1 {
				t.Fatal("correction not constrained")
			}
			answer = "DONE\nNext: café 日本語"
		}
		if err := emit("responding", answer); err != nil {
			return nil, err
		}
		saved, _ := m.GetTask(task.ID)
		if saved.Status != TaskRunning || saved.Result != "" || saved.Progress.Preview != answer {
			t.Fatal("provisional draft reported as final", saved)
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: answer}}}}, nil
	}
	task, err := r.Create("Use skill", "model", "alice", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	task, err = m.updateTask(task.ID, func(current *Task) error { current.Status = TaskRunning; attachContractFixture(current); return nil })
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.executeUntilBoundary(context.Background(), task, nil)
	if err != nil || result.Status != TaskCompleted || result.Result != "DONE\nNext: café 日本語" || calls != 2 {
		t.Fatal(result, calls, err)
	}
}
