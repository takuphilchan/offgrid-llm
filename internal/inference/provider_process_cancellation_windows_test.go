package inference

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Cancel only after real protocol bytes enter the owned worker's pipe. A timer
// started before a large request is built/validated may instead cancel queued
// admission, which must not retire a runtime used by another request.
type providerCancellationWriteWitness struct {
	io.WriteCloser
	written chan struct{}
	once    sync.Once
}

func (w *providerCancellationWriteWitness) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	n, err := w.WriteCloser.Write(data[:1])
	if n == 1 {
		w.once.Do(func() { close(w.written) })
	}
	if err != nil {
		return n, err
	}
	m, err := w.WriteCloser.Write(data[n:])
	return n + m, err
}

func TestProviderDescendantCancellationHelper(t *testing.T) {
	role := os.Getenv("OFFGRID_PROVIDER_DESCENDANT_TEST")
	if role == "" {
		return
	}
	if role == "child" {
		if err := os.WriteFile(os.Getenv("OFFGRID_PROVIDER_DESCENDANT_PID"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			t.Fatal(err)
		}
		for {
			time.Sleep(time.Second)
		}
	}
	child := exec.Command(os.Args[0], "-test.run=^TestProviderDescendantCancellationHelper$")
	child.Env = append(os.Environ(), "OFFGRID_PROVIDER_DESCENDANT_TEST=child")
	child.Stdout = os.Stdout // This pipe deliberately survives its parent.
	configureBackgroundProcess(child)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	var descriptor ProviderDescriptor
	if err := json.Unmarshal([]byte(os.Getenv("OFFGRID_PROVIDER_WORKER_DESCRIPTOR")), &descriptor); err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventReady, Descriptor: &descriptor, Resources: &ProviderResources{Backend: descriptor.Backend, Device: "descendant fixture", MemoryBytes: 1024, MemoryMeasured: true}}); err != nil {
		t.Fatal(err)
	}
	if role == "blocked-write" {
		for {
			time.Sleep(time.Second)
		}
	}
	decoder := json.NewDecoder(os.Stdin)
	for {
		var message struct {
			Operation string          `json:"operation"`
			Request   ProviderRequest `json:"request"`
		}
		if err := decoder.Decode(&message); err != nil {
			return
		}
		if message.Operation == "generate" {
			if role == "malformed" {
				_, _ = os.Stdout.Write([]byte("invalid protocol\n"))
				continue
			}
			_ = encoder.Encode(ProviderEvent{ProtocolRevision: ProviderProtocolRevision, Type: ProviderEventDelta, RequestID: message.Request.RequestID, Sequence: 1, Text: "started"})
		}
		// Ignore cancellation and health to force owned-tree retirement.
	}
}

func TestProcessProviderCancellationRetiresLiveDescendantBeforeReturning(t *testing.T) {
	for _, mode := range []string{"generate", "health", "blocked-write", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			descriptor := ProviderDescriptor{Provider: ProviderMLX, ProtocolRevision: ProviderProtocolRevision, RunnerRevision: "test", AdapterRevision: "test", ModelDigest: "sha256:test", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText, CapabilityStreaming}}
			identity, _ := json.Marshal(descriptor)
			pidPath := filepath.Join(t.TempDir(), "owned-child.pid")
			env := append(os.Environ(), "OFFGRID_PROVIDER_DESCENDANT_TEST="+mode, "OFFGRID_PROVIDER_DESCENDANT_PID="+pidPath, "OFFGRID_PROVIDER_WORKER_DESCRIPTOR="+string(identity))
			provider, err := StartProcessProvider(context.Background(), os.Args[0], []string{"-test.run=^TestProviderDescendantCancellationHelper$"}, descriptor, env)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = provider.Close() })
			var child windows.Handle
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if raw, err := os.ReadFile(pidPath); err == nil {
					if pid, err := strconv.Atoi(string(raw)); err == nil {
						child, err = windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
						if err != nil {
							t.Fatal(err)
						}
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			if child == 0 {
				t.Fatal("owned child never started")
			}
			t.Cleanup(func() { _ = windows.TerminateProcess(child, 1); _ = windows.CloseHandle(child) })
			prompt := json.RawMessage(`{"prompt":"hello"}`)
			if mode == "blocked-write" {
				prompt = json.RawMessage(`{"prompt":"` + strings.Repeat("x", 4<<20) + `"}`)
			}
			timeout := 100 * time.Millisecond
			if mode == "blocked-write" {
				timeout = 5 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			wantCancellation := error(context.DeadlineExceeded)
			if mode == "blocked-write" {
				written := make(chan struct{})
				provider.stdin = &providerCancellationWriteWitness{WriteCloser: provider.stdin, written: written}
				wantCancellation = context.Canceled
				go func() {
					select {
					case <-written:
						cancel()
					case <-ctx.Done():
					}
				}()
			}
			started := time.Now()
			if mode == "health" {
				err = provider.Health(ctx)
			} else {
				req := ProviderRequest{RequestID: "descendant-request", CancellationID: "descendant-cancel", ModelDigest: descriptor.ModelDigest, Prompt: prompt}
				err = provider.Generate(ctx, req, func(ProviderEvent) error { return nil })
			}
			if err == nil || mode != "malformed" && !errors.Is(err, wantCancellation) || time.Since(started) > 5*time.Second {
				t.Fatalf("retirement outcome=%v elapsed=%v", err, time.Since(started))
			}
			if status, err := windows.WaitForSingleObject(child, 0); err != nil || status != windows.WAIT_OBJECT_0 {
				t.Fatalf("provider returned with live descendant: %d %v", status, err)
			}
			if _, err := provider.Resources(context.Background()); err == nil {
				t.Fatal("retired provider still exposed resident resources")
			}
		})
	}
}
