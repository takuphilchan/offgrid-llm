package server

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

const applicationSearchTool = "computer_find_applications"

type applicationQuery struct {
	Query       string `json:"query"`
	CatalogStep int    `json:"catalog_step"`
	Offset      int    `json:"offset,omitempty"`
}

func applicationSearchSchema() api.Tool {
	return api.Tool{Type: "function", Function: api.FunctionDef{Name: applicationSearchTool, Description: "Search the FULL latest saved application catalog by the requested name or registered alias, including omitted entries. Exact names outrank fragments. Copy catalog_step from computer_applications offgrid_step_id. Returns host-issued IDs, resolution and at most 16 matches; follow next_offset. Does not launch or refresh. Only unique matches allow launch; ask if missing or ambiguous.", Parameters: map[string]interface{}{
		"type": "object", "additionalProperties": false,
		"properties": map[string]interface{}{"query": map[string]interface{}{"type": "string", "maxLength": 256}, "catalog_step": map[string]interface{}{"type": "integer", "minimum": 1}, "offset": map[string]interface{}{"type": "integer", "minimum": 0}},
		"required":   []string{"query", "catalog_step"},
	}}}
}

func parseApplicationQuery(args json.RawMessage) (applicationQuery, error) {
	var q applicationQuery
	var fields map[string]json.RawMessage
	if !decodeNativeModelResult(string(args), &q) || json.Unmarshal(args, &fields) != nil || fields == nil || q.CatalogStep < 1 || q.Offset < 0 || q.Offset > 100000 || len(q.Query) > 256 || !utf8.ValidString(q.Query) || strings.ContainsAny(q.Query, "\x00\r\n") {
		return q, computer.ErrInvalidControl
	}
	for _, v := range fields {
		if strings.TrimSpace(string(v)) == "null" {
			return q, computer.ErrInvalidControl
		}
	}
	if _, ok := fields["query"]; !ok || strings.TrimSpace(q.Query) == "" {
		return q, computer.ErrInvalidControl
	}
	return q, nil
}

func applicationSearchResult(task *agents.Task, args json.RawMessage) (string, error) {
	q, err := parseApplicationQuery(args)
	if err != nil {
		return "", err
	}
	if task == nil || task.ComputerSessionExpired || task.Config.ComputerSession == "" || task.Config.ComputerScope != "desktop" || task.Config.ComputerStepOffset < 0 || task.Config.ComputerStepOffset > len(task.Steps) {
		return "", computer.ErrSession
	}
	var catalog struct {
		Applications []computer.NativeApplication `json:"applications"`
	}
	step := 0
	for i := len(task.Steps) - 1; i >= task.Config.ComputerStepOffset; i-- {
		s := task.Steps[i]
		if s.ToolName == "computer_applications" && s.Type == "action" {
			if !decodeNativeModelResult(s.ToolResult, &catalog) {
				return "", computer.ErrInvalidControl
			}
			step = s.ID
			break
		}
	}
	if step == 0 || q.CatalogStep != step {
		return "", computer.ErrStaleObservation
	}
	matches, exact := make([]computer.NativeApplication, 0), make([]computer.NativeApplication, 0)
	query := strings.ToLower(strings.TrimSpace(q.Query))
	for _, app := range catalog.Applications {
		names := append([]string{app.Name}, app.Aliases...)
		exactMatch, fragmentMatch := false, false
		for _, name := range names {
			exactMatch = exactMatch || strings.EqualFold(strings.TrimSpace(name), query)
			fragmentMatch = fragmentMatch || strings.Contains(strings.ToLower(name), query)
		}
		if exactMatch {
			exact = append(exact, app)
		}
		if fragmentMatch {
			matches = append(matches, app)
		}
	}
	// Exact display names take precedence over fragments, but duplicate names
	// remain ambiguous. Never choose the first matching catalog entry.
	if len(exact) > 0 {
		matches = exact
	}
	if q.Offset > len(matches) {
		return "", computer.ErrInvalidControl
	}
	end := q.Offset + 16
	if end > len(matches) {
		end = len(matches)
	}
	var next *int
	if end < len(matches) {
		next = &end
	}
	resolution := "not_found"
	if len(matches) == 1 {
		resolution = "unique"
	} else if len(matches) > 1 {
		resolution = "ambiguous"
	}
	data, err := json.Marshal(map[string]any{"applications": matches[q.Offset:end], "query": q.Query, "resolution": resolution, "catalog_step": step, "total_matches": len(matches), "next_offset": next, "saved_evidence": true})
	return string(data), err
}

// Resolve from the latest successful service-authored lookup, recomputing its
// result against the current full catalog instead of trusting a model-supplied
// selection or result text. Catalog refresh and session replacement invalidate
// the lookup. This is intent consistency, not a substitute for host authority.
func resolvedApplicationLookup(task *agents.Task) (string, error) {
	failure := func(message string) (string, error) {
		return "", &agents.ToolAuthorizationError{Code: "computer_application_lookup_required", Message: message}
	}
	if task == nil || task.Config.ComputerStepOffset < 0 || task.Config.ComputerStepOffset > len(task.Steps) {
		return failure("No launch was dispatched. Obtain a current installed-application catalog and look up the requested name.")
	}
	for i := len(task.Steps) - 1; i >= task.Config.ComputerStepOffset; i-- {
		step := task.Steps[i]
		if step.ToolName == applicationSearchTool && step.Type != "action" {
			break // A failed newer lookup cannot fall back to an older selection.
		}
		if step.Type != "action" {
			continue
		}
		if step.ToolName == "computer_applications" {
			break
		}
		if step.ToolName != applicationSearchTool {
			continue
		}
		if len(task.Instructions) > 0 && !step.Timestamp.After(task.Instructions[len(task.Instructions)-1].At) {
			return failure("No launch was dispatched. The user changed the task after this lookup. Look up the currently requested application again.")
		}
		result, err := applicationSearchResult(task, json.RawMessage(step.ToolArgs))
		var match struct {
			Applications []computer.NativeApplication `json:"applications"`
			Total        int                          `json:"total_matches"`
		}
		if err != nil || json.Unmarshal([]byte(result), &match) != nil {
			break
		}
		if match.Total != 1 || len(match.Applications) != 1 || match.Applications[0].ID == "" {
			return failure("No launch was dispatched. The latest name lookup is missing or ambiguous. Ask which installed application is intended; do not substitute another app or select the first match.")
		}
		return match.Applications[0].ID, nil
	}
	return failure("No launch was dispatched. Call computer_find_applications with the user's requested application name and latest catalog_step before choosing its application_id.")
}

func validateApplicationLookup(task *agents.Task, id string) error {
	matched, err := resolvedApplicationLookup(task)
	if err != nil {
		return err
	}
	if id != matched {
		return &agents.ToolAuthorizationError{Code: "computer_application_lookup_required", Message: "No launch was dispatched. The application_id does not match the latest unique name lookup. Use that lookup's host-issued ID; never substitute a different application."}
	}
	return nil
}

// Validate only newly proposed checks. Existing immutable declarations and old
// checkpoints are not rewritten when switching apps or refreshing a catalog.
func (b *browserRunTools) ValidateDeclaredExpectations(task *agents.Task, proposed []agents.OutcomeExpectation) error {
	if task == nil || task.Config.ComputerScope != "desktop" {
		return nil
	}
	for _, expectation := range proposed {
		if expectation.Kind != "application_open" {
			continue
		}
		existing := false
		if task.Workflow != nil {
			for _, check := range task.Workflow.Checks {
				existing = existing || check.Expectation.ID == expectation.ID
			}
		}
		if !existing {
			if err := validateApplicationLookup(task, expectation.ApplicationID); err != nil {
				return err
			}
		}
	}
	return nil
}
