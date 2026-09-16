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

func (rt *ChatRuntime) Stream(input string) <-chan chatStreamMsg {
	ch := make(chan chatStreamMsg)
	go func() {
		defer close(ch)

		if err := rt.stream(input, ch); err != nil {
			ch <- chatStreamMsg{err: err}
			return
		}
		ch <- chatStreamMsg{done: true}
	}()
	return ch
}

func (rt *ChatRuntime) stream(input string, ch chan<- chatStreamMsg) error {
	var userContent interface{} = input

	if rt.ImagePath != "" {
		imageData, err := os.ReadFile(rt.ImagePath)
		if err != nil {
			return fmt.Errorf("failed to read image file: %w", err)
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
		return err
	}

	apiURL := fmt.Sprintf("http://localhost:%d/v1/chat/completions", rt.ServerPort)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := rt.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, msg)
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
			ch <- chatStreamMsg{token: token}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream interrupted: %w", err)
	}

	assistantText := strings.TrimSpace(assistantMsg.String())
	if assistantText == "" {
		return fmt.Errorf("%s", embeddingModelChatError(rt.ResolvedModel))
	}

	rt.Messages = append(rt.Messages, userMessage, ChatMessage{
		Role:    "assistant",
		Content: assistantText,
	})

	if rt.CurrentSession != nil {
		rt.CurrentSession.AddMessage("user", input)
		rt.CurrentSession.AddMessage("assistant", assistantText)
		if err := rt.SessionMgr.Save(rt.CurrentSession); err != nil {
			return fmt.Errorf("response received, but failed to save session: %w", err)
		}
	}

	return nil
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

type chatStreamMsg struct {
	token string
	err   error
	done  bool
}

type chatTUIModel struct {
	rt              *ChatRuntime
	input           textinput.Model
	viewport        viewport.Model
	spinner         spinner.Model
	lines           []string
	busy            bool
	width           int
	height          int
	statusText      string
	streamCh        <-chan chatStreamMsg
	streamLineIndex int
	streamText      string
	history         []string
	historyIndex    int
}

var (
	chatTextColor   = lipgloss.AdaptiveColor{Light: terminalLightText, Dark: terminalDarkText}
	chatMutedColor  = lipgloss.AdaptiveColor{Light: terminalLightMuted, Dark: terminalDarkMuted}
	chatFaintColor  = lipgloss.AdaptiveColor{Light: terminalLightFaint, Dark: terminalDarkFaint}
	chatLineColor   = lipgloss.AdaptiveColor{Light: terminalLightLine, Dark: terminalDarkLine}
	chatTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(chatTextColor)
	chatMuteStyle   = lipgloss.NewStyle().Foreground(chatMutedColor)
	chatFaintStyle  = lipgloss.NewStyle().Foreground(chatFaintColor)
	chatUserStyle   = lipgloss.NewStyle().Bold(true).Foreground(chatTextColor)
	chatAIStyle     = lipgloss.NewStyle().Bold(true).Foreground(chatTextColor)
	chatReadyStyle  = lipgloss.NewStyle().Foreground(chatMutedColor)
	chatErrStyle    = lipgloss.NewStyle().Bold(true).Foreground(chatTextColor)
	chatHeaderStyle = lipgloss.NewStyle().Padding(0, 1).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(chatLineColor)
)

func newChatTUIModel(rt *ChatRuntime) chatTUIModel {
	ti := textinput.New()
	ti.Placeholder = "Ask, draft, analyze, or plan..."
	ti.Prompt = "› "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(chatTextColor).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(chatTextColor)
	ti.PlaceholderStyle = chatFaintStyle
	ti.Focus()
	ti.CharLimit = 8000
	ti.Width = 80

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	m := chatTUIModel{
		rt:           rt,
		input:        ti,
		viewport:     viewport.New(80, 20),
		spinner:      sp,
		statusText:   "Ready",
		historyIndex: -1,
	}

	if len(rt.Messages) > 0 {
		m.appendSystem("Loaded previous conversation")
		for _, msg := range rt.Messages {
			if msg.Role == "user" {
				userText := msg.StringContent()
				m.appendUser(userText)
				m.addHistory(userText)
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
			m.addHistory(input)
			if strings.HasPrefix(input, "/") {
				cmd := strings.TrimPrefix(input, "/")
				return m.handleCommand(cmd)
			}
			m.appendUser(input)
			m.streamText = ""
			m.streamLineIndex = m.appendAssistantStream()
			m.streamCh = m.rt.Stream(input)
			m.busy = true
			m.statusText = "Streaming..."
			return m, tea.Batch(m.spinner.Tick, waitForStream(m.streamCh))
		case "up":
			if !m.busy {
				m.previousHistory()
			}
		case "down":
			if !m.busy {
				m.nextHistory()
			}
		}

	case spinner.TickMsg:
		if m.busy {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case chatStreamMsg:
		if msg.err != nil {
			m.busy = false
			m.statusText = "Ready"
			m.removeEmptyAssistantStream()
			m.appendError(msg.err.Error())
			return m, nil
		}
		if msg.token != "" {
			m.streamText += msg.token
			m.updateAssistantStream(m.streamText)
		}
		if msg.done {
			m.busy = false
			m.statusText = "Ready"
			m.streamCh = nil
			m.streamLineIndex = -1
			m.streamText = ""
			return m, nil
		}
		return m, waitForStream(m.streamCh)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m chatTUIModel) View() string {
	header := chatTitleStyle.Render("◆ OffGrid") + "  " + chatMuteStyle.Render("Private workspace") + "  " + chatFaintStyle.Render(m.rt.ResolvedModel)
	if m.rt.UseKnowledgeBase {
		header += "  " + chatReadyStyle.Render("● Knowledge on")
	}

	status := m.statusText
	if m.busy {
		status = m.spinner.View() + " " + status
	}

	statusStyle := chatReadyStyle
	if m.busy {
		statusStyle = chatTitleStyle
	}
	footer := statusStyle.Render("● "+status) + chatFaintStyle.Render("   /help  /status  /clear  /rag  /exit   ↑↓ history")
	return chatHeaderStyle.Width(maxInt(20, m.width-2)).Render(header) + "\n\n" + m.viewport.View() + "\n\n" + m.input.View() + "\n" + footer
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

func (m *chatTUIModel) appendAssistantStream() int {
	m.lines = append(m.lines, chatAIStyle.Render("OffGrid")+"\n")
	m.refreshViewport()
	return len(m.lines) - 1
}

func (m *chatTUIModel) updateAssistantStream(text string) {
	if m.streamLineIndex < 0 || m.streamLineIndex >= len(m.lines) {
		return
	}
	m.lines[m.streamLineIndex] = chatAIStyle.Render("OffGrid") + "\n" + text
	m.refreshViewport()
}

func (m *chatTUIModel) removeEmptyAssistantStream() {
	if strings.TrimSpace(m.streamText) != "" {
		return
	}
	if m.streamLineIndex < 0 || m.streamLineIndex >= len(m.lines) {
		return
	}
	m.lines = append(m.lines[:m.streamLineIndex], m.lines[m.streamLineIndex+1:]...)
	m.streamLineIndex = -1
	m.streamText = ""
	m.refreshViewport()
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

func (m *chatTUIModel) addHistory(input string) {
	if len(m.history) == 0 || m.history[len(m.history)-1] != input {
		m.history = append(m.history, input)
	}
	m.historyIndex = len(m.history)
}

func (m *chatTUIModel) previousHistory() {
	if len(m.history) == 0 {
		return
	}
	if m.historyIndex < 0 || m.historyIndex > len(m.history) {
		m.historyIndex = len(m.history)
	}
	if m.historyIndex > 0 {
		m.historyIndex--
	}
	m.input.SetValue(m.history[m.historyIndex])
	m.input.CursorEnd()
}

func (m *chatTUIModel) nextHistory() {
	if len(m.history) == 0 || m.historyIndex < 0 {
		return
	}
	if m.historyIndex < len(m.history)-1 {
		m.historyIndex++
		m.input.SetValue(m.history[m.historyIndex])
		m.input.CursorEnd()
		return
	}
	m.historyIndex = len(m.history)
	m.input.SetValue("")
}

func waitForStream(ch <-chan chatStreamMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return chatStreamMsg{done: true}
		}
		return msg
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
