package main

import (
	"bufio"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"os"
	"testing"
)

func TestWorkspaceAccessPTY(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_SESSION_PTY") != "1" {
		t.Skip("isolated PTY fixture only")
	}
	answer, err := chooseWorkspaceAccess(bufio.NewReader(os.Stdin), os.Stdout, cli.PresentationStyle{}, true)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("ACCESS_RESULT:%s\n", answer)
}

func TestWorkspaceAccessPickerRequiresExplicitChoice(t *testing.T) {
	for _, test := range []struct {
		name     string
		remember bool
		keys     []string
		want     string
	}{
		{"default denies", true, []string{"\r"}, "n"},
		{"once", true, []string{"\x1b[B", "\r"}, "y"},
		{"remember", true, []string{"2", "\r"}, "remember"},
		{"remember unavailable", false, []string{"2", "\r"}, "n"},
		{"escape overrides selection", true, []string{"1", "\x1b"}, "n"},
		{"interrupt overrides selection", true, []string{"2", "\x03"}, "n"},
		{"unknown never approves", true, []string{"allow", "\r"}, "n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := accessPicker{remember: test.remember}
			for i, k := range test.keys {
				answer, done := p.key(k)
				if i < len(test.keys)-1 && done {
					t.Fatal("approved without confirmation")
				}
				if i == len(test.keys)-1 && (!done || answer != test.want) {
					t.Fatal(answer, done)
				}
			}
		})
	}
}
