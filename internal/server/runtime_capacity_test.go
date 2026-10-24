package server

import (
	"context"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"net/http"
	"testing"
	"time"
)

func TestLoadCapacityFailureDoesNotShrinkRequest(t *testing.T) {
	model := &api.ModelMetadata{Size: 4 << 30, ContextSize: 131072, KVElementsPerToken: 32768}
	for _, free := range []int64{0, 1024, 5000} {
		err := checkContextCapacity(model, &resource.SystemResources{TotalRAM: 10000, AvailableRAM: free}, 65536, "q4_0", 1, false)
		var capacity *contextCapacityError
		if !errors.As(err, &capacity) || capacity.requested != 65536 {
			t.Fatalf("capacity error: %v", err)
		}
		mapped := newServiceError(500, "load failed", err).(*serviceError)
		if mapped.status != http.StatusServiceUnavailable || mapped.code != "insufficient_runtime_capacity" {
			t.Fatal(mapped)
		}
	}
	if err := checkContextCapacity(model, &resource.SystemResources{TotalRAM: 10000, AvailableRAM: 9000}, 65536, "q4_0", 1, false); err != nil {
		t.Fatal(err)
	}
}

func TestLoadCapacityWaitsForReleasedMemoryWithoutChangingAllocation(t *testing.T) {
	calls := 0
	err := waitForLoadCapacity(context.Background(), time.Second, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return &contextCapacityError{65536, 32768}
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("calls=%d error=%v", calls, err)
	}
}

func TestLoadCapacityWaitIsBoundedAndCancellationSafe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := waitForLoadCapacity(ctx, time.Second, time.Millisecond, func() error {
		calls++
		cancel()
		return &contextCapacityError{65536, 32768}
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal(calls, err)
	}
	err = waitForLoadCapacity(context.Background(), time.Millisecond, time.Second, func() error {
		return &contextCapacityError{65536, 32768}
	})
	var capacity *contextCapacityError
	if !errors.As(err, &capacity) || capacity.requested != 65536 {
		t.Fatal(err)
	}
	failure := errors.New("measurement unavailable")
	calls = 0
	err = waitForLoadCapacity(context.Background(), time.Second, time.Millisecond, func() error { calls++; return failure })
	if !errors.Is(err, failure) || calls != 1 {
		t.Fatal(calls, err)
	}
}

func TestFullGPUDoesNotCountTotalVRAMAsAvailable(t *testing.T) {
	model := &api.ModelMetadata{Size: 4 << 30, KVElementsPerToken: 32768}
	resources := &resource.SystemResources{TotalRAM: 10000, AvailableRAM: 1024, GPUAvailable: true, GPUMemory: 24000, GPUAvailableMemory: 0}
	if err := checkContextCapacity(model, resources, 65536, "q4_0", 1, true); err == nil {
		t.Fatal("full GPU admitted as empty")
	}
	resources.GPUAvailableMemory = 12000
	if err := checkContextCapacity(model, resources, 65536, "q4_0", 1, true); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticPlacementAdmitsHybridContextButFixedPlacementDoesNot(t *testing.T) {
	model := &api.ModelMetadata{Size: 5100 << 20, KVElementsPerToken: 65536}
	resources := &resource.SystemResources{TotalRAM: 10000, AvailableRAM: 8500, GPUAvailable: true, GPUMemory: 8151, GPUAvailableMemory: 7800}
	if err := checkContextCapacity(model, resources, 65536, "q8_0", 1, true, false); err == nil {
		t.Fatal("fixed placement assumed hybrid memory")
	}
	if err := checkContextCapacity(model, resources, 65536, "q8_0", 1, true, true); err != nil {
		t.Fatal(err)
	}
	resources.GPUAvailableMemory = 512
	if err := checkContextCapacity(model, resources, 65536, "q8_0", 1, true, true); err == nil {
		t.Fatal("unavailable GPU inflated hybrid capacity")
	}
}
