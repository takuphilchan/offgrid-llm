package models

import (
	"fmt"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"os"
	"path/filepath"
)

func (r *Registry) listONNXEmbeddingModels() []api.Model {
	// Legacy read-only diagnostics must not create model or package directories.
	info, statErr := os.Lstat(filepath.Join(r.modelsDir, PackageDirectory))
	if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	states, err := r.packages.List()
	if err != nil {
		return nil
	}
	result := []api.Model{}
	for _, state := range states {
		m := state.Manifest
		if !state.Installed || state.Integrity == "failed" || m == nil || m.Architecture != "onnx-embedding" || m.Validate() != nil {
			continue
		}
		var size int64
		for _, artifact := range m.Artifacts {
			size += artifact.Size
		}
		result = append(result, api.Model{ID: ONNXEmbeddingSelector(m.ID, m.Revision), Object: "model", OwnedBy: "offgrid-llm", Type: "embedding", Size: size, SizeGB: fmt.Sprintf("%.0f MB", float64(size)/(1024*1024)), ContextWindow: m.ContextLength, ContextLength: m.ContextLength, Capabilities: []string{"embeddings"}, CapabilityStatus: map[string]string{"embeddings": "installed_unqualified"}, Engine: EngineONNX, Format: FormatONNX, Backend: "cpu", RunnerRevision: m.RunnerRevision, AdapterRevision: m.AdapterRevision})
	}
	return result
}

func (r *Registry) getONNXEmbeddingMetadata(selector string) (*api.ModelMetadata, error) {
	if _, _, err := ParseONNXEmbeddingSelector(selector); err != nil {
		return nil, err
	}
	for _, model := range r.listONNXEmbeddingModels() {
		if model.ID == selector {
			return &api.ModelMetadata{ID: selector, Name: selector, Path: selector, Size: model.Size, Format: FormatONNX, ContextSize: model.ContextLength, Type: "embedding"}, nil
		}
	}
	return nil, fmt.Errorf("installed ONNX embedding package not found: %s", selector)
}
