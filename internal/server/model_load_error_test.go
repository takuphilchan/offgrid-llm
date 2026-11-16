package server

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

func TestModelLoadRecoveryPreservesSpecificCauses(t *testing.T) {
	for _, tc := range []struct {
		cause  error
		code   string
		status int
	}{
		{errors.New("worker exited before readiness"), "model_load_failed", 503},
		{&contextCapacityError{requested: 8192, available: 4096}, "insufficient_runtime_capacity", 503},
		{inference.ErrInferenceQueueFull, "inference_queue_full", 429},
		{context.Canceled, "request_cancelled", 499},
		{context.DeadlineExceeded, "inference_timeout", 504},
	} {
		t.Run(tc.code, func(t *testing.T) {
			err := modelLoadError(fmt.Errorf("acquire model: %w", tc.cause))
			var service *serviceError
			if !errors.As(err, &service) || service.code != tc.code || service.status != tc.status || !errors.Is(err, tc.cause) {
				t.Fatalf("wrong recovery: %#v", err)
			}
		})
	}
}
