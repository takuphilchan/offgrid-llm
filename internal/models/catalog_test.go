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

func TestCurrentCatalogIsPinnedAndKeepsCompatibility(t *testing.T) {
	catalog := DefaultCatalog()
	seen := map[string]bool{}
	starters := 0
	starterProviders := map[string]bool{}
	for _, model := range catalog.Models {
		if seen[model.ID] {
			t.Fatalf("duplicate ID %s", model.ID)
		}
		seen[model.ID] = true
		if model.Type == "llm" && model.Recommended {
			starters++
			if starterProviders[model.Provider] {
				t.Fatalf("starter selection repeats provider %s", model.Provider)
			}
			starterProviders[model.Provider] = true
			v := model.Variants[0]
			u, _ := url.Parse(v.Sources[0].URL)
			parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
			revision, digest := catalog.PinnedSource(model.ID, parts[0]+"/"+parts[1], path.Base(u.Path))
			if !sourceRevision.MatchString(revision) || !packageDigest.MatchString(digest) || v.Size <= 0 || model.MinRAM <= 0 {
				t.Fatalf("unreviewed starter: %s", model.ID)
			}
		}
	}
	if starters != 3 {
		t.Fatalf("starter menu must stay bounded, got %d", starters)
	}
	providers := map[string]bool{}
	for _, model := range currentSmallModels() {
		if providers[model.Provider] {
			t.Fatalf("current selection repeats provider %s", model.Provider)
		}
		providers[model.Provider] = true
	}
	if len(providers) != 5 {
		t.Fatalf("want five independent families, got %d", len(providers))
	}
	if catalog.FindModel("lfm2.5-1.2b-instruct").License != "LFM 1.0" {
		t.Fatal("Liquid's custom license must not be relabeled Apache")
	}
	for _, id := range []string{"tinyllama-1.1b-chat", "phi-2", "llama-2-7b-chat", "mistral-7b-instruct"} {
		if m := catalog.FindModel(id); m == nil || m.Recommended {
			t.Fatalf("lost compatibility or recommended an obsolete starter: %s", id)
		}
	}
	for _, args := range [][3]string{
		{"missing", "unsloth/Qwen3.5-2B-GGUF", "Qwen3.5-2B-Q4_K_M.gguf"},
		{"qwen3.5-2b", "other/Qwen3.5-2B-GGUF", "Qwen3.5-2B-Q4_K_M.gguf"},
		{"qwen3.5-2b", "unsloth/Qwen3.5-2B-GGUF", "other.gguf"},
	} {
		if rev, digest := catalog.PinnedSource(args[0], args[1], args[2]); rev != "" || digest != "" {
			t.Fatal("catalog identity attached to an unrelated artifact")
		}
	}
}
