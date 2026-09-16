package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
)

// Interactive Agent UI

func startInteractiveAgent(modelName string) {
	cfg := config.LoadConfig()

	// If no model specified, fetch available models
	if modelName == "" {
		fmt.Println("Connecting to server to fetch models...")
		models, err := fetchModels(cfg.ServerPort)
		if err != nil {
			printError(fmt.Sprintf("Failed to fetch models: %v", err))
			fmt.Println("Make sure the server is running: offgrid serve")
			return
		}
		if len(models) == 0 {
			printError("No models found. Please download a model first using: offgrid download")
			return
		}
		// Prefer a model with "instruct" or "chat" in the name if multiple exist
		selected := models[0]
		for _, m := range models {
			candidate := strings.ToLower(m)
			if strings.Contains(candidate, "instruct") || strings.Contains(candidate, "chat") {
				selected = m
				break
			}
		}
		modelName = selected
	}

	printSectionHeader("Agent workspace")
	printKeyValue("Model", modelName)
	printKeyValue("Mode", "governed task runner")
	fmt.Printf("  %sEach prompt creates a durable task. Type /help for commands.%s\n\n", brandMuted, colorReset)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	serverURL := fmt.Sprintf("http://localhost:%d/v1/agents/run", cfg.ServerPort)

	for {
		fmt.Printf("%s%s%s ", brandPrimary, iconChevron, colorReset)

		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			continue
		}
		switch strings.ToLower(input) {
		case "exit", "quit", "/exit", "/quit":
			fmt.Printf("\n%sSession closed.%s\n", brandMuted, colorReset)
			return
		case "clear", "/clear":
			fmt.Print("\033[H\033[2J")
			continue
		case "help", "/help", "?":
			fmt.Printf("\n  %s/help%s   Show commands\n", brandPrimary, colorReset)
			fmt.Printf("  %s/clear%s  Clear the terminal\n", brandPrimary, colorReset)
			fmt.Printf("  %s/exit%s   Close the workspace\n\n", brandPrimary, colorReset)
			continue
		}

		runAgentRequest(serverURL, input, modelName, "react", 10)
		fmt.Println()
	}
	if err := scanner.Err(); err != nil {
		printError(fmt.Sprintf("Input failed: %v", err))
	}
}

func runAgentRequest(url, prompt, model, style string, maxSteps int) {
	jsonBody, _ := json.Marshal(map[string]any{
		"prompt": prompt, "model": model, "style": style,
		"max_steps": maxSteps, "stream": true,
	})
	resp, err := agentRequest(httpClientLong, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		printError(err.Error())
		return
	}
	defer resp.Body.Close()
	if err := renderAgentStream(os.Stdout, resp.Body); err != nil {
		printError(err.Error())
	}
}

// The durable runner streams committed steps and state, not speculative model
// tokens. Losing this connection does not cancel work or resubmit the prompt.
func renderAgentStream(w io.Writer, body io.Reader) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	runID := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var event struct {
			agentRunSnapshot
			Type       string `json:"type"`
			StepType   string `json:"step_type"`
			ToolName   string `json:"tool_name"`
			ToolResult string `json:"tool_result"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &event); err != nil {
			return fmt.Errorf("invalid agent progress; inspect the saved run before retrying: %w", err)
		}
		if runID == "" && event.RunID != "" {
			runID = event.RunID
			fmt.Fprintf(w, "Run: %s\n", terminalSafe(runID))
		}
		switch event.Type {
		case "step":
			if event.ToolName != "" {
				fmt.Fprintf(w, "  %s · %s\n", terminalSafe(event.ToolName), truncateTerminalText(terminalSafe(event.ToolResult), 180))
			}
		case "done", "approval_required", "error":
			renderAgentSnapshot(w, event.agentRunSnapshot)
			if event.Type == "error" {
				return fmt.Errorf("agent run stopped; use offgrid agent status %s before taking further action", terminalSafe(runID))
			}
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("progress disconnected; run %s may still be active: %w", terminalSafe(runID), err)
	}
	return fmt.Errorf("progress ended before a terminal state; inspect with offgrid agent status %s", terminalSafe(runID))
}

func showSpinner(msg string, stop chan bool) {
	i := 0
	for {
		select {
		case <-stop:
			return
		default:
			fmt.Printf("\r%s%s%s %s", brandPrimary, spinnerFrames[i%len(spinnerFrames)], colorReset, msg)
			time.Sleep(100 * time.Millisecond)
			i++
		}
	}
}

func printAgentHelp() {
	printSectionHeader("Agent commands")
	fmt.Println("  offgrid agent status RUN_ID                 Inspect a saved run")
	fmt.Println("  offgrid agent approve RUN_ID APPROVAL_ID     Approve one pending call")
	fmt.Println("  offgrid agent deny RUN_ID APPROVAL_ID        Deny a pending call")
	fmt.Println("  offgrid agent cancel RUN_ID                 Stop work (does not undo tools)")
	fmt.Println("  offgrid agent resume RUN_ID                 Resume a safe checkpoint")
	fmt.Println("  offgrid agent reconcile RUN_ID CALL_ID RESULT  Record a verified outcome")
	fmt.Printf("  %soffgrid agent [chat]%s           Start interactive agent session (default)\n", brandPrimary, colorReset)
	fmt.Printf("  %soffgrid agent run <prompt>%s     Run a single agent task\n", brandPrimary, colorReset)
	fmt.Printf("  %soffgrid agent templates%s        List pre-built agent personas\n", brandPrimary, colorReset)
	fmt.Printf("  %soffgrid agent tools%s            List available tools\n", brandPrimary, colorReset)
	fmt.Printf("  %soffgrid agent mcp%s              Manage MCP servers (add/list/remove)\n", brandPrimary, colorReset)
	fmt.Printf("  %soffgrid agent list%s             List active agents\n", brandPrimary, colorReset)
	fmt.Println()
	fmt.Printf("%sOptions:%s\n", colorBold, colorReset)
	fmt.Printf("  %s--model <name>%s                 Specify model to use\n", colorCyan, colorReset)
	fmt.Printf("  %s--style react|cot|plan%s         Instruction style for agent run\n", brandPrimary, colorReset)
	fmt.Printf("  %s--max-steps <1-50>%s             Bound agent run model iterations\n", brandPrimary, colorReset)
	fmt.Println()
	fmt.Println("  Review available personas with 'offgrid agent templates'; task commands do not apply --template.")
	fmt.Println()
}

func fetchModels(port int) ([]string, error) {
	url := fmt.Sprintf("http://localhost:%d/v1/models", port)
	resp, err := agentRequest(httpClient, http.MethodGet, url, nil)
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

	var models []string
	for _, m := range response.Data {
		models = append(models, m.ID)
	}
	return models, nil
}
