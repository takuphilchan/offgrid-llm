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
			{
				ID:          "llama-3-8b-instruct",
				Name:        "Llama 3 8B Instruct",
				Description: "Meta's latest Llama 3 model with improved performance",
				Parameters:  "8B",
				License:     "Llama 3 Community",
				Provider:    "Meta",
				MinRAM:      8,
				Recommended: true,
				Tags:        []string{"instruct", "general", "latest"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         4661219968, // ~4.3GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/QuantFactory/Meta-Llama-3-8B-Instruct-GGUF/resolve/main/Meta-Llama-3-8B-Instruct.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "codellama-7b-instruct",
				Name:        "CodeLlama 7B Instruct",
				Description: "Specialized for code generation and programming tasks",
				Parameters:  "7B",
				License:     "Llama 2 Community",
				Provider:    "Meta",
				MinRAM:      8,
				Recommended: false,
				Tags:        []string{"code", "programming", "specialized"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         4081004224, // ~3.8GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/TheBloke/CodeLlama-7B-Instruct-GGUF/resolve/main/codellama-7b-instruct.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "neural-chat-7b",
				Name:        "Neural Chat 7B",
				Description: "Intel's fine-tuned chat model with strong conversational abilities",
				Parameters:  "7B",
				License:     "Apache 2.0",
				Provider:    "Intel",
				MinRAM:      8,
				Recommended: false,
				Tags:        []string{"chat", "conversation", "general"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         4108946752, // ~3.8GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/TheBloke/neural-chat-7B-v3-1-GGUF/resolve/main/neural-chat-7b-v3-1.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "zephyr-7b-beta",
				Name:        "Zephyr 7B Beta",
				Description: "HuggingFace's aligned chat model, excellent instruction following",
				Parameters:  "7B",
				License:     "MIT",
				Provider:    "HuggingFace",
				MinRAM:      8,
				Recommended: false,
				Tags:        []string{"chat", "instruct", "aligned"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         4368439584, // ~4.1GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/TheBloke/zephyr-7B-beta-GGUF/resolve/main/zephyr-7b-beta.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			// ========== EMBEDDING MODELS ==========
			{
				ID:          "all-minilm-l6-v2",
				Name:        "all-MiniLM-L6-v2",
				Description: "Lightweight sentence embedding model, 384 dimensions, perfect for semantic search",
				Parameters:  "22M",
				License:     "Apache 2.0",
				Provider:    "sentence-transformers",
				MinRAM:      1,
				Recommended: true,
				Tags:        []string{"embedding", "semantic-search", "lightweight", "beginner"},
				Variants: []ModelVariant{
					{
						Quantization: "F16",
						Size:         44588032, // ~42MB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/ggml-model-f16.gguf",
								Priority: 1,
							},
						},
					},
					{
						Quantization: "Q8_0",
						Size:         23875100, // ~23MB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/ggml-model-q8_0.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "bge-small-en-v1.5",
				Name:        "BGE Small EN v1.5",
				Description: "High-quality English embedding model from BAAI, 384 dimensions",
				Parameters:  "33M",
				License:     "MIT",
				Provider:    "BAAI",
				MinRAM:      1,
				Recommended: true,
				Tags:        []string{"embedding", "semantic-search", "retrieval", "rag"},
				Variants: []ModelVariant{
					{
						Quantization: "F16",
						Size:         67240064, // ~64MB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/BAAI/bge-small-en-v1.5/resolve/main/ggml-model-f16.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "e5-small-v2",
				Name:        "E5 Small v2",
				Description: "Multilingual embedding model from Microsoft, 384 dimensions",
				Parameters:  "33M",
				License:     "MIT",
				Provider:    "Microsoft",
				MinRAM:      1,
				Recommended: false,
				Tags:        []string{"embedding", "multilingual", "semantic-search"},
				Variants: []ModelVariant{
					{
						Quantization: "F16",
						Size:         67108864, // ~64MB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/intfloat/e5-small-v2/resolve/main/ggml-model-f16.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "nomic-embed-text-v1",
				Name:        "Nomic Embed Text v1",
				Description: "Open-source embedding model with strong performance, 768 dimensions",
				Parameters:  "137M",
				License:     "Apache 2.0",
				Provider:    "Nomic AI",
				MinRAM:      2,
				Recommended: true,
				Tags:        []string{"embedding", "rag", "long-context", "open-source"},
				Variants: []ModelVariant{
					{
						Quantization: "F16",
						Size:         274800000, // ~262MB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/nomic-ai/nomic-embed-text-v1/resolve/main/ggml-model-f16.gguf",
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

// IsEmbeddingModel checks if this model is an embedding model
func (e *CatalogEntry) IsEmbeddingModel() bool {
	for _, tag := range e.Tags {
		if tag == "embedding" {
			return true
		}
	}
	return false
}
