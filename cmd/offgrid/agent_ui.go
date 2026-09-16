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
	reqBody := map[string]interface{}{
		"prompt":    prompt,
		"model":     model,
		"style":     style,
		"max_steps": maxSteps,
		"stream":    true,
	}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := httpClientLong.Post(url, "application/json", bytes.NewBuffer(jsonBody))
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

	fmt.Printf("\n%sOffGrid%s\n", colorBold, colorReset)

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
							fmt.Print(lineBuffer.String())
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
				fmt.Print(lineBuffer.String())
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
				fmt.Printf("%s%s  Using %s%s\n", brandAccent, iconChevron, toolName, colorReset)
				if len(toolArgs) > 0 {
					// Truncate args if too long
					toolArgs = truncateTerminalText(toolArgs, 60)
					fmt.Printf("    %s%s%s\n", colorDim, toolArgs, colorReset)
				}

				// Tool execution takes time
				stopSpinner = make(chan bool)
				go showSpinner("Running tool...", stopSpinner)
				isSpinnerRunning = true

			} else if stepType == "tool_result" {
				result, _ := event["result"].(string)
				result = truncateTerminalText(result, 100)
				fmt.Printf("\r\033[K") // Clear any spinner line
				fmt.Printf("%s%s  Result: %s%s\n", brandSuccess, iconCheck, result, colorReset)

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
				fmt.Print(lineBuffer.String())
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
