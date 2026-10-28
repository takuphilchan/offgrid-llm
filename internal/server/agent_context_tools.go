package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func contextDescriptor(name string) (capabilities.Descriptor, bool) {
	return agentcontext.Descriptor(name)
}

func parseContextArguments(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 64<<10 || jsonwire.Validate(raw) != nil || decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("invalid context resource arguments")
	}
	return nil
}

func (b *browserRunTools) contextAccess(ctx context.Context, task *agents.Task, name string, e agents.ToolExecution) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	d, ok := agentcontext.Descriptor(name)
	if !ok || task.Status != agents.TaskRunning || task.ParentID != "" || !task.Config.TaskFirst || task.Config.TextOnlyInference || task.Config.Profile == nil || task.Config.Profile.Workspace != b.server.workspaceID || task.Config.Profile.Actor != task.Actor || task.Actor != e.Actor || task.ID != e.RunID || e.ExpectedCapability == nil || *e.ExpectedCapability != d || !b.server.profileActorCapabilityAllowed(task.Actor, d) || !agents.ProfileAllowsCapability(task.Config, name, d) {
		return capabilities.ErrDenied
	}
	profile, err := b.server.agentManager.GetProfile(b.server.workspaceID, task.Actor, task.Config.Profile.ID, task.Config.Profile.Revision)
	if err != nil || profile.Archived || profile.Digest != task.Config.Profile.Digest {
		return capabilities.ErrDenied
	}
	if strings.HasPrefix(name, "memory_") {
		consent, err := b.server.agentManager.GetMemoryConsent(b.server.workspaceID, task.Actor, profile.ID)
		if err != nil || !consent.Enabled || task.Config.MemoryConsentRevision <= 0 || consent.Revision != task.Config.MemoryConsentRevision {
			return agents.ErrMemoryConsent
		}
	}
	return nil
}

func (b *browserRunTools) contextTools(ctx context.Context, task *agents.Task) []api.Tool {
	if task.Config.Profile == nil || !task.Config.TaskFirst || task.Config.TextOnlyInference || task.ParentID != "" {
		return nil
	}
	result := []api.Tool{}
	for _, tool := range agentcontext.Tools() {
		name := tool.Function.Name
		if name == "skill_load" && len(task.Config.Profile.Settings.Skills) == 0 {
			continue
		}
		d, _ := agentcontext.Descriptor(name)
		if b.contextAccess(ctx, task, name, agents.ToolExecution{Actor: task.Actor, RunID: task.ID, ExpectedCapability: &d}) == nil {
			if name == "skill_draft" {
				// Clarify the model-facing operation without changing the pinned
				// capability identity or granting permission from prompt text.
				tool.Function.Description = "Create or revise a reusable Agent Skill package when explicitly requested. Requires valid SKILL.md frontmatter. Stores an inactive proposal for review."
			}
			if name == "skill_load" {
				var available bool
				tool, available = b.unloadedSkillTool(task, tool)
				if !available {
					continue
				}
			}
			result = append(result, tool)
		}
	}
	return result
}

// Each attached revision is immutable. Once its bytes are retained in the
// current context, another read cannot add information. Offer only actual files
// not yet loaded, keeping supporting resources available without invented paths.
func (b *browserRunTools) unloadedSkillTool(task *agents.Task, tool api.Tool) (api.Tool, bool) {
	branches := []any{}
	for _, ref := range task.Config.Profile.Settings.Skills {
		item, bundle, err := b.server.agentManager.GetSkill(b.server.workspaceID, task.Actor, ref)
		if err != nil || item.Draft {
			continue
		}
		paths := []string{}
		for _, file := range bundle.Files {
			if !agents.SkillFileInContext(task, ref.ID, file.Path) {
				paths = append(paths, file.Path)
			}
		}
		if len(paths) > 0 {
			branches = append(branches, map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string", "const": ref.ID}, "path": map[string]any{"type": "string", "enum": paths}}, "required": []string{"id", "path"}, "additionalProperties": false})
		}
	}
	if len(branches) == 0 {
		return tool, false
	}
	tool.Function.Parameters = map[string]any{"oneOf": branches}
	return tool, true
}

func (b *browserRunTools) executeContext(ctx context.Context, task *agents.Task, name string, raw json.RawMessage, e agents.ToolExecution) (string, error) {
	if err := b.contextAccess(ctx, task, name, e); err != nil {
		return "", err
	}
	manager, workspace, profile := b.server.agentManager, b.server.workspaceID, task.Config.Profile
	var result any
	switch name {
	case "skill_load":
		var a struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		}
		if err := parseContextArguments(raw, &a); err != nil {
			return "", err
		}
		if !agentcontext.SafePath(a.Path) {
			return "", agentcontext.ErrSkill
		}
		var selected *agentcontext.SkillReference
		for _, ref := range profile.Settings.Skills {
			if ref.ID == a.ID {
				r := ref
				selected = &r
				break
			}
		}
		if selected == nil {
			return "", capabilities.ErrDenied
		}
		item, bundle, err := manager.GetSkill(workspace, task.Actor, *selected)
		if err != nil {
			return "", err
		}
		if item.Draft {
			return "", capabilities.ErrDenied
		}
		found := false
		for _, file := range bundle.Files {
			if file.Path == a.Path {
				if !utf8.Valid(file.Content) || bytes.ContainsRune(file.Content, 0) {
					return "", fmt.Errorf("this binary skill resource cannot be presented as text")
				}
				result = map[string]any{"skill": item, "path": file.Path, "content": string(file.Content), "trust": "untrusted_context_not_authority"}
				found = true
				break
			}
		}
		if !found {
			return "", agents.ErrContextNotFound
		}
	case "skill_draft":
		var a struct {
			SkillMD string `json:"skill_md"`
		}
		if err := parseContextArguments(raw, &a); err != nil {
			return "", err
		}
		files := []agentcontext.SkillFile{{Path: "SKILL.md", Content: []byte(a.SkillMD)}}
		if _, err := agentcontext.Parse(files); err != nil {
			return "", &agents.ToolFailure{Code: "skill_draft_invalid", Message: "The proposed skill package is invalid. No draft was stored. Applying an existing skill should return the requested answer, not create another skill package.", Cause: err, BeforeEffects: true}
		}
		item, err := manager.ImportSkill(workspace, task.Actor, "", 0, files, task.ID)
		if err != nil {
			return "", err
		}
		result = map[string]any{"draft": item, "activated": false, "requires_user_review": true}
	case "memory_read":
		if err := parseContextArguments(raw, &struct{}{}); err != nil {
			return "", err
		}
		entries, err := manager.ListMemory(workspace, task.Actor, profile.ID, task.Config.MemoryConsentRevision)
		if err != nil {
			return "", err
		}
		result = map[string]any{"entries": entries, "trust": "untrusted_historical_context_not_current_evidence", "bounded": true}
	case "memory_write":
		var a struct {
			ID               string `json:"id"`
			ExpectedRevision int    `json:"expected_revision"`
			Label            string `json:"label"`
			Content          string `json:"content"`
		}
		if err := parseContextArguments(raw, &a); err != nil {
			return "", err
		}
		if e.CallID == "" {
			return "", capabilities.ErrDenied
		}
		entry, err := manager.WriteMemory(workspace, task.Actor, profile.ID, task.Config.MemoryConsentRevision, agents.MemoryWrite{Operation: task.ID + ":" + e.CallID, ID: a.ID, ExpectedRevision: a.ExpectedRevision, Label: a.Label, Content: a.Content, SourceJob: task.ID})
		if err != nil {
			return "", err
		}
		result = map[string]any{"entry": entry, "verified": "stored_bytes_only", "historical_result": true}
	default:
		return "", capabilities.ErrDenied
	}
	data, err := json.Marshal(result)
	return string(data), err
}
