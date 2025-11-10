package templates

import (
	"fmt"
	"strings"
)

// Template represents a prompt template
type Template struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	System      string            `json:"system"`
	Template    string            `json:"template"`
	Variables   []string          `json:"variables"`
	Examples    map[string]string `json:"examples,omitempty"`
}

// BuiltinTemplates contains all built-in prompt templates
var BuiltinTemplates = map[string]*Template{
	"summarize": {
		Name:        "summarize",
		Description: "Summarize text concisely",
		System:      "You are a helpful assistant that creates clear, concise summaries.",
		Template:    "Please summarize the following text:\n\n{{.text}}\n\nProvide a summary in {{.length}} format.",
		Variables:   []string{"text", "length"},
		Examples: map[string]string{
			"length": "one paragraph (default: 3-5 sentences)",
		},
	},
	"code-review": {
		Name:        "code-review",
		Description: "Review code for quality, bugs, and improvements",
		System:      "You are an expert code reviewer. Analyze code for bugs, security issues, performance problems, and best practices.",
		Template:    "Please review the following {{.language}} code:\n\n```{{.language}}\n{{.code}}\n```\n\nProvide:\n1. Bug/security issues\n2. Performance concerns\n3. Best practice violations\n4. Suggested improvements",
		Variables:   []string{"code", "language"},
		Examples: map[string]string{
			"language": "python, javascript, go, etc. (default: auto-detect)",
		},
	},
	"translate": {
		Name:        "translate",
		Description: "Translate text between languages",
		System:      "You are a professional translator with expertise in multiple languages.",
		Template:    "Translate the following text from {{.from}} to {{.to}}:\n\n{{.text}}",
		Variables:   []string{"text", "from", "to"},
		Examples: map[string]string{
			"from": "source language (e.g., English)",
			"to":   "target language (e.g., Spanish)",
		},
	},
	"explain": {
		Name:        "explain",
		Description: "Explain a concept simply",
		System:      "You are a teacher who excels at explaining complex topics in simple terms.",
		Template:    "Explain the following concept to someone at a {{.level}} level:\n\n{{.concept}}\n\nUse {{.style}} and provide examples.",
		Variables:   []string{"concept", "level", "style"},
		Examples: map[string]string{
			"level": "beginner, intermediate, expert (default: beginner)",
			"style": "simple language, technical terms, analogies (default: simple language)",
		},
	},
	"brainstorm": {
		Name:        "brainstorm",
		Description: "Generate creative ideas",
		System:      "You are a creative brainstorming assistant who generates innovative ideas.",
		Template:    "Generate {{.count}} creative ideas for:\n\n{{.topic}}\n\nFocus on {{.focus}} approaches.",
		Variables:   []string{"topic", "count", "focus"},
		Examples: map[string]string{
			"count": "number of ideas (default: 10)",
			"focus": "practical, innovative, budget-friendly (default: innovative)",
		},
	},
	"debug": {
		Name:        "debug",
		Description: "Debug code and find issues",
		System:      "You are an expert debugger who identifies issues in code and suggests fixes.",
		Template:    "Debug the following {{.language}} code:\n\n```{{.language}}\n{{.code}}\n```\n\nError message:\n{{.error}}\n\nProvide:\n1. Root cause analysis\n2. Step-by-step fix\n3. Prevention tips",
		Variables:   []string{"code", "language", "error"},
		Examples: map[string]string{
			"language": "programming language",
			"error":    "error message or description of the bug",
		},
	},
	"document": {
		Name:        "document",
		Description: "Generate documentation for code",
		System:      "You are a technical writer who creates clear, comprehensive documentation.",
		Template:    "Generate {{.style}} documentation for the following {{.language}} code:\n\n```{{.language}}\n{{.code}}\n```\n\nInclude:\n1. Overview\n2. Parameters/arguments\n3. Return values\n4. Usage examples\n5. Edge cases",
		Variables:   []string{"code", "language", "style"},
		Examples: map[string]string{
			"language": "programming language",
			"style":    "inline comments, markdown, docstring (default: markdown)",
		},
	},
	"refactor": {
		Name:        "refactor",
		Description: "Refactor code for better quality",
		System:      "You are a senior software engineer who specializes in code refactoring.",
		Template:    "Refactor the following {{.language}} code to improve {{.focus}}:\n\n```{{.language}}\n{{.code}}\n```\n\nProvide:\n1. Refactored code\n2. Explanation of changes\n3. Benefits of the refactoring",
		Variables:   []string{"code", "language", "focus"},
		Examples: map[string]string{
			"language": "programming language",
			"focus":    "readability, performance, maintainability (default: all)",
		},
	},
	"test": {
		Name:        "test",
		Description: "Generate test cases for code",
		System:      "You are a QA engineer who writes comprehensive test cases.",
		Template:    "Generate {{.framework}} test cases for the following {{.language}} code:\n\n```{{.language}}\n{{.code}}\n```\n\nInclude:\n1. Unit tests for core functionality\n2. Edge cases\n3. Error conditions\n4. Integration scenarios (if applicable)",
		Variables:   []string{"code", "language", "framework"},
		Examples: map[string]string{
			"language":  "programming language",
			"framework": "pytest, jest, go test, etc. (default: standard library)",
		},
	},
	"cli": {
		Name:        "cli",
		Description: "Generate CLI tool implementation",
		System:      "You are a developer who builds excellent command-line tools.",
		Template:    "Create a {{.language}} CLI tool that:\n\n{{.description}}\n\nFeatures needed:\n{{.features}}\n\nProvide:\n1. Complete implementation\n2. Help/usage text\n3. Example commands",
		Variables:   []string{"description", "language", "features"},
		Examples: map[string]string{
			"language": "python, go, bash, etc. (default: python)",
			"features": "comma-separated list of features",
		},
	},
}

// Apply applies variables to a template and returns the formatted prompt
func (t *Template) Apply(variables map[string]string) (string, error) {
	result := t.Template

	// Set defaults if not provided
	if variables == nil {
		variables = make(map[string]string)
	}

	// Check for required variables
	for _, varName := range t.Variables {
		if _, exists := variables[varName]; !exists {
			// Set reasonable defaults for common variables
			switch varName {
			case "length":
				variables[varName] = "3-5 sentences"
			case "level":
				variables[varName] = "beginner"
			case "style":
				variables[varName] = "simple language"
			case "count":
				variables[varName] = "10"
			case "focus":
				variables[varName] = "all aspects"
			case "framework":
				variables[varName] = "standard library"
			default:
				return "", fmt.Errorf("missing required variable: %s", varName)
			}
		}
	}

	// Replace variables
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result, nil
}

// GetTemplate returns a template by name
func GetTemplate(name string) (*Template, error) {
	template, exists := BuiltinTemplates[name]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", name)
	}
	return template, nil
}

// ListTemplates returns all available template names
func ListTemplates() []string {
	names := make([]string, 0, len(BuiltinTemplates))
	for name := range BuiltinTemplates {
		names = append(names, name)
	}
	return names
}
