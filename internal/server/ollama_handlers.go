package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// These handlers implement the stable, local Ollama HTTP surface while
// keeping OffGrid's OpenAI-compatible request path as the single inference
// implementation. External agents can therefore switch base URLs without a
// bespoke OffGrid SDK.

type ollamaRequest struct {
	Model    string                 `json:"model"`
	Messages []ollamaMessage        `json:"messages,omitempty"`
	Prompt   string                 `json:"prompt,omitempty"`
	System   string                 `json:"system,omitempty"`
	Stream   *bool                  `json:"stream,omitempty"`
	Tools    []api.Tool             `json:"tools,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content,omitempty"`
	Thinking  string           `json:"thinking,omitempty"`
	ToolName  string           `json:"tool_name,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaFunctionCall `json:"function"`
}

type ollamaFunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ollamaChatResponse struct {
	Model           string        `json:"model"`
	CreatedAt       string        `json:"created_at"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	DoneReason      string        `json:"done_reason,omitempty"`
	TotalDuration   int64         `json:"total_duration,omitempty"`
	PromptEvalCount int           `json:"prompt_eval_count,omitempty"`
	EvalCount       int           `json:"eval_count,omitempty"`
}

type ollamaGenerateResponse struct {
	Model           string `json:"model"`
	CreatedAt       string `json:"created_at"`
	Response        string `json:"response"`
	Done            bool   `json:"done"`
	DoneReason      string `json:"done_reason,omitempty"`
	TotalDuration   int64  `json:"total_duration,omitempty"`
	PromptEvalCount int    `json:"prompt_eval_count,omitempty"`
	EvalCount       int    `json:"eval_count,omitempty"`
}

type ollamaModelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

type ollamaModel struct {
	Name          string             `json:"name"`
	Model         string             `json:"model"`
	ModifiedAt    time.Time          `json:"modified_at"`
	Size          int64              `json:"size"`
	Digest        string             `json:"digest"`
	Details       ollamaModelDetails `json:"details"`
	ExpiresAt     *time.Time         `json:"expires_at,omitempty"`
	SizeVRAM      int64              `json:"size_vram,omitempty"`
	ContextLength int                `json:"context_length,omitempty"`
}

func (s *Server) handleOllamaVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOllamaError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	version := strings.TrimPrefix(strings.TrimSpace(s.version), "v")
	if version == "" || version == "dev" || version == "unknown" {
		version = "0.3.0"
	}
	writeJSON(w, http.StatusOK, map[string]string{"version": version})
}

func (s *Server) handleOllamaTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOllamaError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	result := make([]ollamaModel, 0)
	for _, model := range s.registry.ListModels() {
		metadata, err := s.registry.GetModel(model.ID)
		if err != nil {
			continue
		}
		result = append(result, ollamaModelFromMetadata(metadata))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	writeJSON(w, http.StatusOK, map[string]interface{}{"models": result})
}

func (s *Server) handleOllamaShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOllamaError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var request struct {
		Model string `json:"model"`
		Name  string `json:"name"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeOllamaError(w, http.StatusBadRequest, err.Error())
		return
	}
	modelID := request.Model
	if modelID == "" {
		modelID = request.Name
	}
	metadata, err := s.registry.GetModel(modelID)
	if err != nil {
		writeOllamaError(w, http.StatusNotFound, fmt.Sprintf("model %q not found", modelID))
		return
	}
	model := ollamaModelFromMetadata(metadata)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"modified_at": model.ModifiedAt,
		"details":     model.Details,
		"model_info": map[string]interface{}{
			"general.architecture":   model.Details.Family,
			"general.file_type":      model.Details.QuantizationLevel,
			"offgrid.context_length": metadata.ContextSize,
		},
		"capabilities": []string{"completion", "tools"},
	})
}

func (s *Server) handleOllamaPS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOllamaError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	result := make([]ollamaModel, 0)
	for _, model := range s.registry.ListModels() {
		metadata, err := s.registry.GetModel(model.ID)
		if err != nil || !metadata.IsLoaded {
			continue
		}
		entry := ollamaModelFromMetadata(metadata)
		expires := time.Now().UTC().Add(5 * time.Minute)
		entry.ExpiresAt = &expires
		entry.ContextLength = metadata.ContextSize
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	writeJSON(w, http.StatusOK, map[string]interface{}{"models": result})
}

func (s *Server) handleOllamaChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOllamaError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var request ollamaRequest
	if err := decodeJSON(r, &request); err != nil {
		writeOllamaError(w, http.StatusBadRequest, err.Error())
		return
	}
	chatRequest, err := request.toChatRequest()
	if err != nil {
		writeOllamaError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.runOllamaInference(w, r, chatRequest, false)
}

func (s *Server) handleOllamaGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOllamaError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var request ollamaRequest
	if err := decodeJSON(r, &request); err != nil {
		writeOllamaError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Model == "" || request.Prompt == "" {
		writeOllamaError(w, http.StatusBadRequest, "model and prompt are required")
		return
	}
	messages := make([]api.ChatMessage, 0, 2)
	if request.System != "" {
		messages = append(messages, api.ChatMessage{Role: "system", Content: request.System})
	}
	messages = append(messages, api.ChatMessage{Role: "user", Content: request.Prompt})
	chatRequest := &api.ChatCompletionRequest{Model: request.Model, Messages: messages}
	request.applyOptions(chatRequest)
	stream := request.Stream == nil || *request.Stream
	chatRequest.Stream = stream
	s.runOllamaInference(w, r, chatRequest, true)
}

func (s *Server) handleOllamaEmbed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOllamaError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var request struct {
		Model string      `json:"model"`
		Input interface{} `json:"input"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeOllamaError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Model == "" || request.Input == nil {
		writeOllamaError(w, http.StatusBadRequest, "model and input are required")
		return
	}
	started := time.Now()
	openAIRequest := api.EmbeddingRequest{Model: request.Model, Input: request.Input}
	recorder := invokeJSONHandler(r, openAIRequest, s.handleEmbeddings)
	if recorder.statusCode() >= http.StatusBadRequest {
		writeCapturedOllamaError(w, recorder)
		return
	}
	var response api.EmbeddingResponse
	if err := json.Unmarshal(recorder.body.Bytes(), &response); err != nil {
		writeOllamaError(w, http.StatusInternalServerError, "invalid embedding response")
		return
	}
	embeddings := make([][]float32, len(response.Data))
	for _, item := range response.Data {
		if item.Index >= 0 && item.Index < len(embeddings) {
			embeddings[item.Index] = item.Embedding
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"model":             request.Model,
		"embeddings":        embeddings,
		"total_duration":    time.Since(started).Nanoseconds(),
		"prompt_eval_count": response.Usage.PromptTokens,
	})
}

func (s *Server) runOllamaInference(w http.ResponseWriter, r *http.Request, request *api.ChatCompletionRequest, generate bool) {
	// The engine's token callback cannot represent partial structured tool
	// calls. Preserve correctness for agent clients by returning one terminal
	// NDJSON frame when a streamed request includes tools.
	if request.Stream && len(request.Tools) > 0 {
		copy := *request
		copy.Stream = false
		started := time.Now()
		recorder := invokeJSONHandler(r, &copy, s.handleChatCompletions)
		if recorder.statusCode() >= http.StatusBadRequest {
			writeCapturedOllamaError(w, recorder)
			return
		}
		var response api.ChatCompletionResponse
		if err := json.Unmarshal(recorder.body.Bytes(), &response); err != nil || len(response.Choices) == 0 {
			writeOllamaError(w, http.StatusInternalServerError, "invalid inference response")
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		message := fromAPIMessage(response.Choices[0].Message)
		if generate {
			_ = json.NewEncoder(w).Encode(ollamaGenerateResponse{
				Model: response.Model, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
				Response: message.Content, Done: true, DoneReason: response.Choices[0].FinishReason,
				TotalDuration: time.Since(started).Nanoseconds(), PromptEvalCount: response.Usage.PromptTokens,
				EvalCount: response.Usage.CompletionTokens,
			})
		} else {
			_ = json.NewEncoder(w).Encode(ollamaChatResponse{
				Model: response.Model, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
				Message: message, Done: true, DoneReason: response.Choices[0].FinishReason,
				TotalDuration: time.Since(started).Nanoseconds(), PromptEvalCount: response.Usage.PromptTokens,
				EvalCount: response.Usage.CompletionTokens,
			})
		}
		return
	}
	if request.Stream {
		writer := newOllamaStreamWriter(w, request.Model, generate)
		invokeStreamingHandler(r, request, writer, s.handleChatCompletions)
		return
	}
	started := time.Now()
	recorder := invokeJSONHandler(r, request, s.handleChatCompletions)
	if recorder.statusCode() >= http.StatusBadRequest {
		writeCapturedOllamaError(w, recorder)
		return
	}
	var response api.ChatCompletionResponse
	if err := json.Unmarshal(recorder.body.Bytes(), &response); err != nil || len(response.Choices) == 0 {
		writeOllamaError(w, http.StatusInternalServerError, "invalid inference response")
		return
	}
	createdAt := time.Unix(response.Created, 0).UTC().Format(time.RFC3339Nano)
	if generate {
		writeJSON(w, http.StatusOK, ollamaGenerateResponse{
			Model: response.Model, CreatedAt: createdAt,
			Response: response.Choices[0].Message.StringContent(), Done: true,
			DoneReason: response.Choices[0].FinishReason, TotalDuration: time.Since(started).Nanoseconds(),
			PromptEvalCount: response.Usage.PromptTokens, EvalCount: response.Usage.CompletionTokens,
		})
		return
	}
	writeJSON(w, http.StatusOK, ollamaChatResponse{
		Model: response.Model, CreatedAt: createdAt,
		Message: fromAPIMessage(response.Choices[0].Message), Done: true,
		DoneReason: response.Choices[0].FinishReason, TotalDuration: time.Since(started).Nanoseconds(),
		PromptEvalCount: response.Usage.PromptTokens, EvalCount: response.Usage.CompletionTokens,
	})
}

func (r *ollamaRequest) toChatRequest() (*api.ChatCompletionRequest, error) {
	if r.Model == "" || len(r.Messages) == 0 {
		return nil, fmt.Errorf("model and messages are required")
	}
	messages := make([]api.ChatMessage, 0, len(r.Messages))
	for _, message := range r.Messages {
		converted := api.ChatMessage{Role: message.Role, Content: message.Content, Name: message.ToolName}
		if message.Role == "tool" {
			converted.ToolCallID = message.ToolName
		}
		for index, call := range message.ToolCalls {
			arguments, err := json.Marshal(call.Function.Arguments)
			if err != nil {
				return nil, fmt.Errorf("invalid tool arguments: %w", err)
			}
			converted.ToolCalls = append(converted.ToolCalls, api.ToolCall{
				ID: fmt.Sprintf("call_%d", index), Type: "function",
				Function: api.FunctionCall{Name: call.Function.Name, Arguments: string(arguments)},
			})
		}
		messages = append(messages, converted)
	}
	request := &api.ChatCompletionRequest{Model: r.Model, Messages: messages, Tools: r.Tools}
	stream := r.Stream == nil || *r.Stream
	request.Stream = stream
	r.applyOptions(request)
	return request, nil
}

func (r *ollamaRequest) applyOptions(request *api.ChatCompletionRequest) {
	if value, ok := numberOption(r.Options, "temperature"); ok {
		converted := float32(value)
		request.Temperature = &converted
	}
	if value, ok := numberOption(r.Options, "top_p"); ok {
		converted := float32(value)
		request.TopP = &converted
	}
	if value, ok := numberOption(r.Options, "num_predict"); ok {
		converted := int(value)
		request.MaxTokens = &converted
	}
	if stop, ok := r.Options["stop"].([]interface{}); ok {
		for _, item := range stop {
			if value, ok := item.(string); ok {
				request.Stop = append(request.Stop, value)
			}
		}
	}
}

func numberOption(options map[string]interface{}, key string) (float64, bool) {
	if options == nil {
		return 0, false
	}
	value, ok := options[key].(float64)
	return value, ok
}

func fromAPIMessage(message api.ChatMessage) ollamaMessage {
	result := ollamaMessage{Role: message.Role, Content: message.StringContent()}
	for _, call := range message.ToolCalls {
		arguments := make(map[string]interface{})
		if err := json.Unmarshal([]byte(call.Function.Arguments), &arguments); err != nil {
			arguments["_raw"] = call.Function.Arguments
		}
		result.ToolCalls = append(result.ToolCalls, ollamaToolCall{Function: ollamaFunctionCall{
			Name: call.Function.Name, Arguments: arguments,
		}})
	}
	return result
}

func ollamaModelFromMetadata(metadata *api.ModelMetadata) ollamaModel {
	modified := time.Time{}
	if info, err := os.Stat(metadata.Path); err == nil {
		modified = info.ModTime().UTC()
	}
	family := modelFamily(metadata.ID)
	return ollamaModel{
		Name: metadata.ID, Model: metadata.ID, ModifiedAt: modified, Size: metadata.Size,
		// Digest remains empty until the artifact store has computed a verified
		// content digest. Returning a fabricated digest would break trust and P2P.
		Digest: "",
		Details: ollamaModelDetails{
			Format: metadata.Format, Family: family, Families: []string{family},
			ParameterSize: metadata.Parameters, QuantizationLevel: metadata.Quantization,
		},
	}
}

func modelFamily(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, separator := range []string{"-", "_", ":"} {
		if index := strings.Index(name, separator); index > 0 {
			return name[:index]
		}
	}
	if name == "" {
		return "unknown"
	}
	return name
}

func decodeJSON(r *http.Request, target interface{}) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 8<<20))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	return nil
}

type captureResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newCaptureResponseWriter() *captureResponseWriter {
	return &captureResponseWriter{header: make(http.Header)}
}

func (w *captureResponseWriter) Header() http.Header    { return w.header }
func (w *captureResponseWriter) WriteHeader(status int) { w.status = status }
func (w *captureResponseWriter) Write(content []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(content)
}
func (w *captureResponseWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func invokeJSONHandler(r *http.Request, payload interface{}, handler http.HandlerFunc) *captureResponseWriter {
	encoded, _ := json.Marshal(payload)
	request := r.Clone(r.Context())
	request.Method = http.MethodPost
	request.Body = io.NopCloser(bytes.NewReader(encoded))
	request.ContentLength = int64(len(encoded))
	writer := newCaptureResponseWriter()
	handler(writer, request)
	return writer
}

func invokeStreamingHandler(r *http.Request, payload interface{}, writer http.ResponseWriter, handler http.HandlerFunc) {
	encoded, _ := json.Marshal(payload)
	request := r.Clone(r.Context())
	request.Method = http.MethodPost
	request.Body = io.NopCloser(bytes.NewReader(encoded))
	request.ContentLength = int64(len(encoded))
	handler(writer, request)
}

type ollamaStreamWriter struct {
	target   http.ResponseWriter
	header   http.Header
	model    string
	generate bool
	status   int
	started  time.Time
}

func newOllamaStreamWriter(target http.ResponseWriter, model string, generate bool) *ollamaStreamWriter {
	return &ollamaStreamWriter{target: target, header: make(http.Header), model: model, generate: generate, started: time.Now()}
}

func (w *ollamaStreamWriter) Header() http.Header    { return w.header }
func (w *ollamaStreamWriter) WriteHeader(status int) { w.status = status }
func (w *ollamaStreamWriter) Flush() {
	if flusher, ok := w.target.(http.Flusher); ok {
		flusher.Flush()
	}
}
func (w *ollamaStreamWriter) Write(content []byte) (int, error) {
	if w.status >= http.StatusBadRequest || !bytes.HasPrefix(bytes.TrimSpace(content), []byte("data:")) {
		status := w.status
		if status == 0 {
			status = http.StatusInternalServerError
		}
		message := capturedErrorMessage(content)
		writeOllamaError(w.target, status, message)
		return len(content), nil
	}
	for _, line := range bytes.Split(content, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if bytes.Equal(payload, []byte("[DONE]")) || len(payload) == 0 {
			continue
		}
		var chunk api.ChatCompletionChunk
		if err := json.Unmarshal(payload, &chunk); err != nil || len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		done := choice.FinishReason != nil
		reason := ""
		if choice.FinishReason != nil {
			reason = *choice.FinishReason
		}
		w.target.Header().Set("Content-Type", "application/x-ndjson")
		w.target.Header().Set("Cache-Control", "no-cache")
		if w.generate {
			_ = json.NewEncoder(w.target).Encode(ollamaGenerateResponse{
				Model: w.model, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
				Response: choice.Delta.StringContent(), Done: done, DoneReason: reason,
				TotalDuration: durationWhenDone(w.started, done),
			})
		} else {
			_ = json.NewEncoder(w.target).Encode(ollamaChatResponse{
				Model: w.model, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
				Message: fromAPIMessage(choice.Delta), Done: done, DoneReason: reason,
				TotalDuration: durationWhenDone(w.started, done),
			})
		}
		w.Flush()
	}
	return len(content), nil
}

func durationWhenDone(started time.Time, done bool) int64 {
	if !done {
		return 0
	}
	return time.Since(started).Nanoseconds()
}

func writeCapturedOllamaError(w http.ResponseWriter, captured *captureResponseWriter) {
	writeOllamaError(w, captured.statusCode(), capturedErrorMessage(captured.body.Bytes()))
}

func capturedErrorMessage(content []byte) string {
	var response api.ErrorResponse
	if json.Unmarshal(content, &response) == nil && response.Error.Message != "" {
		return response.Error.Message
	}
	var simple map[string]interface{}
	if json.Unmarshal(content, &simple) == nil {
		if value, ok := simple["error"].(string); ok {
			return value
		}
	}
	message := strings.TrimSpace(string(content))
	if message == "" {
		return "request failed"
	}
	return message
}

func writeOllamaError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
