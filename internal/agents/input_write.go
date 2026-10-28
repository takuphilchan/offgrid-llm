package agents

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

var inputOpeningTag = regexp.MustCompile(`<([A-Za-z][A-Za-z0-9_-]{0,31})>`)

// This is only a schema hint for an unambiguous delimiter pair in the root
// request. SelectInput still validates the immutable source and exact bytes.
func uniqueInputMarkers(input string) (string, string, bool) {
	opening, closing := "", ""
	for _, candidate := range inputOpeningTag.FindAllStringSubmatch(input, -1) {
		before := "</" + candidate[1] + ">"
		if strings.Count(input, candidate[0]) != 1 || strings.Count(input, before) != 1 || strings.Index(input, before) < strings.Index(input, candidate[0])+len(candidate[0]) {
			continue
		}
		if opening != "" {
			return "", "", false
		}
		opening, closing = candidate[0], before
	}
	return opening, closing, opening != ""
}

func projectInputWriteMarkers(task *Task, tools []api.Tool) []api.Tool {
	if task == nil || task.ParentID != "" || task.Config.Execution == nil || task.Config.InputWriteVersion != execution.InputWriteVersion {
		return tools
	}
	after, before, ok := uniqueInputMarkers(task.Prompt)
	if !ok {
		return tools
	}
	projected := append([]api.Tool(nil), tools...)
	for i, tool := range projected {
		if tool.Function.Name != execution.InputWriteTool {
			continue
		}
		raw, _ := json.Marshal(tool)
		var narrowed api.Tool
		if json.Unmarshal(raw, &narrowed) != nil {
			return tools
		}
		fields, ok := narrowed.Function.Parameters["properties"].(map[string]any)
		if !ok {
			return tools
		}
		fields["input"] = map[string]any{"type": "string", "const": "request"}
		fields["after"] = map[string]any{"type": "string", "const": after}
		fields["before"] = map[string]any{"type": "string", "const": before}
		narrowed.Function.Description += " For this request, use its unique opening and closing markers exactly as specified by this schema. The markers are excluded from the written bytes."
		projected[i] = narrowed
	}
	return projected
}

// Inputs are owned by this durable root. Conversation/assistant/tool text and
// queued corrections never become sources. No caller-supplied job lookup occurs.
func InputWriteSources(task *Task) []string {
	if task == nil || task.ParentID != "" || task.Config.InputWriteVersion != execution.InputWriteVersion {
		return nil
	}
	ids := []string{"request"}
	counts := map[string]int{}
	for _, entry := range task.Instructions {
		counts[entry.RequestID]++
	}
	for _, entry := range task.Instructions[:min(len(task.Instructions), 32)] {
		if len(entry.RequestID) >= 8 && validTaskID(entry.RequestID) && counts[entry.RequestID] == 1 {
			ids = append(ids, "correction:"+entry.RequestID)
		}
	}
	return ids
}

func ResolveInputWrite(task *Task, args json.RawMessage) (execution.FileWritePayload, execution.InputProof, error) {
	var empty execution.FileWritePayload
	var proof execution.InputProof
	selection, err := execution.ParseInputWrite(args)
	if err != nil {
		return empty, proof, err
	}
	for _, id := range InputWriteSources(task) {
		if selection.Input != id {
			continue
		}
		if id == "request" {
			return execution.SelectInput(task.Prompt, selection)
		}
		for _, entry := range task.Instructions {
			if id == "correction:"+entry.RequestID {
				return execution.SelectInput(entry.Text, selection)
			}
		}
	}
	return empty, proof, execution.ErrInputSelection
}

func inputWriteIntentMatches(task *Task, modelArgs json.RawMessage, request execution.Request) bool {
	expected, _, err := ResolveInputWrite(task, modelArgs)
	if err != nil {
		return false
	}
	canonical, err := CanonicalArguments(request.Arguments)
	if err != nil {
		return false
	}
	var actual struct {
		Path     string  `json:"path"`
		Data     *[]byte `json:"data"`
		Revision string  `json:"revision,omitempty"`
	}
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.DisallowUnknownFields()
	return decoder.Decode(&actual) == nil && actual.Data != nil && actual.Path == expected.Path && actual.Revision == expected.Revision && bytes.Equal(*actual.Data, expected.Data)
}

func inputWriteContext(task *Task) string {
	ids := InputWriteSources(task)
	if len(ids) == 0 || task.Config.ExecutionWorkspace == nil || !task.Config.ExecutionWorkspace.Permissions.Write || task.Config.ExecutionPolicy != nil && !task.Config.ExecutionPolicy.Write {
		return ""
	}
	encoded, _ := json.Marshal(ids)
	guidance := "\nUse workspace_write for ordinary requested text, including short literal phrases. ONLY when the input contains actual unique opening AND closing delimiters around the desired text, use workspace_write_from_input to preserve bytes BETWEEN them, excluding delimiters. Never invent markers or ask the user to add them for ordinary writing. Source IDs: " + string(encoded) + ". Read an existing target first and supply its preimage revision to replace it. Empty revision is create-only."
	if after, before, ok := uniqueInputMarkers(task.Prompt); ok {
		guidance += " This request has one delimiter pair: after=" + after + " before=" + before + ". Use these exact strings; do not include them in file content."
	}
	return guidance
}

// Completion rechecks typed evidence, not the assistant's account of a write.
// Original source records are immutable even after newer corrections apply.
func ValidateInputWrites(task *Task) error {
	for index, step := range task.Steps {
		if step.Type != "action" || step.ToolName != execution.InputWriteTool {
			continue
		}
		if task.Config.Execution == nil {
			return execution.ErrInputVerification
		}
		payload, proof, err := ResolveInputWrite(task, json.RawMessage(step.ToolArgs))
		if err != nil {
			return execution.ErrInputVerification
		}
		var view struct {
			Host      string                `json:"host"`
			Session   string                `json:"session"`
			Operation string                `json:"operation"`
			Receipt   execution.Receipt     `json:"receipt"`
			Source    *execution.InputProof `json:"source_input"`
		}
		ref, valid := executionReferenceAtStep(task, index)
		if !valid || json.Unmarshal([]byte(step.ToolResult), &view) != nil || view.Source == nil || *view.Source != proof || view.Host != ref.Host || view.Session != ref.Session || view.Operation != "file_write" {
			return execution.ErrInputVerification
		}
		args, _ := json.Marshal(payload)
		request := execution.Request{Operation: "file_write", Arguments: args, Binding: execution.Binding{Host: task.Config.Execution.Host}}
		if !execution.WriteReceiptMatches(request, view.Receipt) {
			return execution.ErrInputVerification
		}
	}
	return nil
}
