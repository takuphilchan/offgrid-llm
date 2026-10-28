package agents

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Produced only by the service's typed process-receipt validator, never decoded
// from tool prose. It requests more observation, not another process start.
type ExecutionPending struct {
	// Set only by the typed receipt validator for the current execution session.
	Handle    string
	Reference execution.Reference
	Kind      string
}

func (p *ExecutionPending) Error() string {
	if p.Kind == "workspace_edit" {
		return "Edit access was requested but no verified file write was recorded. Complete the requested edit with the available workspace tools before answering, or explain why it cannot be done. Do not claim the file changed"
	}
	return "A workspace command is still running or unverified. Inspect/wait for its actual exit before finishing; do not restart it"
}

func (r *Runner) repairExecutionCompletion(task *Task, cause error) (bool, error) {
	var pending *ExecutionPending
	cp := task.Checkpoint
	if !task.Config.TaskFirst || task.Config.ExecutionRequestVersion != execution.AccessRequestVersion || !errors.As(cause, &pending) || cp == nil || cp.ExecutingCall != "" || len(cp.Calls) != 0 {
		return false, nil
	}
	cp.WorkspaceCompletionRepairs++
	feedback := pending.Error() + ". Keep the original task and existing process handle. This proposed answer was not accepted as completion."
	cp.Messages = append(cp.Messages, api.ChatMessage{Role: "system", Content: feedback})
	task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "completion_rejected", Content: feedback, Timestamp: time.Now().UTC()})
	if cp.WorkspaceCompletionRepairs > maxProposalRepairs {
		task.ErrorCode = "execution_completion_repair_limit"
		settled, err := r.fail(task, "Stopped after repeated premature completion attempts. Inspect saved command state before continuing; no command was restarted.")
		if settled != nil {
			*task = *settled
		}
		return true, err
	} else {
		// Observation is part of completion verification, not another model
		// decision. Persist it through the ordinary call boundary so revocation,
		// cancellation, steering, receipt recording and restart still apply.
		if pending.Handle != "" && task.Config.Execution != nil && *task.Config.Execution == pending.Reference && r.tools != nil {
			const name = "workspace_process_wait"
			expected, _ := execution.Descriptor(name)
			actual, exists := r.tools.Capability(name)
			if exists && actual == expected && ProfileAllowsCapability(task.Config, name, actual) {
				args, _ := json.Marshal(map[string]any{"handle": pending.Handle, "wait_ms": 1000})
				call := api.ToolCall{ID: "call-" + runID(), Type: "function", Function: api.FunctionCall{Name: name, Arguments: string(args)}}
				cp.Calls = []api.ToolCall{call}
				cp.Messages = append(cp.Messages, api.ChatMessage{Role: "assistant", ToolCalls: cp.Calls})
				task.Steps[len(task.Steps)-1].Content += " The harness scheduled a bounded observation of that same owned process; no command was restarted."
			}
		}
		setRunPhase(task, "repairing_tool", "")
	}
	return true, r.persist(task)
}
