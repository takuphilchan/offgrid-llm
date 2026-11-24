package agents

// JSON represents a non-nil pointer to a nil slice as null. For resource
// restrictions that pointer means "explicitly none", while a nil pointer means
// "inherit". Canonicalize the detached copy to [] so persistence retains the
// ceiling rather than broadening it. Never rewrite the caller's shared object.
func retainEmptyConfigSelections(copy *AgentConfig, original AgentConfig) {
	retainEmptySelection(&copy.KnowledgeSelection, original.KnowledgeSelection)
	if copy.ConversationConfiguration != nil && original.ConversationConfiguration != nil {
		retainEmptyRestrictions(&copy.ConversationConfiguration.Restrictions, original.ConversationConfiguration.Restrictions)
	}
	if copy.EffectivePreferences != nil && original.EffectivePreferences != nil {
		retainEmptyRestrictions(&copy.EffectivePreferences.Restrictions, original.EffectivePreferences.Restrictions)
	}
	for id, child := range copy.DelegateProfiles {
		retainEmptyConfigSelections(&child, original.DelegateProfiles[id])
		copy.DelegateProfiles[id] = child
	}
}

func retainEmptyRestrictions(copy *ResourceRestrictions, original ResourceRestrictions) {
	retainEmptySelection(&copy.Tools, original.Tools)
	retainEmptySelection(&copy.Knowledge, original.Knowledge)
}

func retainEmptySelection[T any](copy **[]T, original *[]T) {
	if original != nil && *copy == nil {
		empty := []T{}
		*copy = &empty
	}
}
