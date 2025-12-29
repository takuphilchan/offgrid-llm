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
			if strings.Contains(m, "instruct") || strings.Contains(m, "chat") {
				selected = m
				break
			}
		}
		modelName = selected
	}

	// Clear screen
	fmt.Print("\033[H\033[2J")

	// Print Header
	fmt.Printf("%s%s OffGrid Agent CLI %s%s\n", brandPrimary+colorBold, iconBolt, Version, colorReset)
	fmt.Printf("%sInteractive Session • Model: %s%s\n", colorDim, modelName, colorReset)
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	serverURL := fmt.Sprintf("http://localhost:%d/v1/agents/run", cfg.ServerPort)

	// History context (simple session memory)
	// In a real implementation, the server handles memory, but we might want to keep track locally if needed.
	// For now, we rely on the server's session management if we pass a session ID,
	// but the current /v1/agents/run endpoint might be stateless per request unless we update it.
	// Let's assume for now each run is independent or the server handles context if we implement it.
	// Actually, the current agent implementation in server.go creates a NEW agent for each request.
	// To support chat, we need to persist the agent or pass history.
	// For this "futuristic" demo, we'll stick to single-turn or assume the server is updated later for multi-turn.
	// But to make it useful, let's just run the loop.

	for {
		// Prompt
		fmt.Printf("\n%sYou%s\n%s>%s ", colorBold, colorReset, brandPrimary, colorReset)

		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			break
		}
		if input == "clear" {
			fmt.Print("\033[H\033[2J")
			continue
		}

		// Run Agent
		runAgentRequest(serverURL, input, modelName, "react", 10)
	}
}

func runAgentRequest(url, prompt, model, style string, maxSteps int) {
	reqBody := map[string]interface{}{
		"prompt":    prompt,
		"model":     model,
		"style":     style,
		"max_steps": maxSteps,
		"stream":    true,
	}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		printError(fmt.Sprintf("Connection failed: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errResp map[string]interface{}
		if err := json.Unmarshal(body, &errResp); err == nil {
			if e, ok := errResp["error"].(string); ok {
				printError(fmt.Sprintf("Server error: %s", e))
				return
			}
		}
		printError(fmt.Sprintf("Server error (%d): %s", resp.StatusCode, string(body)))
		return
	}

	// Spinner for initial connection/thinking
	stopSpinner := make(chan bool)
	go showSpinner("Thinking...", stopSpinner)

	reader := bufio.NewReader(resp.Body)

	// State tracking
	isSpinnerRunning := true
	var lineBuffer strings.Builder
	suppressLine := false
	isStartOfLine := true

	fmt.Printf("\n%sAgent%s\n", brandPrimary, colorReset)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		eventType, _ := event["type"].(string)

		switch eventType {
		case "token":
			token, _ := event["token"].(string)

			if isSpinnerRunning {
				stopSpinner <- true
				isSpinnerRunning = false
				fmt.Printf("\r\033[K")
			}

			// Process token character by character to handle filtering
			for _, char := range token {
				if char == '\n' {
					if !suppressLine {
						if lineBuffer.Len() > 0 {
							fmt.Printf("%s%s%s", brandPrimary, lineBuffer.String(), colorReset)
						}
						fmt.Print("\n")
					}
					lineBuffer.Reset()
					suppressLine = false
					isStartOfLine = true
					continue
				}

				lineBuffer.WriteRune(char)

				if suppressLine {
					continue
				}

				if isStartOfLine {
					currentStr := lineBuffer.String()
					forbidden := []string{"Action:", "Action Input:", "Observation:"}

					matchedPrefix := false
					fullMatch := false

					for _, prefix := range forbidden {
						if strings.HasPrefix(prefix, currentStr) {
							matchedPrefix = true
							if prefix == currentStr {
								fullMatch = true
							}
							break
						}
					}

					if fullMatch {
						suppressLine = true
						continue
					}

					if matchedPrefix {
						// Wait for more chars
						continue
					}
				}

				// Safe to print
				fmt.Printf("%s%s%s", brandPrimary, lineBuffer.String(), colorReset)
				lineBuffer.Reset()
				isStartOfLine = false
			}

		case "step":
			// Step event confirms what happened
			stepType, _ := event["step_type"].(string)

			if isSpinnerRunning {
				stopSpinner <- true
				isSpinnerRunning = false
				fmt.Printf("\r\033[K")
			}

			if stepType == "tool_use" {
				toolName, _ := event["tool"].(string)
				toolArgs, _ := event["args"].(string)
				fmt.Printf("\r\033[K") // Clear any spinner line
				fmt.Printf("%s🛠  Using %s%s\n", colorYellow, toolName, colorReset)
				if len(toolArgs) > 0 {
					// Truncate args if too long
					if len(toolArgs) > 60 {
						toolArgs = toolArgs[:57] + "..."
					}
					fmt.Printf("    %s%s%s\n", colorDim, toolArgs, colorReset)
				}

				// Tool execution takes time
				stopSpinner = make(chan bool)
				go showSpinner("Running tool...", stopSpinner)
				isSpinnerRunning = true

			} else if stepType == "tool_result" {
				result, _ := event["result"].(string)
				if len(result) > 100 {
					result = result[:100] + "..."
				}
				fmt.Printf("\r\033[K") // Clear any spinner line
				fmt.Printf("%s✓  Result: %s%s\n", colorGreen, result, colorReset)

				// Back to thinking
				stopSpinner = make(chan bool)
				go showSpinner("Thinking...", stopSpinner)
				isSpinnerRunning = true
			}

		case "error":
			if isSpinnerRunning {
				stopSpinner <- true
				isSpinnerRunning = false
				fmt.Printf("\r\033[K")
			}
			errMsg, _ := event["error"].(string)
			fmt.Printf("\n%sError: %s%s\n", brandError, errMsg, colorReset)

		case "done":
			// Flush remaining buffer
			if !suppressLine && lineBuffer.Len() > 0 {
				fmt.Printf("%s%s%s", brandPrimary, lineBuffer.String(), colorReset)
			}

			// Ensure spinner is stopped
			if isSpinnerRunning {
				stopSpinner <- true
				isSpinnerRunning = false
				fmt.Printf("\r\033[K")
			}
		}
	}

	if isSpinnerRunning {
		stopSpinner <- true
	}

	fmt.Println()
}

func showSpinner(msg string, stop chan bool) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-stop:
			return
		default:
			fmt.Printf("\r%s %s %s", brandPrimary, frames[i%len(frames)], msg)
			time.Sleep(100 * time.Millisecond)
			i++
		}
	}
}

func printAgentHelp() {
	fmt.Printf("%sOffGrid Agent Commands:%s\n", colorBold, colorReset)
	fmt.Println()
	fmt.Printf("  %soffgrid agent [chat]%s           Start interactive agent session (default)\n", brandPrimary, colorReset)
	fmt.Printf("  %soffgrid agent run <prompt>%s     Run a single agent task\n", brandPrimary, colorReset)
	fmt.Printf("  %soffgrid agent templates%s        List pre-built agent personas\n", brandPrimary, colorReset)
	fmt.Printf("  %soffgrid agent tools%s            List available tools\n", brandPrimary, colorReset)
	fmt.Printf("  %soffgrid agent mcp%s              Manage MCP servers (add/list/remove)\n", brandPrimary, colorReset)
	fmt.Printf("  %soffgrid agent list%s             List active agents\n", brandPrimary, colorReset)
	fmt.Println()
	fmt.Printf("%sOptions:%s\n", colorBold, colorReset)
	fmt.Printf("  %s--model <name>%s                 Specify model to use\n", colorCyan, colorReset)
	fmt.Printf("  %s--template <id>%s                Use a pre-built agent template\n", colorCyan, colorReset)
	fmt.Println()
	fmt.Printf("%sTemplates:%s researcher, coder, analyst, writer, sysadmin, planner\n", colorBold, colorReset)
	fmt.Println()
}

func fetchModels(port int) ([]string, error) {
	url := fmt.Sprintf("http://localhost:%d/v1/models", port)
	resp, err := http.Get(url)
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
