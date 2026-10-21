package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

// runPlainChat owns the legacy line-oriented terminal exchange. All I/O is
// supplied by the invocation environment; completed exchanges are saved once.
func runPlainChat(ctx context.Context, env Environment, cfg *config.Config, client *http.Client, resolvedModelName string, useKnowledgeBase bool, imagePath string, messages []ChatMessage, currentSession *sessions.Session, sessionMgr *sessions.SessionManager) error {
	s := env.Style
	brandPrimary, brandMuted, brandSuccess := s.Primary, s.Muted, s.Success
	colorBold, colorReset, colorYellow := s.Bold, s.Reset, s.Accent
	presenter := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, Style: s}
	reader := bufio.NewReader(env.Stdin)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		fmt.Fprintf(env.Stdout, "\n%sYou%s\n> ", brandPrimary+colorBold, colorReset)
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		// Normalize slash commands
		if strings.HasPrefix(input, "/") {
			input = strings.TrimPrefix(input, "/")
		}

		if input == "exit" || input == "quit" || input == "q" {
			fmt.Fprintln(env.Stdout)
			fmt.Fprintf(env.Stdout, "  %sSession ended%s\n\n", brandMuted, colorReset)
			break
		}

		if input == "help" || input == "?" {
			fmt.Fprintln(env.Stdout)
			fmt.Fprintf(env.Stdout, "  %sChat Commands%s\n", brandPrimary+colorBold, colorReset)
			fmt.Fprintln(env.Stdout)
			fmt.Fprintf(env.Stdout, "    %s/help%s     Show this help\n", colorBold, colorReset)
			fmt.Fprintf(env.Stdout, "    %s/status%s   Show model, session, and device info\n", colorBold, colorReset)
			fmt.Fprintf(env.Stdout, "    %s/clear%s    Clear screen and conversation history\n", colorBold, colorReset)
			fmt.Fprintf(env.Stdout, "    %s/rag%s      Toggle knowledge base retrieval\n", colorBold, colorReset)
			fmt.Fprintf(env.Stdout, "    %s/exit%s     Exit the chat\n", colorBold, colorReset)
			fmt.Fprintln(env.Stdout)
			fmt.Fprintf(env.Stdout, "  %sAliases:%s exit, quit, q, help, status, clear, rag\n", brandMuted, colorReset)
			continue
		}

		if input == "status" {
			fmt.Fprintln(env.Stdout)
			fmt.Fprintf(env.Stdout, "  %sSession Status%s\n", brandPrimary+colorBold, colorReset)
			fmt.Fprintln(env.Stdout)
			fmt.Fprintf(env.Stdout, "    %sModel:%s       %s\n", brandMuted, colorReset, resolvedModelName)
			fmt.Fprintf(env.Stdout, "    %sMessages:%s    %d (%d user, %d assistant)\n", brandMuted, colorReset, len(messages), (len(messages)+1)/2, len(messages)/2)
			if currentSession != nil {
				fmt.Fprintf(env.Stdout, "    %sSession:%s     %s\n", brandMuted, colorReset, currentSession.Name)
			}
			fmt.Fprintf(env.Stdout, "    %sRAG:%s         %s\n", brandMuted, colorReset, func() string {
				if useKnowledgeBase {
					return brandSuccess + "enabled" + colorReset
				}
				return "disabled"
			}())
			if sysInfo, err := env.Interactive.DetectResources(); err == nil {
				if sysInfo.GPUAvailable {
					fmt.Fprintf(env.Stdout, "    %sGPU:%s         %s%s%s\n", brandMuted, colorReset, brandSuccess, sysInfo.GPUName, colorReset)
				} else {
					fmt.Fprintf(env.Stdout, "    %sDevice:%s      CPU (%d cores)\n", brandMuted, colorReset, sysInfo.CPUCores)
				}
			}
			fmt.Fprintln(env.Stdout)
			continue
		}

		if input == "clear" {
			messages = []ChatMessage{}
			// Clear session too if active
			if currentSession != nil {
				currentSession.Messages = []sessions.Message{}
				if err := sessionMgr.Save(currentSession); err != nil {
					fmt.Fprintf(env.Stdout, "  %sWarning: Failed to save cleared session: %v%s\n", colorYellow, err, colorReset)
				}
			}
			// Clear image path so it doesn't get re-attached
			imagePath = ""
			// Clear the terminal screen
			fmt.Fprint(env.Stdout, "\033[2J\033[H")
			// Reprint the header
			printRunHeader(env, resolvedModelName, useKnowledgeBase, "")
			fmt.Fprintf(env.Stdout, "  %sStatus:%s  %sReady%s\n", brandMuted, colorReset, brandSuccess, colorReset)
			if sysInfo, err := env.Interactive.DetectResources(); err == nil {
				if sysInfo.GPUAvailable {
					fmt.Fprintf(env.Stdout, "  %sGPU:%s     %s%s%s\n", brandMuted, colorReset, brandSuccess, sysInfo.GPUName, colorReset)
				}
			}
			fmt.Fprintln(env.Stdout)
			printRunCommands(env)
			fmt.Fprintf(env.Stdout, "\n  %sConversation cleared%s\n", brandMuted, colorReset)
			continue
		}

		if input == "rag" {
			useKnowledgeBase = !useKnowledgeBase
			if useKnowledgeBase {
				fmt.Fprintf(env.Stdout, "\n  %sKnowledge Base enabled%s\n", brandSuccess, colorReset)
			} else {
				fmt.Fprintf(env.Stdout, "\n  %sKnowledge Base disabled%s\n", brandMuted, colorReset)
			}
			continue
		}

		// Build the next user message. Only commit it to history after a successful response.
		var userContent interface{} = input

		// If we have an image pending, attach it to this message
		if imagePath != "" {
			// Read image file again (or we could have cached the dataURI)
			imageData, err := env.ReadFile(imagePath)
			if err == nil {
				base64Image := base64.StdEncoding.EncodeToString(imageData)
				mimeType := "image/jpeg"
				if strings.HasSuffix(strings.ToLower(imagePath), ".png") {
					mimeType = "image/png"
				} else if strings.HasSuffix(strings.ToLower(imagePath), ".webp") {
					mimeType = "image/webp"
				} else if strings.HasSuffix(strings.ToLower(imagePath), ".gif") {
					mimeType = "image/gif"
				}

				dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

				// Create multimodal content
				userContent = []map[string]interface{}{
					{
						"type": "text",
						"text": input,
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": dataURI,
						},
					},
				}

				// Clear image path so it's only sent once
				imagePath = ""
			} else {
				presenter.Error(fmt.Sprintf("Failed to read image file: %v", err))
			}
		}

		userMessage := ChatMessage{
			Role:    "user",
			Content: userContent,
		}
		requestMessages := append(append([]ChatMessage{}, messages...), userMessage)

		// Make API request
		reqBody := ChatCompletionRequest{
			Model:            resolvedModelName,
			Messages:         requestMessages,
			Stream:           true,
			UseKnowledgeBase: useKnowledgeBase,
		}

		jsonData, _ := json.Marshal(reqBody)
		apiURL := fmt.Sprintf("http://localhost:%d/v1/chat/completions", cfg.ServerPort)

		req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Fprintln(env.Stdout)
			presenter.Error(fmt.Sprintf("Error creating request: %v", err))
			fmt.Fprintln(env.Stdout)
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintln(env.Stdout)
			presenter.Error(fmt.Sprintf("Request failed: %v", err))
			fmt.Fprintln(env.Stdout)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			fmt.Fprintln(env.Stdout)
			presenter.Error(fmt.Sprintf("Request failed with status %d", resp.StatusCode))
			if len(strings.TrimSpace(string(body))) > 0 {
				fmt.Fprintf(env.Stdout, "  %s%s%s\n", brandMuted, strings.TrimSpace(string(body)), colorReset)
			}
			continue
		}

		// Handle streaming response
		fmt.Fprintf(env.Stdout, "\n%sOffGrid%s\n", brandSuccess+colorBold, colorReset)
		scanner := bufio.NewScanner(resp.Body)
		var assistantMsg strings.Builder
		lineLength := 0

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk ChatCompletionChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 {
				tokenInterface := chunk.Choices[0].Delta.Content
				var token string
				if str, ok := tokenInterface.(string); ok {
					token = str
				}
				fmt.Fprint(env.Stdout, token)
				syncInteractiveOutput(env) // Flush output immediately for streaming
				assistantMsg.WriteString(token)
				lineLength += len(token)

				// Wrap long lines
				if lineLength > 80 && strings.Contains(token, " ") {
					lineLength = 0
				}
			}
		}

		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout)

		if err := scanner.Err(); err != nil {
			resp.Body.Close()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			presenter.Error(fmt.Sprintf("Stream interrupted: %v", err))
			fmt.Fprintln(env.Stdout)
			continue
		}
		resp.Body.Close()

		assistantText := strings.TrimSpace(assistantMsg.String())
		if assistantText == "" {
			presenter.Warning("No response text received")
			continue
		}

		// Add the completed exchange to history.
		messages = append(messages, userMessage)
		messages = append(messages, ChatMessage{
			Role:    "assistant",
			Content: assistantText,
		})

		// Save messages to session after the exchange succeeds.
		if currentSession != nil {
			currentSession.AddMessage("user", input)
			currentSession.AddMessage("assistant", assistantText)
			// Auto-save after each exchange
			if err := sessionMgr.Save(currentSession); err != nil {
				// Don't interrupt the conversation, just log the error
				fmt.Fprintf(env.Stdout, "%sFailed to save session: %v%s\n", brandMuted, err, colorReset)
			}
		}
	}
	return nil
}
