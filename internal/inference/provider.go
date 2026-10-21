package inference

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

// ProviderID identifies a native inference implementation. It is persisted in
// model metadata and is deliberately not inferred from a file extension.
type ProviderID string

const (
	ProviderProtocolRevision            = "provider-v1"
	ProviderLlamaCPP         ProviderID = "llama_cpp"
	ProviderMLX              ProviderID = "mlx"
	ProviderTransformers     ProviderID = "transformers"
)

// ModelFormat identifies the on-disk model representation.
type ModelFormat string

const (
	FormatGGUF           ModelFormat = "gguf"
	FormatMLXSafetensors ModelFormat = "mlx_safetensors"
	FormatSafetensors    ModelFormat = "safetensors"
)

// ProviderCapability is intentionally small; model-family adapters provide the
// more detailed template and modality qualification.
type ProviderCapability string

const (
	CapabilityText      ProviderCapability = "text"
	CapabilityToolCalls ProviderCapability = "tool_calls"
	CapabilityVision    ProviderCapability = "vision"
	CapabilityAudio     ProviderCapability = "audio"
	CapabilityStreaming ProviderCapability = "streaming"
)

// ProviderDescriptor is returned during the handshake and stored with
// capability evidence. All identity fields are required for qualification.
type ProviderDescriptor struct {
	ProtocolRevision string               `json:"protocol_revision"`
	Provider         ProviderID           `json:"provider"`
	RunnerRevision   string               `json:"runner_revision"`
	AdapterRevision  string               `json:"adapter_revision,omitempty"`
	ModelDigest      string               `json:"model_digest"`
	Format           ModelFormat          `json:"format"`
	Backend          string               `json:"backend"`
	Capabilities     []ProviderCapability `json:"capabilities"`
	ContextLength    int                  `json:"context_length,omitempty"`
	TemplateDigest   string               `json:"template_digest,omitempty"`
	ProjectorDigest  string               `json:"projector_digest,omitempty"`
	PreprocessDigest string               `json:"preprocess_digest,omitempty"`
}

func completeSHA256Digest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") {
		return false
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && len(decoded) == 32 && value == strings.ToLower(value)
}

func (d ProviderDescriptor) Validate() error {
	if d.ProtocolRevision != ProviderProtocolRevision || strings.TrimSpace(d.RunnerRevision) == "" || strings.TrimSpace(d.ModelDigest) == "" {
		return errors.New("provider descriptor requires protocol, runner, and model identity")
	}
	switch d.Provider {
	case ProviderLlamaCPP, ProviderMLX, ProviderTransformers:
	default:
		return fmt.Errorf("unsupported provider %q", d.Provider)
	}
	switch d.Format {
	case FormatGGUF, FormatMLXSafetensors, FormatSafetensors:
	default:
		return fmt.Errorf("unsupported model format %q", d.Format)
	}
	if d.Provider == ProviderLlamaCPP && d.Format != FormatGGUF || d.Provider == ProviderMLX && d.Format != FormatMLXSafetensors || d.Provider == ProviderTransformers && d.Format != FormatSafetensors {
		return errors.New("provider and model format are incompatible")
	}
	if d.Provider != ProviderLlamaCPP && (d.AdapterRevision == "" || d.Backend == "") {
		return errors.New("native provider requires pinned adapter and backend")
	}
	if d.ContextLength < 0 {
		return errors.New("provider context length cannot be negative")
	}
	seen := map[ProviderCapability]bool{}
	for _, c := range d.Capabilities {
		switch c {
		case CapabilityText, CapabilityToolCalls, CapabilityVision, CapabilityAudio, CapabilityStreaming:
		default:
			return fmt.Errorf("unknown provider capability %q", c)
		}
		if seen[c] {
			return fmt.Errorf("duplicate or empty provider capability %q", c)
		}
		seen[c] = true
	}
	if d.ProjectorDigest != "" && !completeSHA256Digest(d.ProjectorDigest) || d.PreprocessDigest != "" && !completeSHA256Digest(d.PreprocessDigest) {
		return errors.New("perception identities must be complete SHA-256 digests")
	}
	if seen[CapabilityVision] && (d.ProjectorDigest == "" || d.PreprocessDigest == "" || d.TemplateDigest == "") {
		return errors.New("vision capability requires projector, preprocessing, and template identities")
	}
	if seen[CapabilityAudio] && d.PreprocessDigest == "" {
		return errors.New("audio capability requires a preprocessing identity")
	}
	return nil
}

type ProviderRequest struct {
	RequestID      string          `json:"request_id"`
	CancellationID string          `json:"cancellation_id"`
	ModelDigest    string          `json:"model_digest"`
	Prompt         json.RawMessage `json:"prompt"`
	Tools          json.RawMessage `json:"tools,omitempty"`
	ContextLength  int             `json:"context_length,omitempty"`
	// These limits belong to the scheduler, not the worker wire protocol. The
	// worker still receives its model-specific limit inside Prompt.
	MaxOutputTokens int `json:"-"`
	MaxOutputBytes  int `json:"-"`
}

func (r ProviderRequest) Validate() error {
	if strings.TrimSpace(r.RequestID) == "" || strings.TrimSpace(r.CancellationID) == "" || strings.TrimSpace(r.ModelDigest) == "" {
		return errors.New("provider request requires request, cancellation, and model identity")
	}
	if len(r.Prompt) == 0 || len(r.Prompt) > 8<<20 || jsonwire.Validate(r.Prompt) != nil {
		return errors.New("provider request prompt must be unambiguous JSON")
	}
	if len(r.Tools) > 1<<20 || len(r.Tools) != 0 && jsonwire.Validate(r.Tools) != nil {
		return errors.New("provider request tool menu must be unambiguous bounded JSON")
	}
	if r.ContextLength < 0 {
		return errors.New("provider request context length cannot be negative")
	}
	if r.MaxOutputTokens < 0 || r.MaxOutputTokens > 1<<20 || r.ContextLength > 0 && r.MaxOutputTokens > r.ContextLength {
		return errors.New("provider request output token budget is invalid")
	}
	if r.MaxOutputBytes < 0 || r.MaxOutputBytes > 8<<20 {
		return errors.New("provider request output byte budget is invalid")
	}
	return nil
}

type ProviderEventType string

const (
	ProviderEventReady ProviderEventType = "ready"
	ProviderEventDelta ProviderEventType = "delta"
	ProviderEventTool  ProviderEventType = "tool_decision"
	ProviderEventUsage ProviderEventType = "usage"
	ProviderEventDone  ProviderEventType = "done"
	ProviderEventError ProviderEventType = "error"
)

type ProviderEvent struct {
	ProtocolRevision string              `json:"protocol_revision"`
	Type             ProviderEventType   `json:"type"`
	RequestID        string              `json:"request_id"`
	Sequence         uint64              `json:"sequence"`
	Descriptor       *ProviderDescriptor `json:"descriptor,omitempty"`
	Resources        *ProviderResources  `json:"resources,omitempty"`
	Text             string              `json:"text,omitempty"`
	FinishReason     string              `json:"finish_reason,omitempty"`
	Tool             json.RawMessage     `json:"tool,omitempty"`
	Usage            *UsageMetrics       `json:"usage,omitempty"`
	Error            *ProviderError      `json:"error,omitempty"`
}

// ProviderResources describes observed runtime placement. Zero memory with
// MemoryMeasured=false is not a claim that the model occupies no memory.
type ProviderResources struct {
	Backend          string `json:"backend"`
	Device           string `json:"device"`
	MemoryBytes      int64  `json:"memory_bytes,omitempty"`
	MemoryLimitBytes int64  `json:"memory_limit_bytes,omitempty"`
	MemoryMeasured   bool   `json:"memory_measured"`
}

func (r ProviderResources) Validate(descriptor ProviderDescriptor) error {
	if r.Backend != descriptor.Backend || strings.TrimSpace(r.Device) == "" || len(r.Device) > 256 || r.MemoryBytes < 0 || r.MemoryLimitBytes < 0 {
		return errors.New("provider resource report does not match selected backend or has invalid memory")
	}
	if r.MemoryMeasured && r.MemoryBytes == 0 {
		return errors.New("measured provider memory cannot be zero")
	}
	if r.MemoryMeasured && r.MemoryLimitBytes > 0 && r.MemoryBytes > r.MemoryLimitBytes {
		return errors.New("provider memory use exceeds the reported device limit")
	}
	if !r.MemoryMeasured && (r.MemoryBytes != 0 || r.MemoryLimitBytes != 0) {
		return errors.New("unmeasured provider memory cannot carry estimated bytes")
	}
	return nil
}

type UsageMetrics struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type ProviderError struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Operational bool   `json:"operational"`
}

// ProviderRuntime is the native lifecycle boundary. Implementations may be
// local HTTP workers, loopback processes, or the existing llama server.
type ProviderRuntime interface {
	Descriptor(context.Context) (ProviderDescriptor, error)
	Resources(context.Context) (ProviderResources, error)
	Generate(context.Context, ProviderRequest, func(ProviderEvent) error) error
	Cancel(context.Context, string) error
	Health(context.Context) error
	Close() error
}

// ProviderFactory creates a runtime after the model and provider have passed
// admission. Factories are registered by trusted service code only.
type ProviderFactory func(context.Context, ProviderDescriptor) (ProviderRuntime, error)

type ProviderRegistry struct {
	mu        sync.RWMutex
	factories map[ProviderID]ProviderFactory
}

func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{factories: map[ProviderID]ProviderFactory{}}
}

func (r *ProviderRegistry) Register(id ProviderID, factory ProviderFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id == "" || factory == nil {
		return errors.New("provider registration requires an id and factory")
	}
	if _, ok := r.factories[id]; ok {
		return fmt.Errorf("provider %q already registered", id)
	}
	r.factories[id] = factory
	return nil
}

func (r *ProviderRegistry) Open(ctx context.Context, descriptor ProviderDescriptor) (ProviderRuntime, error) {
	if err := descriptor.Validate(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	factory, ok := r.factories[descriptor.Provider]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("provider %q is unavailable", descriptor.Provider)
	}
	return factory(ctx, descriptor)
}
