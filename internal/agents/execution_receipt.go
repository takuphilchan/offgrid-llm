package agents

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type ExecutionIntent struct {
	Tool      string            `json:"tool"`
	Arguments string            `json:"arguments"`
	Request   execution.Request `json:"request"`
}

type ExecutionReceipt struct {
	Receipt execution.Receipt `json:"receipt"`
	At      time.Time         `json:"at"`
}

func validExecutionIntent(t *Task, intent *ExecutionIntent) bool {
	cp := t.Checkpoint
	if intent == nil || cp == nil || t.Config.Execution == nil || len(cp.Calls) == 0 {
		return false
	}
	r := intent.Request
	call := cp.Calls[0]
	d, ok := execution.Descriptor(intent.Tool)
	ref := t.Config.Execution
	if !ok || cp.ExecutingCapability == nil || *cp.ExecutingCapability != d || !execution.MutatingOperation(r.Operation) || execution.Operation(intent.Tool) != r.Operation || r.Token != "" || r.Version != execution.ProtocolVersion || r.ID != cp.ExecutingCall || r.ID != call.ID || call.Function.Name != intent.Tool || r.Binding.Actor != t.Actor || r.Binding.Job != t.ID || r.Binding.Host != ref.Host || r.Binding.Session != ref.Session || r.Binding.Grant != ref.Grant || r.Binding.Revision != ref.Revision {
		return false
	}
	a, err := CanonicalArguments(json.RawMessage(call.Function.Arguments))
	if err != nil || string(a) != intent.Arguments {
		return false
	}
	rebuilt, err := execution.NewIntent(r.Binding, r.ID, r.Operation, r.Arguments, r.Approved)
	if intent.Tool == execution.InputWriteTool && !inputWriteIntentMatches(t, json.RawMessage(intent.Arguments), r) {
		return false
	}
	return err == nil && rebuilt.OperationID == r.OperationID
}

// RecordExecutionIntent has no public route; only the existing runner's trusted
// provider adapter calls it after intent persistence and before host dispatch.
func (m *Manager) RecordExecutionIntent(id, actor, tool string, modelArgs json.RawMessage, request execution.Request) error {
	args, err := CanonicalArguments(modelArgs)
	if err != nil {
		return ErrRunConflict
	}
	intent := &ExecutionIntent{Tool: tool, Arguments: string(args), Request: request}
	_, err = m.updateTask(id, func(t *Task) error {
		if t.Actor != actor || t.Status != TaskRunning || !validExecutionIntent(t, intent) {
			return ErrRunConflict
		}
		if old := t.Checkpoint.ExecutionIntent; old != nil {
			if old.Tool != intent.Tool || old.Arguments != intent.Arguments || !bytes.Equal(old.Request.Payload(), request.Payload()) {
				return ErrRunConflict
			}
			return nil
		}
		t.Checkpoint.ExecutionIntent = intent
		return nil
	})
	return err
}

// RecordExecutionReceipt accepts authenticated host evidence, never a model or
// user claim. Saving must succeed before a host reply is acknowledged.
func (m *Manager) RecordExecutionReceipt(id, actor string, request execution.Request, response execution.Response) error {
	if request.Token != "" || response.Version != execution.ProtocolVersion || response.ID != request.ID || response.Error != "" || response.Receipt == nil || len(response.Receipt.Result) > execution.MaxFrameBytes {
		return ErrRunConflict
	}
	receipt := *response.Receipt
	switch receipt.State {
	case "complete", "dispatched", "rejected", "partial", "uncertain":
	default:
		return ErrRunConflict
	}
	if len(receipt.Result) == 0 && receipt.State != "uncertain" || len(receipt.Result) > 0 && !json.Valid(receipt.Result) {
		return ErrRunConflict
	}
	_, err := m.updateTask(id, func(t *Task) error {
		cp := t.Checkpoint
		if t.Actor != actor || t.Status != TaskRunning && t.Status != TaskUncertain || cp == nil || !validExecutionIntent(t, cp.ExecutionIntent) || cp.ExecutionIntent.Request.ID != request.ID || cp.ExecutionIntent.Request.OperationID != request.OperationID || !bytes.Equal(cp.ExecutionIntent.Request.Payload(), request.Payload()) {
			return ErrRunConflict
		}
		if old := cp.ExecutionReceipt; old != nil {
			if old.Receipt.State == receipt.State && bytes.Equal(old.Receipt.Result, receipt.Result) {
				return nil
			}
			if old.Receipt.State != "dispatched" || receipt.State != "complete" {
				return ErrRunConflict
			}
		}
		cp.ExecutionReceipt = &ExecutionReceipt{Receipt: receipt, At: time.Now().UTC()}
		return nil
	})
	return err
}

func recoverExecutionReceipt(t *Task) bool {
	cp := t.Checkpoint
	if t.Status != TaskUncertain || cp == nil || !validExecutionIntent(t, cp.ExecutionIntent) || cp.ExecutionReceipt == nil {
		return false
	}
	intent, evidence := cp.ExecutionIntent, cp.ExecutionReceipt
	receipt := evidence.Receipt
	if receipt.State != "complete" && receipt.State != "rejected" {
		return false
	}
	if intent.Request.Operation == "process_start" && receipt.State == "complete" {
		var process execution.ProcessResult
		if json.Unmarshal(receipt.Result, &process) != nil || process.Handle == "" || process.ExitCode == nil || process.State == "running" {
			return false
		}
		switch process.State {
		case "exited", "failed", "cancelled", "timed_out":
		default:
			return false
		}
	}
	result, err := execution.PresentReceipt(*t.Config.Execution, intent.Request.Operation, receipt)
	if intent.Tool == execution.InputWriteTool && receipt.State == "complete" {
		_, proof, sourceErr := ResolveInputWrite(t, json.RawMessage(intent.Arguments))
		if sourceErr != nil {
			return false
		}
		result, err = execution.PresentInputWrite(*t.Config.Execution, intent.Request, receipt, proof)
	}
	if err != nil {
		return false
	}
	var historical map[string]json.RawMessage
	if json.Unmarshal([]byte(result), &historical) != nil {
		return false
	}
	historical["provenance"], _ = json.Marshal("execution_receipt")
	historical["historical"], _ = json.Marshal(true)
	historical["observed_at"], _ = json.Marshal(evidence.At)
	historical["guidance"], _ = json.Marshal("Recovered the original saved invocation result without replay. This is historical evidence, not proof of current file/application state or whole-task success. Re-read outstanding outcomes through fresh authorized observation before claiming completion.")
	encoded, err := json.Marshal(historical)
	if err != nil {
		return false
	}
	result = string(encoded)
	kind := "action"
	if receipt.State == "rejected" {
		kind = "tool_error"
	}
	if descriptor, ok := execution.Descriptor(intent.Tool); ok {
		resolveInputWriteRepair(t, t, descriptor)
	}
	t.Steps = append(t.Steps, Step{ID: len(t.Steps) + 1, CallID: intent.Request.ID, Type: kind, Provenance: "execution_receipt", ToolName: intent.Tool, ToolArgs: intent.Arguments, ToolResult: result, Timestamp: evidence.At})
	descriptor, _ := execution.Descriptor(intent.Tool)
	cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: intent.Request.ID, Name: intent.Tool, Content: executionModelResult(t, intent.Tool, descriptor, result, len(t.Steps))})
	cp.Calls, cp.ExecutingCall, cp.ExecutingCapability = cp.Calls[1:], "", nil
	cp.ExecutionIntent, cp.ExecutionReceipt = nil, nil
	t.Status, t.ErrorCode, t.Error, t.CompletedAt = TaskInterrupted, "execution_result_recovered", "Saved host result recovered without repeating the operation. Review and resume explicitly; this is not proof of the whole task's completion.", nil
	return true
}

func recoverProviderReceipts(t *Task) bool {
	return recoverExecutionReceipt(t) || recoverComputerReceipt(t)
}
