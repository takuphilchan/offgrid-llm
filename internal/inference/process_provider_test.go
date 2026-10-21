package inference

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProcessProviderStreamsAndCancelsWithoutBlocking(t *testing.T) {
	if os.Getenv("OFFGRID_PROVIDER_WORKER_TEST") == "1" {
		providerTestWorker()
		return
	}
	descriptor := ProviderDescriptor{Provider: ProviderMLX, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText, CapabilityStreaming}}
	identity, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	startup, stopStartup := context.WithCancel(context.Background())
	provider, err := StartProcessProvider(startup, os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity)))
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	stopStartup()
	if resources, err := provider.Resources(context.Background()); err != nil || resources.Backend != "metal" || resources.MemoryBytes != 1024 || !resources.MemoryMeasured {
		t.Fatalf("worker resource handshake was not retained: %+v, %v", resources, err)
	}
	if err := provider.Health(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := ProviderRequest{RequestID: "req-1", CancellationID: "cancel-1", ModelDigest: "sha256:test", Prompt: json.RawMessage(`{"prompt":"hello"}`)}
	done := make(chan error, 1)
	go func() {
		done <- provider.Generate(ctx, req, func(event ProviderEvent) error {
			if event.Type != ProviderEventDone {
				t.Errorf("unexpected event: %#v", event)
			}
			return nil
		})
	}()
	select {
	case <-time.After(200 * time.Millisecond):
	case err := <-done:
		t.Fatalf("generation ended before cancellation: %v", err)
	}
	if err := provider.Cancel(ctx, req.CancellationID); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled terminal was not reported as cancellation: %v", err)
	}
	if err := provider.Generate(context.Background(), req, func(ProviderEvent) error { return nil }); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("duplicate request was accepted: %v", err)
	}
}

func TestProviderTerminalReasonsKeepTokenLimitDistinctFromCancellation(t *testing.T) {
	for _, reason := range []string{"stop", "length", "cancelled", "tool_calls"} {
		if err := validateProviderEvent(ProviderEvent{Type: ProviderEventDone, FinishReason: reason}); err != nil {
			t.Fatalf("valid terminal reason %q was rejected: %v", reason, err)
		}
	}
	if err := validateProviderEvent(ProviderEvent{Type: ProviderEventDone, FinishReason: "unknown"}); err == nil {
		t.Fatal("unknown terminal reason was accepted")
	}
}

func TestProviderGenerationEventsRejectCrossTypePayloads(t *testing.T) {
	for _, test := range []struct {
		name  string
		event ProviderEvent
	}{
		{"tool hidden in text", ProviderEvent{Type: ProviderEventDelta, Text: "answer", Tool: json.RawMessage(`[]`)}},
		{"text hidden in tool", ProviderEvent{Type: ProviderEventTool, Tool: json.RawMessage(`[]`), Text: "answer"}},
		{"tool hidden in terminal", ProviderEvent{Type: ProviderEventDone, FinishReason: "stop", Tool: json.RawMessage(`[]`)}},
		{"finish hidden in usage", ProviderEvent{Type: ProviderEventUsage, FinishReason: "stop", Usage: &UsageMetrics{PromptTokens: 1, TotalTokens: 1}}},
		{"identity changed in terminal", ProviderEvent{Type: ProviderEventDone, FinishReason: "stop", Descriptor: &ProviderDescriptor{}}},
		{"resources sent with text", ProviderEvent{Type: ProviderEventDelta, Text: "answer", Resources: &ProviderResources{}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateProviderEvent(test.event); err == nil {
				t.Fatal("cross-type provider payload was accepted")
			}
		})
	}
}

func providerTestWorker() {
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	var descriptor ProviderDescriptor
	if json.Unmarshal([]byte(os.Getenv("OFFGRID_PROVIDER_WORKER_DESCRIPTOR")), &descriptor) != nil {
		return
	}
	mode := os.Getenv("OFFGRID_PROVIDER_WORKER_MODE")
	requestCount := 0
	if mode == "bad_handshake" {
		descriptor.RunnerRevision = "wrong"
	}
	if mode == "bad_handshake_sequence" {
		_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventReady, Sequence: 1, Descriptor: &descriptor, Resources: &ProviderResources{Backend: descriptor.Backend, Device: "fixture", MemoryBytes: 1024, MemoryMeasured: true}})
		return
	}
	resources := &ProviderResources{Backend: descriptor.Backend, Device: "fixture", MemoryBytes: 1024, MemoryMeasured: true}
	if mode == "bad_resources" {
		resources.Backend = "wrong"
	}
	handshake := ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventReady, Descriptor: &descriptor, Resources: resources}
	if mode == "duplicate_handshake" {
		body, _ := json.Marshal(handshake)
		_, _ = os.Stdout.Write(append([]byte(`{"type":"ready",`), append(body[1:], '\n')...))
		return
	}
	_ = enc.Encode(handshake)
	if mode == "no_read" {
		time.Sleep(5 * time.Second)
		return
	}
	for {
		var message struct {
			Operation string          `json:"operation"`
			Request   ProviderRequest `json:"request"`
			ID        string          `json:"cancellation_id"`
		}
		if err := dec.Decode(&message); err != nil {
			return
		}
		switch message.Operation {
		case "generate":
			requestCount++
			if mode == "duplicate_stream" {
				_, _ = os.Stdout.Write([]byte(`{"type":"done","protocol_revision":"provider-v1","type":"done","request_id":"req-1","sequence":1,"finish_reason":"stop"}` + "\n"))
				return
			}
			if mode == "unqualified_tool" {
				_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventTool, RequestID: message.Request.RequestID, Sequence: 1, Tool: json.RawMessage(`[]`)})
				return
			}
			if mode == "unqualified_tool_finish" {
				_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventDone, RequestID: message.Request.RequestID, Sequence: 1, FinishReason: "tool_calls"})
				return
			}
			if mode == "terminal_error_then_poison" {
				_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventError, RequestID: message.Request.RequestID, Sequence: 1, Error: &ProviderError{Code: "generation_failed", Message: "local provider generation failed", Operational: true}})
				continue
			}
			if mode == "usage_then_done" || mode == "length" || mode == "bad_usage" || mode == "over_context_usage" {
				usage := &UsageMetrics{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3}
				if mode == "bad_usage" {
					usage.TotalTokens = 99
				} else if mode == "over_context_usage" {
					usage = &UsageMetrics{PromptTokens: 4, CompletionTokens: 1, TotalTokens: 5}
				}
				_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventUsage, RequestID: message.Request.RequestID, Sequence: 1, Usage: usage})
				reason := "stop"
				if mode == "length" {
					reason = "length"
				}
				_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventDone, RequestID: message.Request.RequestID, Sequence: 2, FinishReason: reason})
				continue
			}
			if mode == "signal_generate" {
				_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventDelta, RequestID: message.Request.RequestID, Sequence: 1, Text: "started"})
				continue
			}
			if mode == "terminal_error_then_reuse" {
				if requestCount == 1 {
					_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventError, RequestID: message.Request.RequestID, Sequence: 1, Error: &ProviderError{Code: "context_overflow", Message: "prompt exceeds context"}})
				} else {
					_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventDone, RequestID: message.Request.RequestID, Sequence: 1, FinishReason: "stop"})
				}
				continue
			}
			if mode == "complete_with_resources" || mode == "bad_completion_resources" {
				final := *resources
				final.MemoryBytes = 2048
				if mode == "bad_completion_resources" {
					final.Backend = "wrong"
				}
				_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventDone, RequestID: message.Request.RequestID, Sequence: 1, FinishReason: "stop", Resources: &final})
				return
			}
			if mode == "truncated" {
				return
			}
			if mode == "malformed" {
				_, _ = os.Stdout.Write([]byte("not-json\n"))
				return
			}
			if mode == "bad_done" {
				_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventDone, RequestID: message.Request.RequestID, Sequence: 1})
				return
			}
			if mode == "oversize" {
				_, _ = os.Stdout.Write([]byte(strings.Repeat("x", 1024*1024+1) + "\n"))
				return
			}
			// Keep the request open so the parent can prove cancellation is independent.
		case "cancel":
			sequence := uint64(1)
			if mode == "signal_generate" {
				sequence = 2
			}
			_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventDone, RequestID: "req-1", Sequence: sequence, FinishReason: "cancelled"})
			return
		case "health":
			if mode == "duplicate_health" {
				_, _ = os.Stdout.Write([]byte(`{"type":"ready","protocol_revision":"provider-v1","type":"ready","resources":{"backend":"metal","device":"fixture","memory_bytes":1024,"memory_measured":true}}` + "\n"))
			} else if mode == "terminal_error_then_poison" && requestCount > 0 {
				_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventError, Error: &ProviderError{Code: "worker_unhealthy", Message: "provider generation did not stop cleanly", Operational: true}})
			} else if mode == "bad_health_identity" {
				_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventReady, RequestID: "different-request", Resources: resources})
			} else if mode != "health_hang" {
				_ = enc.Encode(ProviderEvent{ProtocolRevision: descriptor.ProtocolRevision, Type: ProviderEventReady, Resources: resources})
			}
		}
	}
}

func TestProcessProviderRejectsUnqualifiedToolEnvelopes(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderTransformers, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	identity, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"unqualified_tool", "unqualified_tool_finish"} {
		t.Run(mode, func(t *testing.T) {
			env := append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity), "OFFGRID_PROVIDER_WORKER_MODE="+mode)
			provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, env)
			if err != nil {
				t.Fatal(err)
			}
			defer provider.Close()
			request := ProviderRequest{RequestID: "req-1", CancellationID: "cancel-1", ModelDigest: descriptor.ModelDigest, Prompt: json.RawMessage(`{"messages":[]}`)}
			if err := provider.Generate(context.Background(), request, func(ProviderEvent) error {
				t.Fatal("unqualified native tool decision reached consumer")
				return nil
			}); err == nil {
				t.Fatal("unqualified native tool envelope was accepted")
			}
		})
	}
}

func TestProcessProviderClosesAfterWorkerExitsDuringGeneration(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderTransformers, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	identity, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity), "OFFGRID_PROVIDER_WORKER_MODE=truncated")
	provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, env)
	if err != nil {
		t.Fatal(err)
	}
	request := ProviderRequest{RequestID: "req-1", CancellationID: "cancel-1", ModelDigest: descriptor.ModelDigest, Prompt: json.RawMessage(`{"messages":[]}`)}
	if err := provider.Generate(context.Background(), request, func(ProviderEvent) error {
		t.Fatal("exited worker produced a completed event")
		return nil
	}); err == nil {
		t.Fatal("worker exit was treated as completed generation")
	}
	if err := provider.Close(); err != nil {
		t.Fatalf("verified exited worker could not be retired: %v", err)
	}
}

func TestProcessProviderBlockedPipeWriteHonorsStopContext(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderMLX, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	identity, _ := json.Marshal(descriptor)
	env := append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity), "OFFGRID_PROVIDER_WORKER_MODE=no_read")
	provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, env)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	prompt := json.RawMessage(`{"messages":[{"role":"user","content":"` + strings.Repeat("x", 4<<20) + `"}]}`)
	request := ProviderRequest{RequestID: "blocked-write", CancellationID: "stop-blocked-write", ModelDigest: descriptor.ModelDigest, Prompt: prompt}
	called := false
	err = provider.Generate(ctx, request, func(ProviderEvent) error { called = true; return nil })
	if err == nil || time.Since(started) > 2*time.Second {
		t.Fatalf("blocked provider pipe ignored Stop: %v after %v", err, time.Since(started))
	}
	if called {
		t.Fatal("blocked worker emitted a trusted result")
	}
}

func TestProcessProviderHealthTimeoutStopsUnresponsiveWorker(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderMLX, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	identity, _ := json.Marshal(descriptor)
	env := append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity), "OFFGRID_PROVIDER_WORKER_MODE=health_hang")
	provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, env)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := provider.Health(ctx); err == nil {
		t.Fatal("unresponsive worker was accepted as healthy")
	}
}

func TestProcessProviderRejectsBadHandshakeAndIncompleteStreams(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderMLX, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText, CapabilityStreaming}}
	identity, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"bad_handshake", "bad_handshake_sequence", "bad_resources", "duplicate_handshake", "duplicate_stream", "bad_completion_resources", "truncated", "malformed", "bad_done", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			env := append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity), "OFFGRID_PROVIDER_WORKER_MODE="+mode)
			provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, env)
			if mode == "bad_handshake" || mode == "bad_handshake_sequence" || mode == "bad_resources" || mode == "duplicate_handshake" {
				if err == nil {
					_ = provider.Close()
					t.Fatal("accepted mismatched worker identity")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer provider.Close()
			request := ProviderRequest{RequestID: "req-1", CancellationID: "cancel-1", ModelDigest: descriptor.ModelDigest, Prompt: json.RawMessage(`{"messages":[]}`)}
			called := false
			if err := provider.Generate(context.Background(), request, func(ProviderEvent) error { called = true; return nil }); err == nil || called {
				t.Fatalf("incomplete or malformed stream was accepted: err=%v called=%v", err, called)
			}
		})
	}
}

func TestProcessProviderRejectsAmbiguousHealthEvent(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderMLX, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	identity, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity), "OFFGRID_PROVIDER_WORKER_MODE=duplicate_health")
	provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, env)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	if err := provider.Health(context.Background()); err == nil {
		t.Fatal("ambiguous health event was trusted")
	}
	request := ProviderRequest{RequestID: "after-health", CancellationID: "after-health", ModelDigest: descriptor.ModelDigest, Prompt: json.RawMessage(`{"messages":[]}`)}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := provider.Generate(ctx, request, func(ProviderEvent) error {
		t.Fatal("retired worker emitted a generation result")
		return nil
	}); err == nil {
		t.Fatal("worker was reused after an ambiguous health reply")
	}
}

func TestProcessProviderValidatesNativeUsageBeforeTrustingCompletion(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderMLX, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText, CapabilityStreaming}, ContextLength: 4}
	identity, _ := json.Marshal(descriptor)
	for _, mode := range []string{"usage_then_done", "length", "bad_usage", "over_context_usage"} {
		t.Run(mode, func(t *testing.T) {
			env := append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity), "OFFGRID_PROVIDER_WORKER_MODE="+mode)
			provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, env)
			if err != nil {
				t.Fatal(err)
			}
			defer provider.Close()
			request := ProviderRequest{RequestID: "req-1", CancellationID: "cancel-1", ModelDigest: descriptor.ModelDigest, Prompt: json.RawMessage(`{"messages":[]}`), ContextLength: 4}
			var received []ProviderEventType
			var finishReason string
			err = provider.Generate(context.Background(), request, func(event ProviderEvent) error {
				received = append(received, event.Type)
				if event.Type == ProviderEventDone {
					finishReason = event.FinishReason
				}
				return nil
			})
			if mode == "usage_then_done" || mode == "length" {
				expectedReason := "stop"
				if mode == "length" {
					expectedReason = "length"
				}
				if err != nil || len(received) != 2 || received[0] != ProviderEventUsage || received[1] != ProviderEventDone || finishReason != expectedReason {
					t.Fatalf("valid usage did not complete: events=%v err=%v", received, err)
				}
			} else if err == nil || len(received) != 0 {
				t.Fatalf("invalid usage reached callback: events=%v err=%v", received, err)
			}
		})
	}
}

func TestProviderUsageEventRequiresCompleteConsistentCounts(t *testing.T) {
	for _, usage := range []*UsageMetrics{nil, {PromptTokens: -1, CompletionTokens: 2, TotalTokens: 1}, {PromptTokens: 1, CompletionTokens: 2, TotalTokens: 4}} {
		if err := validateProviderEvent(ProviderEvent{Type: ProviderEventUsage, Usage: usage}); err == nil {
			t.Fatalf("accepted incomplete usage: %+v", usage)
		}
	}
	if err := validateProviderEvent(ProviderEvent{Type: ProviderEventUsage, Usage: &UsageMetrics{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}}); err != nil {
		t.Fatalf("rejected valid usage: %v", err)
	}
}

func TestProcessProviderIntentionalCloseAndSwitchAreNotFailures(t *testing.T) {
	first := ProviderDescriptor{Provider: ProviderMLX, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:first", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	second := first
	second.ModelDigest = "sha256:second"
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderMLX, func(ctx context.Context, descriptor ProviderDescriptor) (ProviderRuntime, error) {
		identity, _ := json.Marshal(descriptor)
		return StartProcessProvider(ctx, os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity)))
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	firstRuntime, err := scheduler.Runtime(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.Runtime(context.Background(), second); err != nil {
		t.Fatalf("intentional retirement blocked safe provider switch: %v", err)
	}
	if err := firstRuntime.Health(context.Background()); err == nil {
		t.Fatal("retired worker remained healthy")
	}
	if err := scheduler.Close(); err != nil {
		t.Fatalf("intentional scheduler close was reported as a failure: %v", err)
	}
}

func TestProcessProviderRejectsHealthEventForAnotherRequest(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderMLX, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	identity, _ := json.Marshal(descriptor)
	env := append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity), "OFFGRID_PROVIDER_WORKER_MODE=bad_health_identity")
	provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, env)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	if err := provider.Health(context.Background()); err == nil {
		t.Fatal("health response with another request identity was trusted")
	}
}

func TestProcessProviderKeepsWorkerAfterCompleteErrorEvent(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderTransformers, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	identity, _ := json.Marshal(descriptor)
	env := append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity), "OFFGRID_PROVIDER_WORKER_MODE=terminal_error_then_reuse")
	provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, env)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	first := ProviderRequest{RequestID: "first", CancellationID: "cancel-first", ModelDigest: descriptor.ModelDigest, Prompt: json.RawMessage(`{"messages":[]}`)}
	if err := provider.Generate(context.Background(), first, func(ProviderEvent) error { return nil }); err == nil || !strings.Contains(err.Error(), "context_overflow") {
		t.Fatalf("complete worker error was not reported: %v", err)
	}
	if err := provider.Health(context.Background()); err != nil {
		t.Fatalf("complete worker error killed the resident runtime: %v", err)
	}
	second := first
	second.RequestID = "second"
	second.CancellationID = "cancel-second"
	if err := provider.Generate(context.Background(), second, func(ProviderEvent) error { return nil }); err != nil {
		t.Fatalf("explicit next request could not reuse the worker: %v", err)
	}
}

func TestProcessProviderRejectsPoisonedWorkerAfterCompleteError(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderTransformers, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	identity, _ := json.Marshal(descriptor)
	env := append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity), "OFFGRID_PROVIDER_WORKER_MODE=terminal_error_then_poison")
	provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, env)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	request := ProviderRequest{RequestID: "first", CancellationID: "first", ModelDigest: descriptor.ModelDigest, Prompt: json.RawMessage(`{"messages":[]}`)}
	if err := provider.Generate(context.Background(), request, func(ProviderEvent) error { return nil }); err == nil {
		t.Fatal("poisoning generation was reported as complete")
	}
	if err := provider.Health(context.Background()); err == nil {
		t.Fatal("worker with an unreturned native producer was accepted as healthy")
	}
}

func TestProviderSchedulerReplacesPoisonedProcessOnlyOnExplicitNewRequest(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderTransformers, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	identity, _ := json.Marshal(descriptor)
	registry := NewProviderRegistry()
	var starts atomic.Int32
	if err := registry.Register(ProviderTransformers, func(ctx context.Context, selected ProviderDescriptor) (ProviderRuntime, error) {
		mode := "terminal_error_then_poison"
		if starts.Add(1) > 1 {
			mode = "usage_then_done"
		}
		env := append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity), "OFFGRID_PROVIDER_WORKER_MODE="+mode)
		return StartProcessProvider(ctx, os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, selected, env)
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	request := ProviderRequest{RequestID: "first", CancellationID: "first", ModelDigest: descriptor.ModelDigest, Prompt: json.RawMessage(`{"messages":[]}`)}
	if err := scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil }); err == nil || starts.Load() != 1 {
		t.Fatalf("poisoning generation was replayed or completed: starts=%d err=%v", starts.Load(), err)
	}
	request.RequestID, request.CancellationID = "second", "second"
	if err := scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil }); err != nil || starts.Load() != 2 {
		t.Fatalf("explicit new request did not replace unhealthy worker: starts=%d err=%v", starts.Load(), err)
	}
}

func TestProcessProviderQueuedStreamWaitHonorsCancellation(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderTransformers, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	identity, _ := json.Marshal(descriptor)
	env := append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity), "OFFGRID_PROVIDER_WORKER_MODE=signal_generate")
	provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, env)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	first := ProviderRequest{RequestID: "req-1", CancellationID: "cancel-1", ModelDigest: descriptor.ModelDigest, Prompt: json.RawMessage(`{"messages":[]}`)}
	started := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- provider.Generate(context.Background(), first, func(event ProviderEvent) error {
			if event.Type == ProviderEventDelta {
				close(started)
			}
			return nil
		})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first worker request did not begin")
	}
	queuedCtx, cancelQueued := context.WithCancel(context.Background())
	second := first
	second.RequestID = "req-2"
	second.CancellationID = "cancel-2"
	secondDone := make(chan error, 1)
	go func() { secondDone <- provider.Generate(queuedCtx, second, func(ProviderEvent) error { return nil }) }()
	cancelQueued()
	select {
	case err := <-secondDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued stream returned %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("queued stream ignored cancellation while another request was active")
	}
	if err := provider.Cancel(context.Background(), first.CancellationID); err != nil {
		t.Fatal(err)
	}
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("first request did not stop: %v", err)
	}
}

func TestProcessProviderRetainsMeasuredCompletionResources(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderMLX, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	identity, _ := json.Marshal(descriptor)
	env := append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity), "OFFGRID_PROVIDER_WORKER_MODE=complete_with_resources")
	provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, env)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	request := ProviderRequest{RequestID: "measured", CancellationID: "measured", ModelDigest: descriptor.ModelDigest, Prompt: json.RawMessage(`{"messages":[]}`)}
	if err := provider.Generate(context.Background(), request, func(ProviderEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	resources, err := provider.Resources(context.Background())
	if err != nil || resources.MemoryBytes != 2048 {
		t.Fatalf("completion resource report was lost: %+v, %v", resources, err)
	}
}

func TestStartProcessProviderRejectsMissingCommand(t *testing.T) {
	_, err := StartProcessProvider(context.Background(), "", nil, ProviderDescriptor{}, nil)
	if err == nil || !strings.Contains(err.Error(), "command") {
		t.Fatalf("expected missing command error, got %v", err)
	}
}

func TestProcessProviderStartupFailurePreservesUncertainStop(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(map[bool]string{false: "confirmed_stop", true: "uncertain_stop"}[uncertain], func(t *testing.T) {
			reader, writer := io.Pipe()
			defer reader.Close()
			var releases atomic.Int32
			provider := &ProcessProvider{
				cmd: &exec.Cmd{}, stdin: writer, closedCh: make(chan struct{}),
				release: func() { releases.Add(1) },
				terminate: func(*exec.Cmd) error {
					if uncertain {
						return errors.New("worker exit was not confirmed")
					}
					return nil
				},
			}
			cause := errors.New("bad startup handshake")
			err := provider.rejectStartup(cause)
			if !errors.Is(err, cause) || errors.Is(err, errProviderStopUncertain) != uncertain {
				t.Fatalf("startup error lost its stop status: %v", err)
			}
			if got := releases.Load(); got != map[bool]int32{false: 1, true: 0}[uncertain] {
				t.Fatalf("package lease release count = %d", got)
			}
		})
	}
}

func TestNativeProviderConstructorsValidateEngineAndFormat(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r", AdapterRevision: "a", ModelDigest: "sha256:m", Format: FormatGGUF, Backend: "cpu"}
	if _, err := NewTransformersProcessProvider(context.Background(), "worker", "script", "model", descriptor, nil, nil); err == nil {
		t.Fatal("expected transformers format validation")
	}
	descriptor.Format = FormatSafetensors
	descriptor.Capabilities = []ProviderCapability{CapabilityText, CapabilityToolCalls}
	if _, err := NewTransformersProcessProvider(context.Background(), "worker", "script", "model", descriptor, nil, nil); err == nil {
		t.Fatal("accepted unqualified native tool execution")
	}
	descriptor.Capabilities = []ProviderCapability{CapabilityText}
	descriptor.Backend = "metal"
	if _, err := NewTransformersProcessProvider(context.Background(), "worker", "script", "model", descriptor, nil, nil); err == nil || !strings.Contains(err.Error(), "CPU, CUDA, or MPS") {
		t.Fatalf("accepted unsupported Transformers backend: %v", err)
	}
	descriptor.Provider = ProviderMLX
	descriptor.Format = FormatMLXSafetensors
	descriptor.Backend = "cpu"
	if _, err := NewMLXProcessProvider(context.Background(), "worker", "script", "model", descriptor, nil, nil); err == nil || !strings.Contains(err.Error(), "Metal") {
		t.Fatalf("accepted MLX without Metal backend: %v", err)
	}
}

func TestNativeWorkerEnvironmentCannotInheritCredentialsOrCodePaths(t *testing.T) {
	for _, setting := range []string{"OFFGRID_API_KEY=secret", "PYTHONPATH=untrusted", "HF_TOKEN=secret", "PATH=untrusted"} {
		if _, err := nativeWorkerEnvironment([]string{setting}); err == nil {
			t.Fatalf("native worker accepted %q", setting)
		}
	}
	env, err := nativeWorkerEnvironment([]string{"CUDA_VISIBLE_DEVICES=0"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(env, "\n")
	for _, required := range []string{"HF_HUB_OFFLINE=1", "TRANSFORMERS_OFFLINE=1", "HF_HUB_DISABLE_IMPLICIT_TOKEN=1", "PYTHONNOUSERSITE=1", "CUDA_VISIBLE_DEVICES=0"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("missing %q from worker environment", required)
		}
	}
}
