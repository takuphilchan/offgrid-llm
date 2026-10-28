package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestAgentStreamPresentsCommittedListingOnceBeforeAssistantSummary(t *testing.T) {
	result := map[string]any{"operation": "file_list", "receipt": map[string]any{"state": "complete", "result": map[string]any{"host": "local-host", "path": "/selected", "revision": strings.Repeat("a", 64), "complete": true, "entries": []map[string]any{{"name": "café 日本語.txt"}, {"name": "reports", "directory": true}}}}}
	encoded, _ := json.Marshal(result)
	step := map[string]any{"id": 1, "type": "action", "tool_name": "workspace_list", "tool_result": string(encoded)}
	var stream, out, progress bytes.Buffer
	for i := 0; i < 3; i++ {
		event := map[string]any{"type": "snapshot", "run_id": "run-1", "status": "running", "steps": []any{step}, "output": ""}
		if i == 2 {
			event["status"] = "completed"
			event["output"] = "Assistant summary remains unchanged."
		}
		data, _ := json.Marshal(event)
		fmt.Fprintf(&stream, "data: %s\n\n", data)
	}
	if err := renderAgentStreamTo(&out, &progress, &stream); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "Observed files") != 1 || !strings.Contains(out.String(), "café 日本語.txt") || !strings.Contains(out.String(), "reports/") || !strings.Contains(out.String(), "Assistant summary remains unchanged.") || strings.Index(out.String(), "Observed files") > strings.Index(out.String(), "Assistant summary") {
		t.Fatal(out.String(), progress.String())
	}
}
