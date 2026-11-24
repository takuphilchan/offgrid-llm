package inference

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func httpProviderChat(req ProviderRequest, descriptor ProviderDescriptor) (*api.ChatCompletionRequest, error) {
	var chat api.ChatCompletionRequest
	decoder := json.NewDecoder(bytes.NewReader(req.Prompt))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&chat) != nil || len(chat.Messages) == 0 {
		return nil, errors.New("HTTP provider requires a supported chat request")
	}
	if req.ContextLength > descriptor.ContextLength || !hasProviderCapability(descriptor, CapabilityText) {
		return nil, errors.New("HTTP provider context or text capability is unavailable")
	}
	if chat.N != nil && *chat.N != 1 {
		return nil, errors.New("HTTP provider supports one choice")
	}
	if err := chat.ResponseFormat.Validate(); err != nil {
		return nil, err
	}
	for _, message := range chat.Messages {
		if _, ok := message.Content.(string); !ok && message.Content != nil {
			return nil, errors.New("HTTP provider multimodal input is not qualified")
		}
		if !slices.Contains([]string{"system", "user", "assistant", "tool"}, message.Role) {
			return nil, errors.New("HTTP provider message role is unsupported")
		}
	}
	var declared []api.Tool
	if len(req.Tools) > 0 && json.Unmarshal(req.Tools, &declared) != nil || !sameTools(declared, chat.Tools) {
		return nil, errors.New("HTTP provider tool catalog differs from admitted request")
	}
	if len(chat.Tools) > 0 && !hasProviderCapability(descriptor, CapabilityToolCalls) {
		return nil, errors.New("HTTP provider tool calls are not qualified")
	}
	if chat.MaxTokens == nil {
		limit := req.MaxOutputTokens
		if limit == 0 {
			limit = 512
		}
		chat.MaxTokens = &limit
	}
	if *chat.MaxTokens < 1 || *chat.MaxTokens > 1<<20 || descriptor.ContextLength > 0 && *chat.MaxTokens > descriptor.ContextLength || req.MaxOutputTokens > 0 && *chat.MaxTokens > req.MaxOutputTokens {
		return nil, errors.New("HTTP provider output allocation exceeds admission")
	}
	chat.User = ""
	chat.UseKnowledgeBase = nil
	return &chat, nil
}

func ollamaChatBody(chat *api.ChatCompletionRequest, contextLength int) ([]byte, error) {
	if chat.PresencePenalty != nil || chat.FrequencyPenalty != nil {
		return nil, &api.UnsupportedControlError{Control: "penalties for Ollama"}
	}
	if chat.ToolChoice != nil && chat.ToolChoice != "auto" && chat.ToolChoice != "none" {
		return nil, &api.UnsupportedControlError{Control: "tool_choice for Ollama"}
	}
	options := map[string]any{"num_predict": *chat.MaxTokens}
	if contextLength > 0 {
		options["num_ctx"] = contextLength
	}
	if chat.Temperature != nil {
		options["temperature"] = *chat.Temperature
	}
	if chat.TopP != nil {
		options["top_p"] = *chat.TopP
	}
	if chat.Seed != nil {
		options["seed"] = *chat.Seed
	}
	if len(chat.Stop) > 0 {
		options["stop"] = chat.Stop
	}
	messages := make([]map[string]any, 0, len(chat.Messages))
	names := map[string]string{}
	for _, message := range chat.Messages {
		wire := map[string]any{"role": message.Role, "content": message.StringContent()}
		if len(message.ToolCalls) > 0 {
			calls := make([]any, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				var arguments map[string]any
				if json.Unmarshal([]byte(call.Function.Arguments), &arguments) != nil {
					return nil, errors.New("invalid Ollama historical tool arguments")
				}
				names[call.ID] = call.Function.Name
				calls = append(calls, map[string]any{"function": map[string]any{"name": call.Function.Name, "arguments": arguments}})
			}
			wire["tool_calls"] = calls
		}
		if message.Role == "tool" {
			name := names[message.ToolCallID]
			if name == "" {
				return nil, errors.New("Ollama tool result has no matching historical call")
			}
			wire["tool_name"] = name
		}
		messages = append(messages, wire)
	}
	body := map[string]any{"model": chat.Model, "messages": messages, "stream": true, "options": options}
	if len(chat.Tools) > 0 && chat.ToolChoice != "none" {
		body["tools"] = chat.Tools
	}
	if chat.ResponseFormat != nil {
		switch chat.ResponseFormat.Type {
		case "json_object":
			body["format"] = "json"
		case "json_schema":
			// Marshal the schema through ResponseJSONSchema to preserve reviewed key order.
			raw, err := json.Marshal(chat.ResponseFormat.JSONSchema)
			if err != nil {
				return nil, err
			}
			var fields map[string]json.RawMessage
			_ = json.Unmarshal(raw, &fields)
			body["format"] = fields["schema"]
		}
	}
	return json.Marshal(body)
}
