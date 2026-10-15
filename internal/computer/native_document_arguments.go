package computer

import (
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

// DocumentTool identifies the negotiated extension, not permission to use it.
// replace_text retains its original control-value contract on older hosts.
func DocumentTool(name string, raw json.RawMessage) bool {
	switch name {
	case "computer_documents", "computer_read_text", "computer_create_document", "computer_select_document", "computer_select_text", "computer_insert_text", "computer_replace_selection":
		return true
	case "computer_replace_text":
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) == nil {
			_, present := fields["document"]
			return present
		}
	}
	return false
}

// ValidateDocumentArguments is shared by service admission and transport. It
// preserves Unicode exactly and rejects duplicate, unknown and null fields.
func ValidateDocumentArguments(name string, raw json.RawMessage) error {
	if len(raw) == 0 || len(raw) > 128<<10 || jsonwire.Validate(raw) != nil {
		return ErrInvalidControl
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return ErrInvalidControl
	}
	keys := []string{}
	switch name {
	case "computer_documents":
	case "computer_create_document":
		keys = []string{"observation_id", "command"}
	case "computer_select_document", "computer_read_text":
		keys = []string{"observation_id", "document"}
		if name == "computer_read_text" && fields["page"] != nil {
			var page map[string]json.RawMessage
			if json.Unmarshal(fields["page"], &page) != nil || len(page) != 3 || !documentArgumentID(page["snapshot"]) || !documentArgumentID(page["revision"]) {
				return ErrInvalidControl
			}
			var start int
			if string(page["start"]) == "null" || json.Unmarshal(page["start"], &start) != nil || start < 0 || start > MaxDocumentSnapshotBytes {
				return ErrInvalidControl
			}
			delete(fields, "page")
		}
	case "computer_select_text":
		keys = []string{"observation_id", "document", "snapshot", "start", "end"}
	case "computer_insert_text", "computer_replace_selection", "computer_replace_text":
		keys = []string{"observation_id", "document", "snapshot", "text"}
	default:
		return ErrInvalidControl
	}
	if len(fields) != len(keys) {
		return ErrInvalidControl
	}
	start, end := 0, 0
	for _, key := range keys {
		value, ok := fields[key]
		if !ok || string(value) == "null" {
			return ErrInvalidControl
		}
		switch key {
		case "text":
			var text string
			if json.Unmarshal(value, &text) != nil || ValidateDocumentInput(text) != nil {
				return ErrInvalidControl
			}
		case "start", "end":
			var offset int
			if json.Unmarshal(value, &offset) != nil || offset < 0 || offset > MaxDocumentSnapshotBytes {
				return ErrInvalidControl
			}
			if key == "start" {
				start = offset
			} else {
				end = offset
			}
		default:
			if !documentArgumentID(value) {
				return ErrInvalidControl
			}
		}
	}
	if end < start {
		return ErrInvalidControl
	}
	return nil
}

func documentArgumentID(raw json.RawMessage) bool {
	var value string
	return json.Unmarshal(raw, &value) == nil && opaqueID(value)
}

func documentTransportAction(kind string, raw json.RawMessage) (bool, error) {
	if kind == "computer_prepare" {
		var action struct {
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if DecodeControlMessage(raw, &action) != nil {
			return false, ErrInvalidControl
		}
		kind, raw = action.Tool, action.Arguments
	}
	if !DocumentTool(kind, raw) {
		return false, nil
	}
	return true, ValidateDocumentArguments(kind, raw)
}
