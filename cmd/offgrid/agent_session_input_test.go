package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

func TestAgentClearDoesNotWriteControlsToPipes(t *testing.T) {
	var output bytes.Buffer
	if clearAgentDisplay(&output) || output.Len() != 0 {
		t.Fatal("terminal controls written to pipe")
	}
}

// Run only through the isolated PTY driver; never reads a developer terminal
// during ordinary go test or touches installed profiles/workspaces.
func TestAgentSessionPTYInput(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_SESSION_PTY") != "1" {
		t.Skip("isolated PTY fixture only")
	}
	input := bufio.NewReader(os.Stdin)
	line, err := readAgentSessionLine(input, os.Stdout, cli.PresentationStyle{}, cli.AgentCommandSuggestions())
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(line)
	fmt.Printf("\nINPUT_RESULT:%s\nAPPROVAL_READY\n", data)
	approval, err := input.ReadString('\n')
	if err != nil || approval != "y\n" {
		t.Fatal("approval input lost", approval, err)
	}
	fmt.Println("APPROVAL_RECEIVED")
}

func TestAgentInputPickerAndEditing(t *testing.T) {
	e := agentInputEditor{commands: cli.AgentCommandSuggestions()}
	e.insert("/")
	if !e.menuOpen() {
		t.Fatal("slash did not open menu")
	}
	e.key("\x1b[B")
	e.key("\t")
	if string(e.text) != "/agent" || e.menuOpen() {
		t.Fatal("tab did not complete", string(e.text))
	}
	if done, err := e.key("\r"); !done || err != nil {
		t.Fatal(done, err)
	}
	e = agentInputEditor{commands: cli.AgentCommandSuggestions()}
	e.insert("/skills")
	e.key("\x1b")
	if e.menuOpen() {
		t.Fatal("Escape failed to dismiss")
	}
	e = agentInputEditor{}
	e.insert("café 日本語")
	e.key("\x1b[D")
	e.key("\x7f")
	e.insert("本")
	if string(e.text) != "café 日本語" {
		t.Fatal("Unicode editing corrupted message", string(e.text))
	}
	e.key("\x01")
	e.insert("Read: ")
	if string(e.text) != "Read: café 日本語" {
		t.Fatal(string(e.text))
	}
}

func TestAgentInputExactCommandBeatsPrefix(t *testing.T) {
	e := agentInputEditor{commands: cli.AgentCommandSuggestions()}
	e.insert("/agent")
	done, err := e.key("\r")
	if !done || err != nil || string(e.text) != "/agent" {
		t.Fatal(done, err, string(e.text))
	}
}

func TestAgentInputViewportAndPlainSharedReader(t *testing.T) {
	e := agentInputEditor{}
	e.insert("OffGrid test — café 日本語\nline two")
	for _, width := range []int{1, 4, 28, 76} {
		text, cursor := e.view(width)
		if ansi.StringWidth(text) > width || cursor > width || strings.Contains(text, "\n") {
			t.Fatal(width, text, cursor)
		}
	}
	var out bytes.Buffer
	reader := bufio.NewReader(strings.NewReader("tell me a story\ny\nnext message\n"))
	line, err := readAgentSessionLine(reader, &out, cli.PresentationStyle{}, cli.AgentCommandSuggestions())
	if err != nil || strings.TrimSpace(line) != "tell me a story" {
		t.Fatal(line, err)
	}
	approval, _ := reader.ReadString('\n')
	remaining, _ := io.ReadAll(reader)
	if approval != "y\n" || string(remaining) != "next message\n" || strings.Contains(out.String(), "\x1b") {
		t.Fatal("editor consumed consent or added terminal controls", approval, string(remaining), out.String())
	}
}

func TestAgentInputHistoryRetainsDraftWithoutSubmitting(t *testing.T) {
	e := agentInputEditor{history: []string{"hello", "read the file"}, historyIndex: 2}
	e.insert("unfinished")
	for _, key := range []string{"\x1b[A", "\x1b[A", "\x1b[B", "\x1b[B"} {
		if done, err := e.key(key); done || err != nil {
			t.Fatal("history submitted input", done, err)
		}
	}
	if string(e.text) != "unfinished" {
		t.Fatal("history lost current draft", string(e.text))
	}
}

func TestAgentCommandMenuFitsShortAndNarrowTerminals(t *testing.T) {
	commands := cli.AgentCommandSuggestions()
	for _, size := range [][2]int{{32, 10}, {48, 12}, {80, 24}, {120, 40}} {
		for selected := range commands {
			rows := agentCommandMenu(commands, selected, size[0], size[1])
			if len(rows) > size[1]-2 {
				t.Fatalf("palette exceeds viewport: %v, %d", size, len(rows))
			}
			seen := false
			for _, row := range rows {
				if ansi.StringWidth(row.text) >= size[0] || strings.ContainsAny(row.text, "\r\n\x1b") {
					t.Fatalf("unsafe row %q", row.text)
				}
				if row.selected && strings.Contains(row.text, commands[selected].Command) {
					seen = true
				}
			}
			if !seen {
				t.Fatalf("selected command missing: %v, %d", size, selected)
			}
		}
	}
	rows := agentCommandMenu([]cli.AgentCommandSuggestion{{Command: "/clear", Description: "Clear display; saved context remains"}}, 0, 40, 10)
	var text strings.Builder
	for _, row := range rows {
		text.WriteString(row.text + "\n")
	}
	if !strings.Contains(text.String(), "saved context remains") {
		t.Fatal(text.String())
	}
}
