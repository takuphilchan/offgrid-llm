package inference

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// NativeTextChatResult contains only a terminally verified response. Deltas
// may be shown while generating, but callers must not persist them as a final
// answer after a worker error, cancellation, or truncated stream.
type NativeTextChatResult struct {
	Response *api.ChatCompletionResponse
	Timing   ProviderTiming
}

// ValidateNativeTextChatRequest checks the same adapter contract used at
// dispatch, allowing transports to reject unsupported inputs before SSE starts.
func ValidateNativeTextChatRequest(request *api.ChatCompletionRequest, descriptor ProviderDescriptor) error {
	_, err := nativeTextPrompt(request, descriptor)
	return err
}

func nativeTextPrompt(request *api.ChatCompletionRequest, descriptor ProviderDescriptor) (json.RawMessage, error) {
	if request == nil || request.Model == "" || len(request.Messages) == 0 {
		return nil, errors.New("native text request requires a model and messages")
	}
	if err := request.ResponseFormat.Validate(); err != nil {
		return nil, err
	}
	if request.ResponseFormat.Constrained() {
		return nil, &api.UnsupportedControlError{Control: "response_format for native text provider"}
	}
	if request.Temperature != nil || request.TopP != nil || request.Seed != nil || len(request.Stop) != 0 || request.PresencePenalty != nil || request.FrequencyPenalty != nil || request.N != nil && *request.N != 1 || len(request.Tools) != 0 || request.ToolChoice != nil || request.ParallelToolCalls != nil {
		return nil, errors.New("native text worker does not support the requested generation or tool controls")
	}
	if request.MaxTokens != nil && (*request.MaxTokens < 1 || *request.MaxTokens > 8192) {
		return nil, errors.New("native text max_tokens is outside the worker limit")
	}
	if descriptor.Provider != ProviderMLX && descriptor.Provider != ProviderTransformers || !hasProviderCapability(descriptor, CapabilityText) || !hasProviderCapability(descriptor, CapabilityStreaming) {
		return nil, errors.New("selected provider is not a qualified streaming text runtime")
	}
	if err := descriptor.Validate(); err != nil {
		return nil, err
	}
	values := make([]struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}, 0, len(request.Messages))
	for _, message := range request.Messages {
		content, ok := message.Content.(string)
		if !ok || (message.Role != "system" && message.Role != "user" && message.Role != "assistant") || len(message.ToolCalls) != 0 || message.ToolCallID != "" || message.Name != "" {
			return nil, errors.New("native text worker does not support multimodal or tool-role messages")
		}
		values = append(values, struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{message.Role, content})
	}
	return json.Marshal(struct {
		Messages  any  `json:"messages"`
		MaxTokens *int `json:"max_tokens,omitempty"`
	}{values, request.MaxTokens})
}

func hasProviderCapability(descriptor ProviderDescriptor, capability ProviderCapability) bool {
	for _, candidate := range descriptor.Capabilities {
		if candidate == capability {
			return true
		}
	}
	return false
}

// GenerateNativeTextChat is the strict text-only adapter to the shared
// provider scheduler. It grants no model selection, authorization, or tool
// capability; those boundaries remain with the service and NativePackageRoutes.
func GenerateNativeTextChat(ctx context.Context, scheduler *ProviderScheduler, descriptor ProviderDescriptor, request *api.ChatCompletionRequest, onDelta func(string) error) (NativeTextChatResult, error) {
	var result NativeTextChatResult
	if scheduler == nil {
		return result, errors.New("native provider scheduler is unavailable")
	}
	prompt, err := nativeTextPrompt(request, descriptor)
	if err != nil {
		return result, err
	}
	var identity [16]byte
	if _, err := rand.Read(identity[:]); err != nil {
		return result, err
	}
	id := hex.EncodeToString(identity[:])
	providerRequest := ProviderRequest{RequestID: id, CancellationID: id, ModelDigest: descriptor.ModelDigest, Prompt: prompt, ContextLength: descriptor.ContextLength}
	providerRequest.MaxOutputTokens = 512 // worker's implicit text limit
	if request.MaxTokens != nil {
		providerRequest.MaxOutputTokens = *request.MaxTokens
	} else if descriptor.ContextLength > 0 && descriptor.ContextLength < providerRequest.MaxOutputTokens {
		providerRequest.MaxOutputTokens = descriptor.ContextLength
	}
	providerRequest.MaxOutputBytes = max(64<<10, providerRequest.MaxOutputTokens*512)
	if providerRequest.MaxOutputBytes > 8<<20 {
		providerRequest.MaxOutputBytes = 8 << 20
	}
	var answer strings.Builder
	var usage UsageMetrics
	var usageSeen bool
	var finished bool
	var finishReason string
	timing, err := scheduler.GenerateTimed(ctx, descriptor, providerRequest, func(event ProviderEvent) error {
		switch event.Type {
		case ProviderEventDelta:
			answer.WriteString(event.Text)
			if onDelta != nil {
				return onDelta(event.Text)
			}
		case ProviderEventUsage:
			if event.Usage != nil {
				usage = *event.Usage
				usageSeen = true
			}
		case ProviderEventDone:
			if event.FinishReason != "stop" && event.FinishReason != "length" {
				return errors.New("native text generation did not finish normally")
			}
			finished = true
			finishReason = event.FinishReason
		case ProviderEventTool:
			return errors.New("native text worker emitted an unqualified tool decision")
		case ProviderEventError:
			return errors.New("native text worker reported a generation failure")
		}
		return nil
	})
	result.Timing = timing
	if err != nil {
		return result, err
	}
	if !finished || !usageSeen || answer.Len() == 0 {
		return result, errors.New("native text worker did not return a complete answer")
	}
	result.Response = &api.ChatCompletionResponse{
		ID: "chatcmpl-" + id, Object: "chat.completion", Created: time.Now().Unix(), Model: request.Model,
		Choices: []api.ChatCompletionChoice{{Index: 0, Message: api.ChatMessage{Role: "assistant", Content: answer.String()}, FinishReason: finishReason}},
		Usage:   api.Usage{PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens, TotalTokens: usage.TotalTokens},
	}
	return result, nil
}
