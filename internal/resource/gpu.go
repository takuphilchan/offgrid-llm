package resource

// GPUMonitor monitors GPU usage (stub implementation)
type GPUMonitor struct {
available bool
}

// GPUInfo contains GPU information
type GPUInfo struct {
Name        string  `json:"name"`
MemoryUsed  int64   `json:"memory_used_mb"`
MemoryTotal int64   `json:"memory_total_mb"`
Utilization float64 `json:"utilization_percent"`
}

// NewGPUMonitor creates a new GPU monitor
func NewGPUMonitor() *GPUMonitor {
return &GPUMonitor{
available: false,
}
}

// GetGPUInfo returns current GPU information
func (g *GPUMonitor) GetGPUInfo() []GPUInfo {
// Stub: Return empty slice
// TODO: Implement NVIDIA/AMD GPU monitoring
return []GPUInfo{}
}

// IsAvailable returns true if GPU monitoring is available
func (g *GPUMonitor) IsAvailable() bool {
return g.available
}
