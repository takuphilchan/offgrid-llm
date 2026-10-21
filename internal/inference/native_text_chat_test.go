package inference

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestNativeTextPromptPreservesExplicitLimitAndDefersDefaultToTokenizer(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", ContextLength: 1024, Capabilities: []ProviderCapability{CapabilityText, CapabilityStreaming}}
	request := &api.ChatCompletionRequest{Model: "native", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}}
	prompt, err := nativeTextPrompt(request, descriptor)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(prompt, &body); err != nil {
		t.Fatal(err)
	}
	if _, supplied := body["max_tokens"]; supplied {
		t.Fatal("implicit output limit was fixed before tokenizer context admission")
	}
	limit := 16
	request.MaxTokens = &limit
	prompt, err = nativeTextPrompt(request, descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(prompt, &body); err != nil {
		t.Fatal(err)
	}
	if string(body["max_tokens"]) != "16" {
		t.Fatalf("explicit user output limit was lost: %s", prompt)
	}
}

type nativeTextChatFixture struct {
	schedulerFake
	terminal     bool
	finishReason string
	omitUsage    bool
	lastRequest  ProviderRequest
}

func (f *nativeTextChatFixture) Generate(_ context.Context, request ProviderRequest, callback func(ProviderEvent) error) error {
	f.generates.Add(1)
	f.lastRequest = request
	if err := callback(ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDelta, RequestID: request.RequestID, Sequence: 1, Text: "café 日本語"}); err != nil {
		return err
	}
	if f.terminal {
		sequence := uint64(2)
		if !f.omitUsage {
			if err := callback(ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventUsage, RequestID: request.RequestID, Sequence: sequence, Usage: &UsageMetrics{PromptTokens: 8, CompletionTokens: 3, TotalTokens: 11}}); err != nil {
				return err
			}
			sequence++
		}
		reason := f.finishReason
		if reason == "" {
			reason = "stop"
		}
		return callback(ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDone, RequestID: request.RequestID, Sequence: sequence, FinishReason: reason})
	}
	return nil
}

func TestNativeTextChatNormalizesOnlyCompleteTextAndRejectsUnsupportedControls(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", ContextLength: 2048, Capabilities: []ProviderCapability{CapabilityText, CapabilityStreaming}}
	fixture := &nativeTextChatFixture{schedulerFake: schedulerFake{descriptor: descriptor}, terminal: true}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return fixture, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	request := &api.ChatCompletionRequest{Model: "native", Messages: []api.ChatMessage{{Role: "user", Content: "say hello"}}}
	var deltas []string
	result, err := GenerateNativeTextChat(context.Background(), scheduler, descriptor, request, func(s string) error { deltas = append(deltas, s); return nil })
	if err != nil || result.Response == nil || len(result.Response.Choices) != 1 || result.Response.Choices[0].Message.Content != "café 日本語" || len(deltas) != 1 {
		t.Fatalf("normalized native response: %+v, deltas=%v, err=%v", result, deltas, err)
	}
	if result.Response.Usage.PromptTokens != 8 || result.Response.Usage.CompletionTokens != 3 || result.Response.Usage.TotalTokens != 11 {
		t.Fatalf("native token usage was lost: %+v", result.Response.Usage)
	}
	if fixture.lastRequest.MaxOutputTokens != 512 || fixture.lastRequest.MaxOutputBytes < 64<<10 {
		t.Fatalf("native default output budget was not admitted: %+v", fixture.lastRequest)
	}
	explicitLimit := 2
	request.MaxTokens = &explicitLimit
	if overBudget, err := GenerateNativeTextChat(context.Background(), scheduler, descriptor, request, nil); err == nil || overBudget.Response != nil || fixture.lastRequest.MaxOutputTokens != explicitLimit {
		t.Fatal("native worker exceeded the user's explicit output token budget")
	}
	request.MaxTokens = nil
	fixture.finishReason = "length"
	limited, err := GenerateNativeTextChat(context.Background(), scheduler, descriptor, request, nil)
	if err != nil || limited.Response == nil || limited.Response.Choices[0].FinishReason != "length" {
		t.Fatalf("token-limited response lost its finish reason: %+v, err=%v", limited, err)
	}
	fixture.finishReason = "cancelled"
	if cancelled, err := GenerateNativeTextChat(context.Background(), scheduler, descriptor, request, nil); err == nil || cancelled.Response != nil {
		t.Fatal("cancelled generation became a completed answer")
	}
	fixture.finishReason = ""
	fixture.omitUsage = true
	if missing, err := GenerateNativeTextChat(context.Background(), scheduler, descriptor, request, nil); err == nil || missing.Response != nil {
		t.Fatal("native worker without measured usage became a completed answer")
	}
	fixture.omitUsage = false
	fixture.terminal = false
	if result, err := GenerateNativeTextChat(context.Background(), scheduler, descriptor, request, nil); err == nil || result.Response != nil {
		t.Fatal("truncated worker stream became a completed response")
	}
	fixture.terminal = true
	request.Tools = []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "unsafe"}}}
	before := fixture.generates.Load()
	if _, err := GenerateNativeTextChat(context.Background(), scheduler, descriptor, request, nil); err == nil || fixture.generates.Load() != before {
		t.Fatal("unqualified native tool request reached worker")
	}
	request.Tools = nil
	request.Messages = []api.ChatMessage{{Role: "user", Content: []api.ChatContentPart{{Type: "image_url"}}}}
	if _, err := GenerateNativeTextChat(context.Background(), scheduler, descriptor, request, nil); err == nil || fixture.generates.Load() != before {
		t.Fatal("unqualified multimodal request reached worker")
	}
	request.Messages = []api.ChatMessage{{Role: "user", Content: "hello"}}
	if _, err := GenerateNativeTextChat(context.Background(), scheduler, descriptor, request, func(string) error { return errors.New("client left") }); err == nil {
		t.Fatal("stream callback failure became a completed answer")
	}
}
