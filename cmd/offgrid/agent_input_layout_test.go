package main

import (
	"bufio"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestAgentMultilineNewlineAndPasteDoNotSubmit(t *testing.T) {
	for _, key := range []string{"\n", "\x1b\r", "\x1b[13;2u", "\x1b[27;2;13~"} {
		e := agentInputEditor{}
		e.insert("first")
		if done, err := e.key(key); done || err != nil {
			t.Fatal("newline submitted", key)
		}
		e.insert("second")
		if string(e.text) != "first\nsecond" {
			t.Fatal(string(e.text))
		}
		if done, err := e.key("\r"); !done || err != nil {
			t.Fatal("Enter did not submit")
		}
	}
}

func TestAgentMultilineGraphemeEditing(t *testing.T) {
	for _, cluster := range []string{"e\u0301", "👨‍👩‍👧‍👦", "🇿🇼", "👍🏽", "日"} {
		e := agentInputEditor{}
		e.insert("a" + cluster + "b")
		e.key("\x1b[D")
		e.key("\x7f")
		if string(e.text) != "ab" || e.cursor != 1 {
			t.Fatalf("backspace split %q: %q", cluster, string(e.text))
		}
		e = agentInputEditor{}
		e.insert("a" + cluster + "b")
		e.key("\x01")
		e.key("\x1b[C")
		e.key("\x1b[3~")
		if string(e.text) != "ab" {
			t.Fatalf("delete split %q: %q", cluster, string(e.text))
		}
	}
}

func TestAgentInsertionMergingClustersPreservesCaretBoundary(t *testing.T) {
	e := agentInputEditor{}
	e.insert("\u0301b")
	e.key("\x01")
	e.insert("e")
	if e.cursor != 2 {
		t.Fatalf("caret inside combining cluster: %d", e.cursor)
	}
	e.key("\x7f")
	if string(e.text) != "b" {
		t.Fatalf("backspace split merged cluster: %q", string(e.text))
	}
}

func TestAgentMultilineViewportAndMovement(t *testing.T) {
	e := agentInputEditor{width: 10}
	e.insert("ab日本語\n12345\nlast")
	e.key("\x1b[A")
	if e.cursor != len([]rune("ab日本語\n1234")) {
		t.Fatal("vertical column", e.cursor)
	}
	e.key("\x1b[B")
	if e.cursor != len(e.text) {
		t.Fatal("vertical return", e.cursor)
	}
	for _, width := range []int{1, 4, 10, 40, 80} {
		for cursor := range len(e.text) + 1 {
			e.cursor = cursor
			view := e.multilineView(width, 2)
			if len(view.lines) > 2 || view.row < 0 || view.row >= len(view.lines) || view.column >= width {
				t.Fatalf("bad caret %+v", view)
			}
			for _, line := range view.lines {
				if ansi.StringWidth(line) > width || strings.ContainsAny(line, "\n\r\x1b") {
					t.Fatalf("unsafe row %q", line)
				}
			}
		}
	}
}

func TestAgentMultilineHistoryRestoresDraft(t *testing.T) {
	e := agentInputEditor{width: 20, history: []string{"older\nparagraph"}, historyIndex: 1}
	e.insert("new\ndraft")
	e.key("\x1b[A") // Multiline arrows edit, never replace with history.
	if string(e.text) != "new\ndraft" {
		t.Fatal("arrow lost draft")
	}
	e.key("\x10")
	if string(e.text) != "older\nparagraph" {
		t.Fatal("history not recalled")
	}
	e.key("\x0e")
	if string(e.text) != "new\ndraft" {
		t.Fatal("draft not restored")
	}
}

func TestAgentInputHistorySearchDoesNotSubmitOrLoseDraft(t *testing.T) {
	e := agentInputEditor{history: []string{"Read café", "second paragraph", "Read 日本語"}, historyIndex: 3}
	e.insert("unfinished\ndraft")
	e.key("\x1b[D")
	cursor := e.cursor
	e.key("\x12")
	for _, key := range []string{"R", "e", "a", "d", "\x12"} {
		if done, err := e.key(key); done || err != nil {
			t.Fatal("search submitted", done, err)
		}
	}
	if string(e.text) != "Read café" {
		t.Fatal("reverse match", string(e.text))
	}
	e.key("\x1b")
	if string(e.text) != "unfinished\ndraft" || e.cursor != cursor {
		t.Fatal("search lost original draft/caret")
	}
	e.key("\x12")
	if done, _ := e.key("\r"); done || e.searchActive || string(e.text) != "Read 日本語" {
		t.Fatal("accept must return to editing")
	}
}

func TestAgentPasteDrainsOversizeAndPreservesFollowingConsent(t *testing.T) {
	for _, body := range []string{"café\r\n日本語", strings.Repeat("x", 32001), "abc\x1b[20oops"} {
		reader := bufio.NewReader(strings.NewReader(body + "\x1b[201~y\n"))
		text, large, err := readAgentPaste(reader, 32000)
		if err != nil || large != (len(body) > 32000) {
			t.Fatal(large, err)
		}
		if !large && text != strings.ReplaceAll(body, "\r\n", "\n") {
			t.Fatal(text)
		}
		if large && text != "" {
			t.Fatal("partial paste returned")
		}
		approval, _ := reader.ReadString('\n')
		if approval != "y\n" {
			t.Fatal("consumed subsequent input", approval)
		}
	}
	e := agentInputEditor{}
	e.insert("retained draft")
	e.insert(strings.Repeat("x", 32000))
	if string(e.text) != "retained draft" || e.notice == "" {
		t.Fatal("limit lost draft")
	}
}
