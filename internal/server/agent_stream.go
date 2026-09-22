package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) streamAgentModel(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool, emit func(string, string) error) (*api.ChatCompletionResponse, error) {
	if err := emit("queued", ""); err != nil {
		return nil, err
	}
	release, err := s.acquireInferenceContext(ctx, task.Model, s.effectiveContextWindow(), func() error { return emit("loading", "") })
	if err != nil {
		return nil, err
	}
	defer release()
	messages, err = s.attachTransientComputerCapture(task, messages)
	if err != nil {
		return nil, err
	}
	if err := emit("processing", ""); err != nil {
		return nil, err
	}
	temperature := float32(task.Config.Temperature)
	maxTokens := task.Config.MaxTokens
	request := &api.ChatCompletionRequest{Model: task.Model, Messages: messages, Tools: tools, ToolChoice: "auto", Temperature: &temperature, MaxTokens: &maxTokens, Stream: true}
	configureComputerRequest(request, task)
	raw, ok := s.engine.(inference.RawStreamingEngine)
	if !ok {
		// A text-only stream discards structured tool calls and finish reasons.
		// Preserve safety for such engines instead of fabricating a completion.
		request.Stream = false
		return s.engine.ChatCompletion(ctx, request)
	}
	accumulator := agentStreamAccumulator{calls: make(map[int]*api.ToolCall)}
	err = raw.ChatCompletionStreamRaw(ctx, request, func(data json.RawMessage) error {
		text, toolDelta, err := accumulator.add(data)
		if err != nil {
			return err
		}
		if text != "" {
			return emit("generating", text)
		}
		if toolDelta {
			return emit("preparing_tool", "")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return accumulator.response(), nil
}

// Assemble structured calls privately. Never expose arguments or reasoning
// deltas, and never execute a partial call. Runner validates terminal reasons
// and complete arguments before its existing exact-call approval boundary.
type agentStreamAccumulator struct {
	text   strings.Builder
	calls  map[int]*api.ToolCall
	finish string
	bytes  int
}

func (a *agentStreamAccumulator) add(data json.RawMessage) (string, bool, error) {
	var chunk struct {
		Error   json.RawMessage `json:"error"`
		Choices []struct {
			Index int `json:"index"`
			Delta struct {
				Content   string `json:"content"`
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
	if err := json.Unmarshal(data, &chunk); err != nil {
		return "", false, fmt.Errorf("invalid agent stream chunk: %w", err)
	}
	if len(chunk.Error) > 0 && string(chunk.Error) != "null" {
		return "", false, fmt.Errorf("agent model stream failed")
	}
	if len(chunk.Choices) == 0 {
		return "", false, nil
	}
	if len(chunk.Choices) != 1 || chunk.Choices[0].Index != 0 || a.finish != "" {
		return "", false, fmt.Errorf("unexpected agent stream choice")
	}
	choice := chunk.Choices[0]
	a.bytes += len(choice.Delta.Content)
	a.text.WriteString(choice.Delta.Content)
	for _, delta := range choice.Delta.ToolCalls {
		if delta.Index == nil || *delta.Index < 0 || *delta.Index >= 16 || (delta.Type != "" && delta.Type != "function") {
			return "", false, fmt.Errorf("invalid streamed tool index or type")
		}
		call := a.calls[*delta.Index]
		if call == nil {
			call = &api.ToolCall{Type: "function"}
			a.calls[*delta.Index] = call
		}
		call.ID += delta.ID
		call.Function.Name += delta.Function.Name
		call.Function.Arguments += delta.Function.Arguments
		a.bytes += len(delta.ID) + len(delta.Function.Name) + len(delta.Function.Arguments)
		if len(call.ID) > 512 || len(call.Function.Name) > 256 || len(call.Function.Arguments) > 256*1024 {
			return "", false, fmt.Errorf("streamed tool exceeds limit")
		}
	}
	if a.bytes > 4*1024*1024 {
		return "", false, fmt.Errorf("agent stream exceeds response limit")
	}
	if choice.FinishReason != nil {
		a.finish = *choice.FinishReason
	}
	return choice.Delta.Content, len(choice.Delta.ToolCalls) > 0, nil
}

func (a *agentStreamAccumulator) response() *api.ChatCompletionResponse {
	message := api.ChatMessage{Role: "assistant", Content: a.text.String()}
	indexes := make([]int, 0, len(a.calls))
	for index := range a.calls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	for _, index := range indexes {
		message.ToolCalls = append(message.ToolCalls, *a.calls[index])
	}
	return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Index: 0, Message: message, FinishReason: a.finish}}}
}
