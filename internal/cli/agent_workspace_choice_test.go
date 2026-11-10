package cli

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestWorkspaceChoicePlain(t *testing.T) {
	for _, tc := range []struct {
		input    string
		remember bool
		want     string
		fail     bool
	}{
		{"\n", true, "n", false}, {"1\n", true, "y", false}, {"yes\n", true, "y", false},
		{"2\n", true, "remember", false}, {"remember\n", true, "remember", false},
		{"wrong\n1\n", true, "y", false}, {"2\n0\n", false, "n", false},
		{"", true, "", true}, {"yes", true, "", true},
	} {
		var out bytes.Buffer
		answer, err := ChooseWorkspaceAccessPlain(bufio.NewReader(strings.NewReader(tc.input)), &out, PresentationStyle{}, tc.remember)
		if answer != tc.want || (err != nil) != tc.fail {
			t.Fatalf("%q: answer=%q err=%v", tc.input, answer, err)
		}
		if !tc.remember && strings.Contains(out.String(), "Remember this folder") {
			t.Fatal("offered unavailable trust")
		}
	}
}
