package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Manager handles function calling and tool use
type Manager struct {
	registeredTools map[string]api.Tool
}

// NewManager creates a new tool manager
func NewManager() *Manager {
	return &Manager{
		registeredTools: make(map[string]api.Tool),
	}
}

// RegisterTool registers a tool for use
func (m *Manager) RegisterTool(tool api.Tool) {
	m.registeredTools[tool.Function.Name] = tool
}

// FormatToolsPrompt formats tools into a system prompt for the model
func (m *Manager) FormatToolsPrompt(tools []api.Tool) string {
	if len(tools) == 0 {
		return ""
	}

	prompt := `You have access to the following tools. To use a tool, respond with a JSON object in the following format:
{"tool_calls": [{"id": "call_<unique_id>", "type": "function", "function": {"name": "<function_name>", "arguments": "<json_arguments>"}}]}

Available tools:
`

	for _, tool := range tools {
		prompt += fmt.Sprintf("\n### %s\n", tool.Function.Name)
		if tool.Function.Description != "" {
			prompt += fmt.Sprintf("Description: %s\n", tool.Function.Description)
		}
		if tool.Function.Parameters != nil {
			params, _ := json.MarshalIndent(tool.Function.Parameters, "", "  ")
			prompt += fmt.Sprintf("Parameters: %s\n", string(params))
		}
	}

	prompt += `
When you need to use a tool, respond ONLY with the JSON object. Do not include any other text.
When you don't need a tool, respond normally.
`
	return prompt
}

// ParseToolCalls attempts to parse tool calls from the model's response
func (m *Manager) ParseToolCalls(response string) ([]api.ToolCall, string, bool) {
	// Try to find JSON in the response
	response = strings.TrimSpace(response)

	// Check for explicit tool_calls JSON format
	if strings.HasPrefix(response, "{") {
		var result struct {
			ToolCalls []api.ToolCall `json:"tool_calls"`
		}
		if err := json.Unmarshal([]byte(response), &result); err == nil && len(result.ToolCalls) > 0 {
			return result.ToolCalls, "", true
		}
	}

	// Try to extract JSON from the response (model might include other text)
	jsonPattern := regexp.MustCompile(`\{[\s\S]*"tool_calls"[\s\S]*\}`)
	match := jsonPattern.FindString(response)
	if match != "" {
		var result struct {
			ToolCalls []api.ToolCall `json:"tool_calls"`
		}
		if err := json.Unmarshal([]byte(match), &result); err == nil && len(result.ToolCalls) > 0 {
			return result.ToolCalls, "", true
		}
	}

	// Alternative format: direct function call
	funcPattern := regexp.MustCompile(`\{[\s\S]*"function"[\s\S]*"name"[\s\S]*\}`)
	match = funcPattern.FindString(response)
	if match != "" {
		var singleCall api.ToolCall
		if err := json.Unmarshal([]byte(match), &singleCall); err == nil && singleCall.Function.Name != "" {
			if singleCall.ID == "" {
				singleCall.ID = fmt.Sprintf("call_%d", len(m.registeredTools))
			}
			if singleCall.Type == "" {
				singleCall.Type = "function"
			}
			return []api.ToolCall{singleCall}, "", true
		}
	}

	// No tool calls found, return original response
	return nil, response, false
}

// GenerateToolCallID generates a unique ID for a tool call
func GenerateToolCallID() string {
	return fmt.Sprintf("call_%d", generateRandomID())
}

// Simple random ID generator
var idCounter int64 = 0

func generateRandomID() int64 {
	idCounter++
	return idCounter
}

// ValidateToolCall validates that a tool call matches a registered tool
func (m *Manager) ValidateToolCall(call api.ToolCall, tools []api.Tool) error {
	for _, tool := range tools {
		if tool.Function.Name == call.Function.Name {
			// Tool exists, validate arguments if schema is provided
			if tool.Function.Parameters != nil {
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
					return fmt.Errorf("invalid arguments for function %s: %w", call.Function.Name, err)
				}
				// Could add JSON Schema validation here
			}
			return nil
		}
	}
	return fmt.Errorf("unknown function: %s", call.Function.Name)
}

// FormatToolResult formats a tool result for the model
func FormatToolResult(toolCallID, functionName, result string, isError bool) api.ChatMessage {
	content := result
	if isError {
		content = fmt.Sprintf("Error: %s", result)
	}
	return api.ChatMessage{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
		Name:       functionName,
	}
}

// BuiltInTools returns a list of built-in tools that OffGrid supports
func BuiltInTools() []api.Tool {
	return []api.Tool{
		{
			Type: "function",
			Function: api.FunctionDef{
				Name:        "search_documents",
				Description: "Search through uploaded documents using semantic search. Use this to find relevant information from the user's knowledge base.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "The search query to find relevant documents",
						},
						"top_k": map[string]interface{}{
							"type":        "integer",
							"description": "Number of results to return (default: 5)",
							"default":     5,
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: api.FunctionDef{
				Name:        "get_current_time",
				Description: "Get the current date and time",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: api.FunctionDef{
				Name:        "calculate",
				Description: "Perform mathematical calculations. Supports basic arithmetic and common math functions.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"expression": map[string]interface{}{
							"type":        "string",
							"description": "Mathematical expression to evaluate (e.g., '2 + 2', 'sqrt(16)', '10 * 5')",
						},
					},
					"required": []string{"expression"},
				},
			},
		},
	}
}
