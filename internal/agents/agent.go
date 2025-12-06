package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
		TimeoutPerStep: 60 * time.Second,
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

	// Main reasoning loop
	for i := 0; i < a.config.MaxIterations; i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
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
		if action != "" {
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

			if err != nil {
				result = fmt.Sprintf("Error: %s", err.Error())
				actionStep.ToolResult = result
			} else {
				actionStep.ToolResult = result
			}
			a.addStep(actionStep)

			// Truncate long results for context management
			observationText := result
			if len(observationText) > 2000 {
				observationText = observationText[:2000] + "\n... (truncated for context)"
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
			messages = append(messages, api.ChatMessage{
				Role:    "user",
				Content: fmt.Sprintf("Observation: %s", observationText),
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
func (a *Agent) parseResponse(response string) (thought, action, actionInput, answer string) {
	lines := strings.Split(response, "\n")

	var inThought, inAction, inActionInput, inAnswer bool
	var thoughtLines, actionLines, actionInputLines, answerLines []string

	for _, line := range lines {
		lineLower := strings.ToLower(strings.TrimSpace(line))

		// Check for section markers
		if strings.HasPrefix(lineLower, "thought:") || strings.HasPrefix(lineLower, "thinking:") {
			inThought = true
			inAction = false
			inActionInput = false
			inAnswer = false
			content := strings.TrimPrefix(strings.TrimPrefix(line, "Thought:"), "Thinking:")
			content = strings.TrimPrefix(strings.TrimPrefix(content, "thought:"), "thinking:")
			if strings.TrimSpace(content) != "" {
				thoughtLines = append(thoughtLines, strings.TrimSpace(content))
			}
			continue
		}
		if strings.HasPrefix(lineLower, "action:") {
			inThought = false
			inAction = true
			inActionInput = false
			inAnswer = false
			content := strings.TrimPrefix(strings.TrimPrefix(line, "Action:"), "action:")
			if strings.TrimSpace(content) != "" {
				actionLines = append(actionLines, strings.TrimSpace(content))
			}
			continue
		}
		if strings.HasPrefix(lineLower, "action_input:") || strings.HasPrefix(lineLower, "action input:") {
			inThought = false
			inAction = false
			inActionInput = true
			inAnswer = false
			content := strings.TrimPrefix(strings.TrimPrefix(line, "Action_Input:"), "Action Input:")
			content = strings.TrimPrefix(strings.TrimPrefix(content, "action_input:"), "action input:")
			if strings.TrimSpace(content) != "" {
				actionInputLines = append(actionInputLines, strings.TrimSpace(content))
			}
			continue
		}
		if strings.HasPrefix(lineLower, "final answer:") || strings.HasPrefix(lineLower, "answer:") {
			inThought = false
			inAction = false
			inActionInput = false
			inAnswer = true
			content := strings.TrimPrefix(strings.TrimPrefix(line, "Final Answer:"), "Answer:")
			content = strings.TrimPrefix(strings.TrimPrefix(content, "final answer:"), "answer:")
			if strings.TrimSpace(content) != "" {
				answerLines = append(answerLines, strings.TrimSpace(content))
			}
			continue
		}

		// Append to current section
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if inThought {
			thoughtLines = append(thoughtLines, trimmed)
		} else if inAction {
			actionLines = append(actionLines, trimmed)
		} else if inActionInput {
			actionInputLines = append(actionInputLines, trimmed)
		} else if inAnswer {
			answerLines = append(answerLines, trimmed)
		}
	}

	thought = strings.Join(thoughtLines, "\n")
	action = strings.TrimSpace(strings.Join(actionLines, " "))
	actionInput = strings.TrimSpace(strings.Join(actionInputLines, "\n"))
	answer = strings.Join(answerLines, "\n")

	// Clean up action input - try to extract JSON if present
	if actionInput != "" {
		if start := strings.Index(actionInput, "{"); start != -1 {
			if end := strings.LastIndex(actionInput, "}"); end != -1 && end > start {
				actionInput = actionInput[start : end+1]
			}
		}
	}

	// If no JSON found, create a simple JSON object
	if actionInput == "" && action != "" {
		actionInput = "{}"
	}

	return
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
		return fmt.Sprintf(`You are an autonomous AI agent that uses a ReAct (Reasoning + Acting) approach to solve problems.

You have access to the following tools:
%s

To use a tool, respond with:
Thought: [your reasoning about what to do next]
Action: [the tool name]
Action_Input: [the JSON arguments for the tool]

After receiving an observation (tool result), continue reasoning.

When you have enough information to answer, respond with:
Thought: [your final reasoning]
Final Answer: [your complete answer to the user]

Important:
- Always think step by step
- Use tools when you need external information
- Be concise but thorough
- If a tool returns an error, try a different approach
- Always end with a Final Answer

Begin!`, toolDescriptions)

	case "cot":
		return fmt.Sprintf(`You are an AI assistant that thinks step by step to solve problems.

You have access to these tools:
%s

Think through problems carefully before providing an answer. Show your reasoning process.

If you need to use a tool:
Action: [tool_name]
Action_Input: {"param": "value"}

Then wait for the observation before continuing.

Always provide a clear Final Answer when done.`, toolDescriptions)

	case "plan-execute":
		return fmt.Sprintf(`You are an AI agent that plans before executing.

Available tools:
%s

When given a task:
1. First, create a step-by-step plan
2. Execute each step using tools as needed
3. Revise the plan if needed based on observations
4. Provide the final answer

Use this format:
Plan:
1. [step 1]
2. [step 2]
...

Then for each step:
Thought: [reasoning for this step]
Action: [tool if needed]
Action_Input: [arguments]

End with:
Final Answer: [complete answer]`, toolDescriptions)

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
