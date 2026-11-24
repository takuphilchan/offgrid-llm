package inference

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func boundedProviderScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(&providerBoundedReader{reader: reader, remaining: 32 << 20})
	scanner.Buffer(make([]byte, 4096), 2<<20)
	return scanner
}

// A byte cap must not masquerade as clean EOF after a terminal event. Read one
// extra byte only to distinguish an exact-size body from overflow, then fail.
type providerBoundedReader struct {
	reader    io.Reader
	remaining int
}

func (r *providerBoundedReader) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	if r.remaining == 0 {
		var extra [1]byte
		n, err := r.reader.Read(extra[:])
		if n > 0 {
			return 0, errors.New("HTTP provider stream exceeds total byte budget")
		}
		return 0, err
	}
	if len(buffer) > r.remaining {
		buffer = buffer[:r.remaining]
	}
	n, err := r.reader.Read(buffer)
	r.remaining -= n
	return n, err
}

func (p *HTTPProvider) generateOllama(ctx context.Context, requestID string, contextLength int, chat *api.ChatCompletionRequest, emit func(ProviderEvent) error) error {
	body, err := ollamaChatBody(chat, contextLength)
	if err != nil {
		return err
	}
	resp, err := p.request(ctx, http.MethodPost, "/api/chat", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	scanner := boundedProviderScanner(resp.Body)
	done := false
	var usage UsageMetrics
	reason := ""
	var calls []api.ToolCall
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		if done {
			return errors.New("Ollama emitted data after terminal response")
		}
		if jsonwire.Validate(raw) != nil {
			return errors.New("Ollama stream contains invalid JSON")
		}
		var chunk struct {
			Model       string          `json:"model"`
			RemoteModel string          `json:"remote_model"`
			RemoteHost  string          `json:"remote_host"`
			Error       json.RawMessage `json:"error"`
			Done        bool            `json:"done"`
			DoneReason  string          `json:"done_reason"`
			Prompt      *int            `json:"prompt_eval_count"`
			Completion  *int            `json:"eval_count"`
			Message     struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		}
		if json.Unmarshal(raw, &chunk) != nil || len(chunk.Error) > 0 && string(chunk.Error) != "null" || chunk.Model != chat.Model || chunk.RemoteModel != "" || chunk.RemoteHost != "" {
			return errors.New("Ollama stream failed or changed model identity")
		}
		if chunk.Message.Role != "" && chunk.Message.Role != "assistant" {
			return errors.New("Ollama emitted an invalid response role")
		}
		if chunk.Message.Content != "" {
			if err := emit(ProviderEvent{Type: ProviderEventDelta, Text: chunk.Message.Content}); err != nil {
				return err
			}
		}
		for _, call := range chunk.Message.ToolCalls {
			if len(calls) >= 16 || len(call.Function.Arguments) > 1<<20 {
				return errors.New("Ollama tool output exceeds bounds")
			}
			calls = append(calls, api.ToolCall{ID: fmt.Sprintf("%s-call-%d", requestID, len(calls)), Type: "function", Function: api.FunctionCall{Name: call.Function.Name, Arguments: string(call.Function.Arguments)}})
		}
		if chunk.Done {
			if chunk.Prompt == nil || chunk.Completion == nil {
				return errors.New("Ollama terminal response has no usage")
			}
			done = true
			reason = chunk.DoneReason
			usage = UsageMetrics{PromptTokens: *chunk.Prompt, CompletionTokens: *chunk.Completion, TotalTokens: *chunk.Prompt + *chunk.Completion}
		}
	}
	if err := scanner.Err(); err != nil {
		return errors.New("Ollama stream read failed")
	}
	if !done {
		return errors.New("Ollama stream ended without terminal response")
	}
	if len(calls) > 0 {
		if reason != "stop" && reason != "tool_calls" {
			return errors.New("Ollama tool response was truncated")
		}
		reason = "tool_calls"
	}
	return finishHTTPGeneration(chat, calls, usage, reason, emit)
}

func (p *HTTPProvider) generateVLLM(ctx context.Context, chat *api.ChatCompletionRequest, emit func(ProviderEvent) error) error {
	copy := *chat
	copy.Stream = true
	copy.StreamOptions = &api.StreamOptions{IncludeUsage: true}
	body, err := json.Marshal(copy)
	if err != nil {
		return err
	}
	resp, err := p.request(ctx, http.MethodPost, "/v1/chat/completions", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	scanner := boundedProviderScanner(resp.Body)
	var data strings.Builder
	done := false
	reason := ""
	var usage *UsageMetrics
	calls := map[int]*api.ToolCall{}
	consume := func() error {
		if data.Len() == 0 {
			return nil
		}
		raw := strings.TrimSuffix(data.String(), "\n")
		data.Reset()
		if done {
			return errors.New("vLLM emitted data after terminal response")
		}
		if raw == "[DONE]" {
			done = true
			return nil
		}
		if jsonwire.Validate([]byte(raw)) != nil {
			return errors.New("vLLM stream contains invalid JSON")
		}
		var chunk struct {
			Model   string          `json:"model"`
			Error   json.RawMessage `json:"error"`
			Usage   *UsageMetrics   `json:"usage"`
			Choices []struct {
				Index        int     `json:"index"`
				FinishReason *string `json:"finish_reason"`
				Delta        struct {
					Role      string  `json:"role"`
					Content   *string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(raw), &chunk) != nil || len(chunk.Error) > 0 && string(chunk.Error) != "null" || chunk.Model != chat.Model || len(chunk.Choices) > 1 {
			return errors.New("vLLM stream failed or changed model identity")
		}
		if chunk.Usage != nil {
			if usage != nil {
				return errors.New("vLLM repeated final usage")
			}
			usage = chunk.Usage
		}
		if len(chunk.Choices) == 1 {
			choice := chunk.Choices[0]
			if choice.Index != 0 || choice.Delta.Role != "" && choice.Delta.Role != "assistant" {
				return errors.New("vLLM returned an unsupported choice")
			}
			if reason != "" {
				return errors.New("vLLM emitted choice after finish")
			}
			if choice.Delta.Content != nil && *choice.Delta.Content != "" {
				if usage != nil {
					return errors.New("vLLM emitted text after final usage")
				}
				if err := emit(ProviderEvent{Type: ProviderEventDelta, Text: *choice.Delta.Content}); err != nil {
					return err
				}
			}
			for _, part := range choice.Delta.ToolCalls {
				if part.Index < 0 || part.Index >= 16 || usage != nil {
					return errors.New("vLLM tool fragment is outside admitted bounds")
				}
				call := calls[part.Index]
				if call == nil {
					call = &api.ToolCall{}
					calls[part.Index] = call
				}
				if part.ID != "" {
					if call.ID != "" && call.ID != part.ID {
						return errors.New("vLLM changed tool identity")
					}
					call.ID = part.ID
				}
				if part.Type != "" {
					if part.Type != "function" {
						return errors.New("vLLM returned non-function tool")
					}
					call.Type = part.Type
				}
				call.Function.Name += part.Function.Name
				call.Function.Arguments += part.Function.Arguments
				if len(call.Function.Name) > 256 || len(call.Function.Arguments) > 1<<20 {
					return errors.New("vLLM tool output exceeds bounds")
				}
			}
			if choice.FinishReason != nil {
				reason = *choice.FinishReason
			}
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := consume(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			part := strings.TrimPrefix(line, "data:")
			part = strings.TrimPrefix(part, " ")
			if data.Len()+len(part) > 2<<20 {
				return errors.New("vLLM event exceeds bounds")
			}
			data.WriteString(part)
			data.WriteByte('\n')
		}
	}
	if scanner.Err() != nil {
		return errors.New("vLLM stream read failed")
	}
	if data.Len() > 0 {
		return errors.New("vLLM stream ended inside an event")
	}
	if !done || usage == nil {
		return errors.New("vLLM stream ended without terminal status and usage")
	}
	ordered := make([]api.ToolCall, len(calls))
	for i := range ordered {
		call := calls[i]
		if call == nil {
			return errors.New("vLLM tool indices are discontinuous")
		}
		ordered[i] = *call
	}
	return finishHTTPGeneration(chat, ordered, *usage, reason, emit)
}

func finishHTTPGeneration(chat *api.ChatCompletionRequest, calls []api.ToolCall, usage UsageMetrics, reason string, emit func(ProviderEvent) error) error {
	if reason != "stop" && reason != "length" && reason != "tool_calls" {
		return errors.New("HTTP provider did not finish normally")
	}
	if len(calls) > 0 {
		response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Index: 0, FinishReason: reason, Message: api.ChatMessage{Role: "assistant", ToolCalls: calls}}}}
		if err := ValidateToolDecision(chat, response); err != nil {
			return err
		}
		raw, _ := json.Marshal(calls)
		if err := emit(ProviderEvent{Type: ProviderEventTool, Tool: raw}); err != nil {
			return err
		}
	}
	if err := emit(ProviderEvent{Type: ProviderEventUsage, Usage: &usage}); err != nil {
		return err
	}
	return emit(ProviderEvent{Type: ProviderEventDone, FinishReason: reason})
}
