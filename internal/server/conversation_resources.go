package server

import (
	"net/http"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type conversationResources struct {
	resolutions []api.ResourceResolution
	skills      []agents.AgentSkill
	knowledge   []string
}

// This catalog reads metadata from resources already owned/authorized by the
// actor. It never enrolls a connection or reads a repository path on the service.
func (s *Server) conversationResources(r *http.Request, project *agents.Project, e *agents.EffectiveConversationPreferences, profile *agents.AgentProfile) (conversationResources, error) {
	out := conversationResources{resolutions: []api.ResourceResolution{}}
	catalog := s.profileToolCatalog(r)
	skillAllowed := func(item agents.AgentSkill) bool {
		if item.Draft {
			return false
		}
		d, _ := agentcontext.Descriptor("skill_load")
		if !s.profileActorCapabilityAllowed(s.agentActor(r), d) || !restrictionAllowsDescriptor(e.Restrictions, d) {
			return false
		}
		if profile != nil {
			for _, ref := range profile.Settings.Skills {
				if ref == item.SkillReference {
					return true
				}
			}
			return false
		}
		return true
	}
	// An optional profile retains its old skill ceiling. Without one, discover
	// only the actor's reviewed skill metadata, never draft packages or memories.
	if profile == nil {
		items, err := s.agentManager.SkillCatalog(s.workspaceID, s.agentActor(r))
		if err != nil {
			return out, err
		}
		for _, item := range items {
			if skillAllowed(item) {
				out.skills = append(out.skills, item)
			}
		}
	}
	for _, ref := range project.Resources {
		resolution := api.ResourceResolution{ProjectResourceReference: ref, Reason: "Resource unavailable in the current authorized context."}
		switch ref.Kind {
		case "skill":
			filtered := out.skills[:0]
			for _, existing := range out.skills {
				if existing.ID != ref.ID {
					filtered = append(filtered, existing)
				}
			}
			out.skills = filtered
			revision := 0
			var err error
			if ref.Revision != "" {
				revision, err = strconv.Atoi(ref.Revision)
			}
			if err != nil || revision < 0 {
				break
			}
			item, err := s.agentManager.SkillRevision(s.workspaceID, s.agentActor(r), ref.ID, revision)
			if err == nil && skillAllowed(*item) {
				out.skills = append(out.skills, *item)
				resolution.Selected, resolution.Revision = true, strconv.Itoa(item.Revision)
			}
		case "knowledge":
			if authorizeChatAccess(r.Context(), s.config != nil && s.config.RequireAuth, true) != nil || s.ragEngine == nil || !s.ragEngine.IsEnabled() {
				break
			}
			if e.Restrictions.Knowledge != nil && !containsResource(*e.Restrictions.Knowledge, ref.ID) {
				break
			}
			if profile != nil && !containsResource(profile.Settings.Knowledge, ref.ID) {
				break
			}
			doc := s.ragEngine.ForActor(modelActor(r)).GetDocument(ref.ID)
			if doc != nil && doc.ContentHash != "" && (ref.Revision == "" || ref.Revision == doc.ContentHash) {
				out.knowledge = append(out.knowledge, ref.ID)
				resolution.Selected, resolution.Revision = true, doc.ContentHash
			}
		case "connection":
			// References identify an existing tool's catalog identity, including
			// its provider and schema revision; they never enable that provider.
			if d, exists := catalog[ref.ID]; exists && d.Namespace == "mcp" && restrictionAllowsDescriptor(e.Restrictions, d) && (ref.Revision == "" || ref.Revision == d.Revision) {
				allowed := profile == nil || profileContainsTool(profile.Settings.Tools, agents.ProfileToolReference{Provider: d.Source, Tool: d.Name, SchemaRevision: d.Revision, Location: d.Location})
				if allowed {
					resolution.Selected, resolution.Revision = true, d.Revision
				}
			}
		case "instructions":
			resolution.Reason = "Project instructions require an approved execution-host folder binding. No service filesystem path was read."
		}
		if resolution.Selected {
			resolution.Reason = "Pinned from the authorized project context."
		}
		out.resolutions = append(out.resolutions, resolution)
	}
	return out, nil
}

func containsResource(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func restrictionAllowsDescriptor(r agents.ResourceRestrictions, d capabilities.Descriptor) bool {
	return r.Tools == nil || profileContainsTool(*r.Tools, agents.ProfileToolReference{Provider: d.Source, Tool: d.Name, SchemaRevision: d.Revision, Location: d.Location})
}

func (s *Server) pinConversationResources(w http.ResponseWriter, r *http.Request, e *agents.EffectiveConversationPreferences, config *agents.AgentConfig, req *jobSubmitRequest) bool {
	project, err := s.agentManager.ProjectRevision(s.controlScope(r), e.ProjectID, e.ProjectRevision)
	if err != nil {
		writeControlError(w, err)
		return false
	}
	resources, err := s.conversationResources(r, project, e, config.Profile)
	if err != nil {
		writeControlError(w, err)
		return false
	}
	for i := range resources.resolutions {
		resolution := &resources.resolutions[i]
		if config.TextOnlyInference && (resolution.Kind == "skill" || resolution.Kind == "connection") {
			resolution.Selected = false
			resolution.Reason = "The selected runtime is qualified for text only. Select a tool-capable model for this resource."
		}
		if resolution.Required && !resolution.Selected {
			writeJobError(w, 422, "project_resource_unavailable", "Required project resource "+resolution.ID+": "+resolution.Reason, false)
			return false
		}
	}
	config.ResourceResolutions = resources.resolutions
	if !config.TextOnlyInference {
		for _, item := range resources.skills {
			found := false
			for _, existing := range config.SkillMetadata {
				found = found || existing.ID == item.ID
			}
			if !found {
				config.SkillMetadata = append(config.SkillMetadata, item)
			}
		}
		if len(config.SkillMetadata) > 0 {
			if config.AllowedCapabilities == nil {
				config.AllowedCapabilities = map[string]capabilities.Descriptor{}
			}
			d, _ := agentcontext.Descriptor("skill_load")
			if restrictionAllowsDescriptor(e.Restrictions, d) && s.profileActorCapabilityAllowed(s.agentActor(r), d) {
				config.AllowedCapabilities[d.Name] = d
			}
		}
	}
	if len(resources.knowledge) > 0 {
		ids := append([]string{}, selectedProfileKnowledge(*config)...)
		for _, id := range resources.knowledge {
			if !containsResource(ids, id) {
				ids = append(ids, id)
			}
		}
		config.KnowledgeSelection = &ids
		if !s.attachJobKnowledge(w, r, req, config) {
			return false
		}
		// Retrieval must not be labelled with a revision observed before a
		// concurrent source update. Retry admission instead of silently repinning.
		for _, resolution := range resources.resolutions {
			if resolution.Kind != "knowledge" || !resolution.Selected {
				continue
			}
			doc := s.ragEngine.ForActor(modelActor(r)).GetDocument(resolution.ID)
			if doc == nil || doc.ContentHash != resolution.Revision {
				writeJobError(w, 409, "project_source_changed", "A project source changed while preparing context. Refresh it before retrying this command.", false)
				return false
			}
		}
	}
	return true
}
