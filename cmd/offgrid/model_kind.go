package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func isLikelyEmbeddingModel(values ...string) bool {
	for _, value := range values {
		if isEmbeddingModelName(value) {
			return true
		}
	}
	return false
}

func isEmbeddingModelName(value string) bool {
	name := strings.ToLower(strings.TrimSpace(value))
	if name == "" {
		return false
	}

	name = strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	embeddingMarkers := []string{
		"minilm",
		"e5-",
		"bge-",
		"bge_",
		"bge.",
		"gte-",
		"embedding",
		"embed",
		"nomic-embed",
	}
	for _, marker := range embeddingMarkers {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func embeddingModelChatError(model string) string {
	display := strings.TrimSpace(model)
	if display == "" {
		display = "this model"
	}
	return fmt.Sprintf("%s is an embedding model for search and RAG, not a conversational chat model. Start a chat with `offgrid run llama3`, `offgrid run qwen`, or `offgrid run mistral`.", display)
}
