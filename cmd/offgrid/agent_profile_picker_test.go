package main

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

func TestProfilePickerKeyboardSearchAndRecovery(t *testing.T) {
	profiles := []agents.AgentProfile{{ID: "a", Revision: 1, Settings: agents.ProfileSettings{Name: "研究", Model: "model-a"}}, {ID: "b", Revision: 2, Settings: agents.ProfileSettings{Name: "Writer", Model: "model-b"}}}
	for _, test := range []struct {
		keys []string
		want string
	}{
		{[]string{"\x1b[B", "\r"}, "b"},
		{[]string{"w", "r", "i", "\r"}, "b"},
		{[]string{"0", "\r", "\x7f", "2", "\r"}, "b"},
		{[]string{"n", "o", "\r", "\x15", "研", "\r"}, "a"},
	} {
		picker := profilePicker{profiles: profiles}
		var selected *agents.AgentProfile
		for i, key := range test.keys {
			p, err := picker.key(key)
			if err != nil {
				t.Fatal(err)
			}
			if p != nil {
				if i != len(test.keys)-1 {
					t.Fatal("premature selection")
				}
				selected = p
			}
		}
		if selected == nil || selected.ID != test.want {
			t.Fatal(test, selected)
		}
	}
	for _, key := range []string{"\x1b", "\x03"} {
		p := profilePicker{profiles: profiles}
		_, err := p.key(key)
		if !errors.Is(err, cli.ErrAgentSelectionCancelled) {
			t.Fatal(err)
		}
	}
	p := profilePicker{profiles: profiles}
	_, err := p.key("\x04")
	if !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	p.key(strings.Repeat("界", 1000))
	if len(p.query) != 128 {
		t.Fatal("unbounded query", len(p.query))
	}
}

func TestProfilePickerReadsOnlyOneKey(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\x1b[Bhello\n"))
	key, err := readAgentPickerKey(reader)
	if err != nil || key != "\x1b[B" {
		t.Fatal(key, err)
	}
	remaining, _ := io.ReadAll(reader)
	if string(remaining) != "hello\n" {
		t.Fatal(string(remaining))
	}
}
