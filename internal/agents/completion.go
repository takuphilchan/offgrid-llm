package agents

import (
	"errors"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// validateCompletion runs before accepting an answer OR executing tools. Valid
// JSON arguments alone do not establish that the model finished its turn.
func validateCompletion(choice api.ChatCompletionChoice) error {
	switch choice.FinishReason {
	case "stop":
		return nil
	case "tool_calls":
		if len(choice.Message.ToolCalls) > 0 {
			return nil
		}
		return errors.New("Model signalled a tool call without providing one.")
	case "length":
		return errors.New("Model output was truncated by its token limit. The task did not complete; no tools from this response were executed.")
	case "content_filter":
		return errors.New("Model output was blocked. The task did not complete; no tools from this response were executed.")
	default:
		return errors.New("Model did not report a supported completion reason. The task did not complete; no tools from this response were executed.")
	}
}
