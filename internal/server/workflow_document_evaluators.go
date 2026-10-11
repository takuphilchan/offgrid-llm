package server

import (
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func documentOutcomeKind(kind string) bool {
	return kind == "document_blank" || kind == "document_text_equals" || kind == "document_edit_matches"
}

func documentMutationTool(name string) bool {
	switch name {
	case "computer_create_document", "computer_insert_text", "computer_replace_selection", "computer_replace_text":
		return true
	}
	return false
}

func documentOperationTool(kind string) string {
	switch kind {
	case "new_document", "new_tab":
		return "computer_create_document"
	case "insert_text":
		return "computer_insert_text"
	case "replace_selection":
		return "computer_replace_selection"
	case "replace_value":
		return "computer_replace_text"
	}
	return ""
}

func matchesDocumentOutcome(binding *agents.OutcomeBinding, document computer.DocumentBinding) bool {
	if binding == nil || binding.Document == nil || document.Validate() != nil {
		return false
	}
	d := binding.Document
	return binding.TargetSHA256 == evidenceTargetDigest(document.Target) && d.ID == document.Document && d.Generation == document.Generation && d.Container == document.Container && d.Editor == document.Editor
}

// Only canonical successful host steps may establish this evidence. Request
// arguments, input echoes, arbitrary MCP results and model claims never do.
func assessDocumentOutcome(task *agents.Task, check *agents.OutcomeCheck) {
	if !agents.WorkflowVersionSupported(task) || task.Workflow.Version != agents.DocumentWorkflowEvidenceVersion {
		check.State, check.Reason = "unavailable", "Document verification requires workflow evidence version 2."
		return
	}
	e, binding := check.Expectation, check.Binding
	if binding == nil || binding.Document == nil || binding.Session == "" || binding.Session != task.Config.ComputerSession || binding.Step < 1 || task.Config.ComputerStepOffset < 0 || task.Config.ComputerStepOffset > len(task.Steps) {
		return
	}
	d := binding.Document
	if e.Kind != "document_blank" && (e.Document != d.ID || e.Snapshot != d.Snapshot) {
		return
	}
	if e.Kind == "document_edit_matches" && e.EditMode != d.Operation {
		return
	}
	var mutation *computer.DocumentActionEvidence
	mutationStep := 0
	reset := func() {
		check.State, check.Reason, check.EvidenceSteps, check.CheckedAt = "pending", "Matching independent document read-back has not been recorded.", nil, nil
	}
	for index := task.Config.ComputerStepOffset; index < len(task.Steps); index++ {
		s := task.Steps[index]
		if s.ID != index+1 {
			reset()
			return
		}
		if s.Type == "task_metadata" || s.Type == "rejected" || s.Type == "completion_rejected" {
			continue
		}
		if documentMutationTool(s.ToolName) {
			reset()
			mutation, mutationStep = nil, 0
			var action computer.DocumentActionEvidence
			if s.Type != "action" || s.ID < binding.Step || !decodeNativeModelResult(s.ToolResult, &action) || action.Validate() != nil || action.ActionID != d.ActionID || documentOperationTool(action.Operation) != s.ToolName || action.Operation != d.Operation || action.After.Target.Driver != task.Config.ComputerDriver || !matchesDocumentOutcome(binding, action.After) || action.ExpectedSHA256 != d.ExpectedSHA256 {
				continue
			}
			if e.Kind == "document_blank" {
				if action.Command != e.Command || action.After.Revision != d.Revision || action.AfterSnapshot != d.Snapshot {
					continue
				}
			} else {
				if action.Before == nil || !matchesDocumentOutcome(binding, *action.Before) || action.Before.Revision != d.Revision || action.BeforeSnapshot != d.Snapshot || action.PreimageSHA256 != d.PreimageSHA256 || e.Text == nil || action.InputSHA256 != computer.DocumentTextDigest(*e.Text) || action.Operation != e.EditMode {
					continue
				}
			}
			mutation, mutationStep = &action, s.ID
			continue
		}
		if s.ToolName == "computer_read_text" {
			var read computer.DocumentReadback
			if s.Type != "action" || s.ID <= binding.Step || !decodeNativeModelResult(s.ToolResult, &read) || read.Validate() != nil {
				reset()
				continue
			}
			// Newer evidence of a different active document or revision invalidates
			// prior success; it must not leave a stale green check behind.
			reset()
			if read.Binding.Target.Driver != task.Config.ComputerDriver || read.Binding.Target.ID != e.TargetID || !matchesDocumentOutcome(binding, read.Binding) || read.Snapshot == d.Snapshot || read.Binding.Observation == binding.Observation {
				continue
			}
			if e.Kind == "document_text_equals" {
				if e.Text != nil && read.TextSHA256 == computer.DocumentTextDigest(*e.Text) && read.Bytes == len(*e.Text) {
					verifyOutcome(check, []int{binding.Step, s.ID}, s.Timestamp)
				}
				continue
			}
			if mutation == nil || mutationStep >= s.ID || read.Snapshot == mutation.AfterSnapshot || read.Binding.Observation == mutation.After.Observation || read.Binding.Revision != mutation.After.Revision || !computer.SameDocument(mutation.After, read.Binding) || read.TextSHA256 != mutation.ExpectedSHA256 {
				continue
			}
			if e.Kind == "document_blank" && read.Bytes != 0 {
				continue
			}
			verifyOutcome(check, []int{binding.Step, mutationStep, s.ID}, s.Timestamp)
			continue
		}
		if s.ToolName == "computer_documents" {
			var inventory computer.DocumentInventory
			if s.Type != "action" || !decodeNativeModelResult(s.ToolResult, &inventory) || !inventory.Complete || inventory.Active == nil || inventory.Validate(inventory.Observation.Target, inventory.Observation.CapturedAt) != nil || !matchesDocumentOutcome(binding, *inventory.Active) {
				reset()
				mutation, mutationStep = nil, 0
				continue
			}
			// A fresh inventory is the normal transition from mutation to
			// independent read-back. It cannot verify text by itself, but must
			// not discard the recorded action it is about to verify.
			if mutation != nil && inventory.Active.Revision != mutation.After.Revision {
				reset()
				mutation, mutationStep = nil, 0
			}
			continue
		}
		// Ordinary native input can invalidate the active document. Known reads
		// do not reset a preparation or replay an action. Unknown native tools
		// are conservative until their effects are reviewed.
		if strings.HasPrefix(s.ToolName, "computer_") {
			switch s.ToolName {
			case "computer_applications", "computer_find_controls", "computer_inspect_control", "computer_verify", "computer_verify_checked":
				continue
			}
			reset()
			mutation, mutationStep = nil, 0
		}
	}
}
