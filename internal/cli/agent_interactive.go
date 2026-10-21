package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// runInteractiveAgent keeps one terminal session while each prompt is
// submitted through the existing authenticated durable-task path.
func runInteractiveAgent(ctx context.Context, modelName string, env Environment) error {
	cfg, err := localConfig(env)
	if err != nil {
		return err
	}
	presenter := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, Style: env.Style}
	if modelName == "" {
		fmt.Fprintln(env.Stdout, "Connecting to server to fetch models...")
		available, err := fetchInteractiveAgentModels(ctx, cfg.ServerPort, env.HTTP)
		if err != nil {
			presenter.Error(fmt.Sprintf("Failed to fetch models: %v", err))
			fmt.Fprintln(env.Stdout, "Make sure the server is running: offgrid serve")
			return nil // Legacy interactive command reported this condition but exited 0.
		}
		if len(available) == 0 {
			presenter.Error("No models found. Please download a model first using: offgrid download")
			return nil
		}
		modelName = available[0]
		for _, candidate := range available {
			name := strings.ToLower(candidate)
			if strings.Contains(name, "instruct") || strings.Contains(name, "chat") {
				modelName = candidate
				break
			}
		}
	}

	presenter.SectionHeader("Agent workspace")
	presenter.KeyValue("Model", modelName)
	presenter.KeyValue("Mode", "governed task runner")
	fmt.Fprintf(env.Stdout, "  %sEach prompt creates a durable task. Type /help for commands.%s\n\n", env.Style.Muted, env.Style.Reset)

	scanner := bufio.NewScanner(env.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		fmt.Fprintf(env.Stdout, "%s%s%s ", env.Style.Primary, env.Style.Chevron, env.Style.Reset)
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		switch strings.ToLower(input) {
		case "exit", "quit", "/exit", "/quit":
			fmt.Fprintf(env.Stdout, "\n%sSession closed.%s\n", env.Style.Muted, env.Style.Reset)
			return nil
		case "clear", "/clear":
			fmt.Fprint(env.Stdout, "\033[H\033[2J")
			continue
		case "help", "/help", "?":
			fmt.Fprintf(env.Stdout, "\n  %s/help%s   Show commands\n", env.Style.Primary, env.Style.Reset)
			fmt.Fprintf(env.Stdout, "  %s/clear%s  Clear the terminal\n", env.Style.Primary, env.Style.Reset)
			fmt.Fprintf(env.Stdout, "  %s/exit%s   Close the workspace\n\n", env.Style.Primary, env.Style.Reset)
			continue
		}
		args := []string{"--model", modelName, "--style", "react", "--max-steps", strconv.Itoa(10), "--wait", "--", input}
		if err := env.LegacyAgent.SubmitTask(ctx, args); err != nil {
			presenter.Error(err.Error())
		}
		fmt.Fprintln(env.Stdout)
	}
	if err := scanner.Err(); err != nil {
		presenter.Error(fmt.Sprintf("Input failed: %v", err))
	}
	return ctx.Err()
}

func fetchInteractiveAgentModels(ctx context.Context, port int, client *http.Client) ([]string, error) {
	endpoint := fmt.Sprintf("http://localhost:%d/v1/models", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(response.Data))
	for _, model := range response.Data {
		models = append(models, model.ID)
	}
	return models, nil
}
