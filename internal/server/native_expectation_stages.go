package server

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// ProjectModelTools narrows planning schemas to durable observed references.
// It never changes canonical checks, capability identities or dispatch validation.
func (b *browserRunTools) ProjectModelTools(task *agents.Task, tools []api.Tool) []api.Tool {
	if task == nil || !task.Config.TaskFirst || task.Config.ComputerScope != "desktop" || !nativeComputerDriver(task.Config.ComputerDriver) || task.Config.NativeWorkflowVersion != agents.DocumentWorkflowEvidenceVersion {
		return tools
	}
	if agents.UsesNativeActionContract(task) {
		return b.projectActionCoupledTools(task, tools)
	}
	documentIntent := nativeDocumentTaskIntent(task)
	recoveryPending := nativeDocumentObservationRecoveryPending(task)
	refs := nativeModelReferences(task)
	matchedApp, _ := resolvedApplicationLookup(task)
	var apps []string
	lastCatalog, lastTarget := 0, 0
	for i, step := range task.Steps {
		if i < task.Config.ComputerStepOffset || step.Type != "action" {
			continue
		}
		if step.ToolName == "computer_verify_application" || step.ToolName == "computer_focus_window" || step.ToolName == "computer_observe" || step.ToolName == "computer_documents" {
			lastTarget = step.ID
		}
		if step.ToolName != "computer_applications" {
			continue
		}
		var catalog struct {
			Applications []computer.NativeApplication `json:"applications"`
		}
		if json.Unmarshal([]byte(step.ToolResult), &catalog) == nil {
			lastCatalog = step.ID
			apps = nil
			for _, app := range catalog.Applications {
				declared := false
				if task.Workflow != nil {
					for _, check := range task.Workflow.Checks {
						declared = declared || (check.Expectation.Kind == "application_open" && check.Expectation.ApplicationID == app.ID)
					}
				}
				if app.ID != "" && app.ID == matchedApp && !declared {
					apps = append(apps, modelReference(refs, app.ID))
				}
			}
		}
	}
	sort.Strings(apps)
	var items []any
	str := func(limit int) map[string]any { return map[string]any{"type": "string", "maxLength": limit} }
	fixed := func(value string) map[string]any { return map[string]any{"type": "string", "const": value} }
	usedIDs := map[string]bool{}
	if task.Workflow != nil {
		for _, check := range task.Workflow.Checks {
			usedIDs[check.Expectation.ID] = true
		}
	}
	nextID := ""
	for i := 1; nextID == ""; i++ {
		candidate := fmt.Sprintf("outcome-%d", i)
		if !usedIDs[candidate] {
			nextID = candidate
		}
	}
	item := func(kind string, fields map[string]any) map[string]any {
		properties := map[string]any{"id": fixed(nextID), "title": str(512), "kind": fixed(kind)}
		required := []string{"id", "title", "kind"}
		var names []string
		for name, value := range fields {
			properties[name] = value
			names = append(names, name)
		}
		sort.Strings(names)
		required = append(required, names...)
		return map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}
	}
	if len(apps) > 0 && lastCatalog > lastTarget {
		items = append(items, item("application_open", map[string]any{"application_id": map[string]any{"type": "string", "enum": apps}}))
	}
	state, err := documentState(task)
	lastPlanStep := 0
	for _, step := range task.Steps {
		if step.Type == "task_metadata" && step.ToolName == "task_plan" {
			lastPlanStep = step.ID
		}
	}
	if err == nil && state.step > 0 {
		target := fixed(state.inventory.Observation.Target.ID)
		var commands []string
		for _, command := range state.inventory.Commands {
			commands = append(commands, command.ID)
		}
		creationDeclared := false
		if task.Workflow != nil {
			for _, check := range task.Workflow.Checks {
				if check.Expectation.Kind != "document_blank" || check.Expectation.TargetID != state.inventory.Observation.Target.ID {
					continue
				}
				if check.State != "verified" {
					creationDeclared = true
					continue
				}
				// A newly verified blank document is the current work item until
				// its declared edit has been independently verified. Once that
				// milestone completes, another creation can be planned normally.
				if check.Binding != nil && check.Binding.Document != nil && state.inventory.Active != nil && check.Binding.Document.ID == state.inventory.Active.Document {
					edited := false
					for _, other := range task.Workflow.Checks {
						edited = edited || (other.Expectation.Kind == "document_edit_matches" && other.Expectation.Document == state.inventory.Active.Document && other.State == "verified")
					}
					lastEvidence := 0
					for _, step := range check.EvidenceSteps {
						if step > lastEvidence {
							lastEvidence = step
						}
					}
					// A completed stage needs an explicit saved plan transition
					// before another creation. Verification is not a reason to
					// repeat the same operation. Multi-document work can advance
					// its plan; canonical APIs and authorization remain unchanged.
					creationDeclared = creationDeclared || !edited || lastPlanStep <= lastEvidence
				}
			}
		}
		if len(commands) > 0 && !creationDeclared {
			items = append(items, item("document_blank", map[string]any{"target_id": target, "command": map[string]any{"type": "string", "enum": commands}}))
		}
		if state.readStep > 0 {
			fields := map[string]any{"target_id": target, "document": fixed(state.read.Binding.Document), "snapshot": fixed(state.read.Snapshot), "text": str(computer.MaxDocumentInputBytes)}
			declared := func(kind, mode string) bool {
				if task.Workflow == nil {
					return false
				}
				for _, check := range task.Workflow.Checks {
					e := check.Expectation
					if check.State == "verified" && check.Binding != nil && check.Binding.Document != nil && matchesDocumentOutcome(check.Binding, state.read.Binding) && check.Binding.Document.ExpectedSHA256 == state.read.TextSHA256 && e.Kind == "document_edit_matches" {
						lastEvidence := 0
						for _, step := range check.EvidenceSteps {
							if step > lastEvidence {
								lastEvidence = step
							}
						}
						if kind == "document_text_equals" || (kind == e.Kind && lastPlanStep <= lastEvidence) {
							return true
						}
					}
					// Fresh observation IDs do not create a new user outcome. Reuse
					// an existing identical content check rather than inviting an
					// endless declare/read/declare loop on unchanged text.
					if kind == "document_text_equals" && e.Kind == kind && e.Document == state.read.Binding.Document && e.Text != nil && state.read.Page != nil && state.read.Page.Start == 0 && state.read.Page.End == state.read.Page.TotalScalars && *e.Text == state.read.Page.Text {
						return true
					}
					if e.Kind == kind && e.Document == state.read.Binding.Document && e.Snapshot == state.read.Snapshot && e.EditMode == mode {
						return true
					}
				}
				return false
			}
			if !creationDeclared && !declared("document_text_equals", "") {
				observedFields := map[string]any{}
				for key, value := range fields {
					observedFields[key] = value
				}
				if state.read.Page != nil && state.read.Page.Start == 0 && state.read.Page.End == state.read.Page.TotalScalars {
					observedFields["text"] = fixed(state.read.Page.Text)
				}
				branch := item("document_text_equals", observedFields)
				branch["description"] = "Check requested final content already present. This does not authorize input. Do not add diagnostic baseline checks or use this instead of document_edit_matches for planned input."
				items = append(items, branch)
			}
			var modes []string
			for _, doc := range state.inventory.Documents {
				if !doc.Selected {
					continue
				}
				if doc.Capabilities.InsertText && state.read.Selection != nil && state.read.Selection.Start == state.read.Selection.End && !declared("document_edit_matches", "insert_text") {
					modes = append(modes, "insert_text")
				}
				if doc.Capabilities.ReplaceSelection && state.read.Selection != nil && state.read.Selection.Start < state.read.Selection.End && !declared("document_edit_matches", "replace_selection") {
					modes = append(modes, "replace_selection")
				}
				if doc.Capabilities.ReplaceValue && !declared("document_edit_matches", "replace_value") {
					modes = append(modes, "replace_value")
				}
			}
			if len(modes) > 0 {
				fields["edit_mode"] = map[string]any{"type": "string", "enum": modes}
				items = append(items, item("document_edit_matches", fields))
			}
		}
	}
	// Legacy structured controls remain available, but never before an actual view.
	for i := len(task.Steps) - 1; i >= task.Config.ComputerStepOffset; i-- {
		if state.step > 0 || (b.documentToolsAvailable(task) && !nativeDocumentControlFallback(task)) {
			break
		}
		step := task.Steps[i]
		if step.Type != "action" || step.ToolName != "computer_observe" {
			continue
		}
		var recorded computer.NativeView
		if !decodeNativeModelResult(step.ToolResult, &recorded) {
			break
		}
		view, err := savedDiscoveryView(task, recorded.Observation.ID)
		if err != nil {
			break
		}
		var writable, checked []string
		for _, control := range view.Elements {
			if control.Writable {
				writable = append(writable, modelReference(refs, control.ID))
			}
			if control.Checkable {
				checked = append(checked, modelReference(refs, control.ID))
			}
		}
		for _, control := range []struct {
			kind  string
			ids   []string
			field string
			value any
		}{
			{"control_text_equals", writable, "text", str(4000)},
			{"control_checked_equals", checked, "checked", map[string]any{"type": "boolean"}},
		} {
			if len(control.ids) > 0 {
				items = append(items, item(control.kind, map[string]any{"target_id": fixed(view.Observation.Target.ID), "element": map[string]any{"type": "string", "enum": control.ids}, control.field: control.value}))
			}
		}
		break
	}
	projected := make([]api.Tool, 0, len(tools))
	verificationPending := pendingDocumentReadback(task, state)
	lastAction, lastPlan := 0, 0
	for _, step := range task.Steps {
		if step.Type == "action" {
			lastAction = step.ID
		}
		if step.Type == "task_metadata" && step.ToolName == "task_plan" {
			lastPlan = step.ID
		}
	}
	for _, tool := range tools {
		name := tool.Function.Name
		if !documentIntent && nativeDocumentOnlyToolName(name) {
			continue
		}
		if recoveryPending && !nativeDocumentRecoveryToolAllowed(name) && strings.HasPrefix(name, "computer_") {
			continue
		}
		// A dispatched input already has its check and intent. Archive lookups,
		// planning and another input cannot substitute for independent read-back.
		// Keep the composed MCP catalog; narrow only this native planning stage.
		if verificationPending && (tool.Function.Name == "task_history" || tool.Function.Name == "task_plan" || tool.Function.Name == "task_expectations" || (strings.HasPrefix(tool.Function.Name, "computer_") && tool.Function.Name != "computer_documents" && tool.Function.Name != "computer_read_text")) {
			continue
		}
		if tool.Function.Name == "computer_verify" || tool.Function.Name == "computer_verify_checked" {
			kind := "control_text_equals"
			if tool.Function.Name == "computer_verify_checked" {
				kind = "control_checked_equals"
			}
			declared := false
			if task.Workflow != nil {
				for _, check := range task.Workflow.Checks {
					declared = declared || (check.Expectation.Kind == kind && check.Binding != nil)
				}
			}
			if !declared {
				continue
			}
		}
		if documentMutationTool(tool.Function.Name) {
			var ok bool
			tool, ok = projectCheckedDocumentMutation(task, tool, state)
			if !ok {
				continue
			}
		}
		if tool.Function.Name == "computer_open_application" {
			var declared []string
			verified := map[string]bool{}
			for _, id := range agents.VerifiedApplications(task) {
				verified[id] = true
			}
			if task.Workflow != nil {
				for _, check := range task.Workflow.Checks {
					if check.Expectation.Kind == "application_open" && check.Expectation.ApplicationID == matchedApp && !verified[check.Expectation.ApplicationID] {
						declared = append(declared, modelReference(refs, check.Expectation.ApplicationID))
					}
				}
			}
			if len(declared) == 0 {
				continue
			}
			tool.Function.Parameters = map[string]any{"type": "object", "additionalProperties": false, "required": []string{"application_id"}, "properties": map[string]any{"application_id": map[string]any{"type": "string", "enum": declared}}}
		}
		if tool.Function.Name == "task_plan" && lastPlan > lastAction {
			continue
		}
		if tool.Function.Name == "task_history" {
			var ok bool
			tool, ok = projectNeededHistory(task, tool, lastCatalog > lastTarget)
			if !ok {
				continue
			}
		}
		if tool.Function.Name == "task_expectations" {
			if len(items) == 0 {
				continue
			}
			tool.Function.Parameters = map[string]any{"type": "object", "additionalProperties": false, "required": []string{"expectations"}, "properties": map[string]any{"expectations": map[string]any{"type": "array", "minItems": 1, "maxItems": 1, "items": map[string]any{"oneOf": items}}}}
			tool.Function.Description = "Add only the next USER-REQUESTED outcome check whose references have already been observed, not diagnostic checks of initial contents. Preserve existing checks; do not repeat declarations or declare future documents or snapshots. For a requested new document, use document_blank with an observed command BEFORE creation, not a text check of the existing document. Use document_edit_matches after reading the intended document and BEFORE input. document_text_equals is for requested content already present, not planned input. Metadata does not dispatch input or prove completion."
		}
		projected = append(projected, tool)
	}
	return projected
}

// New contracts derive mechanical checks at dispatch admission. Projection is
// still only a planning aid: references and host authority are revalidated.
func (b *browserRunTools) projectActionCoupledTools(task *agents.Task, tools []api.Tool) []api.Tool {
	refs := nativeModelReferences(task)
	state, _ := documentState(task)
	documentIntent := nativeDocumentTaskIntent(task)
	pending := documentIntent && pendingDocumentReadback(task, state)
	controlPending := b.pendingNativeControlReadback(task)
	recoveryPending := nativeDocumentObservationRecoveryPending(task)
	matched, lookupErr := resolvedApplicationLookup(task)
	lastAction, lastPlan := 0, lastNativePlanStep(task)
	for _, step := range task.Steps {
		if step.Type == "action" {
			lastAction = step.ID
		}
	}
	var result []api.Tool
	for _, tool := range tools {
		name := tool.Function.Name
		if !documentIntent && nativeDocumentOnlyToolName(name) {
			continue
		}
		if recoveryPending && !nativeDocumentRecoveryToolAllowed(name) && strings.HasPrefix(name, "computer_") {
			continue
		}
		// Once a selected application has been independently verified, a
		// negotiated document provider must get the first post-launch turn.
		// Re-offering the generic observation tool here lets small models loop
		// over an unchanged address field instead of discovering whether the
		// surface supports documents. An unsupported provider returns a typed
		// negative capability and the fallback then exposes a fresh control view.
		if name == "computer_observe" && state.step == 0 && b.documentToolsAvailable(task) && documentIntent && desktopSelected(task) && !nativeDocumentControlFallback(task) && !recoveryPending {
			continue
		}
		if controlPending && (name == "task_history" || name == "task_plan" || name == "task_expectations" || (strings.HasPrefix(name, "computer_") && name != "computer_observe" && name != "computer_verify" && name != "computer_verify_checked" && name != "computer_inspect_control" && name != "computer_find_controls")) {
			continue
		}
		if pending && (name == "task_history" || name == "task_plan" || name == "task_expectations" || (strings.HasPrefix(name, "computer_") && name != "computer_documents" && name != "computer_read_text")) {
			continue
		}
		if name == "task_expectations" {
			// Read-only requested content checks remain available. Mechanical
			// mutation checks are derived instead of adding a metadata round trip.
			var ok bool
			tool, ok = b.projectReadOnlyNativeExpectations(task, tool)
			if !ok {
				continue
			}
		}
		if name == "task_plan" && lastPlan > lastAction {
			continue
		}
		if name == "task_history" {
			var ok bool
			tool, ok = projectNeededHistory(task, tool, !desktopSelected(task))
			if !ok {
				continue
			}
		}
		if name == "computer_documents" && nativeDocumentControlFallback(task) {
			continue
		}
		if name == "computer_open_application" {
			if lookupErr != nil || len(agents.PendingApplicationLaunches(task)) != 0 {
				continue
			}
			verified := false
			for _, id := range agents.VerifiedApplications(task) {
				verified = verified || id == matched
			}
			if verified {
				continue
			}
			tool.Function.Parameters = map[string]any{"type": "object", "additionalProperties": false, "required": []string{"application_id"}, "properties": map[string]any{"application_id": map[string]any{"type": "string", "const": modelReference(refs, matched)}}}
		}
		if documentMutationTool(name) && (name != "computer_replace_text" || state.step > 0) {
			if state.step == 0 {
				continue
			}
			fixed := func(v string) map[string]any { return map[string]any{"type": "string", "const": v} }
			properties := map[string]any{}
			required := []string{"observation_id"}
			if name == "computer_create_document" {
				if !nativeCreationCanAdvance(task, state) {
					continue
				}
				commands := []string{}
				for _, command := range state.inventory.Commands {
					commands = append(commands, command.ID)
				}
				if len(commands) == 0 {
					continue
				}
				properties["observation_id"], properties["command"] = fixed(state.inventory.Observation.ID), map[string]any{"type": "string", "enum": commands}
				required = append(required, "command")
				tool.Function.Description = "Create one fresh blank document using the observed command. The harness records its mechanical check before dispatch. Never clear an existing document; inspect documents and read the new document afterwards."
			} else {
				if !nativeEditCanAdvance(task, state) || !documentEditCapability(state, name) {
					continue
				}
				properties["observation_id"], properties["document"], properties["snapshot"] = fixed(state.read.Binding.Observation), fixed(state.read.Binding.Document), fixed(state.read.Snapshot)
				properties["text"] = map[string]any{"type": "string", "maxLength": computer.MaxDocumentInputBytes}
				required = append(required, "document", "snapshot", "text")
				tool.Function.Description = "Perform the requested " + documentEditModeForTool(name) + " once on the independently read document. The harness binds its mechanical check before dispatch. Preserve exact requested text; inspect documents and independently read back after input. Never repeat uncertain input."
			}
			tool.Function.Parameters = map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}
		}
		if name == "computer_verify" || name == "computer_verify_checked" {
			kind, exists := "control_text_equals", false
			if name == "computer_verify_checked" {
				kind = "control_checked_equals"
			}
			if task.Workflow != nil {
				for _, check := range b.AssessWorkflow(task) {
					exists = exists || (check.Expectation.Kind == kind && check.Binding != nil && check.State != "verified")
				}
			}
			if !exists {
				continue
			}
			if controlPending {
				var ok bool
				tool, ok = b.projectPendingControlReadback(task, tool, kind, refs)
				if !ok {
					continue
				}
			}
		}
		result = append(result, tool)
	}
	return result
}

func (b *browserRunTools) projectPendingControlReadback(task *agents.Task, tool api.Tool, kind string, refs map[string]string) (api.Tool, bool) {
	view, ok := currentNativeControlView(task)
	if !ok {
		return tool, false
	}
	fixed := func(value any) map[string]any { return map[string]any{"const": value} }
	var branches []any
	for _, check := range b.AssessWorkflow(task) {
		if check.Expectation.Kind != kind || check.State == "verified" || check.Binding == nil || check.Binding.TargetSHA256 != evidenceTargetDigest(view.Observation.Target) {
			continue
		}
		var elements []string
		for _, control := range view.Elements {
			if control.Identity != "" && control.Identity == check.Binding.Identity {
				elements = append(elements, modelReference(refs, control.ID))
			}
		}
		if len(elements) != 1 {
			continue
		}
		field, value := "text", any(check.Expectation.Text)
		if kind == "control_checked_equals" {
			field, value = "checked", check.Expectation.Checked
		}
		branches = append(branches, map[string]any{"type": "object", "additionalProperties": false, "required": []string{"observation_id", "element", field}, "properties": map[string]any{"observation_id": fixed(modelReference(refs, view.Observation.ID)), "element": fixed(elements[0]), field: fixed(value)}})
	}
	if len(branches) == 0 {
		return tool, false
	}
	tool.Function.Parameters = map[string]any{"oneOf": branches}
	tool.Function.Description = "Independently verify the recorded edit against its exact expected value in the freshly observed control. Verification must precede Enter, navigation or another input."
	return tool, true
}

func desktopSelected(task *agents.Task) bool {
	_, _, selected := desktopPlanningState(task)
	return selected
}

func (b *browserRunTools) projectReadOnlyNativeExpectations(task *agents.Task, tool api.Tool) (api.Tool, bool) {
	legacy := *task
	legacy.Config.NativeActionContractVersion = 0
	projected := b.ProjectModelTools(&legacy, []api.Tool{tool})
	if len(projected) != 1 {
		return tool, false
	}
	tool = projected[0]
	properties, _ := tool.Function.Parameters["properties"].(map[string]any)
	expectations, _ := properties["expectations"].(map[string]any)
	items, _ := expectations["items"].(map[string]any)
	branches, _ := items["oneOf"].([]any)
	var readOnly []any
	for _, raw := range branches {
		branch, _ := raw.(map[string]any)
		fields, _ := branch["properties"].(map[string]any)
		kind, _ := fields["kind"].(map[string]any)
		if kind["const"] == "document_text_equals" {
			readOnly = append(readOnly, raw)
		}
	}
	if len(readOnly) == 0 {
		return tool, false
	}
	items["oneOf"] = readOnly
	tool.Function.Description = "Record a requested read-only exact-content check already observed in the intended document. This does not permit an edit. For planned creation or edits use the typed action directly; its mechanical check is recorded by the harness."
	return tool, true
}

func pendingDocumentReadback(task *agents.Task, state savedDocumentState) bool {
	if task.Workflow != nil {
		for _, check := range task.Workflow.Checks {
			if check.Expectation.Kind == "document_text_equals" && check.State != "verified" && check.Binding != nil && check.Binding.Document != nil && state.inventory.Active != nil && check.Binding.Document.ID == state.inventory.Active.Document {
				return true
			}
		}
	}
	for i := len(task.Steps) - 1; i >= task.Config.ComputerStepOffset; i-- {
		step := task.Steps[i]
		if step.Type != "action" || !documentMutationTool(step.ToolName) {
			continue
		}
		var evidence computer.DocumentActionEvidence
		if !decodeNativeModelResult(step.ToolResult, &evidence) || evidence.Validate() != nil {
			return false
		}
		return state.readStep <= step.ID
	}
	return false
}

// Exact history remains available through the API. Model retrieval is limited to
// omitted/archived evidence and unread pages, rather than inviting repeated reads
// of evidence already in the prompt. This never dispatches or grants authority.
func projectNeededHistory(task *agents.Task, tool api.Tool, catalogCurrent bool) (api.Tool, bool) {
	refs := map[string]bool{}
	for i := range task.ContextArchive {
		refs[fmt.Sprintf("archive:%d", i+1)] = true
	}
	for _, step := range task.Steps {
		if step.Type != "action" {
			continue
		}
		var value struct {
			Applications []computer.NativeApplication `json:"applications"`
			Elements     []computer.NativeElement     `json:"elements"`
			View         *computer.NativeView         `json:"view"`
			computer.DocumentInventory
		}
		if json.Unmarshal([]byte(step.ToolResult), &value) != nil {
			continue
		}
		omitted := step.ToolName == "computer_applications" && catalogCurrent && len(value.Applications) > 40
		if step.ToolName == "computer_observe" || (step.ToolName == "computer_verify_application" && value.View != nil) {
			var view computer.NativeView
			if step.ToolName == "computer_observe" {
				_ = json.Unmarshal([]byte(step.ToolResult), &view)
			} else {
				view = *value.View
			}
			omitted = boundedNativeModelView(view, step.ID, nil)["omitted_controls"].(int) > 0
		}
		if step.ToolName == "computer_documents" {
			projection := boundedDocumentModelView(value.DocumentInventory, step.ID)
			omitted = projection["omitted_documents"].(int) > 0 || projection["omitted_commands"].(int) > 0
		}
		if omitted {
			refs[fmt.Sprintf("step:%d", step.ID)] = true
		}
	}
	type pageKey struct {
		reference string
		offset    int
	}
	read := map[pageKey]bool{}
	var branches []any
	for _, step := range task.Steps {
		if step.Type != "task_metadata" || step.ToolName != "task_history" {
			continue
		}
		var args struct {
			Reference string `json:"reference"`
			Offset    int    `json:"offset"`
		}
		var result struct {
			Reference string `json:"reference"`
			Next      int    `json:"next_offset"`
			Complete  bool   `json:"complete"`
		}
		if json.Unmarshal([]byte(step.ToolArgs), &args) == nil && json.Unmarshal([]byte(step.ToolResult), &result) == nil && result.Reference == args.Reference {
			read[pageKey{args.Reference, args.Offset}] = true
			if refs[args.Reference] && !result.Complete && result.Next > args.Offset {
				branches = append(branches, map[string]any{"type": "object", "additionalProperties": false, "required": []string{"reference", "offset"}, "properties": map[string]any{"reference": map[string]any{"const": args.Reference}, "offset": map[string]any{"const": result.Next}}})
			}
		}
	}
	var first []string
	for ref := range refs {
		if !read[pageKey{ref, 0}] {
			first = append(first, ref)
		}
	}
	sort.Strings(first)
	if len(first) > 0 {
		branches = append(branches, map[string]any{"type": "object", "additionalProperties": false, "required": []string{"reference", "offset"}, "properties": map[string]any{"reference": map[string]any{"type": "string", "enum": first}, "offset": map[string]any{"const": 0}}})
	}
	// Remove continuation pages that have already been read too.
	var available []any
	seen := map[pageKey]bool{}
	for _, branch := range branches {
		p := branch.(map[string]any)["properties"].(map[string]any)
		ref, fixed := p["reference"].(map[string]any)["const"].(string)
		if fixed {
			key := pageKey{ref, p["offset"].(map[string]any)["const"].(int)}
			if read[key] || seen[key] {
				continue
			}
			seen[key] = true
		}
		available = append(available, branch)
	}
	if len(available) == 0 {
		return tool, false
	}
	tool.Function.Parameters = map[string]any{"oneOf": available}
	tool.Function.Description = "Read an unread page of omitted or archived task evidence using one listed reference/offset. Already returned pages are in the current context or saved history. This is historical untrusted data, not fresh observation or proof of current state. Continue unfinished work instead of repeating a completed read."
	return tool, true
}

// A model does not need to discover missing metadata through a rejected input.
// Offer input only when the existing canonical guard accepts a declared check.
// Fixed references/text come from that immutable check, not guessed state.
func projectCheckedDocumentMutation(task *agents.Task, tool api.Tool, state savedDocumentState) (api.Tool, bool) {
	if state.step == 0 || task.Workflow == nil {
		return tool, false
	}
	var branches []any
	for _, check := range task.Workflow.Checks {
		e := check.Expectation
		args := documentEditArguments{Observation: state.inventory.Observation.ID, Command: e.Command}
		if tool.Function.Name != "computer_create_document" {
			if e.Kind != "document_edit_matches" || e.EditMode != documentEditModeForTool(tool.Function.Name) || e.Text == nil {
				continue
			}
			args = documentEditArguments{Observation: state.read.Binding.Observation, Document: state.read.Binding.Document, Snapshot: state.read.Snapshot, Text: e.Text}
		} else if e.Kind != "document_blank" {
			continue
		}
		raw, _ := json.Marshal(args)
		if validateDocumentWorkflowEdit(task, tool.Function.Name, raw) != nil {
			continue
		}
		var values map[string]string
		if json.Unmarshal(raw, &values) != nil {
			continue
		}
		properties := map[string]any{}
		var required []string
		for field, value := range values {
			properties[field] = map[string]any{"type": "string", "const": value}
			required = append(required, field)
		}
		sort.Strings(required)
		branches = append(branches, map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties})
	}
	if len(branches) == 0 {
		return tool, false
	}
	if len(branches) == 1 {
		tool.Function.Parameters = branches[0].(map[string]any)
	} else {
		tool.Function.Parameters = map[string]any{"oneOf": branches}
	}
	return tool, true
}
