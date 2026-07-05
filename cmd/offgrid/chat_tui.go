package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

type ChatRuntime struct {
	ModelName        string
	ResolvedModel    string
	ServerPort       int
	Client           *http.Client
	UseKnowledgeBase bool
	ImagePath        string
	Messages         []ChatMessage
	SessionMgr       *sessions.SessionManager
	CurrentSession   *sessions.Session
}

func (rt *ChatRuntime) Submit(input string) (string, error) {
	var userContent interface{} = input

	if rt.ImagePath != "" {
		imageData, err := os.ReadFile(rt.ImagePath)
		if err != nil {
			return "", fmt.Errorf("failed to read image file: %w", err)
		}

		base64Image := base64.StdEncoding.EncodeToString(imageData)
		mimeType := "image/jpeg"
		switch strings.ToLower(filepath.Ext(rt.ImagePath)) {
		case ".png":
			mimeType = "image/png"
		case ".webp":
			mimeType = "image/webp"
		case ".gif":
			mimeType = "image/gif"
		}

		userContent = []map[string]interface{}{
			{"type": "text", "text": input},
			{"type": "image_url", "image_url": map[string]string{
				"url": fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image),
			}},
		}
		rt.ImagePath = ""
	}

	userMessage := ChatMessage{Role: "user", Content: userContent}
	requestMessages := append(append([]ChatMessage{}, rt.Messages...), userMessage)

	reqBody := ChatCompletionRequest{
		Model:            rt.ModelName,
		Messages:         requestMessages,
		Stream:           true,
		UseKnowledgeBase: rt.UseKnowledgeBase,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("http://localhost:%d/v1/chat/completions", rt.ServerPort)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := rt.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, msg)
	}

	scanner := bufio.NewScanner(resp.Body)
	var assistantMsg strings.Builder
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
		if len(chunk.Choices) == 0 {
			continue
		}
		if token, ok := chunk.Choices[0].Delta.Content.(string); ok {
			assistantMsg.WriteString(token)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("stream interrupted: %w", err)
	}

	assistantText := strings.TrimSpace(assistantMsg.String())
	if assistantText == "" {
		return "", fmt.Errorf("no response text received")
	}

	rt.Messages = append(rt.Messages, userMessage, ChatMessage{
		Role:    "assistant",
		Content: assistantText,
	})

	if rt.CurrentSession != nil {
		rt.CurrentSession.AddMessage("user", input)
		rt.CurrentSession.AddMessage("assistant", assistantText)
		if err := rt.SessionMgr.Save(rt.CurrentSession); err != nil {
			return assistantText, fmt.Errorf("response received, but failed to save session: %w", err)
		}
	}

	return assistantText, nil
}

func (rt *ChatRuntime) Clear() error {
	rt.Messages = []ChatMessage{}
	rt.ImagePath = ""
	if rt.CurrentSession != nil {
		rt.CurrentSession.Messages = []sessions.Message{}
		return rt.SessionMgr.Save(rt.CurrentSession)
	}
	return nil
}

func (rt *ChatRuntime) StatusText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Model: %s\n", rt.ResolvedModel)
	fmt.Fprintf(&b, "Messages: %d (%d user, %d assistant)\n", len(rt.Messages), (len(rt.Messages)+1)/2, len(rt.Messages)/2)
	if rt.CurrentSession != nil {
		fmt.Fprintf(&b, "Session: %s\n", rt.CurrentSession.Name)
	}
	if rt.UseKnowledgeBase {
		b.WriteString("RAG: enabled\n")
	} else {
		b.WriteString("RAG: disabled\n")
	}
	if sysInfo, err := resource.DetectResources(); err == nil {
		if sysInfo.GPUAvailable {
			fmt.Fprintf(&b, "GPU: %s\n", sysInfo.GPUName)
		} else {
			fmt.Fprintf(&b, "Device: CPU (%d cores)\n", sysInfo.CPUCores)
		}
	}
	return strings.TrimSpace(b.String())
}

type chatResponseMsg struct {
	input    string
	response string
	err      error
}

type chatTUIModel struct {
	rt         *ChatRuntime
	input      textinput.Model
	viewport   viewport.Model
	spinner    spinner.Model
	lines      []string
	busy       bool
	width      int
	height     int
	statusText string
}

var (
	chatTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	chatMuteStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	chatUserStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	chatAIStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	chatErrStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

func newChatTUIModel(rt *ChatRuntime) chatTUIModel {
	ti := textinput.New()
	ti.Placeholder = "Ask locally... (/help for commands)"
	ti.Focus()
	ti.CharLimit = 8000
	ti.Width = 80

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	m := chatTUIModel{
		rt:         rt,
		input:      ti,
		viewport:   viewport.New(80, 20),
		spinner:    sp,
		statusText: "Ready",
	}

	if len(rt.Messages) > 0 {
		m.appendSystem("Loaded previous conversation")
		for _, msg := range rt.Messages {
			if msg.Role == "user" {
				m.appendUser(msg.StringContent())
			} else {
				m.appendAssistant(msg.StringContent())
			}
		}
	} else {
		m.appendSystem("Type /help for commands. Use Ctrl+C or /exit to leave.")
	}
	if rt.ImagePath != "" {
		m.appendSystem(fmt.Sprintf("Image attached: %s", filepath.Base(rt.ImagePath)))
	}
	return m
}

func (m chatTUIModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m chatTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = maxInt(20, msg.Width)
		m.viewport.Height = maxInt(6, msg.Height-8)
		m.input.Width = maxInt(20, msg.Width-4)
		m.refreshViewport()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if m.busy {
				return m, nil
			}
			input := strings.TrimSpace(m.input.Value())
			m.input.SetValue("")
			if input == "" {
				return m, nil
			}
			if strings.HasPrefix(input, "/") {
				cmd := strings.TrimPrefix(input, "/")
				return m.handleCommand(cmd)
			}
			m.appendUser(input)
			m.busy = true
			m.statusText = "Thinking..."
			return m, tea.Batch(m.spinner.Tick, submitChatCmd(m.rt, input))
		}

	case spinner.TickMsg:
		if m.busy {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case chatResponseMsg:
		m.busy = false
		m.statusText = "Ready"
		if msg.err != nil {
			m.appendError(msg.err.Error())
		} else {
			m.appendAssistant(msg.response)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m chatTUIModel) View() string {
	header := chatTitleStyle.Render("OffGrid Chat") + " " + chatMuteStyle.Render(m.rt.ResolvedModel)
	if m.rt.UseKnowledgeBase {
		header += " " + chatMuteStyle.Render("RAG:on")
	}

	status := m.statusText
	if m.busy {
		status = m.spinner.View() + " " + status
	}

	footer := chatMuteStyle.Render(status + "  /help /status /clear /rag /exit")
	return header + "\n\n" + m.viewport.View() + "\n\n" + m.input.View() + "\n" + footer
}

func (m chatTUIModel) handleCommand(cmd string) (tea.Model, tea.Cmd) {
	switch strings.ToLower(strings.TrimSpace(cmd)) {
	case "exit", "quit", "q":
		return m, tea.Quit
	case "help", "?":
		m.appendSystem("Commands: /help, /status, /clear, /rag, /exit")
	case "status":
		m.appendSystem(m.rt.StatusText())
	case "clear":
		if err := m.rt.Clear(); err != nil {
			m.appendError(fmt.Sprintf("failed to clear session: %v", err))
		} else {
			m.lines = nil
			m.appendSystem("Conversation cleared")
		}
	case "rag":
		m.rt.UseKnowledgeBase = !m.rt.UseKnowledgeBase
		if m.rt.UseKnowledgeBase {
			m.appendSystem("Knowledge Base enabled")
		} else {
			m.appendSystem("Knowledge Base disabled")
		}
	default:
		m.appendError("unknown command: /" + cmd)
	}
	return m, nil
}

func (m *chatTUIModel) appendUser(text string) {
	m.appendLine(chatUserStyle.Render("You") + "\n" + text)
}

func (m *chatTUIModel) appendAssistant(text string) {
	m.appendLine(chatAIStyle.Render("OffGrid") + "\n" + text)
}

func (m *chatTUIModel) appendSystem(text string) {
	m.appendLine(chatMuteStyle.Render(text))
}

func (m *chatTUIModel) appendError(text string) {
	m.appendLine(chatErrStyle.Render("[error] " + text))
}

func (m *chatTUIModel) appendLine(text string) {
	m.lines = append(m.lines, text)
	m.refreshViewport()
}

func (m *chatTUIModel) refreshViewport() {
	m.viewport.SetContent(strings.Join(m.lines, "\n\n"))
	m.viewport.GotoBottom()
}

func submitChatCmd(rt *ChatRuntime, input string) tea.Cmd {
	return func() tea.Msg {
		response, err := rt.Submit(input)
		return chatResponseMsg{input: input, response: response, err: err}
	}
}

func runChatTUI(rt *ChatRuntime) error {
	program := tea.NewProgram(newChatTUIModel(rt), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func shouldUseChatTUI() bool {
	if os.Getenv("OFFGRID_PLAIN") == "1" || strings.EqualFold(os.Getenv("OFFGRID_TUI"), "0") {
		return false
	}
	stdin, err := os.Stdin.Stat()
	if err != nil || stdin.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	stdout, err := os.Stdout.Stat()
	return err == nil && stdout.Mode()&os.ModeCharDevice != 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
