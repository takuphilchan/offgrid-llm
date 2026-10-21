package models

import (
	"errors"
	"fmt"
	"slices"
	"sync"
)

// ModelAdapterSpec is trusted service configuration, never model-supplied
// executable code. It defines the smallest contract needed to format a model
// request and normalize its response.
type ModelAdapterSpec struct {
	ID                  string   `json:"id"`
	Revision            string   `json:"revision"`
	Engine              string   `json:"engine"`
	Formats             []string `json:"formats"`
	Modalities          []string `json:"modalities"`
	ToolCalling         string   `json:"tool_calling"`
	Template            string   `json:"template"`
	Tokenization        string   `json:"tokenization"`
	Streaming           string   `json:"streaming"`
	ToolEnvelope        string   `json:"tool_envelope"`
	Preprocessing       []string `json:"preprocessing,omitempty"`
	MaxQualifiedContext int      `json:"max_qualified_context,omitempty"`
	RemoteCodeAllowed   bool     `json:"remote_code_allowed"`
}

func (s ModelAdapterSpec) Validate() error {
	if s.ID == "" || s.Revision == "" || s.Engine == "" || len(s.Formats) == 0 || s.Template == "" {
		return errors.New("adapter requires id, revision, engine, format, and template")
	}
	if !validPackageComponent(s.ID) || !validPackageComponent(s.Revision) || slices.Contains([]string{"main", "master", "latest"}, s.Revision) {
		return errors.New("adapter requires a portable pinned identity")
	}
	if !slices.Contains([]string{EngineLlamaCPP, EngineMLX, EngineTransformers}, s.Engine) {
		return fmt.Errorf("unsupported adapter engine %q", s.Engine)
	}
	expected := FormatGGUF
	if s.Engine == EngineMLX {
		expected = FormatMLXSafetensors
	} else if s.Engine == EngineTransformers {
		expected = FormatSafetensors
	}
	if len(s.Formats) != 1 || s.Formats[0] != expected {
		return errors.New("adapter format does not match its provider")
	}
	if s.ToolCalling != "unsupported" && s.ToolCalling != "unqualified" {
		return errors.New("native adapter tool calling is not qualified")
	}
	// The native worker currently implements exactly one reviewed text path:
	// apply the pinned chat template, count its exact token IDs, then stream
	// text deltas. An adapter cannot opt into an unimplemented tokenizer,
	// streaming format, or executable tool envelope by changing package data.
	if s.Tokenization != "template_exact_ids" || s.Streaming != "text_delta_v1" || s.ToolEnvelope != "none" {
		return errors.New("native adapter requests an unsupported text or tool envelope")
	}
	if s.RemoteCodeAllowed {
		return errors.New("remote model code is not allowed")
	}
	if s.MaxQualifiedContext < 0 {
		return errors.New("adapter context limit cannot be negative")
	}
	if !slices.Contains(s.Modalities, "text") {
		return errors.New("text adapter must declare text capability")
	}
	seen := map[string]bool{}
	for _, modality := range s.Modalities {
		if !slices.Contains([]string{"text", "vision", "audio"}, modality) || seen[modality] {
			return fmt.Errorf("unsupported or duplicate adapter modality %q", modality)
		}
		seen[modality] = true
	}
	return nil
}

type AdapterRegistry struct {
	mu      sync.RWMutex
	entries map[adapterKey]ModelAdapterSpec
}

type adapterKey struct {
	id       string
	revision string
}

func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{entries: map[adapterKey]ModelAdapterSpec{}}
}

func (r *AdapterRegistry) Register(spec ModelAdapterSpec) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := adapterKey{spec.ID, spec.Revision}
	if _, exists := r.entries[key]; exists {
		return fmt.Errorf("adapter %q already registered", spec.ID+"@"+spec.Revision)
	}
	spec.Formats = slices.Clone(spec.Formats)
	spec.Modalities = slices.Clone(spec.Modalities)
	spec.Preprocessing = slices.Clone(spec.Preprocessing)
	r.entries[key] = spec
	return nil
}

func (r *AdapterRegistry) Resolve(id, revision string) (ModelAdapterSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, ok := r.entries[adapterKey{id, revision}]
	spec.Formats = slices.Clone(spec.Formats)
	spec.Modalities = slices.Clone(spec.Modalities)
	spec.Preprocessing = slices.Clone(spec.Preprocessing)
	return spec, ok
}

// MatchPackage is an activation boundary, not an import-time requirement.
// Unknown families may be retained as inspectable metadata, but never run.
func (r *AdapterRegistry) MatchPackage(m ModelPackageManifest) (ModelAdapterSpec, error) {
	if err := m.Validate(); err != nil {
		return ModelAdapterSpec{}, err
	}
	spec, ok := r.Resolve(m.ModelFamily, m.AdapterRevision)
	if !ok {
		return ModelAdapterSpec{}, errors.New("model family has no trusted pinned adapter")
	}
	if spec.Engine != m.Engine || !slices.Contains(spec.Formats, m.Format) || spec.MaxQualifiedContext < m.ContextLength || !slices.Contains(spec.Modalities, "text") {
		return ModelAdapterSpec{}, errors.New("model package exceeds trusted adapter capabilities")
	}
	for _, modality := range m.Modalities {
		if modality != "text" {
			return ModelAdapterSpec{}, errors.New("native non-text modalities require separate qualification")
		}
	}
	return spec, nil
}
