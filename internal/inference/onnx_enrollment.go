package inference

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func (e *EmbeddingEngine) createEmbeddingImplementation(selector string) (EmbeddingImpl, error) {
	if e.newModelImplementation != nil {
		impl, handled, err := e.newModelImplementation(selector)
		if handled || err != nil {
			return impl, err
		}
	}
	if strings.HasPrefix(selector, models.ONNXEmbeddingPrefix) {
		return nil, errors.New("ONNX embedding runtime is not enrolled")
	}
	return e.newImplementation()
}

// ConfigureONNXEmbeddingRuntime is startup-only trusted installation wiring.
// No worker is started, model selected, or knowledge index changed here.
func (e *EmbeddingEngine) ConfigureONNXEmbeddingRuntime(store *models.PackageStore, config ONNXEmbeddingRuntimeConfig) error {
	if store == nil {
		return errors.New("ONNX embeddings require the shared package store")
	}
	if err := config.Validate(); err != nil {
		return err
	}
	// Detach operator-owned maps so later mutation cannot change a loaded identity.
	raw, _ := json.Marshal(config)
	var captured ONNXEmbeddingRuntimeConfig
	if err := json.Unmarshal(raw, &captured); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.loaded || e.implementation != nil || e.newModelImplementation != nil {
		return errors.New("embedding runtime enrollment must precede model use")
	}
	e.newModelImplementation = func(selector string) (EmbeddingImpl, bool, error) {
		if !strings.HasPrefix(selector, models.ONNXEmbeddingPrefix) {
			return nil, false, nil
		}
		if _, _, err := models.ParseONNXEmbeddingSelector(selector); err != nil {
			return nil, true, err
		}
		return &ONNXEmbeddingImpl{store: store, config: captured}, true, nil
	}
	return nil
}
