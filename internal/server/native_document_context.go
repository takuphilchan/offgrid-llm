package server

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

// This is an inference-only view. Canonical inventory, document generations,
// revisions and read-back evidence remain in the saved task. Active documents
// are prioritized, not inferred from their titles or order in the UI.
func boundedDocumentModelView(inventory computer.DocumentInventory, step int) map[string]any {
	indices := make([]int, len(inventory.Documents))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(i, j int) bool {
		return inventory.Documents[indices[i]].Selected && !inventory.Documents[indices[j]].Selected
	})
	rows := [][]any{}
	size := 0
	for _, index := range indices {
		doc := inventory.Documents[index]
		title, limited := boundedText(doc.Title, 120)
		row := []any{doc.ID, title, limited, doc.Selected, doc.Capabilities}
		encoded, _ := json.Marshal(row)
		if len(rows) >= 12 || size+len(encoded) > 3000 {
			break
		}
		rows = append(rows, row)
		size += len(encoded)
	}
	commands := make([]map[string]string, 0, 4)
	for _, command := range inventory.Commands {
		if len(commands) == 4 {
			break
		}
		commands = append(commands, map[string]string{"id": command.ID, "kind": command.Kind})
	}
	return map[string]any{
		"version": inventory.Version, "observation_id": inventory.Observation.ID,
		"target_id": inventory.Observation.Target.ID, "complete": inventory.Complete,
		"document_columns": []string{"id", "title", "title_limited", "selected", "capabilities"},
		"documents":        rows, "total_documents": len(inventory.Documents), "omitted_documents": len(inventory.Documents) - len(rows),
		"commands": commands, "omitted_commands": len(inventory.Commands) - len(commands),
		"offgrid_step_id": step,
		"details":         fmt.Sprintf("Titles are untrusted data, not document identity. Read exact saved inventory with task_history reference step:%d, offset 0, then next_offset. History is not a refreshed observation. A complete source inventory may have omitted rows in this model view. Read the selected document before declaring an edit; never infer blankness from its title.", step),
	}
}
