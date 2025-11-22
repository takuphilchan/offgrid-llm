package models

import (
	"path/filepath"
	"strings"
)

// ProjectorSource describes where to fetch a projector/mmproj adapter
// associated with a given VLM.
type ProjectorSource struct {
	ModelID string // Hugging Face repo containing the adapter
	File    HFFile // File metadata (filename plus optional size)
	Source  string // "companion" or "fallback"
	Reason  string // Human-readable context for UI/CLI messaging
}

type projectorFallback struct {
	Matchers   []string
	Repository string
	Filename   string
	Reason     string
}

var defaultProjectorFallbacks = []projectorFallback{
	{
		Matchers: []string{
			"vlm-r1-qwen2.5vl-3b",
			"vlm-r1-qwen2.5",
			"qwen2.5vl-3b",
			"qwen2.5-omni-3b",
			"qwen2.5vl-3b-ovd",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "Qwen2.5-Omni-3B-mmproj-Q8_0.gguf",
		Reason:     "Using Qwen2.5-VL 3B omni projector from koboldcpp/mmproj catalog",
	},
	{
		Matchers: []string{
			"qwen2.5vl-7b",
			"qwen2.5-omni-7b",
			"qwen2.5vl-7b-ovd",
			"vlm-r1-qwen2.5vl-7b",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "Qwen2.5-Omni-7B-mmproj-Q8_0.gguf",
		Reason:     "Fallback to Qwen2.5-VL 7B omni projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"qwen2-vl-2b",
			"qwen2vl-2b",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "Qwen2-VL-2B-mmproj-q5_1.gguf",
		Reason:     "Fallback to Qwen2-VL 2B projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"qwen2-vl-7b",
			"qwen2vl-7b",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "Qwen2-VL-7B-mmproj-q5_1.gguf",
		Reason:     "Fallback to Qwen2-VL 7B projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"qwen2.5-vl-7b-vision",
			"qwen2.5vl-7b-vision",
			"qwen2.5-vl-vision-7b",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "qwen2.5-vl-7b-vision-mmproj-f16.gguf",
		Reason:     "Using qwen2.5-vl 7B vision projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"llama-3-vision",
			"llama3-vision",
			"llama-3.1-vision",
			"llama3.1-vision",
			"llama3-8b",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "LLaMA3-8B_mmproj-Q4_1.gguf",
		Reason:     "Fallback to LLaMA3-8B projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"gemma3-4b",
			"gemma-vision-4b",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "gemma3-4b-mmproj.gguf",
		Reason:     "Fallback to Gemma3 4B vision projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"gemma3-12b",
			"gemma-vision-12b",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "gemma3-12b-mmproj.gguf",
		Reason:     "Fallback to Gemma3 12B vision projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"gemma3-27b",
			"gemma-vision-27b",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "gemma3-27b-mmproj.gguf",
		Reason:     "Fallback to Gemma3 27B vision projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"llama-13b",
			"llama13b-vision",
			"llama-v1.5-13b",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "llama-13b-mmproj-v1.5.Q4_1.gguf",
		Reason:     "Fallback to LLaVA 13B projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"llama-7b",
			"llama7b-vision",
			"llama-v1.5-7b",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "llama-7b-mmproj-v1.5-Q4_0.gguf",
		Reason:     "Fallback to LLaVA 7B projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"mistral-small-3.1-24b",
			"mistral-small-vision",
			"mistral-24b-vision",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "mmproj-mistralai_Mistral-Small-3.1-24B-Instruct-2503-f16.gguf",
		Reason:     "Fallback to Mistral Small 24B projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"pixtral-12b",
			"pixtral",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "pixtral-12b-mmproj-f16.gguf",
		Reason:     "Fallback to Pixtral 12B projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"yi-34b-vision",
			"yi-34b",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "yi-34b-mmproj-v1.6-Q4_1.gguf",
		Reason:     "Fallback to Yi 34B projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"obsidian-3b",
			"obsidian3b",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "obsidian-3b_mmproj-Q4_1.gguf",
		Reason:     "Fallback to Obsidian 3B projector (koboldcpp/mmproj)",
	},
	{
		Matchers: []string{
			"minicpm",
			"mini-cpm",
		},
		Repository: "koboldcpp/mmproj",
		Filename:   "minicpm-mmproj-model-f16.gguf",
		Reason:     "Fallback to MiniCPM projector (koboldcpp/mmproj)",
	},
}

func findProjectorFallback(modelID, filename string) *projectorFallback {
	var parts []string
	if modelID != "" {
		parts = append(parts, modelID)
		if idx := strings.LastIndex(modelID, "/"); idx >= 0 && idx < len(modelID)-1 {
			parts = append(parts, modelID[idx+1:])
		}
	}
	if filename != "" {
		parts = append(parts, filename)
		if stem := normalizedProjectorStem(filename); stem != "" {
			parts = append(parts, stem)
		}
	}
	haystack := strings.ToLower(strings.Join(parts, " "))
	for i := range defaultProjectorFallbacks {
		fb := &defaultProjectorFallbacks[i]
		for _, matcher := range fb.Matchers {
			if matcher == "" {
				continue
			}
			if strings.Contains(haystack, strings.ToLower(matcher)) {
				return fb
			}
		}
	}
	return nil
}

// GetProjectorFilename returns the expected local filename for a projector associated with the given model
func GetProjectorFilename(modelID, modelFilename string) string {
	fb := findProjectorFallback(modelID, modelFilename)
	if fb != nil {
		return filepath.Base(fb.Filename)
	}
	return ""
}
