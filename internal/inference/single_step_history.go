//go:build !llama

package inference

import (
	"encoding/json"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Round-trip committed decisions through the same envelope the governed model
// is asked to emit. An unrelated previous_tool_calls dialect teaches a different
// format on every turn. This is an inference projection only: no prose parsing,
// dispatch, transcript mutation or promotion of malformed/legacy batches.
func singleStepHistoryMessage(message api.ChatMessage) api.ChatMessage {
	if message.Role == "assistant" {
		var value any
		if len(message.ToolCalls) == 1 && strings.TrimSpace(message.StringContent()) == "" {
			call := message.ToolCalls[0]
			args := []byte(call.Function.Arguments)
			var object map[string]json.RawMessage
			if call.Function.Name != "" && jsonwire.Validate(args) == nil && json.Unmarshal(args, &object) == nil && object != nil {
				// Match the discriminator-first order used by the live grammar.
				// A map would alphabetically place arguments before the tool name.
				type recordedCall struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				}
				value = struct {
					Call recordedCall `json:"tool_call"`
				}{recordedCall{call.Function.Name, json.RawMessage(args)}}
			}
		}
		if value == nil && len(message.ToolCalls) > 0 {
			value = map[string]any{"historical_record_not_a_new_decision": true, "previous_tool_calls": message.ToolCalls, "content": message.Content}
		}
		if len(message.ToolCalls) == 0 {
			if text, ok := message.Content.(string); ok {
				// Plain answers use the other branch of the same contract. Treat
				// JSON-looking prose literally; never parse it into a tool call.
				value = map[string]any{"message": text}
			}
		}
		if value != nil {
			data, _ := json.Marshal(value)
			message.Content = string(data)
			message.ToolCalls = nil
		}
	}
	if message.Role == "tool" {
		content := message.Content
		if text, ok := content.(string); ok && jsonwire.Validate([]byte(text)) == nil {
			// A structured result is already JSON. Embedding it as a string
			// escapes all of its text again, obscuring loaded instructions and
			// observations. Preserve validated JSON values as data, never calls.
			content = json.RawMessage(text)
		}
		data, _ := json.Marshal(map[string]any{"tool_result": map[string]any{"name": message.Name, "content": content}})
		message.Role, message.Content, message.Name, message.ToolCallID = "user", string(data), "", ""
	}
	return message
}
