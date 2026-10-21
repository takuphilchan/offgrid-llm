package inference

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func testProviderDescriptor() ProviderDescriptor {
	return ProviderDescriptor{
		ProtocolRevision: ProviderProtocolRevision,
		Provider:         ProviderTransformers,
		RunnerRevision:   "fixture-1",
		AdapterRevision:  "fixture-1",
		ModelDigest:      "sha256:test",
		Format:           FormatSafetensors,
		Backend:          "cpu",
		Capabilities:     []ProviderCapability{CapabilityText, CapabilityStreaming},
	}
}

func TestProviderDescriptorValidation(t *testing.T) {
	if err := testProviderDescriptor().Validate(); err != nil {
		t.Fatal(err)
	}
	bad := testProviderDescriptor()
	bad.Provider = "unknown"
	if err := bad.Validate(); err == nil {
		t.Fatal("unknown provider accepted")
	}
}

func TestProviderResourcesRejectsMismatchedOrUnmeasuredClaims(t *testing.T) {
	descriptor := testProviderDescriptor()
	valid := ProviderResources{Backend: "cpu", Device: "fixture", MemoryBytes: 1024, MemoryMeasured: true}
	if err := valid.Validate(descriptor); err != nil {
		t.Fatal(err)
	}
	for _, changed := range []ProviderResources{
		{Backend: "cuda", Device: "fixture"},
		{Backend: "cpu", Device: ""},
		{Backend: "cpu", Device: "fixture", MemoryBytes: -1, MemoryMeasured: true},
		{Backend: "cpu", Device: "fixture", MemoryBytes: 1024, MemoryMeasured: false},
		{Backend: "cpu", Device: "fixture", MemoryMeasured: true},
		{Backend: "cpu", Device: "fixture", MemoryBytes: 2048, MemoryLimitBytes: 1024, MemoryMeasured: true},
	} {
		if err := changed.Validate(descriptor); err == nil {
			t.Fatalf("invalid resource claim accepted: %+v", changed)
		}
	}
}

func TestProviderDescriptorKeepsPerceptionSeparateFromText(t *testing.T) {
	d := testProviderDescriptor()
	d.Capabilities = append(d.Capabilities, CapabilityVision)
	if err := d.Validate(); err == nil {
		t.Fatal("text capability implicitly qualified vision")
	}
	d.ProjectorDigest = "sha256:" + strings.Repeat("a", 64)
	d.PreprocessDigest = "sha256:" + strings.Repeat("b", 64)
	d.TemplateDigest = "pinned-template"
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	d.PreprocessDigest = "sha256:unknown"
	if err := d.Validate(); err == nil {
		t.Fatal("incomplete preprocessing identity accepted")
	}
	audio := testProviderDescriptor()
	audio.Capabilities = append(audio.Capabilities, CapabilityAudio)
	if err := audio.Validate(); err == nil {
		t.Fatal("text capability implicitly qualified audio")
	}
}

func TestProviderRequestValidation(t *testing.T) {
	r := ProviderRequest{RequestID: "r1", CancellationID: "c1", ModelDigest: "sha256:test", Prompt: json.RawMessage(`{"messages":[]}`)}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	r.Prompt = json.RawMessage(`{`)
	if err := r.Validate(); err == nil {
		t.Fatal("invalid prompt accepted")
	}
	r.Prompt = json.RawMessage(`{"messages":[],"messages":[{"role":"user","content":"override"}]}`)
	if err := r.Validate(); err == nil {
		t.Fatal("ambiguous prompt keys accepted")
	}
	r.Prompt = json.RawMessage(`{"messages":[{"role":"user","content":"\ud800"}]}`)
	if err := r.Validate(); err == nil {
		t.Fatal("malformed Unicode in prompt accepted")
	}
	r.Prompt = json.RawMessage(`{"messages":[]}`)
	r.Tools = json.RawMessage(`{`)
	if err := r.Validate(); err == nil {
		t.Fatal("invalid tool menu accepted")
	}
	r.Tools = json.RawMessage(`{"name":"safe","name":"unsafe"}`)
	if err := r.Validate(); err == nil {
		t.Fatal("ambiguous tool menu accepted")
	}
	r.Tools = nil
	r.ContextLength = -1
	if err := r.Validate(); err == nil {
		t.Fatal("negative context accepted")
	}
}

type fixtureProvider struct{ descriptor ProviderDescriptor }

func (f fixtureProvider) Descriptor(context.Context) (ProviderDescriptor, error) {
	return f.descriptor, nil
}
func (f fixtureProvider) Resources(context.Context) (ProviderResources, error) {
	return ProviderResources{Backend: f.descriptor.Backend, Device: "fixture"}, nil
}
func (f fixtureProvider) Generate(ctx context.Context, req ProviderRequest, cb func(ProviderEvent) error) error {
	if err := req.Validate(); err != nil {
		return err
	}
	return cb(ProviderEvent{ProtocolRevision: f.descriptor.ProtocolRevision, Type: ProviderEventDone, RequestID: req.RequestID, Sequence: 1})
}
func (f fixtureProvider) Cancel(context.Context, string) error { return nil }
func (f fixtureProvider) Health(context.Context) error         { return nil }
func (f fixtureProvider) Close() error                         { return nil }

func TestProviderRegistryOpensRegisteredFactoryOnly(t *testing.T) {
	r := NewProviderRegistry()
	if err := r.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		return fixtureProvider{testProviderDescriptor()}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Open(context.Background(), testProviderDescriptor()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Open(context.Background(), func() ProviderDescriptor { d := testProviderDescriptor(); d.Provider = ProviderMLX; return d }()); err == nil {
		t.Fatal("unregistered provider opened")
	}
}
