package agents

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type browserRecoveryTools struct {
	runTestTools
	source string
}

func (t *browserRecoveryTools) Capability(name string) (capabilities.Descriptor, bool) {
	risk := capabilities.RiskLow
	if name == "browser_click" {
		risk = capabilities.RiskHigh
	}
	return capabilities.Descriptor{Name: name, Kind: capabilities.Computer, Namespace: "computer", Source: t.source, Risk: risk, Revision: capabilities.BrowserEffectsRevision}, true
}

func TestBrowserReadRepairIsBoundedDurableAndNeverUsesErrorText(t *testing.T) {
	for _, mode := range []string{"safe", "unknown", "external"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			tools := &browserRecoveryTools{source: "local-companion"}
			if mode == "external" {
				tools.source = "mcp:untrusted"
			}
			tools.authorize = func(string, json.RawMessage, ToolExecution) error { return nil }
			tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) {
				if mode == "unknown" {
					return "", errors.New("computer_browser_observation_failed")
				}
				return "", &ToolFailure{Code: "computer_browser_observation_failed", Message: "Read failed", BeforeEffects: true, Recovery: "Inspect again"}
			}
			calls := 0
			r := NewRunner(NewManagerWithPersistence(nil, nil, nil, dir), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				return observationCall("browser_observe"), nil
			})
			cfg := observationConfig()
			cfg.ComputerDriver = "browser"
			cfg.ComputerScope = ""
			task, err := r.Create("Inspect page", "model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if mode == "safe" {
				if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != 4 {
					t.Fatalf("unbounded repair: status=%s code=%s calls=%d err=%v", got.Status, got.ErrorCode, calls, err)
				}
				if err := r.Shutdown(context.Background()); err != nil {
					t.Fatal(err)
				}
				saved, ok := NewManagerWithPersistence(nil, nil, nil, dir).GetTask(task.ID)
				if !ok || !saved.Checkpoint.PreEffectRepair || saved.Checkpoint.ExecutingCall != "" || len(saved.Checkpoint.Calls) != 0 {
					t.Fatal("lost durable repair")
				}
			} else if calls != 1 {
				t.Fatalf("untrusted/unknown failure retried %d times", calls)
			}
			if mode == "unknown" && (err != nil || got.Status != TaskFailed || got.ErrorCode != "tool_read_failed") {
				t.Fatal("read-only failure gained replay authority or mutation uncertainty")
			}
		})
	}
}

func TestBrowserRepairRequiresPinnedDescriptorAndActualProgress(t *testing.T) {
	d := capabilities.Descriptor{Name: "browser_click", Kind: capabilities.Computer, Namespace: "computer", Source: "local-companion", Risk: capabilities.RiskHigh, Revision: capabilities.BrowserEffectsRevision}
	f := &ToolFailure{Code: "computer_browser_precondition_failed", BeforeEffects: true}
	if !trustedBrowserRepair(d, f) {
		t.Fatal("trusted precondition rejected")
	}
	for _, change := range []func(*capabilities.Descriptor){func(d *capabilities.Descriptor) { d.Source = "mcp:local-companion" }, func(d *capabilities.Descriptor) { d.Revision = "" }, func(d *capabilities.Descriptor) { d.Kind = capabilities.Read }} {
		other := d
		change(&other)
		if trustedBrowserRepair(other, f) {
			t.Fatal("forged descriptor trusted")
		}
	}
	cp := &Checkpoint{PreEffectRepair: true, PreEffectRepairTool: "browser_click", ProposalTool: "browser_click", PreEffectRepairs: 2}
	read := d
	read.Name = "browser_observe"
	read.Risk = capabilities.RiskLow
	resolvePreEffectRepair(cp, "browser_observe", read)
	if !cp.PreEffectRepair || cp.PreEffectRepairs != 2 {
		t.Fatal("inspection erased pending click repair")
	}
	resolvePreEffectRepair(cp, "browser_click", d)
	if cp.PreEffectRepair || cp.PreEffectRepairs != 0 {
		t.Fatal("actual progress did not clear repair")
	}
}

func TestBrowserAuthorizationRepairRetainsItsBudgetAcrossInspection(t *testing.T) {
	tools := &browserRecoveryTools{source: "local-companion"}
	tools.authorize = func(name string, _ json.RawMessage, _ ToolExecution) error {
		if name == "browser_click" {
			return &ToolAuthorizationError{Code: "computer_browser_precondition_failed", Message: "stale"}
		}
		return nil
	}
	tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) { return `{"observation_id":"fresh"}`, nil }
	calls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls%2 == 1 {
			return observationCall("browser_observe"), nil
		}
		return observationCall("browser_click"), nil
	})
	cfg := observationConfig()
	cfg.ComputerDriver = "browser"
	cfg.ComputerScope = ""
	task, err := r.Create("Update form", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != 8 || !got.Checkpoint.PreEffectRepair {
		t.Fatalf("inspection erased bounded repair: calls=%d status=%s code=%s err=%v", calls, got.Status, got.ErrorCode, err)
	}
}
