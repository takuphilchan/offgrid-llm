package server

import (
	"context"
	"fmt"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

// retireLegacyForNative runs only after the shared scheduler has excluded
// llama generations. Keep the existing model cache as process owner until its
// lifecycle is fully migrated, but do not leave its processes resident while
// an unqualified native provider loads.
func (s *Server) retireLegacyForNative(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	// A separate model operation may hold this lock while loading or deleting.
	// Stop must cancel the waiting native request without waiting for that
	// unrelated operation to finish.
	for !s.modelMutex.TryLock() {
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return false, ctx.Err()
		case <-timer.C:
		}
	}
	defer s.modelMutex.Unlock()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	process := s.llamaProcess
	if process == nil {
		if s.providerScheduler != nil {
			return false, fmt.Errorf("scheduler-owned llama process is unavailable")
		}
		process = inference.NewLlamaCPPProcess(s.modelCache, s.engine)
	}
	retired, err := process.RetireAll(ctx)
	if err != nil {
		return retired, err
	}
	if s.currentModelID != "" && s.inferenceLifecycle != nil {
		s.inferenceLifecycle.Invalidate(s.currentModelID)
	}
	s.currentModelID = ""
	s.currentContext = 0
	s.currentPort = 0
	s.currentGeneration = 0
	s.syncRegistryLoadState()
	return retired, nil
}
