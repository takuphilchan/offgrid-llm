package models

import "strings"

// ModelCatalog contains trusted model sources
type ModelCatalog struct {
	Models []CatalogEntry `json:"models"`
}

// CatalogEntry represents a model in the catalog
type CatalogEntry struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  string         `json:"parameters"` // "7B", "13B", etc.
	License     string         `json:"license"`
	Provider    string         `json:"provider"` // "meta", "mistral", etc.
	Variants    []ModelVariant `json:"variants"`
	Tags        []string       `json:"tags"`
	MinRAM      int            `json:"min_ram_gb"`
	Recommended bool           `json:"recommended"`
}

// ModelVariant represents a specific quantization of a model
type ModelVariant struct {
	Quantization string        `json:"quantization"` // "Q4_K_M", "Q5_K_S", etc.
	Size         int64         `json:"size_bytes"`
	SHA256       string        `json:"sha256"`
	Sources      []ModelSource `json:"sources"`
	Quality      string        `json:"quality"` // "high", "medium", "low"
}

// ModelSource represents where to download a model from
type ModelSource struct {
	Type     string `json:"type"` // "huggingface", "http", "ipfs"
	URL      string `json:"url"`
	Mirror   bool   `json:"mirror"`   // Is this a mirror/backup source?
	Priority int    `json:"priority"` // Lower number = higher priority
}

// DefaultCatalog returns the built-in model catalog
func DefaultCatalog() *ModelCatalog {
	return &ModelCatalog{
		Models: []CatalogEntry{
			{
				ID:          "tinyllama-1.1b-chat",
				Name:        "TinyLlama 1.1B Chat",
				Description: "Compact model for low-resource environments",
				Parameters:  "1.1B",
				License:     "Apache 2.0",
				Provider:    "TinyLlama",
				MinRAM:      2,
				Recommended: true,
				Tags:        []string{"chat", "lightweight", "beginner"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         668788096, // ~638MB
						SHA256:       "9fecc3b3cd76bba89d504f29b616eedf7da85b96540e490ca5824d3f7d2776a0",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF/resolve/main/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
					{
						Quantization: "Q5_K_M",
						Size:         783017344, // ~768MB
						SHA256:       "aa54a5fb99ace5b964859cf072346631b2da6109715a805d07161d157c66ce7f",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF/resolve/main/tinyllama-1.1b-chat-v1.0.Q5_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "llama-2-7b-chat",
				Name:        "Llama 2 7B Chat",
				Description: "Meta's open-source chat model, good balance of quality and size",
				Parameters:  "7B",
				License:     "Llama 2 Community",
				Provider:    "Meta",
				MinRAM:      8,
				Recommended: true,
				Tags:        []string{"chat", "general", "popular"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         4081004224, // ~3.8GB
						SHA256:       "08a5566d61d7cb6b420c3e4387a39e0078e1f2fe5f055f3a03887385304d4bfa",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/TheBloke/Llama-2-7B-Chat-GGUF/resolve/main/llama-2-7b-chat.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
					{
						Quantization: "Q5_K_M",
						Size:         4783156928, // ~4.6GB
						SHA256:       "e0b99920cf47b94c78d2fb06a1eceb9ed795176dfa3f7feac64629f1b52b997f",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/TheBloke/Llama-2-7B-Chat-GGUF/resolve/main/llama-2-7b-chat.Q5_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "mistral-7b-instruct",
				Name:        "Mistral 7B Instruct",
				Description: "High-quality instruction-following model, excellent for code",
				Parameters:  "7B",
				License:     "Apache 2.0",
				Provider:    "Mistral AI",
				MinRAM:      8,
				Recommended: true,
				Tags:        []string{"instruct", "code", "general"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         4368439584, // ~4.1GB
						SHA256:       "3e0039fd0273fcbebb49228943b17831aadd55cbcbf56f0af00499be2040ccf9",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/TheBloke/Mistral-7B-Instruct-v0.2-GGUF/resolve/main/mistral-7b-instruct-v0.2.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "phi-2",
				Name:        "Phi-2",
				Description: "Microsoft's efficient 2.7B parameter model, great quality for size",
				Parameters:  "2.7B",
				License:     "MIT",
				Provider:    "Microsoft",
				MinRAM:      4,
				Recommended: true,
				Tags:        []string{"efficient", "code", "reasoning"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         1789239136, // ~1.67GB
						SHA256:       "324356668fa5ba9f4135de348447bb2bbe2467eaa1b8fcfb53719de62fbd2499",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/TheBloke/phi-2-GGUF/resolve/main/phi-2.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
		},
	}
}

// FindModel finds a model by ID (case-insensitive)
func (c *ModelCatalog) FindModel(id string) *CatalogEntry {
	id = strings.ToLower(id)
	for i := range c.Models {
		if c.Models[i].ID == id {
			return &c.Models[i]
		}
	}
	return nil
}

// GetBestVariant returns the best variant for a model given available RAM
func (c *ModelCatalog) GetBestVariant(modelID string) *ModelVariant {
	model := c.FindModel(modelID)
	if model == nil {
		return nil
	}
	// Default to 16GB if not specified
	return model.GetBestVariant(16)
}

// FindVariant finds a specific quantization variant
func (e *CatalogEntry) FindVariant(quantization string) *ModelVariant {
	for i := range e.Variants {
		if e.Variants[i].Quantization == quantization {
			return &e.Variants[i]
		}
	}
	return nil
}

// GetBestVariant returns the best variant for available RAM
func (e *CatalogEntry) GetBestVariant(availableRAMGB int) *ModelVariant {
	// Find the highest quality variant that fits in RAM
	var best *ModelVariant
	for i := range e.Variants {
		variant := &e.Variants[i]
		sizeGB := int(variant.Size / 1024 / 1024 / 1024)

		if sizeGB <= availableRAMGB {
			if best == nil || variant.Size > best.Size {
				best = variant
			}
		}
	}
	return best
}
