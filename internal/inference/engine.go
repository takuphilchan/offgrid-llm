package inference

import (
	"context"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// TokenCallback is called for each token during streaming
type TokenCallback func(token string) error

// Engine defines the interface for LLM inference backends
type Engine interface {
	// Load loads a model from the given path
	Load(ctx context.Context, modelPath string, opts LoadOptions) error

	// Unload unloads the currently loaded model
	Unload() error

	// ChatCompletion performs a chat completion
	ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error)

	// ChatCompletionStream performs a streaming chat completion
	ChatCompletionStream(ctx context.Context, req *api.ChatCompletionRequest, callback TokenCallback) error

	// Completion performs a text completion
	Completion(ctx context.Context, req *api.CompletionRequest) (*api.CompletionResponse, error)

	// IsLoaded returns whether a model is currently loaded
	IsLoaded() bool

	// GetModelInfo returns information about the loaded model
	GetModelInfo() (*ModelInfo, error)
}

// LoadOptions contains options for loading a model
type LoadOptions struct {
	ContextSize   int     // Context window size
	NumGPULayers  int     // Number of layers to offload to GPU
	NumThreads    int     // Number of CPU threads to use
	BatchSize     int     // Batch size for processing
	RopeFreqBase  float32 // RoPE frequency base
	RopeFreqScale float32 // RoPE frequency scale
	UseMlock      bool    // Use mlock to keep model in RAM
	UseMmap       bool    // Use mmap for faster loading
}

// ModelInfo contains information about a loaded model
type ModelInfo struct {
	VocabSize   int
	ContextSize int
	EmbedSize   int
	NumLayers   int
	NumHeads    int
}

// GenerationOptions contains options for text generation
type GenerationOptions struct {
	Temperature      float32
	TopP             float32
	TopK             int
	RepeatPenalty    float32
	PresencePenalty  float32
	FrequencyPenalty float32
	Seed             int
	StopSequences    []string
}

// DefaultLoadOptions returns default load options optimized for low-end hardware
func DefaultLoadOptions() LoadOptions {
	return LoadOptions{
		ContextSize:  4096,
		NumGPULayers: 0,     // CPU only by default
		NumThreads:   0,     // 0 = auto-detect based on CPU cores
		BatchSize:    256,   // Lower batch = faster time-to-first-token
		UseMlock:     false, // Don't lock RAM by default (safer for low RAM systems)
		UseMmap:      true,  // Memory-map for low RAM systems
	}
}

// DefaultGenerationOptions returns default generation options
func DefaultGenerationOptions() GenerationOptions {
	return GenerationOptions{
		Temperature:      0.7,
		TopP:             0.95,
		TopK:             40,
		RepeatPenalty:    1.1,
		PresencePenalty:  0.0,
		FrequencyPenalty: 0.0,
		Seed:             -1, // Random seed
	}
}
