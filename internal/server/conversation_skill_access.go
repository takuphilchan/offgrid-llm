package server

import (
	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

func taskSkillReferences(task *agents.Task) []agentcontext.SkillReference {
	if task.Config.Profile != nil {
		return task.Config.Profile.Settings.Skills
	}
	refs := make([]agentcontext.SkillReference, 0, len(task.Config.SkillMetadata))
	for _, item := range task.Config.SkillMetadata {
		if !item.Draft {
			refs = append(refs, item.SkillReference)
		}
	}
	return refs
}

// A profile-free worker inherits only pinned read access, never profile memory
// consent or skill-authoring authority. Verify its durable root/child binding.
func (b *browserRunTools) scopedSkillAccess(task *agents.Task, name string) error {
	if name != "skill_load" || !task.Config.ScopedCapabilities || task.Config.ProjectID == "" || task.Config.ProjectRevision <= 0 {
		return capabilities.ErrDenied
	}
	root := task
	if task.ParentID != "" {
		parent, exists := b.server.agentManager.GetTask(task.ParentID)
		if !exists || parent.Actor != task.Actor || parent.DeletedAt != nil || parent.Status != agents.TaskChildren || len(parent.PendingInstructions) != 0 || parent.InstructionVersion != task.InstructionVersion {
			return capabilities.ErrDenied
		}
		linked := false
		for _, link := range parent.ChildHistory {
			if link.ID == task.ID {
				linked = true
				break
			}
		}
		if !linked || parent.Config.ProjectID != task.Config.ProjectID || parent.Config.ProjectRevision != task.Config.ProjectRevision {
			return capabilities.ErrDenied
		}
		for _, childSkill := range task.Config.SkillMetadata {
			found := false
			for _, parentSkill := range parent.Config.SkillMetadata {
				if parentSkill.SkillReference == childSkill.SkillReference {
					found = true
					break
				}
			}
			if !found {
				return capabilities.ErrDenied
			}
		}
		root = parent
	}
	if !b.server.isCanonicalConversationRun(root) {
		return capabilities.ErrDenied
	}
	return nil
}
