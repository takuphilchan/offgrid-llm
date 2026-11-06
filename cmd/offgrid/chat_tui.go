package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type ChatRuntime struct {
	Context          context.Context
	StartFiles       cli.FileConversationStarter
	fileWork         sync.WaitGroup
	Files            *cli.ServiceFileConversation
	ModelName        string
	ResolvedModel    string
	ServerPort       int
	Client           *http.Client
	UseKnowledgeBase bool
	Images           []api.SessionImage
	Messages         []ChatMessage
	SessionMgr       *sessions.SessionManager
	CurrentSession   *sessions.Session
}

func (rt *ChatRuntime) Stream(input string) <-chan chatStreamMsg {
	ch := make(chan chatStreamMsg)
	if rt.Files != nil {
		rt.fileWork.Add(1)
	}
	go func() {
		if rt.Files != nil {
			defer rt.fileWork.Done()
		}
		defer close(ch)
		err := rt.stream(input, ch)
		message := chatStreamMsg{done: err == nil, err: err}
		if rt.Files != nil {
			select {
			case ch <- message:
			case <-rt.Files.Context.Done():
			}
			return
		}
		ch <- message
	}()
	return ch
}

func (rt *ChatRuntime) stream(input string, ch chan<- chatStreamMsg) error {
	if rt.Files != nil {
		return rt.streamFiles(input, ch)
	}
	if err := cli.ValidateChatImageBudget(rt.Messages, rt.Images); err != nil {
		return err
	}
	userContent := api.SessionContent(input, rt.Images)

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
	complete := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			complete = true
			break
		}

		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil || len(chunk.Error) > 0 && string(chunk.Error) != "null" {
			return fmt.Errorf("invalid or failed response stream; pending images are retained")
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

	if !complete {
		return fmt.Errorf("stream ended before completion; pending images are retained")
	}
	assistantText := strings.TrimSpace(assistantMsg.String())
	if assistantText == "" {
		return fmt.Errorf("%s", embeddingModelChatError(rt.ResolvedModel))
	}

	rt.Messages = append(rt.Messages, userMessage, ChatMessage{
		Role:    "assistant",
		Content: assistantText,
	})

	images := rt.Images
	rt.Images = nil
	if rt.CurrentSession != nil {
		rt.CurrentSession.AddMessage("user", input)
		rt.CurrentSession.Messages[len(rt.CurrentSession.Messages)-1].Images = append([]api.SessionImage(nil), images...)
		rt.CurrentSession.AddMessage("assistant", assistantText)
		if err := rt.SessionMgr.Save(rt.CurrentSession); err != nil {
			return fmt.Errorf("response received, but failed to save session: %w", err)
		}
	}

	return nil
}

func (rt *ChatRuntime) Clear() error {
	if rt.Files != nil {
		return fmt.Errorf("this conversation has retained sources; use /exit and start a new file conversation to keep its history intact")
	}
	rt.Messages = []ChatMessage{}
	rt.Images = nil
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
	pendingInput    string
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
				if count := msg.ImageCount(); count > 0 {
					m.appendSystem(fmt.Sprintf("%d saved image(s) included in context", count))
				}
				m.addHistory(userText)
			} else {
				m.appendAssistant(msg.StringContent())
			}
		}
	} else {
		m.appendSystem("Type /help for commands. Use Ctrl+C or /exit to leave.")
	}
	if len(rt.Images) > 0 {
		m.appendSystem(cli.ChatImagesSummary(rt.Images))
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
		m.viewport.Width = max(1, msg.Width)
		m.viewport.Height = max(1, msg.Height-8)
		m.input.Width = max(1, msg.Width-4)
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
			if input == "" && len(m.rt.Images) == 0 && (m.rt.Files == nil || len(m.rt.Files.Draft.Files) == 0) {
				return m, nil
			}
			m.addHistory(input)
			if strings.HasPrefix(input, "/") {
				cmd := strings.TrimPrefix(input, "/")
				return m.handleCommand(cmd)
			}
			m.appendUser(input)
			m.pendingInput = input
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
			if m.input.Value() == "" {
				m.input.SetValue(m.pendingInput)
			}
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
			m.pendingInput = ""
			return m, nil
		}
		return m, waitForStream(m.streamCh)
	case chatFileCommandMsg:
		m.busy = false
		m.statusText = "Ready"
		if msg.files != nil {
			if m.rt.Files == nil {
				m.appendSystem("Conversation preserved. Resume: offgrid session chat " + cli.TerminalSafe(msg.files.Name))
				m.rt.CurrentSession = msg.files.Saved
			}
			m.rt.Files = msg.files
			m.rt.Files.Draft = msg.draft
		}
		if msg.err != nil {
			m.appendError(msg.err.Error())
		} else {
			m.appendSystem(msg.message)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m chatTUIModel) View() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	header := chatTitleStyle.Render("Chat")
	modelWidth := max(1, width-4)
	model := ansi.Truncate(cli.TerminalSafe(m.rt.ResolvedModel), modelWidth, "...")
	header += "\n" + chatMuteStyle.Render(model)
	if m.rt.UseKnowledgeBase {
		header = chatTitleStyle.Render("Chat with Knowledge") + "\n" + chatMuteStyle.Render(model)
	}

	status := m.statusText
	if m.busy {
		status = m.spinner.View() + " " + status
	}

	statusStyle := chatReadyStyle
	if m.busy {
		statusStyle = chatTitleStyle
	}
	hints := "  /help  /status  /exit  |  up/down: history"
	if width < 72 {
		hints = "  /help  /exit"
	}
	footer := statusStyle.Render(status) + chatMuteStyle.Render(hints)
	footer = ansi.Truncate(footer, width, "...")
	header = ansi.Hardwrap(header, max(1, width-2), false)
	return chatHeaderStyle.Width(max(1, width-2)).Render(header) + "\n" + m.viewport.View() + "\n\n" + m.input.View() + "\n" + footer
}

func (m chatTUIModel) handleCommand(cmd string) (tea.Model, tea.Cmd) {
	if isFileChatCommand(cmd) {
		m.busy = true
		m.statusText = "Preparing files..."
		return m, tea.Batch(m.spinner.Tick, m.rt.fileCommand(cmd))
	}
	if handled, message, err := cli.ChatImageCommand(cmd, &m.rt.Images); handled {
		if err != nil {
			m.appendError(err.Error())
		} else {
			m.appendSystem(message)
		}
		return m, nil
	}
	switch strings.ToLower(strings.TrimSpace(cmd)) {
	case "exit", "quit", "q":
		return m, tea.Quit
	case "help", "?":
		m.appendSystem("Commands: /help, /status, /clear, /rag, /image PATH, /images, /remove-image [N|all], /exit")
		if m.rt.Files != nil || m.rt.StartFiles != nil {
			m.appendSystem("Files: /attach PATH, /attachments, /preview-attachment N [unit], /attachment-text-only N, /remove-attachment N|all, /retry-attachment N, /use-attachment ID")
		}
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

func runChatTUI(ctx context.Context, rt *ChatRuntime) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() { cancel(); rt.fileWork.Wait() }()
	rt.Context = ctx
	if rt.Files != nil {
		rt.Files.Context = ctx
	}
	program := tea.NewProgram(newChatTUIModel(rt), tea.WithAltScreen(), tea.WithContext(ctx))
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
