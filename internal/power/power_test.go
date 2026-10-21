package power

import "testing"

func TestUnknownPowerStateDoesNotImposeBatteryContextCap(t *testing.T) {
	manager := NewPowerManager(DefaultPolicy())
	defer manager.Stop()
	if got := manager.GetMaxContext(); got != 1048576 {
		t.Fatalf("unknown platform context cap = %d, want normal-operation tier", got)
	}
}
