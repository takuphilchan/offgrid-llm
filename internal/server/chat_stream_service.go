package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) streamSessionChat(ctx context.Context, request *api.ChatCompletionRequest, profile string, emit func(ChatStreamEvent) error) (ChatStreamResult, error) {
	result := ChatStreamResult{}
	started := time.Now()
	if err := s.authorizeChat(ctx, request); err != nil {
		return result, err
	}
	if _, err := s.registry.GetModel(request.Model); err != nil {
		return result, newServiceError(http.StatusNotFound, "Model not found", err)
	}
	if s.degradationMgr != nil {
		if !s.degradationMgr.RequestStart() {
			return result, newServiceError(http.StatusServiceUnavailable, "Server under heavy load; retry shortly", nil)
		}
		defer s.degradationMgr.RequestEnd()
	}
	phase := func(name string) error { return emit(ChatStreamEvent{Type: "phase", Phase: name}) }
	prepared := *request
	prepared.Messages = append([]api.ChatMessage(nil), request.Messages...)
	prepared.StreamOptions = &api.StreamOptions{IncludeUsage: true}
	if request.UseKnowledgeBase != nil && *request.UseKnowledgeBase {
		if err := phase("retrieving"); err != nil {
			return result, err
		}
	}
	if err := s.enhanceChatWithKnowledge(ctx, &prepared); err != nil {
		return result, err
	}
	if err := phase("queued"); err != nil {
		return result, err
	}
	result.Metrics.ContextWindow = s.chatContextWindow(profile)
	queued := time.Now()
	var loading time.Time
	release, err := s.acquireInferenceContext(ctx, request.Model, result.Metrics.ContextWindow, func() error { loading = time.Now(); return phase("loading") })
	if err != nil {
		return result, newServiceError(http.StatusServiceUnavailable, "Model could not be loaded; check available memory and runtime logs", err)
	}
	defer release()
	if !loading.IsZero() {
		result.Metrics.LoadMS = time.Since(loading).Milliseconds()
	}
	result.Metrics.QueueMS = time.Since(queued).Milliseconds() - result.Metrics.LoadMS
	result.Metrics.ReadyMS = time.Since(started).Milliseconds()
	if err := phase("processing"); err != nil {
		return result, err
	}
	var answer strings.Builder
	var first time.Time
	emitText := func(text string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if text == "" {
			return nil
		}
		if first.IsZero() {
			first = time.Now()
			result.Metrics.FirstTextMS = first.Sub(started).Milliseconds()
			result.Metrics.PromptMS = result.Metrics.FirstTextMS - result.Metrics.ReadyMS
			if err := phase("generating"); err != nil {
				return err
			}
		}
		answer.WriteString(text)
		return emit(ChatStreamEvent{Type: "delta", Delta: text})
	}
	if raw, ok := s.engine.(inference.RawStreamingEngine); ok {
		err = raw.ChatCompletionStreamRaw(ctx, &prepared, func(data json.RawMessage) error {
			var chunk struct {
				Error   json.RawMessage `json:"error"`
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
				Usage struct {
					CompletionTokens int `json:"completion_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal(data, &chunk); err != nil {
				return err
			}
			if len(chunk.Error) > 0 && string(chunk.Error) != "null" {
				return newServiceError(http.StatusBadGateway, "Inference stream failed", nil)
			}
			if chunk.Usage.CompletionTokens > 0 {
				result.Metrics.CompletionTokens = chunk.Usage.CompletionTokens
			}
			if len(chunk.Choices) > 0 {
				if chunk.Choices[0].FinishReason != nil {
					result.FinishReason = *chunk.Choices[0].FinishReason
				}
				return emitText(chunk.Choices[0].Delta.Content)
			}
			return nil
		})
	} else {
		err = s.engine.ChatCompletionStream(ctx, &prepared, emitText)
		result.FinishReason = "stop"
	}
	if err != nil {
		if typed := inference.AsEngineError(err); typed != nil {
			return result, newServiceError(http.StatusBadRequest, typed.Message, err)
		}
		return result, newServiceError(http.StatusBadGateway, "Generation interrupted. Partial text was not saved; retry or use a smaller model/context.", err)
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if answer.Len() == 0 || (result.FinishReason != "stop" && result.FinishReason != "length") {
		return result, newServiceError(http.StatusBadGateway, "Model did not return a complete text response", nil)
	}
	result.Answer = answer.String()
	result.Metrics.TotalMS = time.Since(started).Milliseconds()
	result.Metrics.GenerationMS = time.Since(first).Milliseconds()
	if result.Metrics.GenerationMS > 0 && result.Metrics.CompletionTokens > 1 {
		result.Metrics.TokensPerSecond = float64(result.Metrics.CompletionTokens-1) * 1000 / float64(result.Metrics.GenerationMS)
	}
	if s.statsTracker != nil {
		s.statsTracker.RecordInference(request.Model, int64(result.Metrics.CompletionTokens), result.Metrics.TotalMS)
	}
	atomic.AddInt64(&s.tokensGenerated, int64(result.Metrics.CompletionTokens))
	if s.offgridMetrics != nil {
		s.offgridMetrics.TokensOutputTotal.Add(float64(result.Metrics.CompletionTokens))
	}
	return result, nil
}
