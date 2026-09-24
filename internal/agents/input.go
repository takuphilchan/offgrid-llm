package agents

// This module owns durable interruptions. Transport adapters may request input,
// but neither a model proposal nor a UI callback grants execution authority.
import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type InputRequest struct {
	PreviousSession string `json:"previous_session,omitempty"`
	ID              string `json:"id"`
	CallID          string `json:"call_id"`
	Kind            string `json:"kind"`
	Mode            string `json:"mode"`
	Target          string `json:"target"`
	URL             string `json:"url,omitempty"`
}

// InputRequired is a pre-dispatch interruption, never an execution result.
type InputRequired struct{ Request InputRequest }

func (e *InputRequired) Error() string { return "Task needs local computer access" }

// ResolveComputerInput attaches a host-consented session to the existing task.
// The caller must validate ownership, driver, policy and reserve that session.
// A repeated resolution returns the saved snapshot without appending messages.
func (r *Runner) ResolveComputerInput(id, actor, inputID string, config AgentConfig) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closing {
		return nil, ErrRunConflict
	}
	if _, active := r.active[id]; active {
		return nil, ErrRunConflict
	}
	task, err := r.manager.updateTask(id, func(task *Task) error {
		if task.Actor != actor {
			return ErrTaskNotFound
		}
		if inputID != "" && task.ResolvedInputID == inputID && task.Config.ComputerSession == config.ComputerSession {
			return nil
		}
		cp := task.Checkpoint
		input := task.PendingInput
		if inputID == "" || task.Status != TaskInput || input == nil || input.ID != inputID || input.Kind != "computer" || cp == nil || cp.ExecutingCall != "" || len(cp.Calls) != 1 || cp.Calls[0].ID != input.CallID || config.ComputerSession == "" || config.ComputerDriver == "" || config.ComputerApprovalMode == "" {
			return ErrRunConflict
		}
		if len(cp.Messages) == 0 || cp.Messages[0].Role != "system" {
			return fmt.Errorf("invalid task checkpoint")
		}
		call := cp.Calls[0]
		result, _ := json.Marshal(map[string]string{"status": "access_granted", "driver": config.ComputerDriver, "next": "This confirms access only: the original operation has NOT executed and no file contents or directory listing were returned. Inspect the selected target using the observation tool before any action. Use only the tools now provided."})
		cp.Messages[0].Content = config.SystemPrompt
		cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: string(result)})
		cp.Calls = nil
		task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "input_resolved", ToolName: call.Function.Name, ToolResult: string(result), Timestamp: time.Now().UTC()})
		config.ComputerStepOffset = len(task.Steps)
		task.LastAccess = input
		task.ComputerSessionExpired = false
		task.Config, task.PendingInput, task.ResolvedInputID = config, nil, inputID
		task.Status, task.Error, task.ErrorCode = TaskPending, "", ""
		return nil
	})
	if err == nil {
		r.observe(task)
	}
	return task, err
}
