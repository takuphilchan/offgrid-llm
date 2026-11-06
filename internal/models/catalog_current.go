package models

import (
	"net/url"
	"strings"
)

// DefaultCatalog returns curated discovery choices, not runtime qualification.
func DefaultCatalog() *ModelCatalog {
	catalog := compatibilityCatalog()
	for i := range catalog.Models {
		entry := &catalog.Models[i]
		if entry.Type == "llm" {
			entry.Recommended = false
			for j, tag := range entry.Tags {
				if tag == "latest" || tag == "beginner" {
					entry.Tags[j] = "compatibility"
				}
			}
		}
	}
	catalog.Models = append(currentSmallModels(), catalog.Models...)
	return catalog
}

// Reviewed against upstream repository metadata on 2026-11-06. These are
// discovery choices, not measured speed/quality or agent qualification claims.
// RAM is a planning estimate; activation still uses actual runtime admission.
func currentSmallModels() []CatalogEntry {
	entry := func(id, name, description, parameters, provider, license, repo, revision, file, digest string, size int64, ram int, recommended bool) CatalogEntry {
		return CatalogEntry{
			ID: id, Name: name, Description: description, Parameters: parameters,
			License: license, Provider: provider, Type: "llm", MinRAM: ram, Recommended: recommended,
			Tags: []string{"chat", "small", "multilingual"},
			Variants: []ModelVariant{{Quantization: "Q4_K_M", Size: size, SHA256: digest,
				Sources: []ModelSource{{Type: "huggingface", URL: "https://huggingface.co/" + repo + "/resolve/" + revision + "/" + file, Priority: 1}}}},
		}
	}
	return []CatalogEntry{
		entry("qwen3.5-2b", "Qwen3.5 2B", "Small multilingual chat model. Bundle download includes its vision projector; image and agent readiness are checked separately.", "2B", "Qwen", "Apache 2.0", "unsloth/Qwen3.5-2B-GGUF", "f6d5376be1edb4d416d56da11e5397a961aca8ae", "Qwen3.5-2B-Q4_K_M.gguf", "aaf42c8b7c3cab2bf3d69c355048d4a0ee9973d48f16c731c0520ee914699223", 1280835840, 6, true),
		entry("smollm3-3b", "SmolLM3 3B", "Text-only multilingual model from Hugging Face. A compact alternative for chat and writing; does not accept images.", "3B", "Hugging Face", "Apache 2.0", "ggml-org/SmolLM3-3B-GGUF", "4965cb60b150737b68a0408c36aeefb65078f894", "SmolLM3-Q4_K_M.gguf", "8334b850b7bd46238c16b0c550df2138f0889bf433809008cc17a8b05761863e", 1915305312, 6, true),
		entry("granite-4.1-3b", "Granite 4.1 3B", "IBM's compact text model for conversation and document tasks. Text only; tool use requires separate runtime checks.", "3B", "IBM", "Apache 2.0", "ibm-granite/granite-4.1-3b-GGUF", "ab4701481089b58a082ef63cc1cee738887293ff", "granite-4.1-3b-Q4_K_M.gguf", "662b0626cd58f443baea23559b469df6576a81d349649c59413b36a9fb32eb29", 2099501664, 6, true),
		entry("lfm2.5-1.2b-instruct", "LFM2.5 1.2B Instruct", "Small text-only download from Liquid AI for everyday conversation. Uses the LFM 1.0 license; review its terms before organizational use.", "1.2B", "Liquid AI", "LFM 1.0", "LiquidAI/LFM2.5-1.2B-Instruct-GGUF", "8ed288026e23958ad9dfa92d53ed773a8eee7125", "LFM2.5-1.2B-Instruct-Q4_K_M.gguf", "b1b3de114215d9507409a662a501a631095a479a419584e8a2ded6304b19b4f5", 730895168, 4, false),
		entry("gemma-4-e2b-it", "Gemma 4 E2B", "Google's compact multimodal model. Bundle download includes its vision projector; image readiness is checked separately. The E2B name is not its total memory footprint.", "E2B", "Google", "Apache 2.0", "unsloth/gemma-4-E2B-it-GGUF", "0314792d7f1f7e229411f620751375812bb9faf2", "gemma-4-E2B-it-Q4_K_M.gguf", "740185b21d22ceb83a11c3aa62ad5842ef32c70f6096d756bbee85a1e4ec34b8", 3106738272, 8, false),
	}
}

// PinnedSource resolves only an exact catalog ID/repository/file combination.
// Arbitrary repository searches keep their existing snapshot-resolution path.
func (c *ModelCatalog) PinnedSource(id, repository, filename string) (revision, digest string) {
	entry := c.FindModel(id)
	if entry == nil {
		return "", ""
	}
	for _, variant := range entry.Variants {
		for _, source := range variant.Sources {
			u, err := url.Parse(source.URL)
			if err != nil || u.Scheme != "https" || u.Host != "huggingface.co" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
				continue
			}
			parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
			if len(parts) >= 5 && parts[2] == "resolve" && parts[0]+"/"+parts[1] == repository && strings.Join(parts[4:], "/") == filename && sourceRevision.MatchString(parts[3]) && packageDigest.MatchString(variant.SHA256) {
				return parts[3], variant.SHA256
			}
		}
	}
	return "", ""
}
