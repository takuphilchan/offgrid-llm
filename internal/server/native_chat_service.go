package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// nativeChatSelector is implemented by the service-owned package/qualification
// route. It is nil in ordinary installations: an imported model alone cannot
// introduce an executable worker or make itself selectable in Chat.
type nativeChatSelector interface {
	Select(context.Context, string, string) (inference.ProviderDescriptor, error)
}

// InstallQualifiedNativeText preserves the original single-target internal
// startup integration for existing service owners. Multiple independently
// measured provider/backend profiles use InstallQualifiedNativeTextTargets.
func (s *Server) InstallQualifiedNativeText(ctx context.Context, adapters *models.AdapterRegistry, workers map[inference.ProviderID]inference.TrustedNativeWorker, target inference.NativeTextEvidenceTarget) error {
	targets := make(map[inference.NativeTextTargetKey]inference.NativeTextEvidenceTarget, len(workers))
	for provider := range workers {
		targets[inference.NativeTextTargetKey{Provider: provider, Backend: target.Backend}] = target
	}
	return s.InstallQualifiedNativeTextTargets(ctx, adapters, workers, targets)
}

// InstallQualifiedNativeTextTargets is a startup-only service integration
// point for reviewed, pinned local worker assets and per-provider hardware
// evidence. It is not reachable from HTTP, CLI, model packages, or a model
// response. An imported package alone cannot start a worker.
func (s *Server) InstallQualifiedNativeTextTargets(ctx context.Context, adapters *models.AdapterRegistry, workers map[inference.ProviderID]inference.TrustedNativeWorker, targets map[inference.NativeTextTargetKey]inference.NativeTextEvidenceTarget) error {
	if s == nil {
		return errors.New("native provider installation requires a service")
	}
	s.nativeRuntimeMu.Lock()
	defer s.nativeRuntimeMu.Unlock()
	if ctx == nil || s.config == nil || s.registry == nil || s.providerRegistry == nil || s.providerScheduler == nil || adapters == nil || s.nativeChatSelector != nil || s.nativeInstallSealed {
		return errors.New("native provider installation requires an unconfigured service and reviewed adapters")
	}
	if len(workers) == 0 {
		return errors.New("native provider installation requires pinned local workers")
	}
	for _, worker := range workers {
		if err := worker.ValidateIdentity(ctx); err != nil {
			return err
		}
	}
	evidence := models.NewCapabilityEvidenceStore(s.config.DataDir)
	policy, err := inference.QualifiedNativeTextTargets(evidence, targets)
	if err != nil {
		return err
	}
	for key := range targets {
		if _, ok := workers[key.Provider]; !ok {
			return fmt.Errorf("native qualification target has no installed %s worker", key.Provider)
		}
	}
	for provider := range workers {
		found := false
		for key := range targets {
			if key.Provider == provider {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("native provider %s has no qualification target", provider)
		}
	}
	routes, err := inference.NewNativePackageRoutes(s.registry.Packages(), adapters, workers, policy)
	if err != nil {
		return err
	}
	if err := routes.Register(s.providerRegistry); err != nil {
		return err
	}
	s.nativeChatSelector = routes
	return nil
}

func (s *Server) selectNativeChat(ctx context.Context, model string) (inference.ProviderDescriptor, bool, error) {
	s.nativeRuntimeMu.RLock()
	selector := s.nativeChatSelector
	s.nativeRuntimeMu.RUnlock()
	if selector == nil {
		return inference.ProviderDescriptor{}, false, nil
	}
	id, revision, ok := strings.Cut(model, "@")
	if !ok || id == "" || revision == "" || strings.Contains(revision, "@") {
		return inference.ProviderDescriptor{}, false, nil
	}
	descriptor, err := selector.Select(ctx, id, revision)
	if err != nil {
		return inference.ProviderDescriptor{}, true, newCodedServiceError(http.StatusServiceUnavailable, "native_model_unavailable", "The selected native model is not ready or qualified on this machine", err)
	}
	if descriptor.Provider != inference.ProviderMLX && descriptor.Provider != inference.ProviderTransformers {
		return inference.ProviderDescriptor{}, true, newCodedServiceError(http.StatusServiceUnavailable, "native_model_unavailable", "The selected native model has no supported provider", errors.New("native selector returned non-native provider"))
	}
	return descriptor, true, nil
}

func (s *Server) completeNativeChat(ctx context.Context, request *api.ChatCompletionRequest, descriptor inference.ProviderDescriptor) (*api.ChatCompletionResponse, error) {
	result, err := inference.GenerateNativeTextChat(ctx, s.providerScheduler, descriptor, request, nil)
	if err != nil {
		return nil, newCodedServiceError(http.StatusBadGateway, "native_inference_failed", "Native inference failed without a completed answer", err)
	}
	return result.Response, nil
}

func (s *Server) streamNativeSessionChat(ctx context.Context, request *api.ChatCompletionRequest, descriptor inference.ProviderDescriptor, started time.Time, emit func(ChatStreamEvent) error) (ChatStreamResult, error) {
	result := ChatStreamResult{Metrics: ChatTimings{ContextWindow: descriptor.ContextLength}}
	if err := emit(ChatStreamEvent{Type: "phase", Phase: "processing"}); err != nil {
		return result, err
	}
	first := time.Time{}
	native, err := inference.GenerateNativeTextChat(ctx, s.providerScheduler, descriptor, request, func(delta string) error {
		if first.IsZero() {
			first = time.Now()
			result.Metrics.FirstTextMS = first.Sub(started).Milliseconds()
			if err := emit(ChatStreamEvent{Type: "phase", Phase: "generating"}); err != nil {
				return err
			}
		}
		return emit(ChatStreamEvent{Type: "delta", Delta: delta})
	})
	if err != nil {
		return result, newCodedServiceError(http.StatusBadGateway, "native_inference_failed", "Native generation was interrupted; partial text was not saved", err)
	}
	result.Answer = native.Response.Choices[0].Message.StringContent()
	result.FinishReason = native.Response.Choices[0].FinishReason
	result.Metrics.TotalMS = time.Since(started).Milliseconds()
	result.Metrics.QueueMS = native.Timing.Queue.Milliseconds()
	result.Metrics.LoadMS = native.Timing.Load.Milliseconds()
	result.Metrics.ReadyMS = result.Metrics.QueueMS + result.Metrics.LoadMS
	result.Metrics.CompletionTokens = native.Response.Usage.CompletionTokens
	if !first.IsZero() {
		result.Metrics.GenerationMS = time.Since(first).Milliseconds()
		result.Metrics.PromptMS = result.Metrics.FirstTextMS - result.Metrics.ReadyMS
	}
	if s.statsTracker != nil {
		s.statsTracker.RecordInference(request.Model, int64(native.Response.Usage.TotalTokens), result.Metrics.TotalMS)
	}
	return result, nil
}

func (s *Server) handleNativeChatCompletionsStream(w http.ResponseWriter, r *http.Request, request *api.ChatCompletionRequest, descriptor inference.ProviderDescriptor) {
	stream, ok := w.(*chatEventStream)
	if !ok {
		var err error
		stream, err = startChatEventStream(w, r, request.Model)
		if err != nil {
			return
		}
		defer stream.Close()
	}
	chunkID := stream.id
	write := stream.writeJSON
	native, err := inference.GenerateNativeTextChat(stream.ctx, s.providerScheduler, descriptor, request, func(delta string) error {
		if err := stream.ctx.Err(); err != nil {
			return err
		}
		if err := stream.setPhase("receiving_output"); err != nil {
			return err
		}
		return write(api.ChatCompletionChunk{ID: chunkID, Object: "chat.completion.chunk", Created: time.Now().Unix(), Model: request.Model,
			Choices: []api.ChatCompletionChoiceChunk{{Index: 0, Delta: api.ChatMessage{Role: "assistant", Content: delta}}}})
	})
	if stream.ctx.Err() != nil {
		err = stream.ctx.Err()
	}
	if err != nil {
		stream.writeError(newCodedServiceError(http.StatusBadGateway, "native_inference_failed", "Native generation was interrupted; no complete answer was produced", err))
		return
	}
	finish := native.Response.Choices[0].FinishReason
	if err := write(api.ChatCompletionChunk{ID: chunkID, Object: "chat.completion.chunk", Created: time.Now().Unix(), Model: request.Model,
		Choices: []api.ChatCompletionChoiceChunk{{Index: 0, Delta: api.ChatMessage{}, FinishReason: &finish}}}); err != nil {
		return
	}
	_, _ = fmt.Fprint(stream, "data: [DONE]\n\n")
	if s.statsTracker != nil {
		s.statsTracker.RecordInference(request.Model, int64(native.Response.Usage.TotalTokens), native.Timing.Queue.Milliseconds()+native.Timing.Load.Milliseconds()+native.Timing.Generation.Milliseconds())
	}
	atomic.AddInt64(&s.tokensGenerated, int64(native.Response.Usage.CompletionTokens))
}
