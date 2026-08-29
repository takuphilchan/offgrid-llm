package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type serviceError struct {
	status  int
	message string
	cause   error
}

func (e *serviceError) Error() string {
	return e.message
}

func (e *serviceError) Unwrap() error   { return e.cause }
func (e *serviceError) HTTPStatus() int { return e.status }

func newServiceError(status int, message string, cause error) error {
	return &serviceError{status: status, message: message, cause: cause}
}

// completeChat is the transport-neutral non-streaming chat path used by both
// OpenAI-compatible requests and durable conversation sessions.
func (s *Server) completeChat(ctx context.Context, request *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	if request == nil {
		return nil, newServiceError(http.StatusBadRequest, "request is required", nil)
	}
	if request.Model == "" {
		return nil, newServiceError(http.StatusBadRequest, "model is required", nil)
	}
	if len(request.Messages) == 0 {
		return nil, newServiceError(http.StatusBadRequest, "messages are required", nil)
	}
	if s.degradationMgr != nil && !s.degradationMgr.RequestStart() {
		return nil, newServiceError(http.StatusServiceUnavailable, "server under heavy load, please try again later", nil)
	}
	if s.degradationMgr != nil {
		defer s.degradationMgr.RequestEnd()
	}
	if _, err := s.registry.GetModel(request.Model); err != nil {
		return nil, newServiceError(http.StatusNotFound, fmt.Sprintf("model not found: %s", request.Model), err)
	}

	releaseInference, err := s.acquireInference(ctx, request.Model)
	if err != nil {
		return nil, newServiceError(http.StatusInternalServerError, "failed to switch model", err)
	}
	defer releaseInference()

	prepared := *request
	prepared.Stream = false
	prepared.Messages = append([]api.ChatMessage(nil), request.Messages...)
	s.enhanceChatWithKnowledge(ctx, &prepared)

	started := time.Now()
	response, err := s.engine.ChatCompletion(ctx, &prepared)
	if err != nil {
		if engineErr := inference.AsEngineError(err); engineErr != nil {
			return nil, newServiceError(http.StatusBadRequest, engineErr.Message, err)
		}
		return nil, newServiceError(http.StatusInternalServerError, "inference failed", err)
	}

	duration := time.Since(started)
	if s.statsTracker != nil {
		s.statsTracker.RecordInference(prepared.Model, int64(response.Usage.TotalTokens), duration.Milliseconds())
	}
	atomic.AddInt64(&s.tokensGenerated, int64(response.Usage.CompletionTokens))
	if s.offgridMetrics != nil {
		s.offgridMetrics.TokensOutputTotal.Add(float64(response.Usage.CompletionTokens))
		s.offgridMetrics.TokensInputTotal.Add(float64(response.Usage.PromptTokens))
	}
	return response, nil
}

func (s *Server) enhanceChatWithKnowledge(ctx context.Context, request *api.ChatCompletionRequest) {
	if request.UseKnowledgeBase == nil || !*request.UseKnowledgeBase || s.ragEngine == nil || !s.ragEngine.IsEnabled() {
		return
	}
	for index := len(request.Messages) - 1; index >= 0; index-- {
		if request.Messages[index].Role != "user" {
			continue
		}
		_, ragContext, err := s.ragEngine.EnhancePrompt(ctx, request.Messages[index].StringContent())
		if err != nil {
			log.Printf("RAG enhancement failed: %v", err)
			return
		}
		if ragContext == nil || len(ragContext.Results) == 0 {
			return
		}
		ragMessage := api.ChatMessage{Role: "system", Content: ragContext.Context}
		request.Messages = append(request.Messages, api.ChatMessage{})
		copy(request.Messages[index+1:], request.Messages[index:])
		request.Messages[index] = ragMessage
		return
	}
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := err.Error()
	if typed, ok := err.(*serviceError); ok {
		status = typed.HTTPStatus()
		message = typed.message
		if typed.cause != nil {
			log.Printf("%s: %v", typed.message, typed.cause)
		}
	} else if typed, ok := err.(interface{ HTTPStatus() int }); ok {
		status = typed.HTTPStatus()
	}
	writeError(w, message, status)
}
