package agents

import (
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// AgentTemplate represents a pre-configured agent persona
type AgentTemplate struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Icon        string      `json:"icon"`
	Config      AgentConfig `json:"config"`
	Tools       []string    `json:"tools"` // Tool names to enable
}

// BuiltInTemplates returns all pre-configured agent templates
func BuiltInTemplates() []AgentTemplate {
	return []AgentTemplate{
		{
			ID:          "researcher",
			Name:        "Research Assistant",
			Description: "Gathers information, analyzes data, and synthesizes findings into reports",
			Icon:        "research",
			Config: AgentConfig{
				MaxIterations:  15,
				MaxTokens:      4096,
				Temperature:    0.3,
				TimeoutPerStep: 300000000000, // 5 minutes
				EnableMemory:   true,
				ReasoningStyle: "cot",
				SystemPrompt: `You are a meticulous research assistant. Your role is to:
1. Break down complex research questions into smaller sub-questions
2. Gather relevant information systematically
3. Cross-reference and verify facts
4. Synthesize findings into clear, well-structured reports
5. Cite sources and highlight areas of uncertainty

Always think step-by-step. When unsure, acknowledge uncertainty rather than guessing.
Format your final answers with clear headings and bullet points.`,
			},
			Tools: []string{"http_get", "read_file", "search", "memory"},
		},
		{
			ID:          "coder",
			Name:        "Code Assistant",
			Description: "Helps write, debug, and refactor code across multiple languages",
			Icon:        "code",
			Config: AgentConfig{
				MaxIterations:  20,
				MaxTokens:      4096,
				Temperature:    0.2, // Lower for more precise code
				TimeoutPerStep: 300000000000,
				EnableMemory:   true,
				ReasoningStyle: "react",
				SystemPrompt: `You are an expert software engineer. Your role is to:
1. Write clean, well-documented, maintainable code
2. Debug issues systematically by analyzing error messages and tracing execution
3. Refactor code to improve readability and performance
4. Follow best practices for the relevant language/framework
5. Write tests when appropriate

When writing code:
- Use meaningful variable and function names
- Add comments for complex logic
- Handle errors appropriately
- Consider edge cases

When debugging:
- Read error messages carefully
- Check file contents with read_file
- Test hypotheses one at a time
- Explain your reasoning`,
			},
			Tools: []string{"read_file", "write_file", "list_files", "shell"},
		},
		{
			ID:          "analyst",
			Name:        "Data Analyst",
			Description: "Analyzes data, identifies patterns, and creates insights",
			Icon:        "chart",
			Config: AgentConfig{
				MaxIterations:  12,
				MaxTokens:      4096,
				Temperature:    0.4,
				TimeoutPerStep: 300000000000,
				EnableMemory:   true,
				ReasoningStyle: "plan-execute",
				SystemPrompt: `You are a data analyst expert. Your role is to:
1. Understand the data and its structure
2. Clean and preprocess data as needed
3. Identify patterns, trends, and anomalies
4. Create meaningful visualizations (describe them)
5. Draw actionable insights and recommendations

When analyzing data:
- Start by understanding what questions need answering
- Check data quality and completeness
- Use appropriate statistical methods
- Present findings clearly with supporting evidence
- Acknowledge limitations in the data or analysis`,
			},
			Tools: []string{"read_file", "calculator", "shell", "memory"},
		},
		{
			ID:          "writer",
			Name:        "Content Writer",
			Description: "Creates articles, documentation, and creative content",
			Icon:        "edit",
			Config: AgentConfig{
				MaxIterations:  10,
				MaxTokens:      4096,
				Temperature:    0.7, // Higher for creativity
				TimeoutPerStep: 300000000000,
				EnableMemory:   true,
				ReasoningStyle: "cot",
				SystemPrompt: `You are a skilled content writer. Your role is to:
1. Understand the audience and purpose of the content
2. Research the topic thoroughly
3. Create engaging, well-structured content
4. Edit for clarity, grammar, and style
5. Optimize for the target platform/medium

Writing principles:
- Start with a compelling hook
- Use clear, concise language
- Structure content logically
- Support claims with evidence
- End with a clear call-to-action or conclusion`,
			},
			Tools: []string{"http_get", "read_file", "search", "memory"},
		},
		{
			ID:          "sysadmin",
			Name:        "System Administrator",
			Description: "Manages servers, troubleshoots issues, and automates tasks",
			Icon:        "server",
			Config: AgentConfig{
				MaxIterations:  15,
				MaxTokens:      2048,
				Temperature:    0.2, // Low for precision
				TimeoutPerStep: 300000000000,
				EnableMemory:   true,
				ReasoningStyle: "react",
				SystemPrompt: `You are an experienced system administrator. Your role is to:
1. Diagnose system issues methodically
2. Monitor system health and performance
3. Automate repetitive tasks with scripts
4. Maintain security best practices
5. Document procedures and changes

When troubleshooting:
- Check logs first
- Verify basic connectivity and services
- Test one change at a time
- Document what you find and what you change
- Always have a rollback plan

IMPORTANT: Be cautious with destructive commands. Always confirm before:
- Deleting files
- Stopping critical services
- Modifying system configurations`,
			},
			Tools: []string{"shell", "read_file", "write_file", "list_files"},
		},
		{
			ID:          "planner",
			Name:        "Project Planner",
			Description: "Breaks down projects into tasks, estimates effort, tracks progress",
			Icon:        "list",
			Config: AgentConfig{
				MaxIterations:  8,
				MaxTokens:      4096,
				Temperature:    0.5,
				TimeoutPerStep: 300000000000,
				EnableMemory:   true,
				ReasoningStyle: "plan-execute",
				SystemPrompt: `You are a project planning expert. Your role is to:
1. Break down projects into actionable tasks
2. Estimate effort and identify dependencies
3. Create realistic timelines
4. Identify risks and mitigation strategies
5. Track progress and adjust plans as needed

Planning principles:
- Start with clear goals and success criteria
- Break large tasks into smaller, measurable pieces
- Consider dependencies between tasks
- Build in buffer time for unexpected issues
- Review and update plans regularly`,
			},
			Tools: []string{"read_file", "write_file", "current_time", "memory"},
		},
	}
}

// GetTemplate returns a template by ID
func GetTemplate(id string) *AgentTemplate {
	for _, t := range BuiltInTemplates() {
		if t.ID == id {
			return &t
		}
	}
	return nil
}

// ListTemplateIDs returns all available template IDs
func ListTemplateIDs() []string {
	templates := BuiltInTemplates()
	ids := make([]string, len(templates))
	for i, t := range templates {
		ids[i] = t.ID
	}
	return ids
}

// CreateAgentFromTemplate creates an agent with template configuration
func CreateAgentFromTemplate(templateID string, tools []api.Tool, executor ToolExecutor, llmCaller LLMCaller) (*Agent, error) {
	template := GetTemplate(templateID)
	if template == nil {
		return nil, &TemplateNotFoundError{ID: templateID}
	}

	return NewAgent(template.Config, tools, executor, llmCaller), nil
}

// TemplateNotFoundError is returned when a template ID doesn't exist
type TemplateNotFoundError struct {
	ID string
}

func (e *TemplateNotFoundError) Error() string {
	return "agent template not found: " + e.ID
}
