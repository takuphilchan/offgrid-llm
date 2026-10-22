package power

import "testing"

func TestUnknownPowerStateDoesNotImposeBatteryContextCap(t *testing.T) {
	manager := NewPowerManager(DefaultPolicy())
	defer manager.Stop()
	if got := manager.GetMaxContext(); got != 1048576 {
		t.Fatalf("unknown platform context cap = %d, want normal-operation tier", got)
	}
}

func TestDefaultBatteryPolicyDoesNotShrinkModelContext(t *testing.T) {
	manager := NewPowerManager(DefaultPolicy())
	defer manager.Stop()
	for _, level := range []BatteryLevel{BatteryFull, BatteryGood, BatteryLow} {
		manager.mu.Lock()
		manager.status = PowerStatus{State: PowerBattery, BatteryLevel: level, BatteryPercent: 85}
		manager.mu.Unlock()
		if got := manager.GetMaxContext(); got != 1048576 {
			t.Errorf("battery level %s context cap = %d, want normal-operation tier", level, got)
		}
		want := manager.policy.BatteryMaxConcurrent
		if level == BatteryLow {
			want = manager.policy.LowBatteryMaxConcurrent
		}
		if got := manager.GetMaxConcurrent(); got != want {
			t.Errorf("battery level %s concurrency = %d, want %d", level, got, want)
		}
	}
}
