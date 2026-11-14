package agents

func ResolveConversationPreferences(requestModel string, conversation ConversationPreferences, profile *AgentProfile, project, user, service ConversationPreferences) EffectiveConversationPreferences {
	e := EffectiveConversationPreferences{Delegation: true, ProfileID: conversation.ProfileID, ProfileRevision: conversation.ProfileRevision, Restrictions: conversation.Restrictions, Provenance: map[string]string{"delegation": "service_default", "restrictions": "conversation"}}
	profilePreferences := ConversationPreferences{}
	if profile != nil {
		profilePreferences.Model = profile.Settings.Model
	}
	layers := []struct {
		name  string
		value ConversationPreferences
	}{
		{"request", ConversationPreferences{Model: requestModel}}, {"conversation", conversation}, {"profile", profilePreferences}, {"project", project}, {"user_default", user}, {"service_default", service},
	}
	delegationFound := false
	for _, layer := range layers {
		if e.Model == "" && layer.value.Model != "" {
			e.Model = layer.value.Model
			e.Provenance["model"] = layer.name
		}
		if !delegationFound && layer.value.Delegation != nil {
			e.Delegation = *layer.value.Delegation
			e.Provenance["delegation"] = layer.name
			delegationFound = true
		}
	}
	return e
}

// Omitted restriction fields retain prior choices, including while changing
// profiles. An explicit empty list means disabled; it never means unrestricted.
func preserveConversationRestrictions(old, next ResourceRestrictions) ResourceRestrictions {
	if next.Tools == nil {
		next.Tools = old.Tools
	}
	if next.Knowledge == nil {
		next.Knowledge = old.Knowledge
	}
	if next.Computer == nil {
		next.Computer = old.Computer
	}
	if next.Execution == nil {
		next.Execution = old.Execution
	}
	return next
}
