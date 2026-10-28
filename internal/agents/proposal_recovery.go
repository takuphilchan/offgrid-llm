package agents

import (
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

const maxProposalRepairs = 3
const proposalRepairLimitCode = "tool_proposal_repair_limit"

// A runner-only marker, not a tool identity. Do not replace an outstanding
// access, read, expectation or edit repair with a weaker sequencing repair.
const computerToolSequenceRepair = ":computer_tool_sequence"

// Reject before executable checkpoint state exists. Keep only tool identity and
// service-authored feedback, never rejected arguments or assistant prose. A
// fresh observation repairs the prerequisite, not the requested operation itself.
func (r *Runner) rejectComputerObservation(task *Task, name, observe string) error {
	cp := task.Checkpoint
	cp.Iteration++
	cp.ProposalRepairs++
	cp.ProposalUnresolved = true
	if cp.ProposalTool == "" {
		cp.ProposalTool = computerToolSequenceRepair
		cp.SequenceRepairTool = name
		cp.SequenceRepairCompleted = false
	}
	feedback := "The proposed tool requires an observation of the selected target. No call from this response was dispatched. Use " + observe + " when available; for a whole-desktop task first discover/select the intended application with the currently available tools. Wait for each result and use only observed references. Do not repeat earlier dispatched actions or claim completion without evidence."
	result, _ := json.Marshal(map[string]any{"error": "computer_observation_required", "tool": name, "message": feedback, "effects": "none", "verified": false})
	cp.Messages = append(cp.Messages, api.ChatMessage{Role: "user", Content: string(result)})
	task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "rejected", ToolName: name, ToolResult: string(result), Timestamp: time.Now().UTC()})
	return r.persistProposalRepair(task)
}

// Reject the entire model batch before queuing, authorization or dispatch.
// Executing just the first call or serializing the proposed batch would accept
// actions planned from observations the model has not actually received.
func (r *Runner) rejectComputerToolBatch(task *Task, message api.ChatMessage) (*Task, error) {
	cp := task.Checkpoint
	message.ToolCalls = append([]api.ToolCall(nil), message.ToolCalls...)
	for i := range message.ToolCalls {
		call := &message.ToolCalls[i]
		args, err := CanonicalArguments(json.RawMessage(call.Function.Arguments))
		if err != nil {
			return r.fail(task, "Model returned invalid tool arguments.")
		}
		call.ID, call.Type, call.Function.Arguments = "call-"+runID(), "function", string(args)
	}
	cp.Iteration++
	cp.Messages = append(cp.Messages, message)
	cp.ProposalRepairs++
	cp.ProposalUnresolved = true
	if cp.ProposalTool == "" {
		cp.ProposalTool = computerToolSequenceRepair
		cp.SequenceRepairTool = message.ToolCalls[0].Function.Name
		cp.SequenceRepairCompleted = false
	}
	result := `{"error":"computer_multiple_tool_calls","effects":"none","verified":false,"message":"No calls in this response were executed. Return exactly ONE tool call, wait for its result, then plan the next step from that result. Earlier completed actions and outstanding checks remain in effect; do not repeat a launch or input that already ran."}`
	for _, call := range message.ToolCalls {
		cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: result})
		task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "rejected", ToolName: call.Function.Name, ToolArgs: call.Function.Arguments, ToolResult: result, Timestamp: time.Now().UTC()})
	}
	// The batch never becomes executable checkpoint state, even on restart.
	if err := r.persistProposalRepair(task); err != nil {
		return nil, err
	}
	return task, nil
}

func resolveComputerToolSequence(cp *Checkpoint, tool string) {
	// Only completing the first rejected dependency repairs its sequence.
	// Catalog/plan/history reads cannot erase a still-rejected launch or input.
	if cp.ProposalTool != computerToolSequenceRepair {
		return
	}
	if cp.SequenceRepairTool == tool {
		cp.SequenceRepairCompleted = true
	}
	// Keep completed dependency progress across a stronger outstanding read
	// repair. Its subsequent verification must not require launching again.
	if cp.SequenceRepairCompleted && cp.NativeReadRepair == nil && !cp.WorkflowEditPending {
		cp.ProposalRepairs, cp.ProposalUnresolved, cp.ProposalTool = 0, false, ""
		cp.SequenceRepairTool = ""
		cp.SequenceRepairCompleted = false
	}
}

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
		if json.Unmarshal([]byte(step.ToolResult), &result) != nil {
			return false
		}
		if result.Code == "computer_multiple_tool_calls" {
			continue
		}
		return result.Code == "computer_application_already_ready"
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
	case "execution_program_repeated":
		if task.Config.TaskFirst && task.ParentID == "" && task.Config.Execution != nil && tool == "workspace_process_start" {
			return "No command was dispatched. The program field is the executable; arguments contains only what comes AFTER it. For go test ., use program=go and arguments=[\"test\",\".\"], not [\"go\",\"test\",\".\"]. Review the user's actual command and submit a corrected proposal yourself. Do not change the command's intended meaning, request broader access, or repeat any earlier dispatched process."
		}
	case "execution_input_selection_invalid":
		if task.Config.TaskFirst && task.ParentID == "" && task.Config.InputWriteVersion == execution.InputWriteVersion && tool == execution.InputWriteTool {
			return "No write was dispatched. Select an offered user-input ID. The after marker is the opening/start boundary; the before marker is the closing/end boundary later in that same input. Use unique exact marker strings, excluding both markers from the content. Correct the proposal yourself; do not retype the payload or repeat any dispatched write. Ask for clarification if boundaries are genuinely ambiguous."
		}
	case "task_artifact_work_pending":
		if task.Config.TaskFirst && task.ParentID == "" && tool == "task_save_artifact" {
			return "No artifact was saved. Continue the original operation using the available discovery and observation tools. Gather actual evidence before writing results; do not create a file merely containing the request."
		}
	case "invalid_task_artifact":
		if task.Config.TaskFirst && task.ParentID == "" && tool == "task_save_artifact" {
			return "No artifact was saved. Correct the indicated field: text requires a .txt filename, markdown .md, json .json, csv .csv, with nonempty content of at most 131072 UTF-8 bytes. Do not invent results or replace requested application work with a file containing the request. Perform outstanding work and gather evidence before saving findings. Earlier dispatched actions must not be repeated."
		}
	case "computer_application_lookup_required":
		if task.Config.ComputerSession != "" && task.Config.ComputerScope == "desktop" && tool == "computer_open_application" {
			return "Search the latest full application catalog by the requested name using computer_find_applications. Use only its unique matching ID. Ask for clarification if missing or ambiguous; do not guess a substitute. This rejection happened before dispatch."
		}
	case "host_access_required":
		if task.Config.TaskFirst && task.ParentID == "" && task.Config.ComputerSession != "" {
			return "No service file operation was dispatched. This is a host path: inspect the native application and use its document tools. Desktop access grants no direct host filesystem API. Do not translate the path or substitute a service artifact."
		}
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
			if tool == "computer_documents" || tool == "computer_read_text" || failure.Code == "computer_document_changed" {
				return "The document read is stale or has no selected target. Call computer_observe once to obtain a fresh application observation, then continue from that result. Do not repeat computer_documents, read text, launch or input until the fresh observation is recorded."
			}
			return "Inspect the existing application using computer_verify_application or computer_windows and select the intended observed target. Do not repeat a prior launch or input."
		}
	}
	return ""
}

func (r *Runner) rejectProposal(task *Task, failure *ToolAuthorizationError, recovery string) error {
	cp := task.Checkpoint
	cp.ProposalRepairs++
	cp.ProposalUnresolved = true
	if failure.Code == "host_access_required" || failure.Code == "tool_write_rejected" || failure.Code == "invalid_task_artifact" || failure.Code == "execution_input_selection_invalid" || failure.Code == "execution_write_precondition" {
		cp.PreEffectRepairs++
	}
	if failure.Code == "workflow_control_expectation_required" {
		cp.WorkflowEditPending = true
	}
	if len(cp.Calls) > 0 {
		cp.ProposalTool = cp.Calls[0].Function.Name
		if !cp.PreEffectRepair && (failure.Code == "host_access_required" || failure.Code == "invalid_task_artifact" || failure.Code == "execution_input_selection_invalid") {
			cp.PreEffectRepair, cp.PreEffectRepairTool = true, cp.ProposalTool
		}
	}
	// Other calls in the same model batch may depend on this invalid proposal.
	// Complete their tool-message pairs as not executed, never dispatch them.
	for index, call := range cp.Calls {
		discardRejectedActionCheck(task, call.ID)
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

// Known no-effects rejection can be repaired by the same service operation or
// its typed host equivalent. Observation/catalog/calculator reads are not edit
// progress. Provider-chosen tool names never qualify as native evidence.
func resolvePreEffectRepair(cp *Checkpoint, tool string, descriptor capabilities.Descriptor) {
	if !cp.PreEffectRepair {
		return
	}
	original := cp.PreEffectRepairTool
	if original == "" { // Checkpoints written before the explicit origin field.
		original = cp.ProposalTool
	}
	repaired := tool == original && descriptor.Source == "builtin" && descriptor.Namespace == "tools"
	if original == "task_save_artifact" {
		repaired = workspaceArtifactCapability(tool, descriptor)
	}
	if original == execution.InputWriteTool || original == "workspace_write" {
		expected, ok := execution.Descriptor(original)
		repaired = ok && tool == original && descriptor == expected
	}
	if companionCapability(descriptor) {
		switch original {
		case "write_file":
			repaired = tool == "computer_insert_text" || tool == "computer_replace_selection" || tool == "computer_replace_text"
		case "read_file":
			repaired = tool == "computer_read_text"
		}
	}
	if !repaired {
		return
	}
	cp.PreEffectRepair, cp.PreEffectRepairTool = false, ""
	cp.PreEffectRepairs = 0
	// A subsequent native expectation/read failure is a separate obligation.
	// Its normal progress handler must resolve it; never clear it here.
	if cp.ProposalTool == original && cp.NativeReadRepair == nil && !cp.WorkflowEditPending {
		cp.ProposalRepairs, cp.ProposalUnresolved, cp.ProposalTool = 0, false, ""
	}
}

// The model must repair its proposal itself, not return instructions or tool
// JSON for the user and have that prose accepted as task completion. This is
// only reached before dispatch with an already outstanding bounded repair.
func (r *Runner) rejectUnrepairedCompletion(task *Task) error {
	task.Checkpoint.ProposalRepairs++
	if task.Checkpoint.PreEffectRepair {
		task.Checkpoint.PreEffectRepairs++
	}
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
	if task.Checkpoint.ProposalRepairs+task.Checkpoint.DecisionRepairs > maxProposalRepairs || task.Checkpoint.PreEffectRepairs > maxProposalRepairs {
		task.Status, task.ErrorCode = TaskInterrupted, proposalRepairLimitCode
		task.Error = "Paused after three unsuccessful proposal repairs. No rejected action was executed. Review the task or choose a compatible model before explicitly continuing."
		if task.Checkpoint.ProposalTool == "request_computer_access" && task.Config.ComputerSession == "" {
			// An invalid request is not evidence that the model/runtime is
			// incompatible. Keep this service-authored, not model/provider prose.
			task.Error = "The agent could not form a valid computer-access request after three correction attempts. This request granted no access and executed no application action. Your task is preserved; resume explicitly after reviewing the access-request details."
		}
		if task.Checkpoint.PreEffectRepair {
			for i := len(task.Steps) - 1; i >= 0; i-- {
				step := task.Steps[i]
				if step.Type != "rejected" {
					continue
				}
				var failure struct {
					Code    string `json:"error"`
					Message string `json:"message"`
				}
				if json.Unmarshal([]byte(step.ToolResult), &failure) == nil && (failure.Code == "host_access_required" || failure.Code == "tool_write_rejected") {
					task.Error += " " + failure.Message
					break
				}
			}
		}
		if task.Checkpoint.NativeReadRepair != nil {
			task.Error = "Paused after three unsuccessful read-recovery attempts. The failed reads made no changes; earlier task actions remain recorded. Review application readiness before explicitly continuing."
		}
		setRunPhase(task, "interrupted", "")
	}
	return r.persist(task)
}
