package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// LegacyAgentDependencies supplies durable task submission and scoped local
// MCP file operations to the migrated terminal command.
type LegacyAgentDependencies struct {
	SubmitTask func(context.Context, []string) error
	MCP        MCPDependencies
}

func legacyAgentCommand(ctx context.Context, args []string, env Environment) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		if len(args) == 2 && args[1] == "all" {
			legacyAgentHelpAll(env)
		} else if len(args) == 1 {
			legacyAgentHelp(env)
		} else {
			return Usage("Use offgrid agent help or offgrid agent help all")
		}
		return nil
	}
	if len(args) == 0 || args[0] == "chat" || strings.HasPrefix(args[0], "--") {
		if len(args) > 0 && args[0] == "chat" {
			args = args[1:]
		}
		return runInteractiveAgent(ctx, args, env)
	}
	switch args[0] {
	case "tasks", "list":
		return legacyAgentList(ctx, "tasks", env)
	case "workflows":
		return legacyAgentList(ctx, "workflows", env)
	case "tools":
		return legacyAgentList(ctx, "tools", env)
	case "templates", "template":
		return legacyAgentTemplates(env)
	case "mcp":
		return legacyMCPCommand(args[1:], env)
	default:
		Presenter{Stderr: env.Stderr, Style: env.Style}.Error(fmt.Sprintf("Unknown agent command: %s", args[0]))
		return nil
	}
}

func legacyAgentList(ctx context.Context, kind string, env Environment) error {
	cfg, err := localConfig(env)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://localhost:%d/v1/agents/%s", cfg.ServerPort, kind)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// Legacy task listing already used the authenticated request path; keep it.
	if kind == "tasks" {
		client, err := serviceClient(env)
		if err != nil {
			return err
		}
		return legacyAgentTasks(ctx, client, env)
	}
	resp, err := env.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		Presenter{Stderr: env.Stderr, Style: env.Style}.Error(fmt.Sprintf("Cannot connect to server: %v\n\n  Start the server with: offgrid serve", err))
		return nil
	}
	defer resp.Body.Close()
	switch kind {
	case "workflows":
		var workflows []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&workflows); err != nil {
			Presenter{Stderr: env.Stderr, Style: env.Style}.Error(fmt.Sprintf("Failed to parse response: %v", err))
			return nil
		}
		if env.Options.JSON {
			return localJSON(env, workflows)
		}
		legacyWorkflowsOutput(workflows, env)
	case "tools":
		var result struct {
			Tools []struct {
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				Parameters  map[string]interface{} `json:"parameters,omitempty"`
				Type        string                 `json:"type"`
			} `json:"tools"`
			Count int `json:"count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			Presenter{Stderr: env.Stderr, Style: env.Style}.Error(fmt.Sprintf("Failed to parse response: %v", err))
			return nil
		}
		if env.Options.JSON {
			return localJSON(env, result)
		}
		legacyToolsOutput(result.Tools, result.Count, env)
	}
	return nil
}

// The legacy task endpoint is accessed through the reviewed service client.
func legacyAgentTasks(ctx context.Context, client *serviceclient.Client, env Environment) error {
	resp, err := client.Do(ctx, http.MethodGet, "/v1/agents/tasks", "application/json", nil)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		Presenter{Stderr: env.Stderr, Style: env.Style}.Error(fmt.Sprintf("Cannot connect to server: %v\n\n  Start the server with: offgrid serve", err))
		return nil
	}
	defer resp.Body.Close()
	var tasks []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		Presenter{Stderr: env.Stderr, Style: env.Style}.Error(fmt.Sprintf("Failed to parse response: %v", err))
		return nil
	}
	if env.Options.JSON {
		return localJSON(env, tasks)
	}
	s := env.Style
	w := env.Stdout
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s◈ Recent Tasks%s\n", s.Primary+s.Bold, s.Reset)
	fmt.Fprintln(w)
	if len(tasks) == 0 {
		fmt.Fprintf(w, "  %sNo tasks yet%s\n", s.Dim, s.Reset)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %sRun your first task:%s\n", s.Dim, s.Reset)
		fmt.Fprintf(w, "    %s$%s offgrid agent run \"Your task here\" --model <name>\n", s.Dim, s.Reset)
	} else {
		for _, task := range tasks {
			id, _ := task["id"].(string)
			status, _ := task["status"].(string)
			prompt, _ := task["prompt"].(string)
			icon, color := "○", s.Accent
			switch status {
			case "completed":
				icon, color = "✓", s.Success
			case "failed":
				icon, color = "✗", s.Error
			case "running":
				icon, color = "›", s.Primary
			}
			prompt = truncateLegacyText(TerminalSafe(prompt), 50)
			fmt.Fprintf(w, "  %s%s%s %s%-10s%s  %s%s%s  %s\n", color, icon, s.Reset, color, status, s.Reset, s.Dim, id, s.Reset, prompt)
		}
	}
	fmt.Fprintln(w)
	return nil
}

func truncateLegacyText(value string, limit int) string {
	characters := []rune(value)
	if limit < 4 || len(characters) <= limit {
		return value
	}
	return string(characters[:limit-3]) + "..."
}

func legacyWorkflowsOutput(workflows []map[string]interface{}, env Environment) {
	s := env.Style
	w := env.Stdout
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s◈ Workflows%s\n", s.Primary+s.Bold, s.Reset)
	fmt.Fprintln(w)
	if len(workflows) == 0 {
		fmt.Fprintf(w, "  %sNo workflows defined yet%s\n", s.Dim, s.Reset)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %sWorkflows are multi-step agent pipelines.%s\n", s.Dim, s.Reset)
		fmt.Fprintf(w, "  %sCreate them via the API:%s\n", s.Dim, s.Reset)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "    POST /v1/agents/workflows")
		fmt.Fprintf(w, "    %s{\"name\": \"my-workflow\", \"steps\": [...]}%s\n", s.Dim, s.Reset)
	} else {
		for _, wf := range workflows {
			name, _ := wf["name"].(string)
			desc, _ := wf["description"].(string)
			steps, _ := wf["steps"].([]interface{})
			fmt.Fprintf(w, "  %s%s%s", s.Bold, name, s.Reset)
			if len(steps) > 0 {
				fmt.Fprintf(w, "  %s%d steps%s", s.Dim, len(steps), s.Reset)
			}
			fmt.Fprintln(w)
			if desc != "" {
				fmt.Fprintf(w, "    %s%s%s\n", s.Dim, desc, s.Reset)
			}
		}
	}
	fmt.Fprintln(w)
}

func legacyToolsOutput(tools []struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Type        string                 `json:"type"`
}, count int, env Environment) {
	s := env.Style
	w := env.Stdout
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s◈ Agent Tools%s\n", s.Primary+s.Bold, s.Reset)
	fmt.Fprintf(w, "  %s%d tools available for agent use%s\n", s.Dim, count, s.Reset)
	fmt.Fprintln(w)
	if len(tools) == 0 {
		fmt.Fprintf(w, "  %sNo tools available%s\n", s.Dim, s.Reset)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %sAdd MCP tools:%s offgrid agent mcp add <url>\n", s.Dim, s.Reset)
	} else {
		for _, typ := range []string{"builtin", "mcp"} {
			group := make([]struct{ Name, Description string }, 0)
			for _, tool := range tools {
				isMCP := tool.Type == "mcp" || tool.Type == "external"
				if (typ == "mcp") == isMCP {
					group = append(group, struct{ Name, Description string }{tool.Name, tool.Description})
				}
			}
			if len(group) == 0 {
				continue
			}
			if typ == "mcp" {
				fmt.Fprintln(w)
			}
			title := "Built-in Tools"
			if typ == "mcp" {
				title = "MCP Tools"
			}
			fmt.Fprintf(w, "  %s%s%s\n", s.Dim, title, s.Reset)
			short := map[string]string{"read_file": "Read file contents", "write_file": "Write to a file", "list_files": "List directory contents", "shell": "Execute shell commands", "http_get": "Make HTTP GET requests", "current_time": "Get current date/time", "calculator": "Evaluate math expressions", "search": "Search the web", "memory": "Store/retrieve info"}
			for _, tool := range group {
				desc := tool.Description
				if typ == "builtin" {
					if value, ok := short[tool.Name]; ok {
						desc = value
					}
				}
				if len(desc) > 40 {
					desc = desc[:37]
					if idx := strings.LastIndex(desc, " "); idx > 20 {
						desc = desc[:idx]
					}
				}
				fmt.Fprintf(w, "    %-16s  %s%s%s\n", tool.Name, s.Dim, desc, s.Reset)
			}
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sAdd MCP tools:%s offgrid agent mcp add <url>\n", s.Dim, s.Reset)
	fmt.Fprintln(w)
}

func legacyAgentTemplates(env Environment) error {
	s := env.Style
	w := env.Stdout
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s◈ Agent Templates%s\n", s.Primary+s.Bold, s.Reset)
	fmt.Fprintf(w, "  %sPre-configured AI personas with specialized system prompts%s\n", s.Dim, s.Reset)
	fmt.Fprintln(w)
	for _, t := range agents.BuiltInTemplates() {
		fmt.Fprintf(w, "    %s%s %s%s\n", t.Icon, s.Bold, t.Name, s.Reset)
		fmt.Fprintf(w, "       %sID:%s %s%s%s\n", s.Dim, s.Reset, s.Primary, t.ID, s.Reset)
		fmt.Fprintf(w, "       %s%s%s\n", s.Dim, t.Description, s.Reset)
		fmt.Fprintf(w, "       %sStyle:%s %s  %sTools:%s %s\n", s.Dim, s.Reset, t.Config.ReasoningStyle, s.Dim, s.Reset, strings.Join(t.Tools, ", "))
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "  %sUsage:%s offgrid agent run \"your task\" --template researcher --model llama3\n", s.Dim, s.Reset)
	fmt.Fprintln(w)
	return nil
}
