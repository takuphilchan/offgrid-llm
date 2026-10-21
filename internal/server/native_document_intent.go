package server

import (
	"regexp"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

// nativeDocumentIntentPattern is deliberately conservative. A negotiated
// document provider is a capability, not a reason to inspect every selected
// window. Keeping browser/search/control tasks on the normal observation path
// prevents small models from choosing computer_documents simply because it is
// available.
var nativeDocumentIntentPattern = regexp.MustCompile(`(?i)(\bdocument(?:s)?\b|\bnotepad\b|\btext ?edit(?:or)?\b|\bword processor\b|\bspreadsheet\b|\blibreoffice\b|\bword\b|\bnew tab\b|\bselect tab\b|\btype exactly\b|\binsert text\b|\breplace (?:the )?selection\b|\bread (?:it|the document) back\b|\bcreate (?:a )?document\b|\bedit (?:the )?document\b)`)

// nativeDocumentTaskIntent reports whether the user's requested outcome
// actually needs the native document/tab protocol. It is model-context
// routing only; dispatch still validates every document operation independently.
func nativeDocumentTaskIntent(task *agents.Task) bool {
	if task == nil {
		return false
	}
	if task.Workflow != nil {
		for _, check := range task.Workflow.Checks {
			if documentOutcomeKind(check.Expectation.Kind) {
				return true
			}
		}
	}
	text := task.Prompt + "\n" + task.Config.Instructions
	for _, instruction := range task.Instructions {
		text += "\n" + instruction.Text
	}
	// Empty prompts occur in low-level projection tests and in a few internal
	// callers that already supplied an explicit document stage. Preserve their
	// negotiated behavior; real user tasks always carry a prompt.
	if strings.TrimSpace(text) == "" {
		return true
	}
	return nativeDocumentIntentPattern.MatchString(text)
}

func nativeDocumentOnlyToolName(name string) bool {
	switch name {
	case "computer_documents", "computer_create_document", "computer_select_document", "computer_read_text", "computer_select_text", "computer_insert_text", "computer_replace_selection":
		return true
	default:
		return false
	}
}

// A failed document read must be repaired with a new observation. Do not
// expose the failed document catalog/read again while the durable repair is
// pending; doing so creates an avoidable retry loop in small local models.
func nativeDocumentObservationRecoveryPending(task *agents.Task) bool {
	if task == nil || task.Checkpoint == nil || task.Checkpoint.NativeReadRepair == nil {
		return false
	}
	switch task.Checkpoint.NativeReadRepair.Tool {
	case "computer_documents", "computer_read_text", "computer_select_document", "computer_select_text":
		return true
	default:
		return false
	}
}

func nativeDocumentRecoveryToolAllowed(name string) bool {
	switch name {
	case "computer_observe", "computer_verify_application", "computer_windows", "computer_focus_window":
		return true
	default:
		return false
	}
}
