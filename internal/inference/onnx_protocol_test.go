package inference

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"math"
	"os"
	"strings"
	"testing"
)

func onnxDescriptorFixture() ProviderDescriptor {
	return ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderONNX, RunnerRevision: "worker-v1", AdapterRevision: "bert-symmetric-v1", ModelDigest: "sha256:" + strings.Repeat("a", 64), Format: FormatONNX, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityEmbeddings}, ContextLength: 8, PreprocessDigest: "sha256:" + strings.Repeat("b", 64)}
}

func TestONNXProviderDescriptorAndVectorBounds(t *testing.T) {
	d := onnxDescriptorFixture()
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"chat", "gpu", "missing-preprocessing", "format", "context"} {
		bad := d
		switch scenario {
		case "chat":
			bad.Capabilities = []ProviderCapability{CapabilityText}
		case "gpu":
			bad.Backend = "cuda"
		case "missing-preprocessing":
			bad.PreprocessDigest = ""
		case "format":
			bad.Format = FormatGGUF
		case "context":
			bad.ContextLength = 8193
		}
		if bad.Validate() == nil {
			t.Fatalf("unsupported descriptor %s accepted", scenario)
		}
	}
	for _, vectors := range [][][]float32{nil, {{0, 0}}, {{1}, {1, 2}}, {{float32(math.NaN())}}, {{float32(math.Inf(1))}}, make([][]float32, 33)} {
		if validateONNXEmbeddingVectors(vectors) == nil {
			t.Fatal("invalid vectors accepted")
		}
	}
	if validateProviderEvent(ProviderEvent{Type: ProviderEventDone, FinishReason: "stop", Embeddings: [][]float32{{1}}}) == nil {
		t.Fatal("hidden embeddings accepted")
	}
	for _, raw := range []string{`{"input":["one"],"input":["two"]}`, `{"input":["one"]}{"input":["two"]}`} {
		if _, err := validateONNXEmbeddingPrompt(ProviderRequest{ContextLength: d.ContextLength, Prompt: json.RawMessage(raw)}, d); err == nil {
			t.Fatal("standalone prompt validator accepted ambiguous JSON")
		}
	}
}

func TestONNXProcessProviderUsesTypedResultsAndPerInputContext(t *testing.T) {
	if os.Getenv("OFFGRID_ONNX_PROTOCOL_FIXTURE") == "1" {
		onnxProtocolWorker()
		return
	}
	for _, mode := range []string{"valid", "over-context", "text", "ragged", "zero", "wrong-provider"} {
		t.Run(mode, func(t *testing.T) {
			d := onnxDescriptorFixture()
			if mode == "wrong-provider" {
				d.Provider = ProviderTransformers
				d.Format = FormatSafetensors
				d.Capabilities = []ProviderCapability{CapabilityText}
			}
			raw, _ := json.Marshal(d)
			p, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=^TestONNXProcessProviderUsesTypedResultsAndPerInputContext$"}, d, append(os.Environ(), "OFFGRID_ONNX_PROTOCOL_FIXTURE=1", "OFFGRID_ONNX_PROTOCOL_MODE="+mode, "OFFGRID_ONNX_PROTOCOL_DESCRIPTOR="+string(raw)))
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			req := ProviderRequest{RequestID: "batch", CancellationID: "batch", ModelDigest: d.ModelDigest, ContextLength: 8, Prompt: json.RawMessage(`{"input":["one","two"]}`)}
			count := 0
			err = p.Generate(context.Background(), req, func(event ProviderEvent) error { count++; return nil })
			if mode == "valid" {
				if err != nil || count != 2 {
					t.Fatalf("valid batch failed: %v count=%d", err, count)
				}
			} else if err == nil {
				t.Fatal("invalid result accepted")
			}
		})
	}
}

func onnxProtocolWorker() {
	var descriptor ProviderDescriptor
	_ = json.Unmarshal([]byte(os.Getenv("OFFGRID_ONNX_PROTOCOL_DESCRIPTOR")), &descriptor)
	encoder, decoder := json.NewEncoder(os.Stdout), json.NewDecoder(os.Stdin)
	_ = encoder.Encode(ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventReady, Descriptor: &descriptor, Resources: &ProviderResources{Backend: descriptor.Backend, Device: "fixture"}})
	var input struct {
		Operation string          `json:"operation"`
		Request   ProviderRequest `json:"request"`
	}
	if decoder.Decode(&input) != nil {
		return
	}
	event := ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventEmbeddings, RequestID: input.Request.RequestID, Sequence: 1, Embeddings: [][]float32{{1, 0}, {0, 1}}}
	mode := os.Getenv("OFFGRID_ONNX_PROTOCOL_MODE")
	switch mode {
	case "text":
		event.Type = ProviderEventDelta
		event.Text = "unexpected"
		event.Embeddings = nil
	case "ragged":
		event.Embeddings[1] = []float32{1}
	case "zero":
		event.Embeddings[0] = []float32{0, 0}
	case "wrong-count":
		event.Embeddings = event.Embeddings[:1]
	}
	_ = encoder.Encode(event)
	if mode == "duplicate" {
		event.Sequence++
		_ = encoder.Encode(event)
	}
	tokens := 12
	if mode == "over-context" {
		tokens = 17
	}
	done := ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDone, RequestID: input.Request.RequestID, Sequence: 3, FinishReason: "stop", Usage: &UsageMetrics{PromptTokens: tokens, TotalTokens: tokens}}
	if mode == "missing-usage" {
		done.Usage = nil
	}
	if mode == "length" {
		done.FinishReason = "length"
	}
	_ = encoder.Encode(done)
	_ = decoder.Decode(&input)
}

func TestONNXEmbeddingWithholdsResultsUntilValidCompletion(t *testing.T) {
	for _, mode := range []string{"valid", "wrong-count", "duplicate", "missing-usage", "length", "over-context", "ragged", "zero"} {
		t.Run(mode, func(t *testing.T) {
			descriptor := onnxDescriptorFixture()
			raw, _ := json.Marshal(descriptor)
			provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=^TestONNXProcessProviderUsesTypedResultsAndPerInputContext$"}, descriptor, append(os.Environ(), "OFFGRID_ONNX_PROTOCOL_FIXTURE=1", "OFFGRID_ONNX_PROTOCOL_MODE="+mode, "OFFGRID_ONNX_PROTOCOL_DESCRIPTOR="+string(raw)))
			if err != nil {
				t.Fatal(err)
			}
			defer provider.Close()
			impl := &ONNXEmbeddingImpl{runtime: provider, descriptor: descriptor, settings: models.ONNXEmbeddingSettings{Dimensions: 2, MaxTokens: 8, NormalizeL2: true}}
			vectors, err := impl.Embed(context.Background(), []string{"first", "second"})
			if mode == "valid" {
				if err != nil || len(vectors) != 2 || impl.UsageTokens() != 12 {
					t.Fatalf("valid vectors rejected: %v", err)
				}
			} else if err == nil || vectors != nil {
				t.Fatalf("partial invalid result escaped: vectors=%v err=%v", vectors, err)
			}
		})
	}
}

func TestONNXReservedSelectorNeverFallsBackToLegacyRuntime(t *testing.T) {
	engine := NewEmbeddingEngine()
	called := false
	engine.newImplementation = func() (EmbeddingImpl, error) { called = true; return nil, errors.New("legacy factory") }
	if err := engine.Load(context.Background(), "embedding:fixture@v1", DefaultEmbeddingOptions()); err == nil || called {
		t.Fatalf("reserved selector fell through: %v called=%v", err, called)
	}
}
