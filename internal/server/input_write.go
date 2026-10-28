package server

import (
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// Validate selection at the runner's pre-dispatch authorization boundary. An
// invalid selector cannot create an intent or be mistaken for unknown effects.
func authorizeWorkspaceInvocation(task *agents.Task, name string, args json.RawMessage, e agents.ToolExecution) error {
	if name == execution.InputWriteTool {
		if _, _, err := agents.ResolveInputWrite(task, args); err != nil {
			return &agents.ToolAuthorizationError{Code: "execution_input_selection_invalid", Message: "Select this root's request or an offered applied correction with unique nonempty after/before markers in the correct order. No write was dispatched.", Cause: err}
		}
	}
	return authorizeExecutionInvocation(name, args, e)
}
