package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// batch contains detached snapshots; the caller checks no active dispatch and
// commits the entire graph with the incorporation receipt, or none of it.
func prepareInboxCorrection(batch []*Task, entry *api.ConversationInboxEntry, version uint64) error {
	if len(batch) == 0 {
		return ErrControlCommand
	}
	root := batch[0]
	if root.InstructionVersion != version || terminalControlTask(root) || root.ControlAction == "stop" || len(root.Instructions) >= 32 || len(root.PendingInstructions) != 0 {
		return ErrRunConflict
	}
	switch root.Status {
	case TaskPending, TaskConversationInput, TaskChildren, TaskInterrupted:
	default:
		return ErrRunConflict
	}
	if root.Delegation != nil {
		children := map[string]*Task{}
		for _, link := range root.Delegation.Children {
			for _, child := range batch[1:] {
				if child.ID == link.ID {
					if child.Status == TaskUncertain || HasOwnedExecutionProcesses(child) {
						return ErrRunConflict
					}
					children[link.Spec.Key] = child
				}
			}
		}
		if len(children) != len(root.Delegation.Children) {
			return ErrRunStorage
		}
		outcomes, err := json.Marshal(childOutcomes(root, children))
		if err != nil {
			return err
		}
		appendDelegationOutcome(root, "Superseded assignments. Retained results are evidence from earlier instructions, not completion of the corrected request: "+string(outcomes))
		root.Checkpoint.WorkerAnswerCommand, root.PendingInput = nil, nil
		root.Checkpoint.Calls, root.Delegation = nil, nil
		for _, child := range children {
			if !terminalControlTask(child) {
				finishTask(child, TaskCancelled, "Assignment superseded by a parent correction. Retained evidence is not current completion.")
				child.ControlEpoch++
				child.ErrorCode, child.SchedulerReady = "instruction_superseded", false
			}
			for index := range root.ChildHistory {
				if root.ChildHistory[index].ID == child.ID {
					root.ChildHistory[index].Invalidated = true
					root.ChildHistory[index].Status, root.ChildHistory[index].Phase = child.Status, ""
				}
			}
		}
		if root.Status == TaskChildren {
			if !hasConversationHold(root) {
				return ErrRunConflict
			}
			root.Status = TaskConversationInput
		}
		if hasConversationHold(root) && root.ConversationHold.ResumeStatus == TaskChildren {
			root.ConversationHold.ResumeStatus, root.ConversationHold.ResumeReady = TaskPending, true
		}
	}
	digest := sha256.Sum256([]byte(entry.Input.Content))
	instruction := Instruction{RequestID: entry.ID, Digest: hex.EncodeToString(digest[:]), Text: entry.Input.Content, At: time.Now().UTC()}
	if err := applyInstruction(root, instruction); err != nil {
		return err
	}
	entry.AppliedInstructionVersion = root.InstructionVersion
	return nil
}
