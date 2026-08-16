package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type responsesRequest struct {
	Model              string          `json:"model"`
	Input              json.RawMessage `json:"input"`
	Instructions       string          `json:"instructions,omitempty"`
	Tools              []responseTool  `json:"tools,omitempty"`
	Stream             bool            `json:"stream,omitempty"`
	Temperature        *float32        `json:"temperature,omitempty"`
	TopP               *float32        `json:"top_p,omitempty"`
	MaxOutputTokens    *int            `json:"max_output_tokens,omitempty"`
	PreviousResponseID string          `json:"previous_response_id,omitempty"`
}

type responseTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request responsesRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Model == "" || len(request.Input) == 0 {
		writeError(w, "model and input are required", http.StatusBadRequest)
		return
	}
	if request.PreviousResponseID != "" {
		writeError(w, "previous_response_id is not supported; resend the conversation input", http.StatusBadRequest)
		return
	}
	messages, err := responseInputMessages(request.Input)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Instructions != "" {
		messages = append([]api.ChatMessage{{Role: "system", Content: request.Instructions}}, messages...)
	}
	chatRequest := api.ChatCompletionRequest{
		Model: request.Model, Messages: messages, Temperature: request.Temperature,
		TopP: request.TopP, MaxTokens: request.MaxOutputTokens,
	}
	for _, tool := range request.Tools {
		if tool.Type != "function" || tool.Name == "" {
			writeError(w, "only named function tools are supported", http.StatusBadRequest)
			return
		}
		chatRequest.Tools = append(chatRequest.Tools, api.Tool{Type: "function", Function: api.FunctionDef{
			Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters,
		}})
	}
	// Use the non-streaming core so function calls remain structured. Streaming
	// clients receive standards-shaped lifecycle events below.
	recorder := invokeJSONHandler(r, &chatRequest, s.handleChatCompletions)
	if recorder.statusCode() >= http.StatusBadRequest {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(recorder.statusCode())
		_, _ = w.Write(recorder.body.Bytes())
		return
	}
	var completion api.ChatCompletionResponse
	if err := json.Unmarshal(recorder.body.Bytes(), &completion); err != nil || len(completion.Choices) == 0 {
		writeError(w, "invalid inference response", http.StatusInternalServerError)
		return
	}
	response := responseFromChat(completion)
	if request.Stream {
		writeResponsesStream(w, response)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func responseInputMessages(input json.RawMessage) ([]api.ChatMessage, error) {
	var text string
	if json.Unmarshal(input, &text) == nil {
		return []api.ChatMessage{{Role: "user", Content: text}}, nil
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(input, &items); err != nil {
		return nil, fmt.Errorf("input must be a string or message array")
	}
	messages := make([]api.ChatMessage, 0, len(items))
	for _, item := range items {
		role, _ := item["role"].(string)
		if role == "" {
			role = "user"
		}
		content := responseContentText(item["content"])
		if content != "" {
			messages = append(messages, api.ChatMessage{Role: role, Content: content})
		}
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("input contains no text messages")
	}
	return messages, nil
}

func responseContentText(value interface{}) string {
	if text, ok := value.(string); ok {
		return text
	}
	parts, _ := value.([]interface{})
	result := ""
	for _, value := range parts {
		part, _ := value.(map[string]interface{})
		kind, _ := part["type"].(string)
		if kind == "input_text" || kind == "output_text" || kind == "text" {
			if text, ok := part["text"].(string); ok {
				result += text
			}
		}
	}
	return result
}

func responseFromChat(completion api.ChatCompletionResponse) map[string]interface{} {
	message := completion.Choices[0].Message
	output := make([]map[string]interface{}, 0, len(message.ToolCalls)+1)
	if text := message.StringContent(); text != "" {
		output = append(output, map[string]interface{}{
			"type": "message", "id": completion.ID + "_message", "status": "completed", "role": "assistant",
			"content": []map[string]interface{}{{"type": "output_text", "text": text, "annotations": []interface{}{}}},
		})
	}
	for index, call := range message.ToolCalls {
		output = append(output, map[string]interface{}{
			"type": "function_call", "id": fmt.Sprintf("%s_tool_%d", completion.ID, index),
			"call_id": call.ID, "name": call.Function.Name, "arguments": call.Function.Arguments, "status": "completed",
		})
	}
	return map[string]interface{}{
		"id": completion.ID, "object": "response", "created_at": completion.Created,
		"status": "completed", "model": completion.Model, "output": output,
		"usage": map[string]int{"input_tokens": completion.Usage.PromptTokens, "output_tokens": completion.Usage.CompletionTokens, "total_tokens": completion.Usage.TotalTokens},
	}
}

func writeResponsesStream(w http.ResponseWriter, response map[string]interface{}) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	events := []struct {
		name string
		data interface{}
	}{
		{"response.created", map[string]interface{}{"type": "response.created", "response": response}},
		{"response.completed", map[string]interface{}{"type": "response.completed", "response": response}},
	}
	for _, event := range events {
		encoded, _ := json.Marshal(event.data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.name, encoded)
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
