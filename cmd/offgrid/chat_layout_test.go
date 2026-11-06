package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestChatLayoutFitsNarrowTerminalAndKeepsExactModelInStatus(t *testing.T) {
	modelID := "日本語-model-with-a-very-long-exact-identity-012345678901234567890123456789"
	for _, width := range []int{20, 32, 60, 100} {
		m := newChatTUIModel(&ChatRuntime{ResolvedModel: modelID, UseKnowledgeBase: true})
		updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 30})
		m = updated.(chatTUIModel)
		for _, line := range strings.Split(m.View(), "\n") {
			if ansi.StringWidth(line) > width {
				t.Errorf("width %d overflow: %q", width, line)
			}
		}
		if m.rt.ResolvedModel != modelID {
			t.Fatal("display abbreviated the selected model identity")
		}
		if len(strings.Split(m.View(), "\n")) > 30 {
			t.Errorf("height overflow at %d", width)
		}
	}
}
