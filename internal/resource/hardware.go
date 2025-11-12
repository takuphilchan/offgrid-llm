package resource

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// SystemResources represents available hardware resources
type SystemResources struct {
	TotalRAM     int64  // Total RAM in MB
	AvailableRAM int64  // Available RAM in MB
	GPUAvailable bool   // Whether GPU is available
	GPUMemory    int64  // GPU VRAM in MB
	GPUName      string // GPU model name
	CPUCores     int    // Number of CPU cores
	OS           string // Operating system
	Arch         string // Architecture
}

// DetectResources detects available system resources
func DetectResources() (*SystemResources, error) {
	res := &SystemResources{
		CPUCores: runtime.NumCPU(),
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
	}

	// Detect RAM
	if err := detectRAM(res); err != nil {
		return nil, fmt.Errorf("failed to detect RAM: %w", err)
	}

	// Detect GPU
	detectGPU(res)

	return res, nil
}

// detectRAM detects total and available RAM
func detectRAM(res *SystemResources) error {
	if runtime.GOOS == "linux" {
		// Read /proc/meminfo
		data, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return err
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}

			if strings.HasPrefix(line, "MemTotal:") {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				res.TotalRAM = kb / 1024 // Convert KB to MB
			} else if strings.HasPrefix(line, "MemAvailable:") {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				res.AvailableRAM = kb / 1024 // Convert KB to MB
			}
		}
		return nil
	}

	// For macOS, use sysctl
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("sysctl", "-n", "hw.memsize")
		output, err := cmd.Output()
		if err != nil {
			return err
		}
		bytes, _ := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
		res.TotalRAM = bytes / (1024 * 1024)       // Convert bytes to MB
		res.AvailableRAM = res.TotalRAM * 80 / 100 // Estimate 80% available
		return nil
	}

	return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
}

// detectGPU detects NVIDIA GPU using nvidia-smi
func detectGPU(res *SystemResources) {
	// Try nvidia-smi
	cmd := exec.Command("nvidia-smi", "--query-gpu=memory.total,name", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		// No NVIDIA GPU or nvidia-smi not installed
		res.GPUAvailable = false
		return
	}

	// Parse output: "8192, NVIDIA GeForce RTX 3070"
	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) >= 2 {
		vram, _ := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		res.GPUMemory = vram
		res.GPUName = strings.TrimSpace(parts[1])
		res.GPUAvailable = true
	}
}

// RecommendedModels returns model recommendations based on resources
func (res *SystemResources) RecommendedModels() []ModelRecommendation {
	var recommendations []ModelRecommendation

	// Determine available memory
	// Use RAM for CPU inference, GPU VRAM if sufficient
	availMemory := res.AvailableRAM
	useGPU := false

	// If GPU has sufficient VRAM (>= 4GB), prefer it
	if res.GPUAvailable && res.GPUMemory >= 4000 {
		availMemory = res.GPUMemory
		useGPU = true
	}

	// Model recommendations based on memory
	// These are approximate sizes for Q4_K_M quantization
	if availMemory >= 40000 { // 40+ GB
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "llama-3-70b-instruct",
			Quantization: "Q4_K_M",
			Reason:       "Large model for best quality",
			SizeGB:       38,
			Priority:     1,
			UseGPU:       useGPU,
		})
	}

	if availMemory >= 16000 { // 16+ GB
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "mistral-7b-instruct",
			Quantization: "Q5_K_M",
			Reason:       "High quality, excellent for code and reasoning",
			SizeGB:       4.8,
			Priority:     1,
			UseGPU:       useGPU,
		})
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "llama-2-13b-chat",
			Quantization: "Q4_K_M",
			Reason:       "Good balance of quality and performance",
			SizeGB:       7.3,
			Priority:     2,
			UseGPU:       useGPU,
		})
	}

	if availMemory >= 8000 { // 8+ GB
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "llama-2-7b-chat",
			Quantization: "Q4_K_M",
			Reason:       "Recommended for most users - best balance",
			SizeGB:       3.8,
			Priority:     1,
			UseGPU:       useGPU,
		})
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "mistral-7b-instruct",
			Quantization: "Q4_K_M",
			Reason:       "Excellent quality, great for general use",
			SizeGB:       4.1,
			Priority:     1,
			UseGPU:       useGPU,
		})
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "codellama-7b-instruct",
			Quantization: "Q4_K_M",
			Reason:       "Specialized for code generation",
			SizeGB:       3.8,
			Priority:     2,
			UseGPU:       useGPU,
		})
	}

	if availMemory >= 4000 { // 4+ GB
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "phi-2",
			Quantization: "Q4_K_M",
			Reason:       "Efficient 2.7B model, great quality for size",
			SizeGB:       1.7,
			Priority:     1,
			UseGPU:       useGPU,
		})
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "tinyllama-1.1b-chat",
			Quantization: "Q4_K_M",
			Reason:       "Compact model for resource-constrained environments",
			SizeGB:       0.6,
			Priority:     2,
			UseGPU:       useGPU,
		})
	}

	if availMemory >= 2000 { // 2+ GB (minimum)
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "tinyllama-1.1b-chat",
			Quantization: "Q4_K_M",
			Reason:       fmt.Sprintf("Fits in available %s", formatMemory(availMemory)),
			SizeGB:       0.6,
			Priority:     1,
			UseGPU:       useGPU,
		})
	}

	// Add embedding models (small, always recommended)
	if availMemory >= 1000 {
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "all-minilm-l6-v2",
			Quantization: "F16",
			Reason:       "Lightweight embeddings for semantic search",
			SizeGB:       0.04,
			Priority:     3,
			UseGPU:       false, // Embeddings typically CPU-only
		})
	}

	return recommendations
}

// formatMemory formats memory size
func formatMemory(mb int64) string {
	if mb >= 1024 {
		return fmt.Sprintf("%.1f GB", float64(mb)/1024)
	}
	return fmt.Sprintf("%d MB", mb)
}

// ModelRecommendation represents a recommended model
type ModelRecommendation struct {
	ModelID      string
	Quantization string
	Reason       string
	SizeGB       float64
	Priority     int  // 1=highest, 2=alternative, 3=supplementary
	UseGPU       bool // Whether this model should use GPU acceleration
}

// String returns a formatted string representation
func (res *SystemResources) String() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "System Resources:\n")
	fmt.Fprintf(&sb, "  OS: %s/%s\n", res.OS, res.Arch)
	fmt.Fprintf(&sb, "  CPU Cores: %d\n", res.CPUCores)
	fmt.Fprintf(&sb, "  RAM: %d MB total, %d MB available\n", res.TotalRAM, res.AvailableRAM)

	if res.GPUAvailable {
		fmt.Fprintf(&sb, "  GPU: %s (%d MB VRAM)\n", res.GPUName, res.GPUMemory)
	} else {
		fmt.Fprintf(&sb, "  GPU: Not detected\n")
	}

	return sb.String()
}
