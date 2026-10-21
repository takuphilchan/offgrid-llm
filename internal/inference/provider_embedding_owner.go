package inference

import "errors"

// AttachEmbeddingEngine gives the shared scheduler final ownership of the
// embedding runtime. It is a startup-only transfer; model loading remains
// behind the engine's existing admission and identity checks.
func (s *ProviderScheduler) AttachEmbeddingEngine(engine *EmbeddingEngine) error {
	if s == nil || engine == nil {
		return errors.New("embedding engine and provider scheduler are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.embeddingOwner != nil || len(s.requests) != 0 || len(s.resident) != 0 || len(s.loading) != 0 || s.legacyRuns != 0 || s.nativeRun {
		return errors.New("embedding engine can only be attached before provider activity")
	}
	if engine.IsLoaded() {
		return errors.New("embedding engine must be idle before scheduler attachment")
	}
	if err := engine.SetProviderScheduler(s); err != nil {
		return err
	}
	s.embeddingOwner = engine
	return nil
}
