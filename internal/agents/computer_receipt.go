package agents

import (
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// ComputerReceipt is a journal entry from the authenticated companion transport,
// not a model claim or a user report. It proves delivery of this invocation's
// result, not completion of the user's entire task or an external transaction.
type ComputerReceipt struct {
	Actor     string    `json:"actor"`
	Session   string    `json:"session"`
	CallID    string    `json:"call_id"`
	Tool      string    `json:"tool"`
	Arguments string    `json:"arguments"`
	Result    string    `json:"result"`
	At        time.Time `json:"at"`
}

func nativeMutation(d *capabilities.Descriptor, tool string) bool {
	if d == nil || d.Name != tool || d.Source != "local-companion" || d.Namespace != "computer" || d.Kind != capabilities.Computer {
		return false
	}
	effect, ok := capabilities.NativeOperationEffect(tool, d.Revision)
	return ok && effect == capabilities.NativeChange
}

// RecordComputerReceipt has no public HTTP route. Only the server's authenticated
// pending-action reply handler calls it, before acknowledging that reply.
func (m *Manager) RecordComputerReceipt(id, actor, session, callID, tool string, args json.RawMessage, result string) error {
	canonical, err := CanonicalArguments(args)
	if err != nil || len(result) > 48<<10 || !json.Valid([]byte(result)) {
		return ErrRunConflict
	}
	_, err = m.updateTask(id, func(t *Task) error {
		cp := t.Checkpoint
		if (t.Status != TaskRunning && t.Status != TaskUncertain) || t.Actor != actor || t.Config.ComputerSession != session || session == "" || cp == nil || cp.ExecutingCall != callID || len(cp.Calls) == 0 || cp.Calls[0].ID != callID || cp.Calls[0].Function.Name != tool || !nativeMutation(cp.ExecutingCapability, tool) {
			return ErrRunConflict
		}
		expected, e := CanonicalArguments(json.RawMessage(cp.Calls[0].Function.Arguments))
		if e != nil || string(expected) != string(canonical) {
			return ErrRunConflict
		}
		if cp.ComputerReceipt != nil {
			r := cp.ComputerReceipt
			if r.Actor != actor || r.Session != session || r.CallID != callID || r.Tool != tool || r.Arguments != string(canonical) || r.Result != result {
				return ErrRunConflict
			}
			return nil
		}
		cp.ComputerReceipt = &ComputerReceipt{actor, session, callID, tool, string(canonical), result, time.Now().UTC()}
		return nil
	})
	return err
}

// No dispatch, observation, consent or inference occurs here. If no matching
// journal entry survived, uncertainty remains. A success-looking screen alone
// cannot establish whether an uncertain invocation happened exactly once.
func recoverComputerReceipt(t *Task) bool {
	cp := t.Checkpoint
	if t.Status != TaskUncertain || cp == nil || cp.ComputerReceipt == nil || len(cp.Calls) == 0 {
		return false
	}
	r, call := cp.ComputerReceipt, cp.Calls[0]
	args, err := CanonicalArguments(json.RawMessage(call.Function.Arguments))
	if err != nil || r.Actor != t.Actor || r.Session != t.Config.ComputerSession || r.CallID != cp.ExecutingCall || r.CallID != call.ID || r.Tool != call.Function.Name || r.Arguments != string(args) || !nativeMutation(cp.ExecutingCapability, r.Tool) || !json.Valid([]byte(r.Result)) {
		return false
	}
	t.Steps = append(t.Steps, Step{ID: len(t.Steps) + 1, CallID: r.CallID, Type: "action", Provenance: "companion_receipt", ToolName: r.Tool, ToolArgs: r.Arguments, ToolResult: r.Result, Timestamp: r.At})
	message, _ := json.Marshal(map[string]any{"provenance": "companion_receipt", "offgrid_step_id": len(t.Steps), "result": json.RawMessage(r.Result), "guidance": "Recovered the original invocation result without repeating it. Verify outstanding outcomes through fresh permitted reads before claiming completion."})
	cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: call.ID, Name: r.Tool, Content: string(message)})
	cp.Calls, cp.ExecutingCall, cp.ComputerReceipt, cp.ExecutingCapability = cp.Calls[1:], "", nil, nil
	cp.ExecutionIntent, cp.ExecutionReceipt = nil, nil
	t.Status, t.ErrorCode, t.Error, t.CompletedAt = TaskInterrupted, "computer_result_recovered", "Saved computer result recovered without repeating the action. Review and resume explicitly.", nil
	return true
}
