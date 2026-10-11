package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestDocumentProjectionIsBoundedWithoutChangingCanonicalEvidence(t *testing.T) {
	task, _, inventory, _ := documentGuardFixture("insert_text")
	for i := 1; i < 256; i++ {
		inventory.Documents = append(inventory.Documents, computer.NativeDocument{ID: fmt.Sprintf("d%d", i), Generation: "g", Container: fmt.Sprintf("tab%d", i), Title: strings.Repeat("日本語 café 😀 ", 30)})
	}
	// A selected tab at the end cannot disappear behind the preview limit.
	inventory.Documents[0], inventory.Documents[255] = inventory.Documents[255], inventory.Documents[0]
	raw := documentJSON(struct {
		computer.DocumentInventory
		Step int `json:"offgrid_step_id"`
	}{inventory, 1})
	task.Steps[0].ToolResult = raw
	messages := []api.ChatMessage{{Role: "system", Content: "Never save or change existing documents"}, {Role: "tool", Name: "computer_documents", ToolCallID: "call", Content: raw}}
	projected := (&browserRunTools{}).ProjectModelContext(task, messages)
	var view struct {
		Rows     [][]any `json:"documents"`
		Omitted  int     `json:"omitted_documents"`
		Complete bool    `json:"complete"`
		Details  string  `json:"details"`
	}
	text := projected[1].StringContent()
	if err := json.Unmarshal([]byte(text), &view); err != nil {
		t.Fatal(err)
	}
	if len(text) > 6000 || !utf8.ValidString(text) || len(view.Rows) > 12 || view.Rows[0][0] != inventory.Active.Document || view.Omitted != 256-len(view.Rows) || !view.Complete || !strings.Contains(view.Details, "step:1") {
		t.Fatal("lost scope, active tab or bounds", len(text), view)
	}
	if task.Steps[0].ToolResult != raw || messages[1].StringContent() != raw || projected[0].StringContent() != messages[0].StringContent() || projected[1].ToolCallID != "call" {
		t.Fatal("modified canonical evidence or original instructions")
	}
	for _, version := range []int{0, 1, 99} {
		task.Config.NativeWorkflowVersion = version
		if got := (&browserRunTools{}).ProjectModelContext(task, messages)[1].StringContent(); got != raw {
			t.Fatal("reinterpreted another evidence version", version)
		}
	}
}

func TestIncompleteDocumentProjectionDoesNotBecomeComplete(t *testing.T) {
	_, _, inventory, _ := documentGuardFixture("insert_text")
	inventory.Complete = false
	view := boundedDocumentModelView(inventory, 1)
	if view["complete"] != false {
		t.Fatal("incomplete inventory promoted")
	}
}
