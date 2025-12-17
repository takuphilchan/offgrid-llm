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
	Type        string         `json:"type"`     // "llm" or "embedding"
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
// These are curated, verified models from HuggingFace with proper download URLs
func DefaultCatalog() *ModelCatalog {
	return &ModelCatalog{
		Models: []CatalogEntry{
			// ========== LIGHTWEIGHT MODELS (2-4GB RAM) ==========
			{
				ID:          "tinyllama-1.1b-chat",
				Name:        "TinyLlama 1.1B Chat",
				Description: "Compact model for low-resource environments",
				Parameters:  "1.1B",
				License:     "Apache 2.0",
				Provider:    "TinyLlama",
				Type:        "llm",
				MinRAM:      2,
				Recommended: true,
				Tags:        []string{"chat", "lightweight", "beginner"},
				Variants: []ModelVariant{
					{
						Quantization: "Q2_K",
						Size:         420000000, // ~0.4GB
						SHA256:       "",
						Quality:      "medium",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF/resolve/main/tinyllama-1.1b-chat-v1.0.Q2_K.gguf",
								Priority: 2,
							},
						},
					},
					{
						Quantization: "Q4_K_M",
						Size:         668788096, // ~638MB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF/resolve/main/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			// Legacy / compatibility models referenced by older docs and tests
			{
				ID:          "phi-2",
				Name:        "Phi-2",
				Description: "Microsoft's efficient 2.7B model",
				Parameters:  "2.7B",
				License:     "MIT",
				Provider:    "Microsoft",
				Type:        "llm",
				MinRAM:      4,
				Recommended: true,
				Tags:        []string{"instruct", "reasoning", "efficient"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         1600000000, // ~1.6GB
						SHA256:       "",
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
				ID:          "mistral-7b-instruct",
				Name:        "Mistral 7B Instruct",
				Description: "Mistral instruct model (compat ID)",
				Parameters:  "7B",
				License:     "Apache 2.0",
				Provider:    "Mistral AI",
				Type:        "llm",
				MinRAM:      8,
				Recommended: true,
				Tags:        []string{"instruct", "code", "general"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         4100000000, // ~4.1GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/TheBloke/Mistral-7B-Instruct-v0.2-GGUF/resolve/main/mistral-7b-instruct-v0.2.Q4_K_M.gguf",
								Priority: 2,
							},
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/MaziyarPanahi/Mistral-7B-Instruct-v0.3-GGUF/resolve/main/Mistral-7B-Instruct-v0.3.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "llama-2-7b-chat",
				Name:        "Llama 2 7B Chat",
				Description: "Meta's popular chat model (compat ID)",
				Parameters:  "7B",
				License:     "Llama 2 Community",
				Provider:    "Meta",
				Type:        "llm",
				MinRAM:      8,
				Recommended: true,
				Tags:        []string{"chat", "assistant", "legacy"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         3800000000, // ~3.8GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/TheBloke/Llama-2-7B-Chat-GGUF/resolve/main/llama-2-7b-chat.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "llama-3.2-1b-instruct",
				Name:        "Llama 3.2 1B Instruct",
				Description: "Meta's latest compact model, excellent for edge devices",
				Parameters:  "1B",
				License:     "Llama 3.2 Community",
				Provider:    "Meta",
				Type:        "llm",
				MinRAM:      2,
				Recommended: true,
				Tags:        []string{"instruct", "lightweight", "latest"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         770000000, // ~0.8GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/hugging-quants/Llama-3.2-1B-Instruct-Q4_K_M-GGUF/resolve/main/llama-3.2-1b-instruct-q4_k_m.gguf",
								Priority: 1,
							},
						},
					},
					{
						Quantization: "Q8_0",
						Size:         1200000000, // ~1.2GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/hugging-quants/Llama-3.2-1B-Instruct-Q8_0-GGUF/resolve/main/llama-3.2-1b-instruct-q8_0.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "llama-3.2-3b-instruct",
				Name:        "Llama 3.2 3B Instruct",
				Description: "Great balance of size and capability for mobile/edge",
				Parameters:  "3B",
				License:     "Llama 3.2 Community",
				Provider:    "Meta",
				Type:        "llm",
				MinRAM:      4,
				Recommended: true,
				Tags:        []string{"instruct", "efficient", "latest"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         1900000000, // ~1.9GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/bartowski/Llama-3.2-3B-Instruct-GGUF/resolve/main/Llama-3.2-3B-Instruct-Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "phi-3.5-mini-instruct",
				Name:        "Phi 3.5 Mini Instruct",
				Description: "Microsoft's efficient model with strong reasoning",
				Parameters:  "3.8B",
				License:     "MIT",
				Provider:    "Microsoft",
				Type:        "llm",
				MinRAM:      4,
				Recommended: true,
				Tags:        []string{"instruct", "reasoning", "efficient"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         2200000000, // ~2.2GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/MaziyarPanahi/Phi-3.5-mini-instruct-GGUF/resolve/main/Phi-3.5-mini-instruct.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			// ========== STANDARD MODELS (8GB RAM) ==========
			{
				ID:          "llama-3.1-8b-instruct",
				Name:        "Llama 3.1 8B Instruct",
				Description: "Meta's powerful 8B model with 128K context",
				Parameters:  "8B",
				License:     "Llama 3.1 Community",
				Provider:    "Meta",
				Type:        "llm",
				MinRAM:      8,
				Recommended: true,
				Tags:        []string{"instruct", "general", "latest", "long-context"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         4600000000, // ~4.6GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/MaziyarPanahi/Meta-Llama-3.1-8B-Instruct-GGUF/resolve/main/Meta-Llama-3.1-8B-Instruct.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "mistral-7b-instruct-v0.3",
				Name:        "Mistral 7B Instruct v0.3",
				Description: "Latest Mistral instruct model, excellent for code and reasoning",
				Parameters:  "7B",
				License:     "Apache 2.0",
				Provider:    "Mistral AI",
				Type:        "llm",
				MinRAM:      8,
				Recommended: true,
				Tags:        []string{"instruct", "code", "general"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         4100000000, // ~4.1GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/MaziyarPanahi/Mistral-7B-Instruct-v0.3-GGUF/resolve/main/Mistral-7B-Instruct-v0.3.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "hermes-3-llama-3.1-8b",
				Name:        "Hermes 3 Llama 3.1 8B",
				Description: "NousResearch's flagship instruct model with strong capabilities",
				Parameters:  "8B",
				License:     "Llama 3.1 Community",
				Provider:    "NousResearch",
				Type:        "llm",
				MinRAM:      8,
				Recommended: true,
				Tags:        []string{"instruct", "chat", "general"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         4600000000, // ~4.6GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/NousResearch/Hermes-3-Llama-3.1-8B-GGUF/resolve/main/Hermes-3-Llama-3.1-8B.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			// ========== MEDIUM MODELS (12-16GB RAM) ==========
			{
				ID:          "mistral-nemo-instruct-2407",
				Name:        "Mistral Nemo Instruct",
				Description: "12B parameter model with excellent instruction following",
				Parameters:  "12B",
				License:     "Apache 2.0",
				Provider:    "Mistral AI",
				Type:        "llm",
				MinRAM:      12,
				Recommended: true,
				Tags:        []string{"instruct", "general", "quality"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         7000000000, // ~7GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/MaziyarPanahi/Mistral-Nemo-Instruct-2407-GGUF/resolve/main/Mistral-Nemo-Instruct-2407.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "phi-4",
				Name:        "Phi 4",
				Description: "Microsoft's latest reasoning model with strong performance",
				Parameters:  "14B",
				License:     "MIT",
				Provider:    "Microsoft",
				Type:        "llm",
				MinRAM:      12,
				Recommended: true,
				Tags:        []string{"reasoning", "code", "quality"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         8400000000, // ~8.4GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/MaziyarPanahi/phi-4-GGUF/resolve/main/phi-4.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			// ========== LARGE MODELS (24GB+ RAM) ==========
			{
				ID:          "mistral-small-24b-instruct",
				Name:        "Mistral Small 24B Instruct",
				Description: "Powerful 24B model for complex tasks",
				Parameters:  "24B",
				License:     "Apache 2.0",
				Provider:    "Mistral AI",
				Type:        "llm",
				MinRAM:      24,
				Recommended: false,
				Tags:        []string{"instruct", "quality", "large"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         13300000000, // ~13.3GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/MaziyarPanahi/Mistral-Small-24B-Instruct-2501-GGUF/resolve/main/Mistral-Small-24B-Instruct-2501.Q4_K_M.gguf",
								Priority: 1,
							},
						},
					},
				},
			},
			{
				ID:          "llama-3.3-70b-instruct",
				Name:        "Llama 3.3 70B Instruct",
				Description: "Meta's flagship 70B model with exceptional capabilities",
				Parameters:  "70B",
				License:     "Llama 3.3 Community",
				Provider:    "Meta",
				Type:        "llm",
				MinRAM:      48,
				Recommended: false,
				Tags:        []string{"instruct", "flagship", "quality"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         39600000000, // ~39.6GB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/MaziyarPanahi/Llama-3.3-70B-Instruct-GGUF/resolve/main/Llama-3.3-70B-Instruct.Q4_K_M.gguf",
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
				Description: "Lightweight sentence embedding model, 384 dimensions",
				Parameters:  "22M",
				License:     "Apache 2.0",
				Provider:    "sentence-transformers",
				Type:        "embedding",
				MinRAM:      1,
				Recommended: true,
				Tags:        []string{"embedding", "semantic-search", "lightweight"},
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
				},
			},
			{
				ID:          "bge-small-en-v1.5",
				Name:        "BGE Small EN v1.5",
				Description: "High-quality English embedding model, 384 dimensions",
				Parameters:  "33M",
				License:     "MIT",
				Provider:    "BAAI",
				Type:        "embedding",
				MinRAM:      1,
				Recommended: true,
				Tags:        []string{"embedding", "semantic-search", "rag"},
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
				ID:          "nomic-embed-text-v1",
				Name:        "Nomic Embed Text v1",
				Description: "Open-source embedding model, 768 dimensions, long context",
				Parameters:  "137M",
				License:     "Apache 2.0",
				Provider:    "Nomic AI",
				Type:        "embedding",
				MinRAM:      2,
				Recommended: true,
				Tags:        []string{"embedding", "rag", "long-context"},
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
			{
				ID:          "bge-m3",
				Name:        "BGE M3",
				Description: "Multilingual embedding model, 1024 dimensions, supports long context",
				Parameters:  "567M",
				License:     "MIT",
				Provider:    "BAAI",
				Type:        "embedding",
				MinRAM:      2,
				Recommended: true,
				Tags:        []string{"embedding", "multilingual", "long-context", "rag"},
				Variants: []ModelVariant{
					{
						Quantization: "Q4_K_M",
						Size:         430800000, // ~411MB
						SHA256:       "",
						Quality:      "high",
						Sources: []ModelSource{
							{
								Type:     "huggingface",
								URL:      "https://huggingface.co/BAAI/bge-m3/resolve/main/ggml-model-q4_k_m.gguf",
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
