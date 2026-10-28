package resource

import (
	"context"
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
)

func TestQualificationHardwareIdentity(t *testing.T) {
	ctx := context.Background()
	r := SystemResources{OS: "linux", Arch: "amd64", TotalRAM: 16384, CPUCores: 8, AvailableRAM: 10000}
	p := []cpu.InfoStat{{VendorID: "vendor", ModelName: "cpu", Flags: []string{"b", "a"}}}
	want, err := qualificationHardwareID(ctx, &r, p)
	if err != nil || len(want) != 64 {
		t.Fatal(want, err)
	}
	r.AvailableRAM = 500
	p[0].Flags = []string{"a", "b"}
	p = append(p, p[0])
	if got, err := qualificationHardwareID(ctx, &r, p); err != nil || got != want {
		t.Fatal("volatile data/order changed identity", got, err)
	}
	for _, change := range []func(*SystemResources){
		func(r *SystemResources) { r.TotalRAM++ },
		func(r *SystemResources) { r.CPUCores++ },
		func(r *SystemResources) { r.Arch = "arm64" },
		func(r *SystemResources) { r.GPUAvailable = true; r.GPUName = "gpu"; r.GPUMemory = 8192 },
	} {
		changed := r
		change(&changed)
		if got, err := qualificationHardwareID(ctx, &changed, p); err != nil || got == want {
			t.Fatal("hardware change not distinguished", got, err)
		}
	}
	p[0].ModelName = "other"
	if got, err := qualificationHardwareID(ctx, &r, p); err != nil || got == want {
		t.Fatal("CPU change not distinguished", got, err)
	}
	for _, invalid := range []*SystemResources{nil, {}, {OS: "linux", Arch: "amd64", TotalRAM: 1, CPUCores: 1, GPUAvailable: true}} {
		if _, err := qualificationHardwareID(ctx, invalid, p); err == nil {
			t.Fatal("incomplete identity accepted")
		}
	}
	if _, err := qualificationHardwareID(ctx, &r, nil); err == nil {
		t.Fatal("missing CPU accepted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := qualificationHardwareID(cancelled, &r, p); err != context.Canceled {
		t.Fatal(err)
	}
}
