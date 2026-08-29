package models

import (
	"net/url"
	"path"
	"strings"
	"testing"
)

func TestEmbeddingCatalogUsesGGUFRepositoriesAndUniqueArtifacts(t *testing.T) {
	seen := map[string]string{}
	count := 0
	for _, model := range DefaultCatalog().Models {
		if model.Type != "embedding" {
			continue
		}
		count++
		if len(model.Variants) == 0 || len(model.Variants[0].Sources) == 0 {
			t.Fatalf("embedding model %s has no source", model.ID)
		}
		source := model.Variants[0].Sources[0].URL
		parsed, err := url.Parse(source)
		if err != nil || parsed.Host != "huggingface.co" {
			t.Fatalf("embedding model %s has invalid source %q", model.ID, source)
		}
		if !strings.Contains(strings.ToLower(parsed.Path), "gguf") || !strings.HasSuffix(strings.ToLower(parsed.Path), ".gguf") {
			t.Fatalf("embedding model %s does not use a GGUF artifact: %s", model.ID, source)
		}
		artifact := strings.ToLower(path.Base(parsed.Path))
		if previous, exists := seen[artifact]; exists {
			t.Fatalf("embedding models %s and %s share ambiguous artifact %s", previous, model.ID, artifact)
		}
		seen[artifact] = model.ID
	}
	if count < 4 {
		t.Fatalf("embedding catalog contains %d entries, want at least 4", count)
	}
}
