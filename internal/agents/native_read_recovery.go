package agents

import (
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// NativeReadRepair is durable progress state, not permission to repeat an input.
// It shares the proposal-repair budget rather than creating an unbounded loop.
type NativeReadRepair struct {
	Tool        string `json:"tool"`
	Application string `json:"application_id,omitempty"`
}

func (r *Runner) repairNativeRead(task *Task, call api.ToolCall) error {
	cp := task.Checkpoint
	if cp.NativeReadRepair == nil {
		var args struct {
			Application string `json:"application_id"`
		}
		_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
		// Observe carries no application argument. When the durable history has
		// exactly one unverified launch, another application's readiness must not
		// reset this repair budget. Never guess between multiple pending launches.
		if args.Application == "" && call.Function.Name == "computer_observe" {
			if pending := PendingApplicationLaunches(task); len(pending) == 1 {
				args.Application = pending[0]
			}
		}
		cp.NativeReadRepair = &NativeReadRepair{Tool: call.Function.Name, Application: args.Application}
	}
	cp.ProposalRepairs++
	cp.ProposalUnresolved = true
	if cp.ProposalTool == "" {
		cp.ProposalTool = call.Function.Name
	}
	// A failure ends the model batch; later calls cannot assume a successful read.
	for _, pending := range cp.Calls {
		result := `{"error":"tool_proposal_abandoned","effects":"none","verified":false,"message":"Not executed because the preceding read failed. Observe the actual state before planning again."}`
		cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: pending.ID, Name: pending.Function.Name, Content: result})
		task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "rejected", ToolName: pending.Function.Name, ToolArgs: pending.Function.Arguments, ToolResult: result, Timestamp: time.Now().UTC()})
	}
	cp.Calls = nil
	return r.persistProposalRepair(task)
}

func resolveNativeReadRepair(cp *Checkpoint, tool, result string) {
	repair := cp.NativeReadRepair
	if repair == nil {
		return
	}
	var view struct {
		Observation struct {
			ID     string `json:"id"`
			Target struct {
				ID string `json:"id"`
			} `json:"target"`
		} `json:"observation"`
		Verified    bool   `json:"verified"`
		Check       string `json:"check"`
		Application string `json:"application_id"`
		Target      struct {
			ID string `json:"id"`
		} `json:"target"`
	}
	if json.Unmarshal([]byte(result), &view) != nil {
		return
	}
	ready := tool == "computer_verify_application" && view.Verified && view.Check == "application_open" && view.Target.ID != "" && (repair.Application == "" || repair.Application == view.Application)
	observed := repair.Application == "" && tool == "computer_observe" && view.Observation.ID != "" && view.Observation.Target.ID != ""
	if ready || observed {
		cp.NativeReadRepair = nil
	}
}
