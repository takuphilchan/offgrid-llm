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
}

func (e *ToolFailure) Error() string { return e.Message }
func (e *ToolFailure) Unwrap() error { return e.Cause }

func readOnlyBuiltin(d capabilities.Descriptor) bool {
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
	code, message := "tool_read_failed", "The read operation failed. No changes were made; no outcome confirmation is needed."
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

// The marker is saved with intent only for our pinned read-only built-ins.
// Old checkpoints without this evidence remain conservative.
func uncertainEffect(cp *Checkpoint) bool {
	return cp != nil && cp.ExecutingCall != "" && cp.ReadOnlyCall != cp.ExecutingCall
}

func releaseInterruptedRead(cp *Checkpoint) {
	if cp != nil && cp.ExecutingCall != "" && cp.ReadOnlyCall == cp.ExecutingCall {
		cp.ExecutingCall, cp.ReadOnlyCall = "", ""
	}
}
