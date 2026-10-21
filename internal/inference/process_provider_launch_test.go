package inference

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestNativePythonWorkerUsesIsolatedInterpreter(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderMLX, Backend: "metal"}
	identity := []byte(`{"protocol_revision":"provider-v1"}`)
	dependencies := []byte(`{"mlx":"1.0","mlx-lm":"1.0","transformers":"1.0"}`)
	args := isolatedPythonWorkerArgs("/trusted/worker.py", "/models/test", descriptor, identity, dependencies)
	want := []string{"-I", "-B", "/trusted/worker.py", "--provider", "mlx", "--model", "/models/test", "--backend", "metal", "--descriptor", base64.RawURLEncoding.EncodeToString(identity), "--dependencies", base64.RawURLEncoding.EncodeToString(dependencies)}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("native worker could run without isolated Python imports: got %q, want %q", args, want)
	}
}

func TestProcessProviderConcurrentCloseWaitsForConfirmedRetirement(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderMLX, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText, CapabilityStreaming}}
	identity, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity)))
	if err != nil {
		t.Fatal(err)
	}
	var released atomic.Int32
	retiring := make(chan struct{})
	finish := make(chan struct{})
	provider.release = func() {
		close(retiring)
		<-finish
		released.Add(1)
	}
	first := make(chan error, 1)
	go func() { first <- provider.Close() }()
	select {
	case <-retiring:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not reach confirmed retirement")
	}
	second := make(chan error, 1)
	go func() { second <- provider.Close() }()
	select {
	case err := <-second:
		t.Fatalf("concurrent close returned before retirement settled: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(finish)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if released.Load() != 1 || provider.cmd.ProcessState == nil {
		t.Fatalf("package lease released %d times before process exit was confirmed", released.Load())
	}
}

func TestProcessProviderFailedRetirementKeepsPackageLease(t *testing.T) {
	descriptor := ProviderDescriptor{Provider: ProviderMLX, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText, CapabilityStreaming}}
	identity, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=TestProcessProviderStreamsAndCancelsWithoutBlocking"}, descriptor, append(os.Environ(), "OFFGRID_PROVIDER_WORKER_TEST=1", "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = provider.cmd.Process.Kill()
		_ = provider.cmd.Wait()
	}()
	var released atomic.Int32
	provider.release = func() { released.Add(1) }
	stopErr := errors.New("owned worker termination not confirmed")
	provider.terminate = func(*exec.Cmd) error { return stopErr }
	if err := provider.Close(); !errors.Is(err, stopErr) {
		t.Fatalf("uncertain process stop was reported as clean: %v", err)
	}
	if err := provider.Close(); !errors.Is(err, stopErr) {
		t.Fatalf("second close hid unresolved stop: %v", err)
	}
	if released.Load() != 0 {
		t.Fatal("package lease released while worker termination was unconfirmed")
	}
}
