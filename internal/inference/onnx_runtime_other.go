//go:build !windows && !linux

package inference

import (
	"context"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func startOwnedONNXEmbedding(_ context.Context, lease *models.ONNXEmbeddingLease, _ ONNXEmbeddingRuntimeConfig, _ int) (ProviderRuntime, models.ONNXEmbeddingSettings, string, error) {
	lease.Release()
	return nil, models.ONNXEmbeddingSettings{}, "", errors.New("owned ONNX runtime is unavailable on this platform: immutable package and process containment qualification is required")
}
