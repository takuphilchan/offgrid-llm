package inference

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestProviderSchedulerUncertainFactoryStopBlocksAllLaterResourceAdmission(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "fixture", AdapterRevision: "fixture", ModelDigest: "sha256:uncertain-start", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	var starts atomic.Int32
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		starts.Add(1)
		return nil, errors.Join(errors.New("fixture startup failed"), errProviderStopUncertain)
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	if _, err := scheduler.Runtime(context.Background(), descriptor); !errors.Is(err, errProviderStopUncertain) {
		t.Fatal("startup uncertainty was lost", err)
	}
	descriptor.ModelDigest = "sha256:another-start"
	if _, err := scheduler.Runtime(context.Background(), descriptor); err == nil {
		t.Fatal("second provider admitted after uncertain startup stop")
	}
	if starts.Load() != 1 {
		t.Fatal("new process factory ran despite unproven release", starts.Load())
	}
	if release, _, err := scheduler.AcquireGeneration(context.Background()); err == nil || release != nil {
		if release != nil {
			release()
		}
		t.Fatal("legacy allocation admitted after uncertain startup stop")
	}
}
