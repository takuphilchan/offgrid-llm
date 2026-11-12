package main

import (
	"strings"
	"unicode"

	"github.com/clipperhouse/uax29/v2/graphemes"
)

// Search is a preview, never a submission. Escape restores the exact draft and
// caret, and Enter accepts the result for editing before a separate submission.
func (e *agentInputEditor) searchKey(key string) {
	switch key {
	case "\x1b":
		e.text, e.cursor = []rune(e.searchOriginal), e.searchCursor
		e.searchActive = false
	case "\r", "\t":
		e.searchActive = false
	case "\x12":
		e.findHistory(true)
	case "\x7f", "\b":
		clusters := graphemes.FromString(e.searchQuery)
		last := 0
		for clusters.Next() {
			last = clusters.Start()
		}
		e.searchQuery = e.searchQuery[:last]
		e.findHistory(false)
	default:
		if strings.HasPrefix(key, "\x1b") {
			return
		}
		for _, r := range key {
			if !unicode.IsControl(r) && len(e.searchQuery) < 512 {
				e.searchQuery += string(r)
			}
		}
		e.findHistory(false)
	}
}

func (e *agentInputEditor) findHistory(next bool) {
	start := len(e.history) - 1
	if next {
		start = e.searchIndex - 1
	}
	for i := start; i >= 0; i-- {
		if strings.Contains(strings.ToLower(e.history[i]), strings.ToLower(e.searchQuery)) {
			e.searchIndex = i
			e.text = []rune(e.history[i])
			e.cursor = len(e.text)
			return
		}
	}
	e.notice = "No matching history; Esc returns to your draft"
}
