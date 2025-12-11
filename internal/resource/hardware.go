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
	// Only recommend models that exist in the catalog with verified HuggingFace sources

	if availMemory >= 48000 { // 48+ GB
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "llama-3.3-70b-instruct",
			Quantization: "Q4_K_M",
			Reason:       "Meta's flagship model - exceptional quality",
			SizeGB:       39.6,
			Priority:     1,
			UseGPU:       useGPU,
		})
	}

	if availMemory >= 16000 { // 16+ GB
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "phi-4",
			Quantization: "Q4_K_M",
			Reason:       "Microsoft's latest reasoning model",
			SizeGB:       8.4,
			Priority:     1,
			UseGPU:       useGPU,
		})
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "mistral-nemo-instruct-2407",
			Quantization: "Q4_K_M",
			Reason:       "12B model with excellent instruction following",
			SizeGB:       7.0,
			Priority:     2,
			UseGPU:       useGPU,
		})
	}

	if availMemory >= 8000 { // 8+ GB
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "llama-3.1-8b-instruct",
			Quantization: "Q4_K_M",
			Reason:       "Latest Llama with 128K context",
			SizeGB:       4.6,
			Priority:     1,
			UseGPU:       useGPU,
		})
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "mistral-7b-instruct-v0.3",
			Quantization: "Q4_K_M",
			Reason:       "Excellent for code and reasoning",
			SizeGB:       4.1,
			Priority:     1,
			UseGPU:       useGPU,
		})
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "hermes-3-llama-3.1-8b",
			Quantization: "Q4_K_M",
			Reason:       "Strong instruction following",
			SizeGB:       4.6,
			Priority:     2,
			UseGPU:       useGPU,
		})
	}

	if availMemory >= 4000 { // 4+ GB
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "llama-3.2-3b-instruct",
			Quantization: "Q4_K_M",
			Reason:       "Latest Llama - great for mobile/edge",
			SizeGB:       1.9,
			Priority:     1,
			UseGPU:       useGPU,
		})
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "phi-3.5-mini-instruct",
			Quantization: "Q4_K_M",
			Reason:       "Microsoft's efficient model with strong reasoning",
			SizeGB:       2.2,
			Priority:     1,
			UseGPU:       useGPU,
		})
	}

	if availMemory >= 2000 { // 2+ GB (minimum)
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "llama-3.2-1b-instruct",
			Quantization: "Q4_K_M",
			Reason:       fmt.Sprintf("Fits in available %s - latest Llama", formatMemory(availMemory)),
			SizeGB:       0.8,
			Priority:     1,
			UseGPU:       useGPU,
		})
		recommendations = append(recommendations, ModelRecommendation{
			ModelID:      "tinyllama-1.1b-chat",
			Quantization: "Q4_K_M",
			Reason:       "Compact and fast for resource-constrained environments",
			SizeGB:       0.6,
			Priority:     2,
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
