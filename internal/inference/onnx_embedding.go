package inference

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync/atomic"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

// ONNXEmbeddingImpl uses the same owned worker and EmbeddingEngine admission
// as other inference. Its factory does not launch or acquire model resources.
type ONNXEmbeddingImpl struct {
	store      *models.PackageStore
	config     ONNXEmbeddingRuntimeConfig
	runtime    ProviderRuntime
	settings   models.ONNXEmbeddingSettings
	descriptor ProviderDescriptor
	identity   string
	usage      int
	startupErr error
	start      func(context.Context, *models.ONNXEmbeddingLease, ONNXEmbeddingRuntimeConfig, int) (ProviderRuntime, models.ONNXEmbeddingSettings, string, error)
}

func (e *ONNXEmbeddingImpl) Load(ctx context.Context, selector string, opts EmbeddingOptions) error {
	if e.runtime != nil || e.startupErr != nil {
		return errors.New("ONNX embedding implementation already owns a runtime")
	}
	id, revision, err := models.ParseONNXEmbeddingSelector(selector)
	if err != nil {
		return err
	}
	lease, err := e.store.AcquireONNXEmbedding(ctx, id, revision, "embedding-engine")
	if err != nil {
		return err
	}
	start := e.start
	if start == nil {
		start = startOwnedONNXEmbedding
	}
	// The starter takes ownership of the model lease even on failure.
	runtime, settings, identity, err := start(ctx, lease, e.config, opts.NumThreads)
	e.runtime = runtime
	if err != nil {
		if errors.Is(err, errProviderStopUncertain) {
			e.startupErr = err
		}
		return err
	}
	if runtime == nil {
		return errors.New("ONNX embedding starter returned no owned runtime")
	}
	descriptor, err := runtime.Descriptor(ctx)
	if err != nil {
		return err
	}
	if err := descriptor.Validate(); err != nil {
		return err
	}
	if descriptor.Provider != ProviderONNX || settings.MaxTokens != descriptor.ContextLength || settings.Dimensions < 1 || identity == "" {
		return errors.New("ONNX embedding runtime identity mismatch")
	}
	if opts.PoolingMethod != "" && opts.PoolingMethod != settings.Pooling {
		return errors.New("pooling is pinned by the installed ONNX package")
	}
	e.settings, e.descriptor, e.identity = settings, descriptor, identity
	return nil
}

var onnxEmbeddingRequestID atomic.Uint64

func (e *ONNXEmbeddingImpl) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	e.usage = 0
	if e.runtime == nil || len(texts) == 0 || e.settings.Dimensions < 1 || e.settings.Dimensions > 4096 {
		return nil, errors.New("ONNX embedding runtime is not loaded")
	}
	batchSize := min(32, maxONNXEmbeddingValues/e.settings.Dimensions)
	result := make([][]float32, 0, len(texts))
	for offset := 0; offset < len(texts); {
		count, prompt, err := nextONNXEmbeddingBatch(texts[offset:], batchSize)
		if err != nil {
			return nil, err
		}
		batch := texts[offset : offset+count]
		offset += count
		id := "embedding-" + strconv.FormatUint(onnxEmbeddingRequestID.Add(1), 10)
		request := ProviderRequest{RequestID: id, CancellationID: id, ModelDigest: e.descriptor.ModelDigest, ContextLength: e.descriptor.ContextLength, Prompt: prompt}
		if _, err := validateONNXEmbeddingPrompt(request, e.descriptor); err != nil {
			return nil, err
		}
		var vectors [][]float32
		var usage *UsageMetrics
		terminal := false
		err = e.runtime.Generate(ctx, request, func(event ProviderEvent) error {
			switch event.Type {
			case ProviderEventEmbeddings:
				if vectors != nil || terminal || len(event.Embeddings) != len(batch) {
					return errors.New("ONNX embedding batch cardinality or event order changed")
				}
				if err := validateONNXEmbeddingVectors(event.Embeddings); err != nil {
					return err
				}
				for _, vector := range event.Embeddings {
					if len(vector) != e.settings.Dimensions {
						return errors.New("ONNX embedding dimensions changed")
					}
					if e.settings.NormalizeL2 {
						var norm float64
						for _, v := range vector {
							norm += float64(v) * float64(v)
						}
						if math.Abs(norm-1) > 1e-4 {
							return errors.New("ONNX embedding normalization changed")
						}
					}
				}
				vectors = event.Embeddings
			case ProviderEventDone:
				if terminal || event.FinishReason != "stop" || vectors == nil || event.Usage == nil || event.Usage.CompletionTokens != 0 || event.Usage.PromptTokens < len(batch) || event.Usage.TotalTokens != event.Usage.PromptTokens || event.Usage.TotalTokens > len(batch)*e.settings.MaxTokens {
					return errors.New("ONNX embedding completion is incomplete")
				}
				terminal = true
				copy := *event.Usage
				usage = &copy
			default:
				return errors.New("ONNX embedding worker emitted an unexpected result")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if !terminal || usage == nil || ctx.Err() != nil {
			return nil, errors.Join(errors.New("ONNX embedding result did not complete"), ctx.Err())
		}
		// No partial vectors leave the implementation before every batch has a
		// validated terminal event and the complete call succeeds.
		result = append(result, vectors...)
		e.usage += usage.TotalTokens
	}
	return result, nil
}

func (e *ONNXEmbeddingImpl) Unload() error {
	if e.runtime != nil {
		if err := e.runtime.Close(); err != nil {
			return err
		}
		e.runtime = nil
	}
	return e.startupErr
}
func (e *ONNXEmbeddingImpl) GetDimensions() int { return e.settings.Dimensions }
func (e *ONNXEmbeddingImpl) Identity() string   { return e.identity }
func (e *ONNXEmbeddingImpl) UsageTokens() int   { return e.usage }

func onnxEmbeddingDescriptor(lease *models.ONNXEmbeddingLease, settingsRaw []byte) ProviderDescriptor {
	digest := sha256.Sum256(settingsRaw)
	return ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderONNX, RunnerRevision: lease.Manifest.RunnerRevision, AdapterRevision: lease.Manifest.AdapterRevision, ModelDigest: lease.ModelDigest, Format: FormatONNX, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityEmbeddings}, ContextLength: lease.Manifest.ContextLength, PreprocessDigest: "sha256:" + hex.EncodeToString(digest[:])}
}

func onnxEmbeddingIdentity(descriptor ProviderDescriptor, config ONNXEmbeddingRuntimeConfig) string {
	// Location is checked independently at every load. Moving identical
	// installed bytes or private writable state must not invalidate a restored
	// Knowledge index; relative worker selection and all byte/version pins stay
	// in the identity, so genuinely changed execution cannot reuse old vectors.
	config.RuntimeDirectory, config.StateDirectory, config.NamespaceLauncher = "", "", ""
	raw, _ := json.Marshal(struct {
		Descriptor ProviderDescriptor
		Runtime    ONNXEmbeddingRuntimeConfig
	}{descriptor, config})
	digest := sha256.Sum256(raw)
	return fmt.Sprintf("onnx-embedding:sha256:%x", digest[:])
}
