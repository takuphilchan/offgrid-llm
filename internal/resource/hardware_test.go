package resource

import "testing"

func TestDetectResourcesReportsUsableMemory(t *testing.T) {
	resources, err := DetectResources()
	if err != nil {
		t.Fatal(err)
	}
	if resources.TotalRAM <= 0 || resources.AvailableRAM <= 0 || resources.AvailableRAM > resources.TotalRAM {
		t.Fatalf("invalid memory snapshot: %+v", resources)
	}
	if resources.CPUCores < 1 || resources.PhysicalCores < 1 {
		t.Fatalf("invalid CPU snapshot: %+v", resources)
	}
	if resources.GPUAvailable && (resources.GPUMemory <= 0 || resources.GPUAvailableMemory < 0 || resources.GPUAvailableMemory > resources.GPUMemory) {
		t.Fatalf("invalid GPU snapshot: %+v", resources)
	}
}
