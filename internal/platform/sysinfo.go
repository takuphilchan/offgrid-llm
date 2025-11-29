package platform

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// SystemInfo holds detected system information
type SystemInfo struct {
	CPU          string `json:"cpu"`
	CPUCores     int    `json:"cpu_cores"`
	TotalMemory  uint64 `json:"total_memory"` // in bytes
	FreeMemory   uint64 `json:"free_memory"`  // in bytes
	GPU          string `json:"gpu"`
	GPUMemory    uint64 `json:"gpu_memory"` // in bytes (if detectable)
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Backend      string `json:"backend"`
}

// GetSystemInfo detects and returns system information
func GetSystemInfo() SystemInfo {
	info := SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		CPUCores:     runtime.NumCPU(),
		Backend:      "llama.cpp",
	}

	// Detect CPU
	info.CPU = detectCPU()

	// Detect Memory
	info.TotalMemory, info.FreeMemory = detectMemory()

	// Detect GPU
	info.GPU, info.GPUMemory = detectGPU()

	return info
}

func detectCPU() string {
	switch runtime.GOOS {
	case "linux":
		return detectCPULinux()
	case "darwin":
		return detectCPUMac()
	case "windows":
		return detectCPUWindows()
	}
	return "Unknown"
}

func detectCPULinux() string {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "Unknown"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return "Unknown"
}

func detectCPUMac() string {
	out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
	if err != nil {
		// Try alternative for Apple Silicon
		out, err = exec.Command("sysctl", "-n", "hw.model").Output()
		if err != nil {
			return "Apple Silicon"
		}
	}
	return strings.TrimSpace(string(out))
}

func detectCPUWindows() string {
	out, err := exec.Command("wmic", "cpu", "get", "name").Output()
	if err != nil {
		return "Unknown"
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != "Name" {
			return line
		}
	}
	return "Unknown"
}

func detectMemory() (total, free uint64) {
	switch runtime.GOOS {
	case "linux":
		return detectMemoryLinux()
	case "darwin":
		return detectMemoryMac()
	case "windows":
		return detectMemoryWindows()
	}
	return 0, 0
}

func detectMemoryLinux() (total, free uint64) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		val, _ := strconv.ParseUint(fields[1], 10, 64)
		val *= 1024 // Convert KB to bytes

		switch fields[0] {
		case "MemTotal:":
			total = val
		case "MemAvailable:":
			free = val
		}
	}
	return total, free
}

func detectMemoryMac() (total, free uint64) {
	// Get total memory
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err == nil {
		total, _ = strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	}

	// Get free memory (approximation via vm_stat)
	out, err = exec.Command("vm_stat").Output()
	if err == nil {
		var pagesFree, pagesInactive uint64
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Pages free") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					pagesFree, _ = strconv.ParseUint(strings.TrimSuffix(parts[2], "."), 10, 64)
				}
			}
			if strings.Contains(line, "Pages inactive") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					pagesInactive, _ = strconv.ParseUint(strings.TrimSuffix(parts[2], "."), 10, 64)
				}
			}
		}
		// Page size is typically 4096 bytes on macOS
		free = (pagesFree + pagesInactive) * 4096
	}

	return total, free
}

func detectMemoryWindows() (total, free uint64) {
	out, err := exec.Command("wmic", "OS", "get", "TotalVisibleMemorySize,FreePhysicalMemory", "/format:list").Output()
	if err != nil {
		return 0, 0
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TotalVisibleMemorySize=") {
			val, _ := strconv.ParseUint(strings.TrimPrefix(line, "TotalVisibleMemorySize="), 10, 64)
			total = val * 1024 // KB to bytes
		}
		if strings.HasPrefix(line, "FreePhysicalMemory=") {
			val, _ := strconv.ParseUint(strings.TrimPrefix(line, "FreePhysicalMemory="), 10, 64)
			free = val * 1024 // KB to bytes
		}
	}
	return total, free
}

func detectGPU() (name string, memory uint64) {
	// Try NVIDIA first
	if gpu, mem := detectNvidiaGPU(); gpu != "" {
		return gpu, mem
	}

	// Try AMD ROCm
	if gpu, mem := detectAMDGPU(); gpu != "" {
		return gpu, mem
	}

	// Check for Apple Silicon GPU
	if runtime.GOOS == "darwin" {
		if gpu := detectAppleGPU(); gpu != "" {
			return gpu, 0 // Apple unified memory, GPU memory = system memory
		}
	}

	return "CPU only", 0
}

func detectNvidiaGPU() (name string, memory uint64) {
	out, err := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return "", 0
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return "", 0
	}

	// Take first GPU
	parts := strings.Split(lines[0], ", ")
	if len(parts) >= 1 {
		name = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		mem, _ := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
		memory = mem * 1024 * 1024 // MB to bytes
	}

	// If multiple GPUs, append count
	if len(lines) > 1 {
		name = name + " (+" + strconv.Itoa(len(lines)-1) + " more)"
	}

	return name, memory
}

func detectAMDGPU() (name string, memory uint64) {
	// Check for ROCm
	out, err := exec.Command("rocm-smi", "--showproductname").Output()
	if err != nil {
		return "", 0
	}

	if bytes.Contains(out, []byte("GPU")) {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "GPU") && strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return "AMD " + strings.TrimSpace(parts[1]), 0
				}
			}
		}
		return "AMD GPU (ROCm)", 0
	}

	return "", 0
}

func detectAppleGPU() string {
	out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output()
	if err != nil {
		return ""
	}

	if bytes.Contains(out, []byte("Apple M")) {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Chipset Model") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1]) + " GPU"
				}
			}
		}
		return "Apple Silicon GPU"
	}

	return ""
}
