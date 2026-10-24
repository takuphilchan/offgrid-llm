package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"time"
)

type contextCapacityError struct{ requested, available int }

func (e *contextCapacityError) Error() string {
	return fmt.Sprintf("insufficient memory for the requested %d-token allocation (estimated available capacity: %d); free memory and retry, or explicitly select a smaller context/model; configuration was not changed", e.requested, e.available)
}

func (s *Server) checkModelLoadCapacity(ctx context.Context, modelID string, requested int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	metadata, err := s.registry.GetChatModel(modelID)
	if err != nil {
		return err
	}
	return waitForLoadCapacity(ctx, 3*time.Second, 100*time.Millisecond, func() error {
		resources, err := resource.DetectResources()
		if err != nil {
			return fmt.Errorf("cannot measure memory before model loading: %w", err)
		}
		return checkContextCapacity(metadata, resources, requested, s.config.KVCacheType, s.config.InferenceSlots, s.config.EnableGPU, s.config.EnableGPU && s.config.NumGPULayers == 0)
	})
}

// The previous process has exited, but GPU drivers can report its allocation
// briefly afterward. Re-measure before rejecting the next load. This retries
// only observation: no model load, generation or external action is replayed.
func waitForLoadCapacity(ctx context.Context, budget, interval time.Duration, check func() error) error {
	deadline := time.NewTimer(budget)
	defer deadline.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := check()
		var capacity *contextCapacityError
		if !errors.As(err, &capacity) {
			return err
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-deadline.C:
			timer.Stop()
			return err
		case <-timer.C:
		}
	}
}

func checkContextCapacity(model *api.ModelMetadata, resources *resource.SystemResources, requested int, cache string, slots int, gpu bool, automaticGPU ...bool) error {
	capacity := estimateContextCapacity(model, resources, cache, slots, gpu, automaticGPU...)
	if requested > capacity {
		return &contextCapacityError{requested, capacity}
	}
	return nil
}
