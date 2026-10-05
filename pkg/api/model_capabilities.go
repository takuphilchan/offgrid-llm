package api

// IsChatModelType preserves old unspecified LLM types, but unknown and speech
// types fail closed instead of treating everything except embeddings as chat.
func IsChatModelType(modelType string) bool {
	return modelType == "" || modelType == "llm" || modelType == "vlm" || modelType == "chat"
}

func (m Model) SupportsChat() bool {
	if !IsChatModelType(m.Type) {
		return false
	}
	if m.Capabilities == nil {
		return true
	}
	for _, capability := range m.Capabilities {
		if capability == "chat" {
			return true
		}
	}
	return false
}
