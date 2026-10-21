package inference

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type scriptedProviderRuntime struct {
	descriptor          ProviderDescriptor
	events              []ProviderEvent
	result              error
	ignoreCallbackError bool
	resourceCalls       int
	dispatchResources   *ProviderResources
	dispatchResourceErr error
	finalResources      *ProviderResources
	finalResourceError  error
	generateCalls       int
}

func (p *scriptedProviderRuntime) Descriptor(context.Context) (ProviderDescriptor, error) {
	return p.descriptor, nil
}
func (p *scriptedProviderRuntime) Resources(context.Context) (ProviderResources, error) {
	p.resourceCalls++
	if p.resourceCalls == 2 {
		if p.dispatchResourceErr != nil {
			return ProviderResources{}, p.dispatchResourceErr
		}
		if p.dispatchResources != nil {
			return *p.dispatchResources, nil
		}
	}
	if p.resourceCalls >= 3 {
		if p.finalResourceError != nil {
			return ProviderResources{}, p.finalResourceError
		}
		if p.finalResources != nil {
			return *p.finalResources, nil
		}
	}
	return ProviderResources{Backend: p.descriptor.Backend, Device: "fixture"}, nil
}
func (p *scriptedProviderRuntime) Health(context.Context) error         { return nil }
func (p *scriptedProviderRuntime) Cancel(context.Context, string) error { return nil }
func (p *scriptedProviderRuntime) Close() error                         { return nil }
func (p *scriptedProviderRuntime) Generate(_ context.Context, _ ProviderRequest, callback func(ProviderEvent) error) error {
	p.generateCalls++
	for _, event := range p.events {
		if err := callback(event); err != nil {
			if !p.ignoreCallbackError {
				return err
			}
		}
	}
	return p.result
}

func TestProviderSchedulerRejectsChangedResourcesBeforeDispatch(t *testing.T) {
	descriptor, request, tool, done := scheduledToolFixture(t)
	worker := &scriptedProviderRuntime{descriptor: descriptor, events: []ProviderEvent{tool, done}, dispatchResources: &ProviderResources{Backend: "wrong", Device: "fixture"}}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	if err := scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error {
		t.Fatal("resource change reached the consumer")
		return nil
	}); err == nil || worker.generateCalls != 0 {
		t.Fatalf("generation began with changed resource identity: calls=%d err=%v", worker.generateCalls, err)
	}
}

func scheduledToolFixture(t *testing.T) (ProviderDescriptor, ProviderRequest, ProviderEvent, ProviderEvent) {
	t.Helper()
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderLlamaCPP, RunnerRevision: "fixture-1", ModelDigest: "sha256:fixture", Format: FormatGGUF, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText, CapabilityToolCalls}}
	tool := api.Tool{Type: "function", Function: api.FunctionDef{Name: "lookup", Parameters: map[string]any{"type": "object", "properties": map[string]any{"topic": map[string]any{"type": "string"}}, "required": []string{"topic"}, "additionalProperties": false}}}
	prompt, err := json.Marshal(api.ChatCompletionRequest{Model: "fixture", Messages: []api.ChatMessage{{Role: "user", Content: "Look up a topic"}}, Tools: []api.Tool{tool}})
	if err != nil {
		t.Fatal(err)
	}
	menu, err := json.Marshal([]api.Tool{tool})
	if err != nil {
		t.Fatal(err)
	}
	calls, err := json.Marshal([]api.ToolCall{{ID: "call-1", Type: "function", Function: api.FunctionCall{Name: "lookup", Arguments: `{"topic":"history"}`}}})
	if err != nil {
		t.Fatal(err)
	}
	request := ProviderRequest{RequestID: "request-1", CancellationID: "cancel-1", ModelDigest: descriptor.ModelDigest, Prompt: prompt, Tools: menu}
	decision := ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventTool, RequestID: request.RequestID, Sequence: 1, Tool: calls}
	done := ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDone, RequestID: request.RequestID, Sequence: 2, FinishReason: "tool_calls"}
	return descriptor, request, decision, done
}

func TestProviderSchedulerRejectsOverBudgetWorkerOutput(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderLlamaCPP, RunnerRevision: "fixture-1", ModelDigest: "sha256:fixture", Format: FormatGGUF, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	for _, test := range []struct {
		name   string
		events []ProviderEvent
		want   int
	}{
		{"reported tokens", []ProviderEvent{
			{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDelta, RequestID: "bounded", Sequence: 1, Text: "hello"},
			{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventUsage, RequestID: "bounded", Sequence: 2, Usage: &UsageMetrics{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}},
			{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDone, RequestID: "bounded", Sequence: 3, FinishReason: "stop"},
		}, 1},
		{"streamed bytes", []ProviderEvent{
			{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDelta, RequestID: "bounded", Sequence: 1, Text: "hello"},
			{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDelta, RequestID: "bounded", Sequence: 2, Text: "world!"},
			{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDone, RequestID: "bounded", Sequence: 3, FinishReason: "stop"},
		}, 1},
		{"missing usage", []ProviderEvent{
			{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDelta, RequestID: "bounded", Sequence: 1, Text: "hello"},
			{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDone, RequestID: "bounded", Sequence: 2, FinishReason: "stop"},
		}, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			worker := &scriptedProviderRuntime{descriptor: descriptor, events: test.events, ignoreCallbackError: true}
			registry := NewProviderRegistry()
			if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
				t.Fatal(err)
			}
			scheduler, err := NewProviderScheduler(registry, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer scheduler.Close()
			request := ProviderRequest{RequestID: "bounded", CancellationID: "bounded", ModelDigest: descriptor.ModelDigest, Prompt: json.RawMessage(`{"messages":[]}`), MaxOutputTokens: 2, MaxOutputBytes: 10}
			var delivered []ProviderEventType
			if err := scheduler.Generate(context.Background(), descriptor, request, func(event ProviderEvent) error {
				delivered = append(delivered, event.Type)
				return nil
			}); err == nil || len(delivered) != test.want || delivered[0] != ProviderEventDelta {
				t.Fatalf("over-budget worker completed or emitted unchecked output: events=%v err=%v", delivered, err)
			}
		})
	}
}

func TestProviderRequestBudgetIsLocalAndValidated(t *testing.T) {
	request := ProviderRequest{RequestID: "bounded", CancellationID: "bounded", ModelDigest: "sha256:fixture", Prompt: json.RawMessage(`{"messages":[]}`), ContextLength: 1024, MaxOutputTokens: 512, MaxOutputBytes: 64 << 10}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(wire, &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["MaxOutputTokens"]; exists {
		t.Fatal("local token budget entered worker protocol")
	}
	if _, exists := fields["MaxOutputBytes"]; exists {
		t.Fatal("local byte budget entered worker protocol")
	}
	request.MaxOutputTokens = 1025
	if err := request.Validate(); err == nil {
		t.Fatal("output budget exceeded admitted context")
	}
	request.MaxOutputTokens = 512
	request.MaxOutputBytes = 8<<20 + 1
	if err := request.Validate(); err == nil {
		t.Fatal("unbounded output byte budget was admitted")
	}
}

func TestProviderUsageClosesOutputStream(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderLlamaCPP, RunnerRevision: "fixture-1", ModelDigest: "sha256:fixture", Format: FormatGGUF, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	usage := ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventUsage, RequestID: "usage-closed", Sequence: 2, Usage: &UsageMetrics{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3}}
	for _, tc := range []struct {
		name  string
		third ProviderEvent
	}{
		{name: "text after usage", third: ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDelta, RequestID: "usage-closed", Sequence: 3, Text: "more"}},
		{name: "duplicate usage", third: ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventUsage, RequestID: "usage-closed", Sequence: 3, Usage: usage.Usage}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			worker := &scriptedProviderRuntime{descriptor: descriptor, ignoreCallbackError: true, events: []ProviderEvent{
				{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDelta, RequestID: "usage-closed", Sequence: 1, Text: "ok"},
				usage,
				tc.third,
				{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDone, RequestID: "usage-closed", Sequence: 4, FinishReason: "stop"},
			}}
			registry := NewProviderRegistry()
			if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
				t.Fatal(err)
			}
			scheduler, err := NewProviderScheduler(registry, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer scheduler.Close()
			request := ProviderRequest{RequestID: "usage-closed", CancellationID: "usage-closed", ModelDigest: descriptor.ModelDigest, Prompt: json.RawMessage(`{"messages":[]}`), MaxOutputTokens: 2}
			var delivered []ProviderEventType
			if err := scheduler.Generate(context.Background(), descriptor, request, func(event ProviderEvent) error {
				delivered = append(delivered, event.Type)
				return nil
			}); err == nil || len(delivered) != 2 || delivered[0] != ProviderEventDelta || delivered[1] != ProviderEventUsage {
				t.Fatalf("post-usage output was accepted: events=%v err=%v", delivered, err)
			}
		})
	}
}

func TestProviderToolDecisionHasBoundedArguments(t *testing.T) {
	_, request, _, _ := scheduledToolFixture(t)
	if err := validateScheduledTool(request, json.RawMessage(strings.Repeat(" ", 1<<20+1))); err == nil {
		t.Fatal("oversized provider tool decision was accepted")
	}
}

func TestSchedulerWithholdsToolUntilCompleteSuccessfulGeneration(t *testing.T) {
	descriptor, request, tool, done := scheduledToolFixture(t)
	for _, test := range []struct {
		name    string
		events  []ProviderEvent
		failure error
		want    []ProviderEventType
	}{
		{"complete", []ProviderEvent{tool, done}, nil, []ProviderEventType{ProviderEventTool, ProviderEventDone}},
		{"crash after tool", []ProviderEvent{tool}, errors.New("worker disconnected"), nil},
		{"truncated after tool", []ProviderEvent{tool}, nil, nil},
		{"crash after terminal", []ProviderEvent{tool, done}, errors.New("worker disconnected"), nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			worker := &scriptedProviderRuntime{descriptor: descriptor, events: test.events, result: test.failure}
			registry := NewProviderRegistry()
			if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
				t.Fatal(err)
			}
			scheduler, err := NewProviderScheduler(registry, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer scheduler.Close()
			var received []ProviderEventType
			err = scheduler.Generate(context.Background(), descriptor, request, func(event ProviderEvent) error {
				received = append(received, event.Type)
				return nil
			})
			if test.want == nil && err == nil || test.want != nil && err != nil {
				t.Fatalf("wrong terminal status: %v", err)
			}
			if len(received) != len(test.want) {
				t.Fatalf("unverified tool escaped or completed tool was lost: %v", received)
			}
			for i := range received {
				if received[i] != test.want[i] {
					t.Fatalf("provider events changed order: %v", received)
				}
			}
		})
	}
}

func TestProviderStreamGateRetainsIgnoredCallbackFailure(t *testing.T) {
	descriptor, request, tool, done := scheduledToolFixture(t)
	tool.RequestID = "forged-request"
	worker := &scriptedProviderRuntime{descriptor: descriptor, events: []ProviderEvent{tool, done}, ignoreCallbackError: true}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	if err := scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error {
		t.Fatal("provider ignored a callback failure and escaped the stream gate")
		return nil
	}); err == nil {
		t.Fatal("provider callback failure was forgotten")
	}
}

func TestProviderStreamGateRequiresTextTerminalEvent(t *testing.T) {
	descriptor, request, _, _ := scheduledToolFixture(t)
	request.Tools = nil
	request.Prompt = []byte(`{"model":"fixture","messages":[{"role":"user","content":"Hello"}]}`)
	text := ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDelta, RequestID: request.RequestID, Sequence: 1, Text: "Hello"}
	worker := &scriptedProviderRuntime{descriptor: descriptor, events: []ProviderEvent{text}}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	var received []ProviderEventType
	if err := scheduler.Generate(context.Background(), descriptor, request, func(event ProviderEvent) error {
		received = append(received, event.Type)
		return nil
	}); err == nil {
		t.Fatal("truncated text stream was treated as complete")
	}
	if len(received) != 1 || received[0] != ProviderEventDelta {
		t.Fatalf("provisional text or terminal status changed: %v", received)
	}
}

func TestProviderStreamGateEnforcesContextUsageForEveryEngine(t *testing.T) {
	descriptor, request, _, _ := scheduledToolFixture(t)
	request.ContextLength = 4
	for _, kind := range []ProviderEventType{ProviderEventUsage, ProviderEventDone} {
		t.Run(string(kind), func(t *testing.T) {
			gate := &providerStreamGate{descriptor: descriptor, request: request, callback: func(ProviderEvent) error {
				t.Fatal("out-of-context usage reached consumer")
				return nil
			}}
			event := ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: kind, RequestID: request.RequestID, Sequence: 1, Usage: &UsageMetrics{PromptTokens: 4, CompletionTokens: 1, TotalTokens: 5}}
			if kind == ProviderEventDone {
				event.FinishReason = "stop"
			}
			if err := gate.accept(event); err == nil {
				t.Fatal("out-of-context provider usage was accepted")
			}
			if err := gate.finish(); err == nil {
				t.Fatal("failed usage was not retained as a terminal error")
			}
		})
	}
}

func TestProviderSchedulerChecksFinalResourcesBeforeToolDispatch(t *testing.T) {
	descriptor, request, tool, done := scheduledToolFixture(t)
	for _, test := range []struct {
		name      string
		resources *ProviderResources
		err       error
	}{
		{"invalid report", &ProviderResources{Backend: "wrong", Device: "fixture"}, nil},
		{"unavailable report", nil, errors.New("worker resources unavailable")},
	} {
		t.Run(test.name, func(t *testing.T) {
			worker := &scriptedProviderRuntime{descriptor: descriptor, events: []ProviderEvent{tool, done}, finalResources: test.resources, finalResourceError: test.err}
			registry := NewProviderRegistry()
			if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
				t.Fatal(err)
			}
			scheduler, err := NewProviderScheduler(registry, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer scheduler.Close()
			if err := scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error {
				t.Fatal("tool decision escaped before final resource verification")
				return nil
			}); err == nil {
				t.Fatal("unsafe final resource state was treated as successful generation")
			}
		})
	}
}

func TestProviderStreamGateRejectsForgedAndMixedToolEvents(t *testing.T) {
	descriptor, request, tool, done := scheduledToolFixture(t)
	for _, test := range []struct {
		name   string
		change func(*providerStreamGate, *ProviderEvent)
	}{
		{"wrong request", func(_ *providerStreamGate, e *ProviderEvent) { e.RequestID = "other" }},
		{"unknown call", func(_ *providerStreamGate, e *ProviderEvent) {
			e.Tool = []byte(`[{"id":"call-1","type":"function","function":{"name":"other","arguments":"{}"}}]`)
		}},
		{"duplicate JSON", func(_ *providerStreamGate, e *ProviderEvent) {
			e.Tool = []byte(`[{"id":"call-1","id":"call-2","type":"function","function":{"name":"lookup","arguments":"{}"}}]`)
		}},
		{"unqualified model", func(g *providerStreamGate, _ *ProviderEvent) {
			g.descriptor.Capabilities = []ProviderCapability{CapabilityText}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			gate := &providerStreamGate{descriptor: descriptor, request: request, callback: func(ProviderEvent) error { t.Fatal("invalid tool reached consumer"); return nil }}
			candidate := tool
			test.change(gate, &candidate)
			if err := gate.accept(candidate); err == nil {
				t.Fatal("invalid provider tool decision was accepted")
			}
		})
	}
	gate := &providerStreamGate{descriptor: descriptor, request: request, callback: func(ProviderEvent) error { return nil }}
	if err := gate.accept(ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDelta, RequestID: request.RequestID, Sequence: 1, Text: "answer"}); err != nil {
		t.Fatal(err)
	}
	tool.Sequence = 2
	if err := gate.accept(tool); err == nil {
		t.Fatal("mixed answer and tool decision was accepted")
	}
	gate = &providerStreamGate{descriptor: descriptor, request: request, callback: func(ProviderEvent) error { return nil }}
	done.Sequence = 1
	if err := gate.accept(done); err == nil {
		t.Fatal("tool-call completion without a tool decision was accepted")
	}
	cancelled := done
	cancelled.FinishReason = "cancelled"
	if err := gate.accept(cancelled); err == nil {
		t.Fatal("provider cancellation was treated as successful completion")
	}
}
