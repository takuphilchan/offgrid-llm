package power

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPowerDefaultsAreInformational(t *testing.T) {
	m := NewPowerManager(DefaultPolicy())
	defer m.Stop()
	for _, state := range []PowerState{PowerAC, PowerBattery, PowerUnknown, PowerUPS} {
		for _, percent := range []int{1, 10, 19, 49, 79, 85, 100} {
			m.status = PowerStatus{State: state, BatteryPercent: percent, BatteryLevel: percentToLevel(percent)}
			if m.GetMaxContext() != m.policy.ACMaxContext || m.GetMaxConcurrent() != 10 || !m.ShouldEnableEmbeddings() || m.ShouldShutdown() {
				t.Fatalf("default restricted usage: %+v", m.status)
			}
		}
	}
}

func TestNativePowerSignals(t *testing.T) {
	for _, tc := range []struct {
		ac, flags, percent byte
		state              PowerState
		charging           bool
		level              BatteryLevel
	}{
		{1, 8, 79, PowerAC, true, BatteryGood}, {1, 1, 85, PowerAC, false, BatteryFull},
		{255, 255, 255, PowerUnknown, false, BatteryUnknown}, {1, 128, 255, PowerAC, false, BatteryUnknown},
	} {
		got := windowsPowerStatus(tc.ac, tc.flags, tc.percent, 0)
		if got.State != tc.state || got.IsCharging != tc.charging || got.BatteryLevel != tc.level {
			t.Fatalf("bad Windows state: %+v", got)
		}
	}
	for _, tc := range []struct {
		text     string
		state    PowerState
		charging bool
	}{
		{"Now drawing from 'AC Power'\n -InternalBattery-0 85%; charged;", PowerAC, false},
		{"Now drawing from 'Battery Power'\n -InternalBattery-0 20%; discharging;", PowerBattery, false},
		{"Now drawing from 'AC Power'\n -InternalBattery-0 79%; charging;", PowerAC, true},
		{"unavailable", PowerUnknown, false},
	} {
		got := parseDarwinPower(tc.text)
		if got.State != tc.state || got.IsCharging != tc.charging {
			t.Fatalf("bad macOS state: %+v", got)
		}
	}
}

func TestLinuxOnlineAdapterWinsAndUnknownStaysUnknown(t *testing.T) {
	dir := t.TempDir()
	if got := readLinuxPowerAt(dir); got.State != PowerUnknown {
		t.Fatal(got)
	}
	for name, fields := range map[string]map[string]string{
		"AC0": {"type": "Mains", "online": "1"}, "AC9": {"type": "Mains", "online": "0"},
		"BAT0": {"type": "Battery", "capacity": "85", "status": "Full"},
	} {
		if err := os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
		for field, value := range fields {
			if err := os.WriteFile(filepath.Join(dir, name, field), []byte(value), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got := readLinuxPowerAt(dir); got.State != PowerAC || got.IsCharging || got.BatteryPercent != 85 {
		t.Fatal(got)
	}
}
