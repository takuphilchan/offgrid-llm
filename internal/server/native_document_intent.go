package server

import (
	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

// Native document discovery is language-independent. Availability is checked by
// the negotiated provider and stage projection; exposing it neither establishes
// user intent nor requires a call. Dispatch still verifies authority and effects.
func nativeDocumentTaskIntent(task *agents.Task) bool {
	return task != nil
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
