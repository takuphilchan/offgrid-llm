package agents

import (
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

const maxProposalRepairs = 3
const proposalRepairLimitCode = "tool_proposal_repair_limit"

// A blocked relaunch of an already verified application is repaired by real
// control progress, not another catalog query. Unverified/uncertain launches
// still require their independent readiness check before this can apply.
func advancedVerifiedApplicationRepair(task *Task, tool string) bool {
	switch tool {
	case "computer_replace_text", "computer_activate", "computer_shortcut", "computer_set_checked", "computer_focus_window", "computer_create_document", "computer_select_document", "computer_select_text", "computer_insert_text", "computer_replace_selection":
	default:
		return false
	}
	for i := len(task.Steps) - 1; i >= 0; i-- {
		step := task.Steps[i]
		if step.Type != "rejected" || step.ToolName != "computer_open_application" {
			continue
		}
		var result struct {
			Code string `json:"error"`
		}
		return json.Unmarshal([]byte(step.ToolResult), &result) == nil && result.Code == "computer_application_already_ready"
	}
	return false
}

// Only service-authored authorization failures are eligible. Never call this
// after dispatch or interpret a provider's error text as permission to retry.
func proposalRecovery(task *Task, tool string, failure *ToolAuthorizationError) string {
	if task.Checkpoint == nil || task.Checkpoint.ExecutingCall != "" {
		return ""
	}
	switch failure.Code {
	case "workflow_control_expectation_required":
		if usesWorkflow(task) && task.Config.NativeWorkflowVersion == DocumentWorkflowEvidenceVersion && (tool == "computer_create_document" || tool == "computer_insert_text" || tool == "computer_replace_selection" || tool == "computer_replace_text") {
			return "Preserve existing checks. Before creation declare document_blank with target_id and the observed command. Before editing declare document_edit_matches with target_id, document, snapshot, edit_mode and exact requested text. Use computer_documents then computer_read_text when references changed. Never invent a future document or repeat dispatched input."
		}
		if usesWorkflow(task) && (tool == "computer_replace_text" || tool == "computer_set_checked") {
			return "Declare the matching control outcome with task_expectations using the observed target_id, element and exact requested value. Preserve existing checks. If the observation changed, observe again before declaring and proposing the edit."
		}
	case "computer_application_already_ready":
		if task.Config.ComputerSession != "" && task.Config.ComputerScope == "desktop" && tool == "computer_open_application" {
			return "The application is already open. Advance the remaining user outcome with its controls, or inspect/focus its current window. Do not repeat opening as a substitute for completing the task."
		}
	case "computer_application_verification_required":
		if task.Config.ComputerSession != "" && task.Config.ComputerScope == "desktop" && tool == "computer_open_application" {
			return "Call computer_verify_application for the already-launched application. Discovery does not verify it. Never substitute another application for the user's named application without asking."
		}
	case "computer_invalid_access_request":
		if task.Config.TaskFirst && task.ParentID == "" && tool == "request_computer_access" {
			return "Correct the access-request arguments using the guidance. Request access alone; do not assume permission or claim that an application opened."
		}
	case "computer_invalid_action", "computer_stale_observation":
		if task.Config.ComputerSession != "" {
			return "Call the observation tool again and use only IDs from its newest result."
		}
	case "computer_target_not_selected", "computer_application_not_ready", "computer_document_changed":
		if task.Config.ComputerSession != "" {
			return "Inspect the existing application using computer_verify_application or computer_windows and select the intended observed target. Do not repeat a prior launch or input."
		}
	}
	return ""
}

func (r *Runner) rejectProposal(task *Task, failure *ToolAuthorizationError, recovery string) error {
	cp := task.Checkpoint
	cp.ProposalRepairs++
	cp.ProposalUnresolved = true
	if failure.Code == "workflow_control_expectation_required" {
		cp.WorkflowEditPending = true
	}
	if len(cp.Calls) > 0 {
		cp.ProposalTool = cp.Calls[0].Function.Name
	}
	// Other calls in the same model batch may depend on this invalid proposal.
	// Complete their tool-message pairs as not executed, never dispatch them.
	for index, call := range cp.Calls {
		code, message := failure.Code, failure.Message
		if index > 0 {
			code, message = "tool_proposal_abandoned", "Not executed because an earlier proposal in this batch was invalid. Plan again from the current state."
		}
		result, _ := json.Marshal(map[string]any{"error": code, "message": message, "recovery": recovery, "effects": "none", "verified": false})
		cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: string(result)})
		task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "rejected", ToolName: call.Function.Name, ToolArgs: call.Function.Arguments, ToolResult: string(result), Timestamp: time.Now().UTC()})
	}
	cp.Calls = nil
	task.PendingApproval = nil
	return r.persistProposalRepair(task)
}

// The model must repair its proposal itself, not return instructions or tool
// JSON for the user and have that prose accepted as task completion. This is
// only reached before dispatch with an already outstanding bounded repair.
func (r *Runner) rejectUnrepairedCompletion(task *Task) error {
	task.Checkpoint.ProposalRepairs++
	feedback := "The previous tool proposal was rejected before dispatch and is still unresolved. Do not ask the user to submit tool JSON or perform the requested work manually. Correct the proposal yourself using the available tools and the previous error guidance. No rejected action was executed; do not claim completion."
	if task.Checkpoint.NativeReadRepair != nil {
		feedback = "The previous application read failed and is still unresolved. Inspect the actual application state with the available read tools. Earlier actions remain recorded: do not repeat a launch or input, invent successful verification, or claim completion."
	}
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, api.ChatMessage{Role: "system", Content: feedback})
	task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "completion_rejected", Content: feedback, Timestamp: time.Now().UTC()})
	return r.persistProposalRepair(task)
}

func (r *Runner) persistProposalRepair(task *Task) error {
	setRunPhase(task, "repairing_tool", "")
	if task.Checkpoint.ProposalRepairs > maxProposalRepairs {
		task.Status, task.ErrorCode = TaskInterrupted, proposalRepairLimitCode
		task.Error = "Paused after three unsuccessful proposal repairs. No rejected action was executed. Review the task or choose a compatible model before explicitly continuing."
		if task.Checkpoint.NativeReadRepair != nil {
			task.Error = "Paused after three unsuccessful read-recovery attempts. The failed reads made no changes; earlier task actions remain recorded. Review application readiness before explicitly continuing."
		}
		setRunPhase(task, "interrupted", "")
	}
	return r.persist(task)
}
