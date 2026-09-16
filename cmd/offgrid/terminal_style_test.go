package main

import "testing"

func TestTerminalSupportsColorHonorsEnvironment(t *testing.T) {
	t.Run("no color wins", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		t.Setenv("FORCE_COLOR", "1")
		if terminalSupportsColor() {
			t.Fatal("terminalSupportsColor() = true with NO_COLOR set")
		}
	})

	t.Run("dumb terminal stays plain", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("TERM", "dumb")
		t.Setenv("FORCE_COLOR", "1")
		if terminalSupportsColor() {
			t.Fatal("terminalSupportsColor() = true for TERM=dumb")
		}
	})

	t.Run("force color enables styling", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("TERM", "xterm-256color")
		t.Setenv("FORCE_COLOR", "1")
		if !terminalSupportsColor() {
			t.Fatal("terminalSupportsColor() = false with FORCE_COLOR set")
		}
	})
}

func TestTruncateTerminalTextPreservesUnicode(t *testing.T) {
	const input = "agent → résumé → ready"
	got := truncateTerminalText(input, 12)
	if got != "agent → r..." {
		t.Fatalf("truncateTerminalText() = %q, want %q", got, "agent → r...")
	}
	if len([]rune(got)) != 12 {
		t.Fatalf("truncateTerminalText() rune length = %d, want 12", len([]rune(got)))
	}
}
