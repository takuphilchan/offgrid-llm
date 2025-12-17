// Package power provides power and battery awareness for edge devices.
// Essential for deployments on battery-powered or solar-powered devices.
package power

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PowerState represents the current power state
type PowerState string

const (
	PowerAC      PowerState = "ac"      // On AC power
	PowerBattery PowerState = "battery" // On battery
	PowerUPS     PowerState = "ups"     // On UPS (battery backup)
	PowerSolar   PowerState = "solar"   // On solar power
	PowerUnknown PowerState = "unknown"
)

// BatteryLevel represents battery charge level
type BatteryLevel string

const (
	BatteryFull     BatteryLevel = "full"     // >80%
	BatteryGood     BatteryLevel = "good"     // 50-80%
	BatteryLow      BatteryLevel = "low"      // 20-50%
	BatteryCritical BatteryLevel = "critical" // <20%
	BatteryUnknown  BatteryLevel = "unknown"
)

// PowerStatus contains current power information
type PowerStatus struct {
	State           PowerState   `json:"state"`
	BatteryLevel    BatteryLevel `json:"battery_level"`
	BatteryPercent  int          `json:"battery_percent"`
	IsCharging      bool         `json:"is_charging"`
	TimeRemaining   int          `json:"time_remaining_minutes"` // Estimated time remaining
	PowerSaveActive bool         `json:"power_save_active"`
	Temperature     float64      `json:"temperature_celsius,omitempty"` // Battery temp
}

// PowerPolicy defines behavior based on power state
type PowerPolicy struct {
	// AC power settings
	ACMaxConcurrent    int  `json:"ac_max_concurrent"`
	ACMaxContext       int  `json:"ac_max_context"`
	ACEnableEmbeddings bool `json:"ac_enable_embeddings"`

	// Battery settings
	BatteryMaxConcurrent    int  `json:"battery_max_concurrent"`
	BatteryMaxContext       int  `json:"battery_max_context"`
	BatteryEnableEmbeddings bool `json:"battery_enable_embeddings"`

	// Low battery settings
	LowBatteryMaxConcurrent    int  `json:"low_battery_max_concurrent"`
	LowBatteryMaxContext       int  `json:"low_battery_max_context"`
	LowBatteryEnableEmbeddings bool `json:"low_battery_enable_embeddings"`

	// Critical battery settings
	CriticalBatteryMaxConcurrent int  `json:"critical_max_concurrent"`
	CriticalBatteryShutdown      bool `json:"critical_shutdown"`
	CriticalBatteryPercent       int  `json:"critical_percent"`
}

// DefaultPolicy returns a sensible default power policy
func DefaultPolicy() PowerPolicy {
	return PowerPolicy{
		ACMaxConcurrent:    10,
		ACMaxContext:       8192,
		ACEnableEmbeddings: true,

		BatteryMaxConcurrent:    5,
		BatteryMaxContext:       4096,
		BatteryEnableEmbeddings: true,

		LowBatteryMaxConcurrent:    2,
		LowBatteryMaxContext:       2048,
		LowBatteryEnableEmbeddings: false,

		CriticalBatteryMaxConcurrent: 1,
		CriticalBatteryShutdown:      true,
		CriticalBatteryPercent:       10,
	}
}

// PowerManager monitors power state and adjusts behavior
type PowerManager struct {
	mu        sync.RWMutex
	status    PowerStatus
	policy    PowerPolicy
	callbacks []func(PowerStatus)
	ctx       context.Context
	cancel    context.CancelFunc
	interval  time.Duration
}

// NewPowerManager creates a new power manager
func NewPowerManager(policy PowerPolicy) *PowerManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &PowerManager{
		policy:   policy,
		ctx:      ctx,
		cancel:   cancel,
		interval: 30 * time.Second,
	}
}

// Start starts power monitoring
func (pm *PowerManager) Start() {
	// Initial check
	pm.updateStatus()

	// Start monitoring loop
	go pm.monitorLoop()
}

// Stop stops power monitoring
func (pm *PowerManager) Stop() {
	pm.cancel()
}

// Status returns current power status
func (pm *PowerManager) Status() PowerStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.status
}

// OnChange registers a callback for power state changes
func (pm *PowerManager) OnChange(callback func(PowerStatus)) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.callbacks = append(pm.callbacks, callback)
}

// GetMaxConcurrent returns max concurrent requests for current power state
func (pm *PowerManager) GetMaxConcurrent() int {
	status := pm.Status()

	if status.State == PowerAC {
		return pm.policy.ACMaxConcurrent
	}

	switch status.BatteryLevel {
	case BatteryCritical:
		return pm.policy.CriticalBatteryMaxConcurrent
	case BatteryLow:
		return pm.policy.LowBatteryMaxConcurrent
	default:
		return pm.policy.BatteryMaxConcurrent
	}
}

// GetMaxContext returns max context size for current power state
func (pm *PowerManager) GetMaxContext() int {
	status := pm.Status()

	if status.State == PowerAC {
		return pm.policy.ACMaxContext
	}

	switch status.BatteryLevel {
	case BatteryCritical:
		return pm.policy.LowBatteryMaxContext / 2
	case BatteryLow:
		return pm.policy.LowBatteryMaxContext
	default:
		return pm.policy.BatteryMaxContext
	}
}

// ShouldEnableEmbeddings returns whether embeddings should be enabled
func (pm *PowerManager) ShouldEnableEmbeddings() bool {
	status := pm.Status()

	if status.State == PowerAC {
		return pm.policy.ACEnableEmbeddings
	}

	switch status.BatteryLevel {
	case BatteryCritical, BatteryLow:
		return pm.policy.LowBatteryEnableEmbeddings
	default:
		return pm.policy.BatteryEnableEmbeddings
	}
}

// ShouldShutdown returns whether the system should shut down
func (pm *PowerManager) ShouldShutdown() bool {
	status := pm.Status()

	if status.State == PowerAC || status.IsCharging {
		return false
	}

	return pm.policy.CriticalBatteryShutdown &&
		status.BatteryPercent > 0 &&
		status.BatteryPercent <= pm.policy.CriticalBatteryPercent
}

// IsPowerSaveRecommended returns whether power save mode is recommended
func (pm *PowerManager) IsPowerSaveRecommended() bool {
	status := pm.Status()
	return status.State == PowerBattery && status.BatteryLevel != BatteryFull
}

// monitorLoop continuously monitors power status
func (pm *PowerManager) monitorLoop() {
	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			oldStatus := pm.Status()
			pm.updateStatus()
			newStatus := pm.Status()

			// Check for significant changes
			if oldStatus.State != newStatus.State ||
				oldStatus.BatteryLevel != newStatus.BatteryLevel {
				pm.notifyCallbacks(newStatus)
			}
		}
	}
}

// updateStatus updates the power status from system
func (pm *PowerManager) updateStatus() {
	status := pm.readSystemPower()

	pm.mu.Lock()
	pm.status = status
	pm.mu.Unlock()
}

// notifyCallbacks notifies all registered callbacks
func (pm *PowerManager) notifyCallbacks(status PowerStatus) {
	pm.mu.RLock()
	callbacks := make([]func(PowerStatus), len(pm.callbacks))
	copy(callbacks, pm.callbacks)
	pm.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(status)
	}
}

// readSystemPower reads power information from the system
func (pm *PowerManager) readSystemPower() PowerStatus {
	status := PowerStatus{
		State:        PowerUnknown,
		BatteryLevel: BatteryUnknown,
	}

	// Try Linux sysfs
	if stat := pm.readLinuxPower(); stat.State != PowerUnknown {
		return stat
	}

	// Try /proc/acpi (older systems)
	if stat := pm.readACPIPower(); stat.State != PowerUnknown {
		return stat
	}

	return status
}

// readLinuxPower reads power info from Linux sysfs
func (pm *PowerManager) readLinuxPower() PowerStatus {
	status := PowerStatus{
		State:        PowerUnknown,
		BatteryLevel: BatteryUnknown,
	}

	powerSupplyPath := "/sys/class/power_supply"
	entries, err := os.ReadDir(powerSupplyPath)
	if err != nil {
		return status
	}

	for _, entry := range entries {
		typePath := filepath.Join(powerSupplyPath, entry.Name(), "type")
		typeBytes, err := os.ReadFile(typePath)
		if err != nil {
			continue
		}

		supplyType := strings.TrimSpace(string(typeBytes))

		if supplyType == "Mains" || strings.HasPrefix(entry.Name(), "AC") {
			// AC adapter
			onlinePath := filepath.Join(powerSupplyPath, entry.Name(), "online")
			if data, err := os.ReadFile(onlinePath); err == nil {
				if strings.TrimSpace(string(data)) == "1" {
					status.State = PowerAC
				} else {
					status.State = PowerBattery
				}
			}
		}

		if supplyType == "Battery" {
			// Battery
			capacityPath := filepath.Join(powerSupplyPath, entry.Name(), "capacity")
			if data, err := os.ReadFile(capacityPath); err == nil {
				if percent, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
					status.BatteryPercent = percent
					status.BatteryLevel = percentToLevel(percent)
				}
			}

			// Check if charging
			statusPath := filepath.Join(powerSupplyPath, entry.Name(), "status")
			if data, err := os.ReadFile(statusPath); err == nil {
				batteryStatus := strings.TrimSpace(string(data))
				status.IsCharging = batteryStatus == "Charging"
			}

			// Temperature (if available)
			tempPath := filepath.Join(powerSupplyPath, entry.Name(), "temp")
			if data, err := os.ReadFile(tempPath); err == nil {
				if temp, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64); err == nil {
					status.Temperature = temp / 10.0 // Usually in deciCelsius
				}
			}

			// Time remaining
			timePath := filepath.Join(powerSupplyPath, entry.Name(), "time_to_empty_now")
			if data, err := os.ReadFile(timePath); err == nil {
				if seconds, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
					status.TimeRemaining = seconds / 60
				}
			}
		}
	}

	return status
}

// readACPIPower reads power from /proc/acpi (legacy)
func (pm *PowerManager) readACPIPower() PowerStatus {
	status := PowerStatus{
		State:        PowerUnknown,
		BatteryLevel: BatteryUnknown,
	}

	// Check AC adapter
	acPath := "/proc/acpi/ac_adapter"
	if entries, err := os.ReadDir(acPath); err == nil && len(entries) > 0 {
		statePath := filepath.Join(acPath, entries[0].Name(), "state")
		if file, err := os.Open(statePath); err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "on-line") {
					status.State = PowerAC
				} else if strings.Contains(line, "off-line") {
					status.State = PowerBattery
				}
			}
		}
	}

	// Check battery
	battPath := "/proc/acpi/battery"
	if entries, err := os.ReadDir(battPath); err == nil && len(entries) > 0 {
		statePath := filepath.Join(battPath, entries[0].Name(), "state")
		if file, err := os.Open(statePath); err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "remaining capacity:") {
					parts := strings.Fields(line)
					if len(parts) >= 3 {
						if val, err := strconv.Atoi(parts[2]); err == nil {
							// This is mAh, we need to also read full capacity
							status.BatteryPercent = val // Simplified
						}
					}
				}
			}
		}
	}

	return status
}

// percentToLevel converts battery percentage to level
func percentToLevel(percent int) BatteryLevel {
	switch {
	case percent > 80:
		return BatteryFull
	case percent > 50:
		return BatteryGood
	case percent > 20:
		return BatteryLow
	default:
		return BatteryCritical
	}
}

// PowerSaveSettings returns recommended settings for power save mode
func (pm *PowerManager) PowerSaveSettings() map[string]interface{} {
	status := pm.Status()

	settings := map[string]interface{}{
		"max_concurrent":    pm.GetMaxConcurrent(),
		"max_context":       pm.GetMaxContext(),
		"enable_embeddings": pm.ShouldEnableEmbeddings(),
		"power_state":       status.State,
		"battery_level":     status.BatteryLevel,
		"battery_percent":   status.BatteryPercent,
	}

	// Additional savings for low battery
	if status.BatteryLevel == BatteryLow || status.BatteryLevel == BatteryCritical {
		settings["reduce_batch_size"] = true
		settings["disable_p2p"] = true
		settings["disable_metrics"] = true
	}

	return settings
}
