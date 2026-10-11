package agents

import (
	"context"
	"errors"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

// ToolFailure is service-authored evidence that an invocation failed without
// changing its target. Never construct it from a model/MCP claim of safety.
// Unknown errors from mutating tools must retain uncertain-outcome recovery.
type ToolFailure struct {
	Code, Message string
	Cause         error
	// Recovery is service-authored guidance for a proven native read failure.
	// The runner additionally requires the reviewed descriptor before using it.
	Recovery  string
	Readiness string
}

func (e *ToolFailure) Error() string { return e.Message }
func (e *ToolFailure) Unwrap() error { return e.Cause }

func readOnlyBuiltin(d capabilities.Descriptor) bool {
	if capabilities.TrustedNativeRead(d) {
		return true
	}
	if d.Source != "builtin" || d.Namespace != "tools" || d.Kind != capabilities.Read {
		return false
	}
	switch d.Name {
	case "list_files", "read_file", "calculator", "current_time":
		return true
	}
	return false
}

func readFailure(err error) *ToolFailure {
	code, message := "tool_read_failed", "This read operation failed without making changes. Earlier task actions are unchanged; no outcome confirmation is needed for this read."
	switch {
	case errors.Is(err, os.ErrNotExist):
		code, message = "tool_path_not_found", "The requested file or folder was not found in the service workspace. Files on your computer require local computer access. No changes were made."
	case errors.Is(err, os.ErrPermission):
		code, message = "tool_read_denied", "The service cannot read that file or folder. Check its access permissions. No changes were made."
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		code, message = "tool_read_interrupted", "The read operation stopped before a result was available. No changes were made."
	}
	return &ToolFailure{Code: code, Message: message, Cause: err}
}

// The marker is saved with intent only for reviewed built-in and native reads.
// Old checkpoints without this evidence remain conservative.
func uncertainEffect(cp *Checkpoint) bool {
	return cp != nil && cp.ExecutingCall != "" && !validReadOnlyIntent(cp)
}

func releaseInterruptedRead(cp *Checkpoint) {
	if validReadOnlyIntent(cp) {
		cp.ExecutingCall, cp.ReadOnlyCall = "", ""
		cp.ReadOnlyCapability = nil
	}
}

func validReadOnlyIntent(cp *Checkpoint) bool {
	if cp == nil || cp.ReadOnlyCall == "" || cp.ReadOnlyCall != cp.ExecutingCall || len(cp.Calls) == 0 || cp.Calls[0].ID != cp.ReadOnlyCall {
		return false
	}
	name := cp.Calls[0].Function.Name
	if cp.ReadOnlyCapability != nil {
		return cp.ReadOnlyCapability.Name == name && readOnlyBuiltin(*cp.ReadOnlyCapability)
	}
	// Only these reads had durable markers before native-effects-v1. New native
	// operations require the pinned descriptor, never retrospective inference.
	switch name {
	case "read_file", "list_files", "calculator", "current_time", "computer_find_controls", "computer_inspect_control":
		return true
	}
	return false
}
