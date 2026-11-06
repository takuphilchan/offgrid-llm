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
	p := Presenter{Stdout: env.Stdout, Style: env.Style}
	p.Heading("OffGrid / Recent tasks")
	if len(tasks) == 0 {
		p.Note("No tasks yet. Start with offgrid agent, or offgrid agent run TASK --model MODEL.")
	} else {
		for _, task := range tasks {
			id, _ := task["id"].(string)
			status, _ := task["status"].(string)
			prompt, _ := task["prompt"].(string)
			p.Heading(truncateLegacyText(terminalLine(prompt), 120))
			p.Detail("Run", id)
			p.Detail("Status", status)
		}
		p.Note("Inspect a run: offgrid agent status RUN_ID")
	}
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
	p := Presenter{Stdout: env.Stdout, Style: env.Style}
	p.Heading("OffGrid / Agent tools")
	p.Note(fmt.Sprintf("%d tools reported. Availability in a run depends on its profile, permissions and connected resources.", count))
	for _, group := range []string{"Built-in tools", "MCP tools"} {
		rows := []presentationRow{}
		for _, tool := range tools {
			external := tool.Type == "mcp" || tool.Type == "external"
			if external == (group == "MCP tools") {
				rows = append(rows, presentationRow{tool.Name, tool.Description})
			}
		}
		if len(rows) > 0 {
			p.Heading(group)
			p.rows(rows)
		}
	}
	if len(tools) == 0 {
		p.Note("No tools available.")
	}
	p.Note("Manage connections: offgrid agent mcp help")
}

func legacyAgentTemplates(env Environment) error {
	p := Presenter{Stdout: env.Stdout, Style: env.Style}
	p.Heading("OffGrid / Agent personas")
	p.Note("Reference personas for reusable profiles. Task commands do not apply --template.")
	for _, t := range agents.BuiltInTemplates() {
		p.Heading(t.Name)
		p.Detail("ID", t.ID)
		p.Note(t.Description)
		p.Detail("Style", t.Config.ReasoningStyle)
		p.Detail("Tools", strings.Join(t.Tools, ", "))
	}
	p.Note("Configure a reusable agent: offgrid agent profiles help")
	return nil
}
