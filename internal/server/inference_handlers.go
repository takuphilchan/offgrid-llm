package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse request
	var req api.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInferenceRequestError(w, err)
		return
	}
	if err := s.authorizeChat(r.Context(), &req); err != nil {
		writeServiceError(w, err)
		return
	}
	if !req.Stream {
		response, err := s.completeChat(r.Context(), &req)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
		return
	}

	// Streaming requests keep their inference lease for the lifetime of the
	// response and therefore perform admission here.
	if s.degradationMgr != nil && !s.degradationMgr.RequestStart() {
		writeError(w, "Server under heavy load, please try again later", http.StatusServiceUnavailable)
		return
	}
	if s.degradationMgr != nil {
		defer s.degradationMgr.RequestEnd()
	}

	// Validate request
	if req.Model == "" {
		writeError(w, "Model is required", http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		writeError(w, "Messages are required", http.StatusBadRequest)
		return
	}
	hasImage, err := api.ValidateChatMessages(req.Messages)
	if err != nil {
		writeErrorWithCode(w, "Message content is invalid or exceeds local safety limits", http.StatusBadRequest, "invalid_message_content")
		return
	}

	// Get model metadata
	metadata, err := s.registry.GetChatModel(req.Model)
	var nativeDescriptor inference.ProviderDescriptor
	native := false
	if err != nil {
		var selectErr error
		nativeDescriptor, native, selectErr = s.selectNativeChat(r.Context(), req.Model)
		if selectErr != nil {
			writeServiceError(w, selectErr)
			return
		}
		if !native {
			writeError(w, fmt.Sprintf("Model not found: %s", req.Model), http.StatusNotFound)
			return
		}
	}
	if hasImage && (native || metadata.ProjectorPath == "") {
		writeErrorWithCode(w, "The selected model has no installed vision projector; no image was sent to inference", http.StatusUnprocessableEntity, "vision_projector_unavailable")
		return
	}
	if native && req.ResponseFormat.Constrained() {
		writeErrorWithCode(w, "response_format is not qualified for the selected native provider", http.StatusBadRequest, "unsupported_parameter")
		return
	}
	if native {
		if err := inference.ValidateNativeTextChatRequest(&req, nativeDescriptor); err != nil {
			writeErrorWithCode(w, "The selected native provider does not support this request", http.StatusBadRequest, "unsupported_parameter")
			return
		}
	}

	// Streaming and durable chat use the same retrieval and error contract.
	// Check optional knowledge before loading a potentially expensive chat model.
	if err := s.enhanceChatWithKnowledge(r.Context(), &req); err != nil {
		writeServiceError(w, err)
		return
	}
	stream, err := startChatEventStream(w, r, req.Model)
	if err != nil {
		return
	}
	defer stream.Close()
	w, r = stream, r.WithContext(stream.ctx)
	if native {
		s.handleNativeChatCompletionsStream(w, r, &req, nativeDescriptor)
		return
	}

	// Admit bounded same-model concurrency; model changes remain exclusive.
	lease, err := s.acquireInferenceLease(r.Context(), req.Model, s.modelContextWindow(req.Model), func() error { return stream.setPhase("preparing_model") })
	if err != nil {
		stream.writeError(newServiceError(http.StatusServiceUnavailable, "Inference admission or model loading failed; check runtime availability", err))
		return
	}
	defer lease.Release()

	s.handleChatCompletionsStream(w, r, &req, lease)
}

// handleChatCompletionsStream handles streaming chat completions using Server-Sent Events
func (s *Server) handleChatCompletionsStream(w http.ResponseWriter, r *http.Request, req *api.ChatCompletionRequest, lease *inference.LegacyGenerationLease) {
	if lease == nil {
		writeError(w, "Inference admission is required", http.StatusServiceUnavailable)
		return
	}
	streamWriter, ok := w.(*chatEventStream)
	if !ok {
		var err error
		streamWriter, err = startChatEventStream(w, r, req.Model)
		if err != nil {
			return
		}
		defer streamWriter.Close()
	}
	w = streamWriter
	flusher := streamWriter
	ctx := streamWriter.ctx
	if err := streamWriter.setPhase("waiting_for_output"); err != nil {
		streamWriter.writeError(err)
		return
	}
	chunkID := streamWriter.id
	tokenIndex := 0
	rawFinished := false
	rawEngine, supportsStructuredStream := s.engine.(inference.RawStreamingEngine)

	// Define callback to reuse
	callback := func(token string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := streamWriter.setPhase("receiving_output"); err != nil {
			return err
		}
		chunk := api.ChatCompletionChunk{
			ID:      chunkID,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []api.ChatCompletionChoiceChunk{
				{
					Index: 0,
					Delta: api.ChatMessage{
						Role:    "assistant",
						Content: token,
					},
					FinishReason: nil,
				},
			},
		}

		// Encode and send chunk
		data, err := json.Marshal(chunk)
		if err != nil {
			return err
		}

		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return err
		}
		flusher.Flush()
		tokenIndex++
		return nil
	}
	rawCallback := func(data json.RawMessage) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := streamWriter.setPhase("receiving_output"); err != nil {
			return err
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal(data, &chunk); err != nil {
			return err
		}
		if chunk == nil || chunk["error"] != nil {
			return fmt.Errorf("inference stream returned an error or invalid chunk")
		}
		choices, hasChoices := chunk["choices"].([]interface{})
		if !hasChoices {
			return fmt.Errorf("inference stream chunk has no choices")
		}
		if rawFinished && len(choices) != 0 {
			return fmt.Errorf("inference stream continued after its terminal choice")
		}
		for _, rawChoice := range choices {
			choice, ok := rawChoice.(map[string]interface{})
			if !ok {
				return fmt.Errorf("inference stream choice is invalid")
			}
			if reason, ok := choice["finish_reason"].(string); ok && reason != "" {
				if rawFinished || reason != "stop" && reason != "length" && reason != "tool_calls" {
					return fmt.Errorf("inference stream terminal reason is invalid or repeated")
				}
				rawFinished = true
			}
		}
		chunk["id"] = chunkID
		chunk["object"] = "chat.completion.chunk"
		chunk["created"] = time.Now().Unix()
		chunk["model"] = req.Model
		for _, rawChoice := range choices {
			choice := rawChoice.(map[string]interface{})
			delta, _ := choice["delta"].(map[string]interface{})
			if content, _ := delta["content"].(string); content != "" {
				tokenIndex++
			}
		}
		normalized, err := json.Marshal(chunk)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", normalized); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	stream := func() error {
		if supportsStructuredStream {
			return rawEngine.ChatCompletionStreamRaw(ctx, req, rawCallback)
		}
		return s.engine.ChatCompletionStream(ctx, req, callback)
	}

	// Send tokens as they arrive
	err := lease.Run(ctx, func(context.Context) error { return stream() })
	if err == nil && supportsStructuredStream && !rawFinished {
		err = fmt.Errorf("inference stream ended without a terminal choice")
	}

	if ctx.Err() != nil {
		err = ctx.Err()
	}

	if err != nil {
		log.Printf("Streaming error: %v", err)
		streamWriter.writeError(err)
		return
	}
	if supportsStructuredStream {
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
		if tokenIndex > 0 {
			atomic.AddInt64(&s.tokensGenerated, int64(tokenIndex))
			s.offgridMetrics.TokensOutputTotal.Add(float64(tokenIndex))
		}
		return
	}

	// Send final chunk with finish_reason
	finishReason := "stop"
	finalChunk := api.ChatCompletionChunk{
		ID:      chunkID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []api.ChatCompletionChoiceChunk{
			{
				Index:        0,
				Delta:        api.ChatMessage{},
				FinishReason: &finishReason,
			},
		},
	}

	data, _ := json.Marshal(finalChunk)
	fmt.Fprintf(w, "data: %s\n\n", data)
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	// Track tokens generated for metrics (approximate from tokenIndex)
	if tokenIndex > 0 {
		atomic.AddInt64(&s.tokensGenerated, int64(tokenIndex))
		s.offgridMetrics.TokensOutputTotal.Add(float64(tokenIndex))
	}
}

func (s *Server) handleCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse request
	var req api.CompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInferenceRequestError(w, err)
		return
	}

	// Validate request
	if req.Model == "" {
		writeError(w, "Model is required", http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		writeError(w, "Prompt is required", http.StatusBadRequest)
		return
	}
	if _, err := s.registry.GetChatModel(req.Model); err != nil {
		writeError(w, fmt.Sprintf("Model not found: %s", req.Model), http.StatusNotFound)
		return
	}

	// Admit bounded same-model concurrency; model changes remain exclusive.
	lease, err := s.acquireInferenceLease(r.Context(), req.Model, s.modelContextWindow(req.Model), nil)
	if err != nil {
		writeServiceError(w, newServiceError(http.StatusServiceUnavailable, "Model could not be loaded; check runtime availability", err))
		return
	}
	defer lease.Release()

	// Perform inference
	ctx := r.Context()
	var response *api.CompletionResponse
	err = lease.Run(ctx, func(ctx context.Context) error {
		var generationErr error
		response, generationErr = s.engine.Completion(ctx, &req)
		return generationErr
	})
	if err != nil {
		if handleEngineError(w, err) {
			return
		}
		writeError(w, fmt.Sprintf("Inference failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleEmbeddings handles POST /v1/embeddings for generating text embeddings
func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse request
	var req api.EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Model == "" {
		writeError(w, "Model is required", http.StatusBadRequest)
		return
	}

	if req.Input == nil {
		writeError(w, "Input is required", http.StatusBadRequest)
		return
	}

	// Check if it's an embedding model
	modelMeta, err := s.registry.GetModel(req.Model)
	if err != nil {
		writeError(w, fmt.Sprintf("Model not found: %s", req.Model), http.StatusNotFound)
		return
	}

	if modelMeta.Type != "embedding" {
		writeError(w, "Selected model is not an embedding model", http.StatusBadRequest)
		return
	}
	if req.EncodingFormat != "" && req.EncodingFormat != "float" {
		writeError(w, "Only float embedding encoding is supported", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	opts := inference.DefaultEmbeddingOptions()
	opts.NumThreads = s.config.NumThreads
	startTime := time.Now()
	response, err := s.embeddingEngine.GenerateEmbeddingsForModel(ctx, modelMeta.Path, opts, &req)
	if err != nil {
		writeError(w, fmt.Sprintf("Failed to generate embeddings: %v", err), http.StatusInternalServerError)
		return
	}

	// Track stats
	duration := time.Since(startTime)
	s.statsTracker.RecordInference(req.Model, int64(response.Usage.TotalTokens), duration.Milliseconds())

	// Send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}
