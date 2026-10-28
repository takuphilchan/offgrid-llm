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
	if string(e.text) != "/skills" || e.menuOpen() {
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
