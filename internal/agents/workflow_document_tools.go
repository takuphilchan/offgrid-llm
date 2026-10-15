package agents

import (
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func guardedWorkflowMutation(name string) bool {
	switch name {
	case "computer_create_document", "computer_insert_text", "computer_replace_selection", "computer_replace_text", "computer_set_checked":
		return true
	}
	return false
}

func documentWorkflowTool(tool *api.Tool) {
	item := tool.Function.Parameters["properties"].(map[string]interface{})["expectations"].(map[string]any)["items"].(map[string]any)
	p := item["properties"].(map[string]any)
	for _, field := range []string{"document", "snapshot", "command"} {
		p[field] = map[string]any{"type": "string", "maxLength": 256, "description": "Exact host-issued reference from the current document inventory/read"}
	}
	p["edit_mode"] = map[string]any{"type": "string", "enum": []string{"insert_text", "replace_selection", "replace_value"}}
	p["text"] = map[string]any{"type": "string", "maxLength": 16000, "description": "Exact requested input; enforced limit is 16000 UTF-8 bytes"}
	p["kind"] = map[string]any{"type": "string", "maxLength": 64, "description": "application_open: application_id; document_blank: target_id, command; document_edit_matches: target_id, document, snapshot, edit_mode, text; document_text_equals: target_id, document, snapshot, text; control_text_equals: target_id, element, text; control_checked_equals: target_id, element, checked"}
	tool.Function.Description = "Add immutable outcome checks before dispatch, preserving existing checks. Discover the app before application_open. Use computer_documents before document_blank; only the recorded creation result can bind its new document. Read the active document before document_edit_matches. Never substitute an application or text-echo check for verified creation/editing. This records metadata, not permission or success."
}

func constrainDocumentRepairTool(tool *api.Tool, name, arguments string) bool {
	if name == "computer_replace_text" {
		var fields map[string]json.RawMessage
		if json.Unmarshal([]byte(arguments), &fields) != nil || fields["document"] == nil {
			return false
		}
	}
	mode := map[string]string{"computer_insert_text": "insert_text", "computer_replace_selection": "replace_selection", "computer_replace_text": "replace_value"}[name]
	if name != "computer_create_document" && mode == "" {
		return false
	}
	item := tool.Function.Parameters["properties"].(map[string]interface{})["expectations"].(map[string]any)["items"].(map[string]any)
	p := item["properties"].(map[string]any)
	kind := "document_edit_matches"
	required := []string{"id", "title", "kind", "target_id", "document", "snapshot", "edit_mode", "text"}
	if name == "computer_create_document" {
		kind = "document_blank"
		required = []string{"id", "title", "kind", "target_id", "command"}
	} else {
		p["edit_mode"] = map[string]any{"type": "string", "enum": []string{mode}}
	}
	allowed := map[string]bool{}
	for _, field := range required {
		allowed[field] = true
	}
	for field := range p {
		if !allowed[field] {
			delete(p, field)
		}
	}
	p["kind"] = map[string]any{"type": "string", "enum": []string{kind}}
	item["required"] = required
	tool.Function.Description = "Repair the rejected document action by adding its matching check with the listed fields. Preserve existing checks. References must already have been observed; creation cannot guess a future document. No input or approval occurs here."
	return true
}
