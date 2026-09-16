package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
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
	if errors.Is(cause, inference.ErrInferenceQueueFull) {
		return &serviceError{status: http.StatusTooManyRequests, message: inference.ErrInferenceQueueFull.Error(), cause: cause}
	}
	return &serviceError{status: status, message: message, cause: cause}
}

func authorizeChatAccess(ctx context.Context, requireAuth, knowledge bool) error {
	if !requireAuth {
		return nil
	}
	user := users.UserFromContext(ctx)
	if user == nil {
		return newServiceError(http.StatusUnauthorized, "Unauthorized", nil)
	}
	if !user.HasPermission(users.PermissionChat) {
		return newServiceError(http.StatusForbidden, "Chat access is not permitted", nil)
	}
	if knowledge && !user.HasPermission(users.PermissionRAG) {
		return newServiceError(http.StatusForbidden, "Knowledge access is not permitted", nil)
	}
	return nil
}

func (s *Server) authorizeChat(ctx context.Context, request *api.ChatCompletionRequest) error {
	return authorizeChatAccess(ctx, s.config != nil && s.config.RequireAuth,
		request.UseKnowledgeBase != nil && *request.UseKnowledgeBase)
}

// completeChat is the transport-neutral non-streaming chat path used by both
// OpenAI-compatible requests and durable conversation sessions.
func (s *Server) completeChat(ctx context.Context, request *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	started := time.Now()
	if request == nil {
		return nil, newServiceError(http.StatusBadRequest, "request is required", nil)
	}
	if request.Model == "" {
		return nil, newServiceError(http.StatusBadRequest, "model is required", nil)
	}
	if len(request.Messages) == 0 {
		return nil, newServiceError(http.StatusBadRequest, "messages are required", nil)
	}
	if err := s.authorizeChat(ctx, request); err != nil {
		return nil, err
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

	prepared := *request
	prepared.Stream = false
	prepared.Messages = append([]api.ChatMessage(nil), request.Messages...)
	if err := s.enhanceChatWithKnowledge(ctx, &prepared); err != nil {
		return nil, err
	}

	releaseInference, err := s.acquireInference(ctx, request.Model)
	if err != nil {
		return nil, newServiceError(http.StatusInternalServerError, "failed to switch model", err)
	}
	defer releaseInference()

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

// knowledgeRetriever keeps retrieval policy independent of the concrete index
// and embedding runtime, including failure/no-evidence behavior.
type knowledgeRetriever interface {
	IsEnabled() bool
	EnhancePrompt(context.Context, string) (string, *rag.RAGContext, error)
}

func (s *Server) enhanceChatWithKnowledge(ctx context.Context, request *api.ChatCompletionRequest) error {
	var knowledge knowledgeRetriever
	if s.ragEngine != nil {
		knowledge = s.ragEngine
	}
	return injectKnowledge(ctx, request, knowledge)
}

func injectKnowledge(ctx context.Context, request *api.ChatCompletionRequest, knowledge knowledgeRetriever) error {
	if request.UseKnowledgeBase == nil || !*request.UseKnowledgeBase {
		return nil
	}
	if knowledge == nil || !knowledge.IsEnabled() {
		return newServiceError(http.StatusServiceUnavailable, "Knowledge is unavailable; configure it in Knowledge or turn off knowledge for this message", nil)
	}
	for index := len(request.Messages) - 1; index >= 0; index-- {
		if request.Messages[index].Role != "user" {
			continue
		}
		_, ragContext, err := knowledge.EnhancePrompt(ctx, request.Messages[index].StringContent())
		if err != nil {
			return newServiceError(http.StatusServiceUnavailable, "Knowledge retrieval failed; retry or explicitly turn off knowledge for this message", err)
		}
		if ragContext == nil || len(ragContext.Results) == 0 {
			return newServiceError(http.StatusUnprocessableEntity, "No supporting knowledge was found; add relevant documents or turn off knowledge for this message", nil)
		}
		ragMessage := api.ChatMessage{Role: "system", Content: ragContext.Context}
		request.Messages = append(request.Messages, api.ChatMessage{})
		copy(request.Messages[index+1:], request.Messages[index:])
		request.Messages[index] = ragMessage
		return nil
	}
	return newServiceError(http.StatusBadRequest, "Knowledge retrieval requires a user message", nil)
}

func writeServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, inference.ErrInferenceQueueFull) {
		w.Header().Set("Retry-After", "2")
		writeErrorWithCode(w, inference.ErrInferenceQueueFull.Error(), http.StatusTooManyRequests, "inference_queue_full")
		return
	}
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
