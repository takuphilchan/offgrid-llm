package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// LlamaCPPProvider adapts the existing llama engine without creating another
// model cache or changing its binary, CPU/GPU, or projector lifecycle.
type LlamaCPPProvider struct {
	engine     Engine
	descriptor ProviderDescriptor
	mu         sync.Mutex
	active     map[string]context.CancelFunc
	seen       map[string]struct{}
	ownEngine  bool
	closed     bool
}

func NewLlamaCPPProvider(serverURL string, descriptor ProviderDescriptor) (*LlamaCPPProvider, error) {
	p, err := NewLlamaCPPProviderForEngine(NewLlamaHTTPEngine(serverURL), descriptor)
	if err != nil {
		return nil, err
	}
	p.ownEngine = true
	return p, nil
}

// NewLlamaCPPProviderForEngine wraps the service's already-owned engine.
func NewLlamaCPPProviderForEngine(engine Engine, descriptor ProviderDescriptor) (*LlamaCPPProvider, error) {
	if engine == nil {
		return nil, errors.New("llama provider requires an engine")
	}
	if descriptor.Provider == "" {
		descriptor.Provider = ProviderLlamaCPP
	}
	if descriptor.Format == "" {
		descriptor.Format = FormatGGUF
	}
	if err := descriptor.Validate(); err != nil {
		return nil, err
	}
	if descriptor.Provider != ProviderLlamaCPP || descriptor.Format != FormatGGUF {
		return nil, errors.New("llama provider requires a GGUF descriptor")
	}
	return &LlamaCPPProvider{engine: engine, descriptor: descriptor, active: make(map[string]context.CancelFunc), seen: make(map[string]struct{})}, nil
}

func (p *LlamaCPPProvider) Descriptor(context.Context) (ProviderDescriptor, error) {
	return p.descriptor, nil
}

func (p *LlamaCPPProvider) Resources(context.Context) (ProviderResources, error) {
	return ProviderResources{Backend: p.descriptor.Backend, Device: "llama.cpp runtime", MemoryMeasured: false}, nil
}

func (p *LlamaCPPProvider) Generate(ctx context.Context, req ProviderRequest, callback func(ProviderEvent) error) error {
	if err := req.Validate(); err != nil {
		return err
	}
	if req.ModelDigest != p.descriptor.ModelDigest || callback == nil {
		return errors.New("llama request identity or callback is invalid")
	}
	if p.descriptor.ContextLength > 0 && req.ContextLength > p.descriptor.ContextLength {
		return errors.New("llama request exceeds the loaded model context")
	}
	var chat api.ChatCompletionRequest
	if err := json.Unmarshal(req.Prompt, &chat); err != nil {
		return fmt.Errorf("decode llama chat request: %w", err)
	}
	var declared []api.Tool
	if len(req.Tools) != 0 && json.Unmarshal(req.Tools, &declared) != nil || !sameTools(declared, chat.Tools) {
		return errors.New("llama request tool menu differs from the validated prompt")
	}
	if len(chat.Tools) > 0 && !slices.Contains(p.descriptor.Capabilities, CapabilityToolCalls) {
		return errors.New("llama tool decisions are not qualified for this runtime")
	}
	p.mu.Lock()
	if p.closed || p.active[req.CancellationID] != nil {
		p.mu.Unlock()
		return errors.New("llama provider is closed or cancellation identity is active")
	}
	if _, used := p.seen[req.RequestID]; used || len(p.seen) >= 10000 {
		p.mu.Unlock()
		return errors.New("llama request identity was already used or exhausted")
	}
	runCtx, cancel := context.WithCancel(ctx)
	p.active[req.CancellationID] = cancel
	p.seen[req.RequestID] = struct{}{}
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.active, req.CancellationID)
		p.mu.Unlock()
		cancel()
	}()

	var sequence uint64
	emit := func(kind ProviderEventType, content string, tool json.RawMessage, usage *UsageMetrics, finish string) error {
		sequence++
		return callback(ProviderEvent{ProtocolRevision: p.descriptor.ProtocolRevision, Type: kind, RequestID: req.RequestID, Sequence: sequence, Text: content, Tool: tool, Usage: usage, FinishReason: finish})
	}
	if len(chat.Tools) == 0 {
		if raw, ok := p.engine.(RawStreamingEngine); ok {
			return p.streamRawText(runCtx, raw, &chat, emit)
		}
		if err := p.engine.ChatCompletionStream(runCtx, &chat, func(token string) error {
			if token == "" {
				return nil
			}
			return emit(ProviderEventDelta, token, nil, nil, "")
		}); err != nil {
			return err
		}
		if err := runCtx.Err(); err != nil {
			return err
		}
		return emit(ProviderEventDone, "", nil, nil, "stop")
	}
	// Structured calls are emitted only after a complete response has passed
	// the same registered-tool and JSON-schema gate used by the agent runner.
	response, err := p.engine.ChatCompletion(runCtx, &chat)
	if err != nil {
		return err
	}
	if err := runCtx.Err(); err != nil {
		return err
	}
	if err := ValidateToolDecision(&chat, response); err != nil {
		return err
	}
	choice := response.Choices[0]
	if len(choice.Message.ToolCalls) > 0 {
		body, err := json.Marshal(choice.Message.ToolCalls)
		if err != nil {
			return err
		}
		if err := emit(ProviderEventTool, "", body, nil, ""); err != nil {
			return err
		}
	} else if content := choice.Message.StringContent(); content != "" {
		if err := emit(ProviderEventDelta, content, nil, nil, ""); err != nil {
			return err
		}
	}
	usage := &UsageMetrics{PromptTokens: response.Usage.PromptTokens, CompletionTokens: response.Usage.CompletionTokens, TotalTokens: response.Usage.TotalTokens}
	return emit(ProviderEventDone, "", nil, usage, choice.FinishReason)
}

// streamRawText keeps llama's OpenAI-compatible stream metadata inside the
// normalized provider contract. It never turns a partial stream or a tool
// delta in a text-only request into a terminal answer.
func (p *LlamaCPPProvider) streamRawText(ctx context.Context, engine RawStreamingEngine, chat *api.ChatCompletionRequest, emit func(ProviderEventType, string, json.RawMessage, *UsageMetrics, string) error) error {
	request := *chat
	request.StreamOptions = &api.StreamOptions{IncludeUsage: true}
	var finishReason string
	var usage *UsageMetrics
	var sawText bool
	err := engine.ChatCompletionStreamRaw(ctx, &request, func(data json.RawMessage) error {
		if jsonwire.Validate(data) != nil {
			return errors.New("llama stream chunk is ambiguous or malformed")
		}
		var chunk struct {
			Error   json.RawMessage `json:"error"`
			Choices []struct {
				Index int `json:"index"`
				Delta struct {
					Content   json.RawMessage `json:"content"`
					ToolCalls json.RawMessage `json:"tool_calls"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *api.Usage `json:"usage"`
		}
		if err := json.Unmarshal(data, &chunk); err != nil {
			return fmt.Errorf("invalid llama stream chunk: %w", err)
		}
		if len(chunk.Error) != 0 && string(chunk.Error) != "null" {
			return errors.New("llama stream reported an inference failure")
		}
		if len(chunk.Choices) > 1 {
			return errors.New("llama stream returned multiple choices")
		}
		if len(chunk.Choices) == 1 {
			choice := chunk.Choices[0]
			if choice.Index != 0 {
				return errors.New("llama text stream returned an unsupported decision")
			}
			if len(choice.Delta.ToolCalls) != 0 && string(choice.Delta.ToolCalls) != "null" {
				var calls []json.RawMessage
				if json.Unmarshal(choice.Delta.ToolCalls, &calls) != nil || len(calls) != 0 {
					return errors.New("llama text stream returned an unsupported decision")
				}
			}
			if len(choice.Delta.Content) != 0 && string(choice.Delta.Content) != "null" {
				if finishReason != "" {
					return errors.New("llama stream continued after completion")
				}
				var content string
				if err := json.Unmarshal(choice.Delta.Content, &content); err != nil {
					return errors.New("llama text stream returned non-text content")
				}
				if content != "" {
					sawText = true
					if err := emit(ProviderEventDelta, content, nil, nil, ""); err != nil {
						return err
					}
				}
			}
			if choice.FinishReason != nil {
				if finishReason != "" || *choice.FinishReason != "stop" && *choice.FinishReason != "length" {
					return errors.New("llama text stream did not finish with a supported reason")
				}
				finishReason = *choice.FinishReason
			}
		}
		if chunk.Usage != nil {
			value := *chunk.Usage
			if usage != nil || value.PromptTokens < 0 || value.CompletionTokens < 0 || value.TotalTokens != value.PromptTokens+value.CompletionTokens {
				return errors.New("llama stream returned inconsistent usage")
			}
			usage = &UsageMetrics{PromptTokens: value.PromptTokens, CompletionTokens: value.CompletionTokens, TotalTokens: value.TotalTokens}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if finishReason == "" || !sawText {
		return errors.New("llama text stream ended without a complete answer")
	}
	if usage != nil {
		if err := emit(ProviderEventUsage, "", nil, usage, ""); err != nil {
			return err
		}
	}
	return emit(ProviderEventDone, "", nil, nil, finishReason)
}

func sameTools(a, b []api.Tool) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && string(left) == string(right)
}

func (p *LlamaCPPProvider) Cancel(_ context.Context, cancellationID string) error {
	p.mu.Lock()
	cancel := p.active[cancellationID]
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (p *LlamaCPPProvider) Health(context.Context) error {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed || !p.engine.IsLoaded() {
		return errors.New("llama provider is not loaded")
	}
	return nil
}

func (p *LlamaCPPProvider) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	for _, cancel := range p.active {
		cancel()
	}
	p.mu.Unlock()
	// A service-owned engine belongs to the existing model cache and must not
	// be unloaded by closing this adapter.
	if p.ownEngine {
		return p.engine.Unload()
	}
	return nil
}
