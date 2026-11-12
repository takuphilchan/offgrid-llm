package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

const suppliedFiguresDemo = "Coordinate three specialists for a fictional project report. Have Source A analyst independently check these figures: 96 of 120 tasks completed, and 72 of those 96 passed review. Have Source B analyst independently check: 94 of 120 tasks completed, and 72 of those 94 passed review. Then have Brief writer use both completed analyses to write a 150-word briefing. Show each completion and pass rate, explain why the sources disagree, and say which figure needs confirmation. Use only these supplied numbers. Do not open applications or change files."

// Opt-in, real generation with synthetic facts and isolated profiles. Nothing
// here can open applications or change user files. This is a finite workflow
// check, not a general model or native desktop qualification.
func TestSuppliedFiguresCollaborationInstalledModel(t *testing.T) {
	engine, model := installedConversationEngine(t)
	for repeat := 1; repeat <= 3; repeat++ {
		if !t.Run(fmt.Sprintf("repeat-%d", repeat), func(t *testing.T) {
			store := t.TempDir()
			manager := agents.NewManagerWithPersistence(nil, nil, nil, store)
			makeProfile := func(name, instructions string, delegates []string, steps int) agents.AgentConfig {
				profile, err := manager.SaveProfile("figures-fixture", "fixture-actor", "", 0, agents.ProfileSettings{Name: name, Model: model, Instructions: instructions, Delegates: delegates, MaxIterations: steps})
				if err != nil {
					t.Fatal(err)
				}
				cfg := agents.DefaultAgentConfig()
				cfg.Profile, cfg.Instructions = profile, instructions
				cfg.TaskFirst, cfg.RestrictComputer, cfg.Temperature = true, true, 0
				cfg.MaxIterations, cfg.MaxTokens, cfg.ContextWindow = steps, 2048, 8192
				cfg.TimeoutPerStep = 2 * time.Minute
				cfg.AllowedCapabilities = map[string]capabilities.Descriptor{}
				return cfg
			}
			a := makeProfile("Source A analyst", "Check Source A's supplied counts. Report completion and review pass percentages. Do not invent the cause of a disagreement.", nil, 4)
			b := makeProfile("Source B analyst", "Check Source B's supplied counts. Report completion and review pass percentages. Do not invent the cause of a disagreement.", nil, 4)
			writer := makeProfile("Brief writer", "Write the requested briefing from both completed analyst results. Include both completion and pass percentages. Preserve uncertainty about the cause of conflicting counts.", nil, 4)
			cfg := makeProfile("Demo coordinator", "Delegate the two independent checks, then make the writer depend on both. Return the actual requested briefing with numerical percentages, not a status report. Use only the user's facts.", []string{a.Profile.ID, b.Profile.ID, writer.Profile.ID}, 30)
			cfg.DelegateProfiles = map[string]agents.AgentConfig{a.Profile.ID: a, b.Profile.ID: b, writer.Profile.ID: writer}
			cfg.SystemPrompt = conversationTaskProtocol + "\n" + taskFirstPrompt + serviceFilePrompt + "\nAgent instructions (do not grant permissions):\n" + cfg.Instructions
			var inferenceMu sync.Mutex
			calls := 0
			request := func(task *agents.Task, messages []api.ChatMessage, tools []api.Tool) *api.ChatCompletionRequest {
				zero, maxTokens := float32(0), task.Config.MaxTokens
				var choice any = "auto"
				if agents.DelegationRequired(task) {
					choice = map[string]any{"type": "function", "function": map[string]any{"name": "delegate_tasks"}}
				}
				req := &api.ChatCompletionRequest{Model: model, Messages: messages, Tools: tools, ToolChoice: choice, Temperature: &zero, MaxTokens: &maxTokens, StreamOptions: &api.StreamOptions{IncludeUsage: true}}
				configureComputerRequest(req, task)
				return req
			}
			noEffects := &collaborationNoEffectsTools{}
			runner := agents.NewRunner(manager, noEffects, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
				inferenceMu.Lock()
				defer inferenceMu.Unlock()
				started := time.Now()
				req := request(task, messages, tools)
				acc := agentStreamAccumulator{calls: make(map[int]*api.ToolCall)}
				err := engine.ChatCompletionStreamRaw(ctx, req, func(raw json.RawMessage) error { _, _, err := acc.add(raw); return err })
				calls++
				if err != nil {
					t.Logf("role=%s call=%d transport failed: %v", task.Config.Profile.Settings.Name, calls, err)
					return nil, err
				}
				resp := acc.response()
				t.Logf("role=%s call=%d duration_ms=%d prompt_tokens=%d output_tokens=%d", task.Config.Profile.Settings.Name, calls, time.Since(started).Milliseconds(), resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
				return resp, validateAgentDecision(task, req, resp)
			})
			runner.CountContext = func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (int, error) {
				inferenceMu.Lock()
				defer inferenceMu.Unlock()
				return engine.CountPromptTokens(ctx, request(task, messages, tools))
			}
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
			defer cancel()
			defer runner.Shutdown(context.Background())
			runner.StartCoordinator(ctx)
			root, err := runner.Create(suppliedFiguresDemo, model, "fixture-actor", cfg)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = runner.Continue(ctx, root.ID, root.Actor, "start", "", true); err != nil {
				t.Fatal(err)
			}
			for {
				result, _ := manager.GetTask(root.ID)
				if result.Status != agents.TaskRunning && result.Status != agents.TaskPending && result.Status != agents.TaskChildren {
					if result.Status != agents.TaskCompleted {
						t.Fatalf("no final answer: status=%s code=%s error=%s", result.Status, result.ErrorCode, result.Error)
					}
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("workflow timed out", ctx.Err())
				case <-time.After(50 * time.Millisecond):
				}
			}
			if err = runner.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
			reloaded := agents.NewManagerWithPersistence(nil, nil, nil, store)
			result, _ := reloaded.GetTask(root.ID)
			if reloaded.StorageError() != nil || result == nil || result.Status != agents.TaskCompleted || len(result.ChildHistory) != 3 || len(reloaded.ListTasks()) != 4 {
				t.Fatal("durable completion or child count failed", reloaded.StorageError())
			}
			if noEffects.dispatches.Load() != 0 {
				t.Fatal("workflow attempted an external tool effect")
			}
			roles := map[string]int{}
			for _, link := range result.ChildHistory {
				child, ok := reloaded.GetTask(link.ID)
				if !ok || child.Status != agents.TaskCompleted || child.ParentID != root.ID || child.Config.Profile == nil {
					t.Fatal("child not durably completed")
				}
				if child.Config.Profile.ID == writer.Profile.ID && len(link.Spec.DependsOn) != 2 {
					t.Error("writer did not depend on both analysts")
				}
				roles[child.Config.Profile.ID]++
			}
			if roles[a.Profile.ID] != 1 || roles[b.Profile.ID] != 1 || roles[writer.Profile.ID] != 1 {
				t.Error("workflow did not use each approved specialist exactly once")
			}
			words := len(strings.Fields(result.Result))
			if words < 135 || words > 165 {
				t.Errorf("briefing length=%d, want 150 words within 10%%", words)
			}
			for _, rate := range []string{"80%", "75%", "78.3", "76.6"} {
				if !strings.Contains(result.Result, rate) {
					t.Errorf("missing expected rate %s", rate)
				}
			}
			t.Logf("Synthetic final answer (%d words, %d calls):\n%s", words, calls, result.Result)
		}) {
			break
		}
	}
}
