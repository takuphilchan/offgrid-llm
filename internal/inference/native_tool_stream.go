//go:build !llama

package inference

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Native tools retain the model's chat template. Only ordinary answer deltas
// may escape before completion; tool fragments and terminal success stay private
// until DONE and the shared decision gate validate the whole response.
type nativeToolStream struct {
	text   strings.Builder
	calls  [16]*api.ToolCall
	finish string
	bytes  int
	meta   api.ChatCompletionChunk
}

func (s *nativeToolStream) add(data json.RawMessage, emit ChatCompletionStreamCallback) error {
	var chunk struct {
		ID      string          `json:"id"`
		Object  string          `json:"object"`
		Created int64           `json:"created"`
		Model   string          `json:"model"`
		Usage   *api.Usage      `json:"usage"`
		Error   json.RawMessage `json:"error"`
		Choices []struct {
			Index int `json:"index"`
			Delta struct {
				Role      string  `json:"role"`
				Content   *string `json:"content"`
				ToolCalls []struct {
					Index    *int             `json:"index"`
					ID       string           `json:"id"`
					Type     string           `json:"type"`
					Function api.FunctionCall `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
	}
	if jsonwire.Validate(data) != nil || json.Unmarshal(data, &chunk) != nil {
		return fmt.Errorf("invalid native tool stream")
	}
	if len(chunk.Error) != 0 && string(chunk.Error) != "null" {
		return classifyNativeStreamError(chunk.Error)
	}
	if s.meta.ID != "" && chunk.ID != "" && s.meta.ID != chunk.ID {
		return fmt.Errorf("native stream response identity changed")
	}
	if chunk.ID != "" {
		s.meta.ID = chunk.ID
	}
	if chunk.Model != "" {
		s.meta.Model = chunk.Model
	}
	if chunk.Object != "" {
		s.meta.Object = chunk.Object
	}
	if chunk.Created != 0 {
		s.meta.Created = chunk.Created
	}
	if chunk.Usage != nil {
		s.meta.Usage = chunk.Usage
	}
	if len(chunk.Choices) == 0 {
		return nil
	}
	if len(chunk.Choices) != 1 || chunk.Choices[0].Index != 0 || s.finish != "" {
		return fmt.Errorf("unexpected native tool stream choice")
	}
	choice := chunk.Choices[0]
	if choice.Delta.Role != "" && choice.Delta.Role != "assistant" {
		return fmt.Errorf("invalid native stream role")
	}
	for _, delta := range choice.Delta.ToolCalls {
		if delta.Index == nil || *delta.Index < 0 || *delta.Index >= len(s.calls) || delta.Type != "" && delta.Type != "function" {
			return fmt.Errorf("invalid native tool index or type")
		}
		call := s.calls[*delta.Index]
		if call == nil {
			call = &api.ToolCall{Type: "function"}
			s.calls[*delta.Index] = call
		}
		call.ID += delta.ID
		call.Function.Name += delta.Function.Name
		call.Function.Arguments += delta.Function.Arguments
		s.bytes += len(delta.ID) + len(delta.Function.Name) + len(delta.Function.Arguments)
		if len(call.ID) > 512 || len(call.Function.Name) > 256 || len(call.Function.Arguments) > 256*1024 {
			return fmt.Errorf("native tool exceeds response limit")
		}
	}
	if choice.Delta.Content != nil {
		s.bytes += len(*choice.Delta.Content)
		s.text.WriteString(*choice.Delta.Content)
	}
	if s.bytes > 4*1024*1024 {
		return fmt.Errorf("native stream exceeds response limit")
	}
	if choice.FinishReason != nil {
		s.finish = *choice.FinishReason
	}
	if choice.Delta.Content != nil || choice.Delta.Role != "" {
		out := s.meta
		out.Usage = nil
		out.Choices = []api.ChatCompletionChoiceChunk{{Index: 0, Delta: api.ChatMessage{Role: choice.Delta.Role, Content: choice.Delta.Content}}}
		encoded, err := json.Marshal(out)
		if err != nil {
			return err
		}
		return emit(encoded)
	}
	return nil
}

// Worker parser errors can contain generated arguments and private content.
// Classify known tool failures without returning or logging that payload.
func classifyNativeStreamError(raw json.RawMessage) error {
	var detail struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &detail)
	message := strings.ToLower(detail.Message)
	if strings.Contains(message, "failed to parse input") || strings.Contains(message, "failed to parse tool") || strings.Contains(message, "unparsed") {
		return &EngineError{Code: "invalid_model_tool_response", Message: "The model produced an invalid tool response. No tool call from this response was released. Try a model qualified for this agent's tool workflow; reinstalling the integration will not repair the response."}
	}
	return &EngineError{Code: "runtime_stream_error", Message: "The inference runtime interrupted its response. No tool call from this response was released."}
}

func (s *nativeToolStream) emit(req *api.ChatCompletionRequest, emit ChatCompletionStreamCallback) error {
	message := api.ChatMessage{Role: "assistant", Content: s.text.String()}
	missing := false
	for _, call := range s.calls {
		if call == nil {
			missing = true
			continue
		}
		if missing {
			return fmt.Errorf("noncontiguous native tool indexes")
		}
		message.ToolCalls = append(message.ToolCalls, *call)
	}
	response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Index: 0, Message: message, FinishReason: s.finish}}}
	if err := ValidateToolDecision(req, response); err != nil {
		return err
	}
	// Text has already streamed. Emit tool arguments once, after validation,
	// retaining native call IDs for the caller's subsequent tool-result turn.
	delta := map[string]any{}
	if len(message.ToolCalls) != 0 {
		calls := make([]any, 0, len(message.ToolCalls))
		for i, call := range message.ToolCalls {
			calls = append(calls, map[string]any{"index": i, "id": call.ID, "type": call.Type, "function": call.Function})
		}
		delta["tool_calls"] = calls
	}
	out := map[string]any{"id": s.meta.ID, "object": "chat.completion.chunk", "created": s.meta.Created, "model": s.meta.Model,
		"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": s.finish}}}
	if s.meta.Usage != nil {
		out["usage"] = s.meta.Usage
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return emit(encoded)
}
