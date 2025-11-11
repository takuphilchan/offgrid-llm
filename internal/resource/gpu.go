package resource

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GPUType represents the type of GPU
type GPUType string

const (
	GPUTypeNVIDIA  GPUType = "nvidia"
	GPUTypeAMD     GPUType = "amd"
	GPUTypeUnknown GPUType = "unknown"
)

// GPUMonitor monitors GPU usage
type GPUMonitor struct {
	mu        sync.RWMutex
	gpuType   GPUType
	available bool
	gpus      []GPUInfo
	lastCheck time.Time
}

// GPUInfo contains GPU information
type GPUInfo struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	MemoryUsed  int64   `json:"memory_used_mb"`
	MemoryTotal int64   `json:"memory_total_mb"`
	MemoryFree  int64   `json:"memory_free_mb"`
	Utilization float64 `json:"utilization_percent"`
	Temperature int     `json:"temperature_celsius,omitempty"`
}

// NewGPUMonitor creates a new GPU monitor
func NewGPUMonitor() *GPUMonitor {
	monitor := &GPUMonitor{
		available: false,
		gpuType:   GPUTypeUnknown,
		gpus:      []GPUInfo{},
	}
	monitor.detectGPU()
	return monitor
}

// detectGPU detects available GPU type
func (g *GPUMonitor) detectGPU() {
	// Try NVIDIA first
	if g.detectNVIDIA() {
		g.gpuType = GPUTypeNVIDIA
		g.available = true
		return
	}

	// Try AMD/ROCm
	if g.detectAMD() {
		g.gpuType = GPUTypeAMD
		g.available = true
		return
	}

	g.gpuType = GPUTypeUnknown
	g.available = false
}

// detectNVIDIA checks for NVIDIA GPU via nvidia-smi
func (g *GPUMonitor) detectNVIDIA() bool {
	cmd := exec.Command("nvidia-smi", "--query-gpu=index,name,memory.total", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return len(lines) > 0 && lines[0] != ""
}

// detectAMD checks for AMD GPU via rocm-smi
func (g *GPUMonitor) detectAMD() bool {
	cmd := exec.Command("rocm-smi", "--showproductname")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), "GPU")
}

// GetGPUInfo returns current GPU information
func (g *GPUMonitor) GetGPUInfo() []GPUInfo {
	if !g.available {
		return []GPUInfo{}
	}

	g.mu.RLock()
	// Return cached data if less than 1 second old
	if time.Since(g.lastCheck) < time.Second {
		defer g.mu.RUnlock()
		return g.gpus
	}
	g.mu.RUnlock()

	// Refresh GPU info
	g.refreshGPUInfo()

	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.gpus
}

// refreshGPUInfo updates GPU information
func (g *GPUMonitor) refreshGPUInfo() {
	var gpus []GPUInfo

	switch g.gpuType {
	case GPUTypeNVIDIA:
		gpus = g.getNVIDIAInfo()
	case GPUTypeAMD:
		gpus = g.getAMDInfo()
	}

	g.mu.Lock()
	g.gpus = gpus
	g.lastCheck = time.Now()
	g.mu.Unlock()
}

// getNVIDIAInfo gets NVIDIA GPU information via nvidia-smi
func (g *GPUMonitor) getNVIDIAInfo() []GPUInfo {
	cmd := exec.Command("nvidia-smi",
		"--query-gpu=index,name,memory.used,memory.total,memory.free,utilization.gpu,temperature.gpu",
		"--format=csv,noheader,nounits")

	output, err := cmd.Output()
	if err != nil {
		return []GPUInfo{}
	}

	var gpus []GPUInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}

		id, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		name := strings.TrimSpace(parts[1])
		memUsed, _ := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
		memTotal, _ := strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
		memFree, _ := strconv.ParseInt(strings.TrimSpace(parts[4]), 10, 64)
		util, _ := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)

		temp := 0
		if len(parts) >= 7 {
			temp, _ = strconv.Atoi(strings.TrimSpace(parts[6]))
		}

		gpus = append(gpus, GPUInfo{
			ID:          id,
			Name:        name,
			MemoryUsed:  memUsed,
			MemoryTotal: memTotal,
			MemoryFree:  memFree,
			Utilization: util,
			Temperature: temp,
		})
	}

	return gpus
}

// getAMDInfo gets AMD GPU information via rocm-smi
func (g *GPUMonitor) getAMDInfo() []GPUInfo {
	// Get basic info
	cmd := exec.Command("rocm-smi", "--showproductname")
	output, err := cmd.Output()
	if err != nil {
		return []GPUInfo{}
	}

	// Parse GPU names
	var gpus []GPUInfo
	re := regexp.MustCompile(`GPU\[(\d+)\].*?: (.+)`)
	matches := re.FindAllStringSubmatch(string(output), -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		id, _ := strconv.Atoi(match[1])
		name := strings.TrimSpace(match[2])

		gpu := GPUInfo{
			ID:   id,
			Name: name,
		}

		// Get memory info for this GPU
		memCmd := exec.Command("rocm-smi", "-d", strconv.Itoa(id), "--showmeminfo", "vram")
		memOutput, err := memCmd.Output()
		if err == nil {
			gpu.MemoryTotal = parseROCmMemory(string(memOutput), "Total")
			gpu.MemoryUsed = parseROCmMemory(string(memOutput), "Used")
			gpu.MemoryFree = gpu.MemoryTotal - gpu.MemoryUsed
		}

		// Get utilization
		utilCmd := exec.Command("rocm-smi", "-d", strconv.Itoa(id), "--showuse")
		utilOutput, err := utilCmd.Output()
		if err == nil {
			gpu.Utilization = parseROCmUtilization(string(utilOutput))
		}

		gpus = append(gpus, gpu)
	}

	return gpus
}

// parseROCmMemory parses memory values from rocm-smi output
func parseROCmMemory(output, field string) int64 {
	re := regexp.MustCompile(field + `.*?(\d+)\s*MB`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return 0
	}
	val, _ := strconv.ParseInt(matches[1], 10, 64)
	return val
}

// parseROCmUtilization parses GPU utilization from rocm-smi output
func parseROCmUtilization(output string) float64 {
	re := regexp.MustCompile(`GPU use.*?(\d+)%`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return 0
	}
	val, _ := strconv.ParseFloat(matches[1], 64)
	return val
}

// IsAvailable returns true if GPU monitoring is available
func (g *GPUMonitor) IsAvailable() bool {
	return g.available
}

// GetGPUType returns the detected GPU type
func (g *GPUMonitor) GetGPUType() GPUType {
	return g.gpuType
}

// GetTotalVRAM returns total VRAM across all GPUs in MB
func (g *GPUMonitor) GetTotalVRAM() int64 {
	gpus := g.GetGPUInfo()
	var total int64
	for _, gpu := range gpus {
		total += gpu.MemoryTotal
	}
	return total
}

// GetFreeVRAM returns free VRAM across all GPUs in MB
func (g *GPUMonitor) GetFreeVRAM() int64 {
	gpus := g.GetGPUInfo()
	var free int64
	for _, gpu := range gpus {
		free += gpu.MemoryFree
	}
	return free
}

// HasEnoughVRAM checks if there's enough VRAM available for a model
func (g *GPUMonitor) HasEnoughVRAM(requiredMB int64) (bool, error) {
	if !g.available {
		return false, fmt.Errorf("no GPU available")
	}

	freeVRAM := g.GetFreeVRAM()
	if freeVRAM < requiredMB {
		return false, fmt.Errorf("insufficient VRAM: need %d MB, have %d MB free", requiredMB, freeVRAM)
	}

	return true, nil
}

// GetBestGPU returns the GPU with most free memory
func (g *GPUMonitor) GetBestGPU() (GPUInfo, error) {
	gpus := g.GetGPUInfo()
	if len(gpus) == 0 {
		return GPUInfo{}, fmt.Errorf("no GPUs available")
	}

	bestGPU := gpus[0]
	for _, gpu := range gpus[1:] {
		if gpu.MemoryFree > bestGPU.MemoryFree {
			bestGPU = gpu
		}
	}

	return bestGPU, nil
}
