package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// AgentState represents the current state of an agent
type AgentState string

const (
	StateIdle      AgentState = "idle"
	StateThinking  AgentState = "thinking"
	StateExecuting AgentState = "executing"
	StateWaiting   AgentState = "waiting"
	StateCompleted AgentState = "completed"
	StateFailed    AgentState = "failed"
)

// Step represents a single step in the agent's execution
type Step struct {
	ID         int           `json:"id"`
	Type       string        `json:"type"` // "thought", "action", "observation", "answer"
	Content    string        `json:"content"`
	ToolName   string        `json:"tool_name,omitempty"`
	ToolArgs   string        `json:"tool_args,omitempty"`
	ToolResult string        `json:"tool_result,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
	Duration   time.Duration `json:"duration,omitempty"`
}

// AgentConfig configures agent behavior
type AgentConfig struct {
	MaxIterations  int           `json:"max_iterations"`   // Maximum reasoning steps
	MaxTokens      int           `json:"max_tokens"`       // Max tokens per LLM call
	Temperature    float64       `json:"temperature"`      // LLM temperature
	TimeoutPerStep time.Duration `json:"timeout_per_step"` // Timeout for each step
	EnableMemory   bool          `json:"enable_memory"`    // Use conversation memory
	SystemPrompt   string        `json:"system_prompt"`    // Custom system prompt
	ReasoningStyle string        `json:"reasoning_style"`  // "react", "cot", "plan-execute"
}

// DefaultAgentConfig returns sensible defaults
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		MaxIterations:  10,
		MaxTokens:      2048,
		Temperature:    0.7,
		TimeoutPerStep: 5 * time.Minute, // Increased for low-end machines
		EnableMemory:   true,
		ReasoningStyle: "react",
	}
}

// ToolExecutor executes a tool and returns the result
type ToolExecutor func(ctx context.Context, name string, args json.RawMessage) (string, error)

// LLMCaller calls the LLM with messages
type LLMCaller func(ctx context.Context, messages []api.ChatMessage, options map[string]interface{}) (string, error)

// Agent is an autonomous agent that can use tools to accomplish tasks
type Agent struct {
	mu          sync.RWMutex
	config      AgentConfig
	tools       []api.Tool
	executor    ToolExecutor
	llmCaller   LLMCaller
	state       AgentState
	steps       []Step
	memory      []api.ChatMessage
	currentTask string
	logger      *log.Logger
	onStep      func(Step) // Callback for each step
}

// NewAgent creates a new agent
func NewAgent(config AgentConfig, tools []api.Tool, executor ToolExecutor, llmCaller LLMCaller) *Agent {
	if config.SystemPrompt == "" {
		config.SystemPrompt = getDefaultSystemPrompt(config.ReasoningStyle, tools)
	}

	return &Agent{
		config:    config,
		tools:     tools,
		executor:  executor,
		llmCaller: llmCaller,
		state:     StateIdle,
		steps:     make([]Step, 0),
		memory:    make([]api.ChatMessage, 0),
		logger:    log.New(log.Writer(), "[Agent] ", log.LstdFlags),
	}
}

// SetStepCallback sets a callback for each step
func (a *Agent) SetStepCallback(callback func(Step)) {
	a.onStep = callback
}

// Run executes the agent with the given task
func (a *Agent) Run(ctx context.Context, task string) (string, error) {
	// Check if LLM caller is configured
	if a.llmCaller == nil {
		return "", fmt.Errorf("no LLM configured - start the server first with 'offgrid serve' or use the API")
	}

	a.mu.Lock()
	a.state = StateThinking
	a.currentTask = task
	a.steps = make([]Step, 0)
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		if a.state != StateFailed {
			a.state = StateCompleted
		}
		a.mu.Unlock()
	}()

	// Build initial messages
	messages := []api.ChatMessage{
		{Role: "system", Content: a.config.SystemPrompt},
	}

	// Add memory if enabled
	if a.config.EnableMemory && len(a.memory) > 0 {
		messages = append(messages, a.memory...)
	}

	// Add the task
	messages = append(messages, api.ChatMessage{Role: "user", Content: task})

	// Track consecutive errors to prevent infinite loops
	var lastErrorAction string
	var lastErrorInput string
	var consecutiveErrors int
	const maxConsecutiveErrors = 2

	// Track total errors - stop if too many errors overall
	var totalErrors int
	const maxTotalErrors = 5

	// Track unknown tool errors - stop if LLM keeps inventing fake tools
	unknownToolCount := 0
	const maxUnknownTools = 3

	// Track repeated successful tool calls - model stuck in loop
	var lastSuccessfulAction string
	var successfulActionCount int
	const maxRepeatedActions = 3

	// Main reasoning loop
	for i := 0; i < a.config.MaxIterations; i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// Context window protection: estimate message size and trim if needed
		// Keep system prompt + last N turns to stay under context limit
		totalLen := 0
		for _, msg := range messages {
			totalLen += len(msg.StringContent())
		}
		// If approaching context limit (roughly 6000 chars ~ 1500 tokens), trim middle messages
		// Keep system prompt (first) and recent conversation (last 4 messages)
		if totalLen > 6000 && len(messages) > 6 {
			// Keep system prompt and last 4 messages
			trimmed := make([]api.ChatMessage, 0, 5)
			trimmed = append(trimmed, messages[0]) // system prompt
			trimmed = append(trimmed, api.ChatMessage{
				Role:    "user",
				Content: "[Previous conversation trimmed for context limit]",
			})
			trimmed = append(trimmed, messages[len(messages)-4:]...) // last 4 messages
			messages = trimmed
			a.logger.Printf("Context trimmed: %d chars -> keeping last 4 messages", totalLen)
		}

		// Create step context with timeout
		stepCtx, cancel := context.WithTimeout(ctx, a.config.TimeoutPerStep)

		// Call LLM
		stepStart := time.Now()
		a.mu.Lock()
		a.state = StateThinking
		a.mu.Unlock()

		response, err := a.llmCaller(stepCtx, messages, map[string]interface{}{
			"temperature": a.config.Temperature,
			"max_tokens":  a.config.MaxTokens,
		})
		cancel()

		if err != nil {
			a.mu.Lock()
			a.state = StateFailed
			a.mu.Unlock()
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		// Parse the response
		thought, action, actionInput, answer := a.parseResponse(response)

		// Record thought step
		if thought != "" {
			step := Step{
				ID:        len(a.steps) + 1,
				Type:      "thought",
				Content:   thought,
				Timestamp: time.Now(),
				Duration:  time.Since(stepStart),
			}
			a.addStep(step)
		}

		// PRIORITIZE checking for actions over answers
		// This ensures tools are actually called before giving final answer
		// Skip "None" or "none" as they indicate no tool is needed
		actionLower := strings.ToLower(action)
		if action != "" && actionLower != "none" && actionLower != "n/a" && actionLower != "null" {
			a.mu.Lock()
			a.state = StateExecuting
			a.mu.Unlock()

			actionStep := Step{
				ID:        len(a.steps) + 1,
				Type:      "action",
				Content:   fmt.Sprintf("Calling tool: %s", action),
				ToolName:  action,
				ToolArgs:  actionInput,
				Timestamp: time.Now(),
			}

			// Execute the tool
			execStart := time.Now()
			result, err := a.executor(ctx, action, json.RawMessage(actionInput))
			actionStep.Duration = time.Since(execStart)

			isError := false
			isUnknownTool := false
			if err != nil {
				result = fmt.Sprintf("Error: %s", err.Error())
				actionStep.ToolResult = result
				isError = true
				if strings.Contains(err.Error(), "unknown tool") {
					isUnknownTool = true
				}
			} else if strings.HasPrefix(result, "Error:") {
				isError = true
				actionStep.ToolResult = result
				if strings.Contains(result, "unknown tool") {
					isUnknownTool = true
				}
			} else {
				actionStep.ToolResult = result
				// Reset error tracking on success
				consecutiveErrors = 0
				lastErrorAction = ""
				lastErrorInput = ""
				unknownToolCount = 0

				// Track repeated successful calls to same tool - model might be stuck
				if action == lastSuccessfulAction {
					successfulActionCount++
					if successfulActionCount >= maxRepeatedActions {
						// Model is stuck calling same tool repeatedly - generate answer from last result
						a.addStep(actionStep)

						// Generate a sensible answer based on the tool and its result
						var finalAnswer string
						switch action {
						case "current_time":
							finalAnswer = fmt.Sprintf("The current time is %s.", result)
						case "calculator":
							finalAnswer = fmt.Sprintf("The result is %s.", result)
						case "read_file":
							finalAnswer = fmt.Sprintf("Here is the file content:\n%s", result)
						case "list_files":
							finalAnswer = fmt.Sprintf("Here are the files:\n%s", result)
						case "http_get":
							finalAnswer = fmt.Sprintf("Here is the response:\n%s", result)
						default:
							finalAnswer = fmt.Sprintf("Based on my analysis: %s", result)
						}

						answerStep := Step{
							ID:        len(a.steps) + 1,
							Type:      "answer",
							Content:   finalAnswer,
							Timestamp: time.Now(),
						}
						a.addStep(answerStep)
						a.mu.Lock()
						a.state = StateCompleted
						a.mu.Unlock()
						return finalAnswer, nil
					}
				} else {
					lastSuccessfulAction = action
					successfulActionCount = 1
				}
			}

			// Track unknown tool errors - LLM is hallucinating tools
			if isUnknownTool {
				unknownToolCount++
				if unknownToolCount >= maxUnknownTools {
					a.addStep(actionStep)
					errorStep := Step{
						ID:        len(a.steps) + 1,
						Type:      "error",
						Content:   fmt.Sprintf("Agent stopped: tried %d non-existent tools. The LLM is hallucinating tools that don't exist.", unknownToolCount),
						Timestamp: time.Now(),
					}
					a.addStep(errorStep)
					return fmt.Sprintf("I cannot complete this task. I only have access to: calculator, read_file, write_file, list_files, shell, http_get, and current_time. Please rephrase your request using only these tools, or ask a simpler question."), nil
				}
			}

			// Track consecutive errors on same action
			if isError {
				totalErrors++

				// Check total errors first
				if totalErrors >= maxTotalErrors {
					a.addStep(actionStep)
					errorStep := Step{
						ID:        len(a.steps) + 1,
						Type:      "error",
						Content:   fmt.Sprintf("Agent stopped: too many errors (%d). Last error: %s", totalErrors, result),
						Timestamp: time.Now(),
					}
					a.addStep(errorStep)
					return "I was unable to complete the task due to multiple errors. Please try a simpler request or check the available tools.", nil
				}

				if action == lastErrorAction && actionInput == lastErrorInput {
					consecutiveErrors++
					if consecutiveErrors >= maxConsecutiveErrors {
						// Stop the loop - agent is stuck
						a.addStep(actionStep)
						errorStep := Step{
							ID:        len(a.steps) + 1,
							Type:      "error",
							Content:   fmt.Sprintf("Agent stopped: repeated failures on %s. The operation cannot be completed due to: %s", action, result),
							Timestamp: time.Now(),
						}
						a.addStep(errorStep)
						return fmt.Sprintf("I was unable to complete the task. The tool '%s' failed repeatedly with error: %s", action, result), nil
					}
				} else {
					// New error, reset counter
					consecutiveErrors = 1
					lastErrorAction = action
					lastErrorInput = actionInput
				}
			}
			a.addStep(actionStep)

			// Truncate long results for context management - smaller limit for LLMs
			observationText := result
			if len(observationText) > 1000 {
				observationText = observationText[:1000] + "\n... (output truncated)"
			}

			// Add observation step
			obsStep := Step{
				ID:        len(a.steps) + 1,
				Type:      "observation",
				Content:   observationText,
				Timestamp: time.Now(),
			}
			a.addStep(obsStep)

			// Add to messages for next iteration
			messages = append(messages, api.ChatMessage{
				Role:    "assistant",
				Content: response,
			})

			// If unknown tool error, add stronger guidance
			observationMessage := fmt.Sprintf("Observation: %s", observationText)
			if isUnknownTool {
				observationMessage = fmt.Sprintf("Observation: %s\n\nIMPORTANT: That tool does not exist. Your ONLY available tools are: calculator, read_file, write_file, list_files, shell, http_get, current_time. Do NOT try other tools. If you already have the answer, use Final Answer now.", observationText)
			}

			messages = append(messages, api.ChatMessage{
				Role:    "user",
				Content: observationMessage,
			})
			// Continue to next iteration to process tool result
			continue
		}

		// Only check for final answer if NO action was requested
		if answer != "" {
			a.mu.Lock()
			a.state = StateCompleted
			a.mu.Unlock()
			return answer, nil
		}

		// Has thought but no action - prompt to continue
		if thought != "" {
			messages = append(messages, api.ChatMessage{
				Role:    "assistant",
				Content: response,
			})
			messages = append(messages, api.ChatMessage{
				Role:    "user",
				Content: "Continue your reasoning. If you need information, use a tool. Otherwise provide your Final Answer.",
			})
		} else {
			// No structured output - treat as final answer
			return response, nil
		}
	}

	return "", fmt.Errorf("agent reached maximum iterations (%d) without completing the task", a.config.MaxIterations)
}

// parseResponse extracts thought, action, action_input, and final answer from LLM response
// Only extracts the FIRST action to prevent multi-action confusion
func (a *Agent) parseResponse(response string) (thought, action, actionInput, answer string) {
	lines := strings.Split(response, "\n")

	var thoughtBuilder strings.Builder
	var actionFound bool
	var actionInputFound bool
	var answerFound bool

	for i, line := range lines {
		lineLower := strings.ToLower(strings.TrimSpace(line))
		trimmed := strings.TrimSpace(line)

		// Check for Final Answer first (highest priority to stop)
		if strings.HasPrefix(lineLower, "final answer:") || strings.HasPrefix(lineLower, "answer:") {
			content := strings.TrimPrefix(strings.TrimPrefix(line, "Final Answer:"), "Answer:")
			content = strings.TrimPrefix(strings.TrimPrefix(content, "final answer:"), "answer:")
			content = strings.TrimSpace(content)

			// Collect remaining lines as part of the answer
			var answerLines []string
			if content != "" {
				answerLines = append(answerLines, content)
			}
			for j := i + 1; j < len(lines); j++ {
				nextLine := strings.TrimSpace(lines[j])
				// Stop if we hit another section marker
				nextLower := strings.ToLower(nextLine)
				if strings.HasPrefix(nextLower, "thought:") ||
					strings.HasPrefix(nextLower, "action:") ||
					strings.HasPrefix(nextLower, "observation:") {
					break
				}
				if nextLine != "" {
					answerLines = append(answerLines, nextLine)
				}
			}
			answer = strings.Join(answerLines, "\n")
			answerFound = true
			break
		}

		// Check for Thought
		if strings.HasPrefix(lineLower, "thought:") || strings.HasPrefix(lineLower, "thinking:") {
			content := strings.TrimPrefix(strings.TrimPrefix(line, "Thought:"), "Thinking:")
			content = strings.TrimPrefix(strings.TrimPrefix(content, "thought:"), "thinking:")
			if strings.TrimSpace(content) != "" {
				thoughtBuilder.WriteString(strings.TrimSpace(content))
				thoughtBuilder.WriteString(" ")
			}
			continue
		}

		// Check for Action (only take the first one)
		if strings.HasPrefix(lineLower, "action:") && !actionFound {
			content := strings.TrimPrefix(strings.TrimPrefix(line, "Action:"), "action:")
			action = strings.TrimSpace(content)
			// Validate action is a single word (tool name)
			if strings.Contains(action, " ") {
				// Take only the first word as the action
				parts := strings.Fields(action)
				if len(parts) > 0 {
					action = parts[0]
				}
			}
			actionFound = true
			continue
		}

		// Check for Action Input (only if we have an action and haven't found input yet)
		if (strings.HasPrefix(lineLower, "action_input:") || strings.HasPrefix(lineLower, "action input:")) && actionFound && !actionInputFound {
			content := strings.TrimPrefix(strings.TrimPrefix(line, "Action_Input:"), "Action Input:")
			content = strings.TrimPrefix(strings.TrimPrefix(content, "action_input:"), "action input:")
			content = strings.TrimSpace(content)

			// Collect content that might span multiple lines
			var inputLines []string
			if content != "" {
				inputLines = append(inputLines, content)
			}

			// Look ahead for more content (JSON or otherwise)
			braceCount := strings.Count(content, "{") - strings.Count(content, "}")
			for j := i + 1; j < len(lines); j++ {
				nextLine := strings.TrimSpace(lines[j])
				if nextLine == "" {
					continue
				}
				// Stop if we hit another section marker or LLM commentary
				nextLower := strings.ToLower(nextLine)
				if strings.HasPrefix(nextLower, "thought:") ||
					strings.HasPrefix(nextLower, "action:") ||
					strings.HasPrefix(nextLower, "observation:") ||
					strings.HasPrefix(nextLower, "answer:") ||
					strings.HasPrefix(nextLower, "result:") ||
					strings.HasPrefix(nextLower, "note:") ||
					strings.HasPrefix(nextLower, "response:") ||
					strings.HasPrefix(nextLower, "output:") ||
					strings.HasPrefix(nextLower, "this ") ||
					strings.HasPrefix(nextLower, "since ") {
					break
				}
				inputLines = append(inputLines, nextLine)
				braceCount += strings.Count(nextLine, "{") - strings.Count(nextLine, "}")
				// Stop if we've closed all braces
				if braceCount == 0 && strings.Contains(strings.Join(inputLines, ""), "}") {
					break
				}
			}

			actionInput = strings.Join(inputLines, " ")
			actionInputFound = true

			// Once we have action + input, we can stop parsing
			break
		}

		// Append content to thought if we're in thought mode
		if !actionFound && !answerFound && trimmed != "" &&
			!strings.HasPrefix(lineLower, "observation:") {
			// Only add non-section content to thought
			thoughtBuilder.WriteString(trimmed)
			thoughtBuilder.WriteString(" ")
		}
	}

	thought = strings.TrimSpace(thoughtBuilder.String())

	// Clean up and normalize action input
	actionInput = normalizeActionInput(action, actionInput)

	return
}

// normalizeActionInput tries to extract or create valid JSON from action input
func normalizeActionInput(action, input string) string {
	input = strings.TrimSpace(input)

	// Strip common LLM commentary/hallucinations that might have leaked in
	commentPrefixes := []string{
		"Response:", "response:",
		"Note:", "note:",
		"Output:", "output:",
		"Answer:", "answer:",
		"This ", "Since ",
		"\nThought:", "\nAction:",
	}
	for _, prefix := range commentPrefixes {
		if idx := strings.Index(input, prefix); idx != -1 {
			input = strings.TrimSpace(input[:idx])
		}
	}

	// Try to extract balanced JSON from the input
	if start := strings.Index(input, "{"); start != -1 {
		extracted := extractBalancedJSON(input[start:])
		if extracted != "" {
			// Validate and fix the JSON
			return fixJSON(extracted)
		}
	}

	// Handle backtick-wrapped commands: `command here`
	if strings.Contains(input, "`") {
		// Extract content between backticks
		re := regexp.MustCompile("`([^`]+)`")
		matches := re.FindStringSubmatch(input)
		if len(matches) > 1 {
			cmd := matches[1]
			// Create JSON based on tool type
			if action == "shell" {
				return fmt.Sprintf(`{"command": "%s"}`, escapeJSON(cmd))
			}
			if action == "calculator" {
				return fmt.Sprintf(`{"expression": "%s"}`, escapeJSON(cmd))
			}
			// Generic: use as first parameter
			return fmt.Sprintf(`{"input": "%s"}`, escapeJSON(cmd))
		}
	}

	// Handle raw string inputs (no JSON, no backticks)
	if input != "" && !strings.HasPrefix(input, "{") {
		// Strip outer quotes if present (LLM sometimes wraps commands in quotes)
		if (strings.HasPrefix(input, `"`) && strings.HasSuffix(input, `"`)) ||
			(strings.HasPrefix(input, `'`) && strings.HasSuffix(input, `'`)) {
			input = input[1 : len(input)-1]
		}

		// Try to infer the parameter name based on the tool
		switch action {
		case "shell":
			return fmt.Sprintf(`{"command": "%s"}`, escapeJSON(input))
		case "calculator":
			return fmt.Sprintf(`{"expression": "%s"}`, escapeJSON(input))
		case "write_file":
			// Can't infer both path and content from raw string
			return "{}"
		case "read_file", "list_files":
			return fmt.Sprintf(`{"path": "%s"}`, escapeJSON(input))
		default:
			// Generic fallback
			return fmt.Sprintf(`{"input": "%s"}`, escapeJSON(input))
		}
	}

	// No input found, return empty JSON
	if action != "" {
		return "{}"
	}
	return ""
}

// extractBalancedJSON extracts the first balanced JSON object from a string
func extractBalancedJSON(s string) string {
	if len(s) == 0 || s[0] != '{' {
		return ""
	}

	depth := 0
	inString := false
	escaped := false

	for i, ch := range s {
		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return s[:i+1]
			}
		}
	}

	// Unbalanced - return what we have
	return s
}

// fixJSON attempts to fix common JSON issues from LLM output
func fixJSON(s string) string {
	s = strings.TrimSpace(s)

	// Try to parse as-is first
	var test map[string]interface{}
	if json.Unmarshal([]byte(s), &test) == nil {
		return s
	}

	// Fix 0: Remove trailing garbage after the JSON (e.g., "Output: ..." lines)
	// Find the last } and truncate there
	if lastBrace := strings.LastIndex(s, "}"); lastBrace != -1 {
		s = s[:lastBrace+1]
	}

	// Fix 1: Replace single quotes with double quotes
	// But be careful not to replace apostrophes inside already double-quoted strings
	s = fixSingleQuotes(s)

	// Try again
	if json.Unmarshal([]byte(s), &test) == nil {
		// Check if values have extra quotes and fix them
		return fixDoubleQuotedValues(s)
	}

	// Fix 2: Remove trailing extra braces (e.g., "{}}" -> "{}")
	for strings.HasSuffix(s, "}}") {
		candidate := s[:len(s)-1]
		if json.Unmarshal([]byte(candidate), &test) == nil {
			return fixDoubleQuotedValues(candidate)
		}
		s = candidate
	}

	// Fix 3: Add missing quotes around keys (e.g., {expression: "x"} -> {"expression": "x"})
	re := regexp.MustCompile(`([{,]\s*)([a-zA-Z_][a-zA-Z0-9_]*)(\s*:)`)
	s = re.ReplaceAllString(s, `$1"$2"$3`)

	// Try again after fixes
	if json.Unmarshal([]byte(s), &test) == nil {
		return fixDoubleQuotedValues(s)
	}

	// Fix 4: If still invalid, try to rebuild from scratch
	return rebuildJSON(s)
}

// fixDoubleQuotedValues removes extra quotes from JSON string values
// e.g., {"command": "\"date\""} -> {"command": "date"}
func fixDoubleQuotedValues(s string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return s
	}

	modified := false
	for key, val := range data {
		if strVal, ok := val.(string); ok {
			// Check if value starts and ends with escaped quotes
			if strings.HasPrefix(strVal, `"`) && strings.HasSuffix(strVal, `"`) {
				data[key] = strVal[1 : len(strVal)-1]
				modified = true
			} else if strings.HasPrefix(strVal, `'`) && strings.HasSuffix(strVal, `'`) {
				data[key] = strVal[1 : len(strVal)-1]
				modified = true
			}
		}
	}

	if modified {
		result, _ := json.Marshal(data)
		return string(result)
	}
	return s
}

// fixSingleQuotes replaces single quotes with double quotes for JSON values
func fixSingleQuotes(s string) string {
	result := strings.Builder{}
	inDoubleQuote := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if ch == '"' && (i == 0 || s[i-1] != '\\') {
			inDoubleQuote = !inDoubleQuote
			result.WriteByte(ch)
		} else if ch == '\'' && !inDoubleQuote {
			// Replace single quote with double quote
			result.WriteByte('"')
		} else {
			result.WriteByte(ch)
		}
	}

	return result.String()
}

// rebuildJSON attempts to rebuild valid JSON from broken input
func rebuildJSON(s string) string {
	// Try to extract key-value pairs using a more flexible approach
	pairs := make(map[string]string)

	// Pattern: key (quoted or unquoted) : value (quoted)
	// Matches: "key": "value" or key: "value" or "key": 'value' or key: 'value'
	re := regexp.MustCompile(`["\']?([a-zA-Z_][a-zA-Z0-9_]*)["\']?\s*:\s*["\']([^"\']*)["\']`)
	matches := re.FindAllStringSubmatch(s, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			pairs[match[1]] = match[2]
		}
	}

	if len(pairs) == 0 {
		return "{}"
	}

	// Rebuild as valid JSON
	result, _ := json.Marshal(pairs)
	return string(result)
}

// escapeJSON escapes a string for use in JSON
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// addStep adds a step and calls the callback
func (a *Agent) addStep(step Step) {
	a.mu.Lock()
	a.steps = append(a.steps, step)
	a.mu.Unlock()

	if a.onStep != nil {
		a.onStep(step)
	}

	a.logger.Printf("Step %d [%s]: %s", step.ID, step.Type, truncate(step.Content, 100))
}

// GetState returns the current agent state
func (a *Agent) GetState() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// GetSteps returns all steps taken
func (a *Agent) GetSteps() []Step {
	a.mu.RLock()
	defer a.mu.RUnlock()
	steps := make([]Step, len(a.steps))
	copy(steps, a.steps)
	return steps
}

// ClearMemory clears the agent's conversation memory
func (a *Agent) ClearMemory() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.memory = make([]api.ChatMessage, 0)
}

// getDefaultSystemPrompt returns the system prompt based on reasoning style
func getDefaultSystemPrompt(style string, tools []api.Tool) string {
	toolDescriptions := formatToolDescriptions(tools)

	switch style {
	case "react":
		return fmt.Sprintf(`You are a helpful AI assistant with access to specific tools.

YOUR ONLY AVAILABLE TOOLS ARE:
%s

CRITICAL RULES:
1. You can ONLY use the tools listed above. Do NOT invent or guess tool names.
2. If a tool returns "unknown tool", that tool does not exist - do NOT try variations.
3. For simple questions, use minimal tools and give the Final Answer quickly.
4. STOP when you have the information needed - do not continue unnecessarily.

RESPONSE FORMAT - Pick ONE:

Option A - Use a tool:
Thought: [why you need this specific tool]
Action: [exact tool name from list above]
Action_Input: {"param": "value"}

Option B - Give final answer (when you have the info):
Thought: [brief summary]
Final Answer: [your answer to the user]

EXAMPLE:
User: What time is it?

Response 1:
Thought: I need the current time. I'll use current_time.
Action: current_time
Action_Input: {}

[System provides: Observation: 2025-12-06 14:30:00 CST]

Response 2:
Thought: I have the time.
Final Answer: It is 2:30 PM CST on December 6, 2025.

COMMON MISTAKES TO AVOID:
- Do NOT call tools that aren't in the list above (parse_xml, puppeteer, selenium, etc. DO NOT EXIST)
- Do NOT continue after getting the answer - give Final Answer immediately
- Do NOT predict/hallucinate tool outputs with "Answer:" before seeing Observation
- Do NOT make up URLs or try to fetch random websites`, toolDescriptions)

	case "cot":
		return fmt.Sprintf(`You are an AI assistant. You can ONLY use these tools:

%s

RULES:
- ONLY use tools from the list above - no others exist
- One tool per response, wait for result
- Give Final Answer as soon as you have the information

FORMAT:
Thought: [reasoning]
Action: [tool from list]
Action_Input: {"key": "value"}

OR when done:
Final Answer: [your answer]`, toolDescriptions)

	case "plan-execute":
		return fmt.Sprintf(`You are an AI agent. You can ONLY use these tools:

%s

RULES:
- ONLY use the tools listed - no others exist
- One action per response
- Stop as soon as you can answer

FORMAT:
Thought: [reasoning]
Action: [tool from list]
Action_Input: {"key": "value"}

When done:
Final Answer: [result]`, toolDescriptions)

	default:
		return getDefaultSystemPrompt("react", tools)
	}
}

// formatToolDescriptions formats tools for the system prompt
func formatToolDescriptions(tools []api.Tool) string {
	if len(tools) == 0 {
		return "(No tools available)"
	}

	var sb strings.Builder
	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", tool.Function.Name, tool.Function.Description))
		if tool.Function.Parameters != nil {
			params, _ := json.Marshal(tool.Function.Parameters)
			sb.WriteString(fmt.Sprintf("  Parameters: %s\n", string(params)))
		}
	}
	return sb.String()
}

// truncate truncates a string to maxLen
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
