package agents

import (
	"encoding/json"
	"fmt"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func validConversationDecision(d *api.ConversationDecision) bool {
	return d != nil && (d.Kind == "new_work" && d.InstructionVersion == 0 && validProfileText(d.ExpectedResult, 4096, true) || d.ExpectedResult == "" && (d.Kind == "answered" && d.InstructionVersion == 0 || d.Kind == "correction" && d.InstructionVersion > 0))
}

func validateConversationDecision(entry *api.ConversationInboxEntry, d *api.ConversationDecision) error {
	if sourceOnlyConversationInput(entry.Input) && (d == nil || d.Kind != "answered") {
		return ErrControlCommand
	}
	if !validConversationDecision(d) || d.Kind == "correction" && entry.WorkID == "" ||
		d.Kind == "answered" && (entry.Input.Intent == "correction" || entry.Input.Intent == "new_work" || entry.Input.Intent == "answer") ||
		d.Kind == "new_work" && (entry.Input.Intent == "correction" || entry.Input.Intent == "answer") {
		return ErrControlCommand
	}
	return nil
}

func conversationDecisionTool() api.Tool {
	return api.Tool{Type: "function", Function: api.FunctionDef{
		Name:        "conversation_decide",
		Description: "Coordinate the current accepted user input. Use correction to apply its exact text to focused work at the next settled checkpoint, with the recorded instruction_version. Use new_work for a distinct objective requiring ongoing work; give its expected_result, with instruction_version zero. The objective is the exact accepted user input, with authorized conversation context. Never use new_work to replenish a focused task's budget or continue stopped work. Use answered for a side question; a direct ordinary answer already does this automatically. A recorded decision is pending, not applied. Finish your reply without polling; the service settles it without inference. This grants no host access and approves nothing.",
		Parameters: map[string]interface{}{"type": "object", "additionalProperties": false, "required": []string{"kind", "instruction_version"}, "properties": map[string]interface{}{
			"kind":                map[string]interface{}{"type": "string", "enum": []string{"answered", "correction", "new_work"}},
			"instruction_version": map[string]interface{}{"type": "integer", "minimum": 0, "description": "Current focused work instruction version for correction; zero for answered."},
			"expected_result":     map[string]interface{}{"type": "string", "maxLength": 4096, "description": "Required only for new_work: the expected outcome and evidence. This is a proposed acceptance condition, not an authority grant."},
		}},
	}}
}

func (r *Runner) prepareConversationDecision(task *Task, arguments string) (string, error) {
	if task.ConversationTurn == nil || task.Origin == nil || task.Status != TaskRunning {
		return "", ErrControlCommand
	}
	var decision api.ConversationDecision
	if err := decodeRuntimeArguments(arguments, &decision); err != nil {
		return "", err
	}
	scope := ControlScope{task.Origin.Workspace, task.Actor}
	entry, err := r.manager.ConversationInput(scope, task.Origin.Conversation, task.ConversationTurn.InputID)
	if err != nil {
		return "", err
	}
	if entry.RunID != task.ID || entry.State != "processing" || entry.AppliedAt != nil {
		return "", ErrRunConflict
	}
	if err = validateConversationDecision(entry, &decision); err != nil {
		return "", &runtimeArgumentError{cause: err}
	}
	if previous := task.ConversationTurn.Decision; previous != nil && *previous != decision {
		return "", fmt.Errorf("this input already has a recorded decision")
	}
	if decision.Kind == "correction" {
		work, err := r.manager.WorkItem(scope, entry.WorkID)
		if err != nil {
			return "", err
		}
		if work.InstructionVersion != decision.InstructionVersion {
			return "", fmt.Errorf("focused work instruction version changed; this correction has not been recorded")
		}
		root, found := r.manager.GetTask(work.RunID)
		if !found || root.Actor != scope.Actor {
			return "", ErrControlNotFound
		}
		if err = r.checkSourceAccess(root); err != nil {
			return "", err
		}
	}
	// The runtime's normal persist commits this proposal and its tool receipt
	// together. No work mutation occurs before that transaction succeeds.
	task.ConversationTurn.Decision = &decision
	data, _ := json.Marshal(map[string]any{"recorded": true, "applied": false, "state": "pending", "decision": decision})
	return string(data), nil
}

// ApplyConversationDecision consumes only completed foreground decisions. It
// never generates, dispatches a tool, or resumes interrupted execution. The
// incorporation transaction rechecks identity, ordering and graph quiescence.
func (r *Runner) ApplyConversationDecision(scope ControlScope, conversation, input string) (*api.ConversationInboxEntry, error) {
	entry, err := r.manager.ConversationInput(scope, conversation, input)
	if err != nil {
		return nil, err
	}
	if entry.State != "responded" || entry.RunStatus != TaskCompleted || entry.Decision == nil {
		return nil, ErrRunConflict
	}
	task, ok := r.manager.GetTask(entry.RunID)
	if !ok || task.Actor != scope.Actor || task.ConversationTurn == nil || task.ConversationTurn.InputID != input || task.Origin == nil || task.Origin.Workspace != scope.Workspace || task.Origin.Conversation != conversation {
		return nil, ErrControlNotFound
	}
	if err = r.checkSourceAccess(task); err != nil {
		return nil, err
	}
	return r.IncorporateConversationInput(scope, conversation, input, entry.RunID, entry.Decision.Kind, conversationDecisionVersion(entry))
}

func conversationDecisionVersion(entry *api.ConversationInboxEntry) uint64 {
	if entry.RevalidatedAt != nil {
		return entry.RevalidatedInstructionVersion
	}
	return entry.Decision.InstructionVersion
}
